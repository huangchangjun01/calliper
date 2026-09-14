"""
训练脚本：仅使用真实历史行情训练 short/medium/long 三个模型。

数据来源：DataLoader（TSDB calliper_tsdb 优先，真实 API yfinance/sina 兜底）。
绝不生成任何合成数据：每只股票要求 >= 120 根真实日线，不足则跳过；
若所有股票都被跳过，报错退出且不写入任何权重。
输出：ml-service/.ml-models/ 下的 *_model 权重文件 + versions.json，供 ModelManager.load_all 加载。
另外将 StandardScaler 持久化为 .ml-models/scaler_{period}.pkl，供预测侧复用。

用法：
    TRAIN_SYMBOLS=000001,600519 python train_models.py
"""

import os
import sys
import json
import pickle
from datetime import date, datetime, timedelta, timezone
from zoneinfo import ZoneInfo

import numpy as np
import pandas as pd

from dotenv import load_dotenv

# 加载 ml-service/.env（DATABASE_URL / TSDB_URL），必须在导入读取环境变量的模块之前
load_dotenv()

from app.utils.data_loader import DataLoader

MODEL_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), ".ml-models")
os.makedirs(MODEL_DIR, exist_ok=True)

DEFAULT_SYMBOLS = "000001,000002,600519,600036,000858"
MIN_ROWS = 120          # 每只股票最少真实日线数量
HISTORY_DAYS = 1825     # 拉取近 5 年日线

# 统一交易日时区
SH_TZ = ZoneInfo("Asia/Shanghai")


def get_symbols():
    """
    读取训练股票列表：优先 TRAIN_SYMBOLS 环境变量（逗号分隔）；
    未设置时从主库 stocks 表拉取全部真实股票代码（真实数据，非合成）。
    最后兜底为内置 DEFAULT_SYMBOLS。
    """
    raw = os.getenv("TRAIN_SYMBOLS", "").strip()
    if raw:
        return [s.strip() for s in raw.split(",") if s.strip()]

    try:
        loader = DataLoader()
        if loader.main_engine is not None:
            from sqlalchemy import text
            with loader.main_engine.connect() as conn:
                rows = conn.execute(text("SELECT symbol FROM stocks ORDER BY symbol")).fetchall()
            symbols = [str(r[0]).strip() for r in rows if str(r[0]).strip()]
            if symbols:
                print(f"[train] loaded {len(symbols)} symbols from stocks table", flush=True)
                return symbols
    except Exception as e:
        print(f"[train] failed to load symbols from DB, fallback to default: {e}", flush=True)

    return [s.strip() for s in DEFAULT_SYMBOLS.split(",") if s.strip()]


def load_real_frames(symbols):
    """
    加载真实日线数据。仅接受 >= MIN_ROWS 行的真实数据，不足则跳过该股票。
    返回 [(symbol, df), ...]；全部跳过则返回空列表。
    """
    loader = DataLoader()
    end = datetime.now(SH_TZ).date()
    start = end - timedelta(days=HISTORY_DAYS)

    frames = []
    for symbol in symbols:
        try:
            df = loader.load_stock_data(
                symbol, start.strftime("%Y-%m-%d"), end.strftime("%Y-%m-%d"), interval="1d"
            )
        except Exception as e:
            print(f"[train] load failed for {symbol}: {e}", flush=True)
            df = pd.DataFrame()

        if df is None or df.empty:
            print(f"[train] SKIP {symbol}: no real data (TSDB/API), no synthetic fallback", flush=True)
            continue

        cols = ["open", "high", "low", "close", "volume"]
        missing = [c for c in cols if c not in df.columns]
        if missing:
            print(f"[train] SKIP {symbol}: missing columns {missing}", flush=True)
            continue

        df = df[cols].dropna()
        if len(df) < MIN_ROWS:
            print(
                f"[train] SKIP {symbol}: only {len(df)} real daily rows (< {MIN_ROWS}), no synthetic fallback",
                flush=True,
            )
            continue

        print(f"[train] {symbol}: loaded {len(df)} real daily rows", flush=True)
        frames.append((symbol, df))

    return frames


def label_fwd(close, horizon, up, down):
    """
    按未来 horizon 日涨跌幅生成三分类标签：2=上涨, 1=震荡, 0=下跌。
    末尾无对应未来样本的 horizon 行为 NaN（由调用方剔除，避免误标为下跌）。
    """
    n = len(close)
    labels = np.full(n, np.nan, dtype=np.float64)
    for i in range(n - horizon):
        fwd = (close[i + horizon] - close[i]) / close[i] * 100 if close[i] > 0 else 0.0
        if fwd > up:
            labels[i] = 2
        elif fwd < -down:
            labels[i] = 0
        else:
            labels[i] = 1
    return labels


def sequences(df, seq_len, horizon, up, down):
    """构建 (N, seq_len, 5) 滑动窗口样本与标签。"""
    cols = ["open", "high", "low", "close", "volume"]
    X = df[cols].values.astype(np.float32)
    close = df["close"].values
    y = label_fwd(close, horizon, up, down)
    xs, ys = [], []
    for i in range(len(X) - seq_len - horizon):
        xs.append(X[i:i + seq_len])
        # 窗口结束处标签：i+seq_len-1 < n-horizon，必为有效标签（非 NaN/非末期标 0）
        ys.append(y[i + seq_len - 1])
    if not xs:
        return None
    ys = np.asarray(ys, dtype=np.int64)
    if np.isnan(ys).any():
        ys = ys[~np.isnan(ys)]
        ys = ys.astype(np.int64)
    return np.array(xs), ys


def _persist_scaler(X, period):
    """拟合并持久化 StandardScaler 到 .ml-models/scaler_{period}.pkl，返回标准化后的 X。"""
    from sklearn.preprocessing import StandardScaler

    arr = np.asarray(X, dtype=np.float32)
    orig_shape = arr.shape
    if arr.ndim == 3:
        flat = arr.reshape(-1, arr.shape[-1])
    else:
        flat = arr

    scaler = StandardScaler()
    scaled = scaler.fit_transform(flat).astype(np.float32)
    if arr.ndim == 3:
        scaled = scaled.reshape(orig_shape)

    path = os.path.join(MODEL_DIR, f"scaler_{period}.pkl")
    with open(path, "wb") as f:
        pickle.dump(scaler, f)
    print(f"[train] saved scaler {period} -> {path}", flush=True)
    return scaled


def prepare_period_data(period, frames):
    """
    为指定 period 构建训练数据 (X, y)。帧为 load_real_frames 的返回列表。
    返回 (标准化的 X, y)；无足够真实样本时返回 None。
    """
    import numpy as _np

    if period == "short_term":
        xs, ys = [], []
        for _, df in frames:
            w = sequences(df, 30, 3, 0.5, 0.5)
            if w:
                xs.append(w[0]); ys.append(w[1])
        if not xs:
            return None
        X_short = _np.concatenate(xs)
        y_short = _np.concatenate(ys)
        X_short = _persist_scaler(X_short, period)
        return X_short, y_short

    if period == "medium_term":
        cols = ["open", "high", "low", "close", "volume"]
        horizon = 10
        Xm, ym = [], []
        for _, df in frames:
            X = df[cols].values.astype(_np.float32)
            y = label_fwd(df["close"].values, horizon, 2, 2)
            # 剔除末期无未来样本的行（label_fwd 对尾部 horizon 行标 NaN，不再误标为下跌）
            valid = ~_np.isnan(y)
            Xm.append(X[valid]); ym.append(y[valid].astype(_np.int64))
        if not Xm:
            return None
        X_med = _np.concatenate(Xm)
        y_med = _np.concatenate(ym)
        X_med = _persist_scaler(X_med, period)
        return X_med, y_med

    if period == "long_term":
        xl, yl = [], []
        for _, df in frames:
            w = sequences(df, 52, 30, 5, 5)
            if w:
                xl.append(w[0]); yl.append(w[1])
        if not xl:
            return None
        X_long = _np.concatenate(xl)
        y_long = _np.concatenate(yl)
        X_long = _persist_scaler(X_long, period)
        return X_long, y_long

    return None


def train_all_models(periods=None):
    """
    加载真实数据并训练模型；不写任何合成权重。
    返回训练成功的 period 列表；无足够真实数据时抛 RuntimeError。

    :param periods: 要训练的周期列表，如 ["short_term","medium_term"]；
        默认 None 表示全部三个周期（全量重训）。
    """
    from app.models.short_term_model import ShortTermPredictor
    from app.models.medium_term_model import EnsemblePredictor
    from app.models.long_term_model import LongTermPredictor
    from app.models.model_manager import ModelManager

    # 读取已持久化的用户参数（同一 MODEL_DIR 目录，与 ModelManager 默认目录一致）
    _mgr = ModelManager(model_dir=MODEL_DIR)

    if periods is None:
        periods = ["short_term", "medium_term", "long_term"]

    frames = load_real_frames(get_symbols())
    if not frames:
        raise RuntimeError(
            "no symbol has enough real data (>= %d daily rows); refusing to train on synthetic data" % MIN_ROWS
        )

    trained = []
    for period in periods:
        data = prepare_period_data(period, frames)
        if data is None:
            print(f"ERROR: no real samples for {period}, skipping (no weights written).", flush=True)
            continue
        X, y = data
        if period == "short_term":
            p = _mgr.get_params("short_term")
            sp = ShortTermPredictor(
                input_size=5,
                hidden_size=p["hidden_size"],
                num_layers=p["num_layers"],
                dropout=p["dropout"],
            )
            sp.train(X, y, epochs=30)
            sp.save(os.path.join(MODEL_DIR, "short_term_model.pt"))
        elif period == "medium_term":
            p = _mgr.get_params("medium_term")
            # 默认参数基础上覆写被用户编辑的键，其余（objective/num_class/n_estimators 等）保留默认
            xgb_params = {
                "objective": "multi:softprob",
                "num_class": 3,
                "max_depth": p["xgb_max_depth"],
                "learning_rate": p["xgb_learning_rate"],
                "n_estimators": 200,
                "subsample": 0.8,
                "colsample_bytree": 0.8,
                "random_state": 42,
                "verbosity": 0,
            }
            lgb_params = {
                "objective": "multiclass",
                "num_class": 3,
                "max_depth": 6,
                "learning_rate": 0.05,
                "num_leaves": p["lgb_num_leaves"],
                "n_estimators": 200,
                "subsample": 0.8,
                "colsample_bytree": 0.8,
                "random_state": 42,
                "verbose": -1,
            }
            mp = EnsemblePredictor(xgb_params=xgb_params, lgb_params=lgb_params)
            mp.train(X, y)
            mp.save(os.path.join(MODEL_DIR, "medium_term_model.pkl"))
        elif period == "long_term":
            p = _mgr.get_params("long_term")
            lp = LongTermPredictor(
                input_size=5,
                d_model=p["d_model"],
                nhead=p["nhead"],
                num_layers=p["num_layers"],
            )
            lp.train(X, y, epochs=30)
            lp.save(os.path.join(MODEL_DIR, "long_term_model.pt"))
        trained.append(period)
        print(f"[train] {period}: saved real-model weight", flush=True)

    if not trained:
        raise RuntimeError("no real samples to train any model, no weights written")

    # 生成 versions.json，供 ModelManager.load_all 加载。
    # 注意：只更新本次成功训练的周期，版本在现有值上递增（与 ModelManager._save_model 同规则），
    # 并保留原 accuracy；其它周期的版本/准确率不得被重置（避免 v2.0.0/0.0 覆盖）。
    def _bump_version(ver: str) -> str:
        ver_str = ver.lstrip("v")
        parts = ver_str.split(".")
        tail = parts[-1]
        num = ""
        suffix = ""
        for ch in tail:
            if ch.isdigit():
                num += ch
            else:
                suffix += ch
        if num:
            num = str(int(num) + 1)
        else:
            num = "1"
            suffix = "." + suffix if suffix else ""
        parts[-1] = num + suffix
        return "v" + ".".join(parts)

    trained_at = datetime.now(timezone.utc).isoformat()
    old_versions = dict(getattr(_mgr, "versions", None) or {})
    versions = {}
    for p in ("short_term", "medium_term", "long_term"):
        path = os.path.join(MODEL_DIR, f"{p}_model" + (".pkl" if p == "medium_term" else ".pt")).replace("\\", "/")
        entry = dict(old_versions.get(p, {}) or {})
        if p in trained:
            entry["version"] = _bump_version(entry.get("version", "v1.0.0-real"))
            entry["trained_at"] = trained_at
        entry.setdefault("version", "v1.0.0-real")
        entry.setdefault("accuracy", 0.0)
        entry["path"] = path
        versions[p] = entry
    with open(os.path.join(MODEL_DIR, "versions.json"), "w") as f:
        json.dump(versions, f, indent=2)

    return trained


def main():
    symbols = get_symbols()
    print(f"[train] symbols: {symbols}", flush=True)

    try:
        trained = train_all_models()
        print(f"[train] trained={trained}", flush=True)
    except RuntimeError as e:
        print(f"ERROR: {e}", flush=True)
        sys.exit(1)

    print("ALL_DONE:", MODEL_DIR)


if __name__ == "__main__":
    main()