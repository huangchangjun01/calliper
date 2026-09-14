"""
预测任务执行
"""

import json
import os
import pickle
import threading
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime
from typing import Dict, List, Optional, Any
from zoneinfo import ZoneInfo

import numpy as np

from ..models.model_manager import ModelManager
from ..utils.data_loader import DataLoader

# 统一交易日时区
SH_TZ = ZoneInfo("Asia/Shanghai")

# 模型权重/缩放器目录（ml-service/.ml-models）
MODEL_DIR = os.path.normpath(
    os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", ".ml-models")
)

# 与训练脚本保持一致的推断滑窗（short=30, long=52；medium 为每日单样本无需滑窗）
_INFERENCE_WINDOW = {"short_term": 30, "long_term": 52}


class PredictionTask:
    """预测任务执行器：每日预测、单股票预测、并发预测"""

    def __init__(self, model_manager: Optional[ModelManager] = None, max_workers: int = 8):
        self.model_manager = model_manager or ModelManager()
        self.max_workers = max_workers
        self.prediction_history: List[Dict] = []
        self.data_loader = DataLoader()
        # 互斥锁：防止定时任务与 POST /run 并发触发
        self._prediction_lock = threading.Lock()

    def _get_active_stocks(self) -> List[str]:
        """获取活跃股票列表，从数据库读取"""
        try:
            # Try to get stocks from database
            if self.data_loader.engine is not None:
                import pandas as pd
                query = "SELECT DISTINCT symbol FROM stocks WHERE is_active = true LIMIT 100"
                df = pd.read_sql_query(query, self.data_loader.engine)
                if not df.empty:
                    return df["symbol"].tolist()
        except Exception as e:
            print(f"[PredictionTask] Failed to get active stocks from DB: {e}")

        # Fallback: use well-known stock codes
        return [
            "000001", "000002", "000858", "002415", "300750",
            "600519", "600036", "601318", "601398", "603259",
        ]

    def _load_with_timeout(self, symbol: str, start: str, end: str, interval: str) -> Any:
        """带超时地加载特征数据，避免网络不可用时长时间阻塞。"""
        from concurrent.futures import TimeoutError as FutTimeout

        attempts = 2
        for attempt in range(attempts):
            try:
                with ThreadPoolExecutor(max_workers=1) as ex:
                    fut = ex.submit(
                        self.data_loader.load_stock_data, symbol, start, end, interval
                    )
                    return fut.result(timeout=8)
            except FutTimeout:
                print(f"[PredictionTask] Data load timeout for {symbol} (attempt {attempt + 1}/{attempts})")
            except Exception as e:
                print(f"[PredictionTask] Data load error for {symbol} (attempt {attempt + 1}/{attempts}): {e}")

        print(f"[PredictionTask] Data load failed for {symbol}, no data available")
        return None

    def _build_features(self, symbol: str, period: str) -> Any:
        """构建特征数据。仅使用真实数据源（带超时）；无真实数据时返回 None，绝不合成。"""
        try:
            from datetime import timedelta
            end = datetime.now(SH_TZ)
            start = end - timedelta(days=60)

            if period == "short_term":
                start = end - timedelta(days=60)
                df = self._load_with_timeout(
                    symbol, start.strftime("%Y-%m-%d"), end.strftime("%Y-%m-%d"), interval="1d"
                )
            elif period == "medium_term":
                df = self._load_with_timeout(
                    symbol, start.strftime("%Y-%m-%d"), end.strftime("%Y-%m-%d"), interval="1d"
                )
            elif period == "long_term":
                start = end - timedelta(days=365)
                df = self._load_with_timeout(
                    symbol, start.strftime("%Y-%m-%d"), end.strftime("%Y-%m-%d"), interval="1d"
                )
            else:
                return None

            # 真实数据缺失或拉取失败：直接跳过，不做任何合成兜底
            if df is None or (hasattr(df, "empty") and df.empty):
                print(f"[PredictionTask] No real data for {symbol}/{period}, skipping (no synthetic fallback)")
                return None

            # Convert to numpy array for model input
            feature_cols = ["open", "high", "low", "close", "volume"]
            missing_cols = [c for c in feature_cols if c not in df.columns]
            if missing_cols:
                print(f"[PredictionTask] No real data for {symbol}/{period}, skipping (no synthetic fallback)")
                print(f"[PredictionTask]   missing OHLCV columns: {missing_cols}")
                return None

            features = df[feature_cols].values.astype(np.float32)

            # 推断滑窗与训练保持一致：short 取最后 30 根、long 取最后 52 根；
            # medium 按每日单样本口径保留全部行（结果层取最新样本）。
            seq_len = _INFERENCE_WINDOW.get(period)
            if seq_len is not None:
                if len(features) < seq_len:
                    print(f"[PredictionTask] Not enough real rows for {symbol}/{period}, skipping")
                    return None
                features = features[-seq_len:]

            return features
        except Exception as e:
            print(f"[PredictionTask] Error building features for {symbol}/{period}: {e}")
            return None

    def _apply_scaler(self, features: Any, period: str) -> Any:
        """加载训练时持久化的 StandardScaler 并标准化特征；scaler 缺失或失败时原样返回。"""
        try:
            path = os.path.join(MODEL_DIR, f"scaler_{period}.pkl")
            if not os.path.exists(path):
                return features
            with open(path, "rb") as f:
                scaler = pickle.load(f)
        except Exception as e:
            print(f"[PredictionTask] scaler load failed for {period}: {e}")
            return features

        try:
            arr = np.asarray(features, dtype=np.float32)
            orig_shape = arr.shape
            if arr.ndim == 3:
                flat = arr.reshape(-1, arr.shape[-1])
            else:
                flat = arr
            scaled = scaler.transform(flat).astype(np.float32)
            if arr.ndim == 3:
                scaled = scaled.reshape(orig_shape)
            return scaled
        except Exception as e:
            print(f"[PredictionTask] scaler transform failed for {period}: {e}")
            return features

    def predict_single_stock(self, symbol: str, period: Optional[str] = None) -> Dict[str, Any]:
        """
        对单只股票进行预测。
        :param symbol: 股票代码
        :param period: 指定预测周期（short_term/medium_term/long_term）；
            默认 None 表示三个周期全部预测。
        :return: 预测结果字典
        """
        if period is not None and period not in ("short_term", "medium_term", "long_term"):
            result = {
                "symbol": symbol,
                "timestamp": datetime.now(SH_TZ).isoformat(),
                "error": f"invalid period: {period}",
            }
            return result

        targets = [period] if period else ["short_term", "medium_term", "long_term"]

        result = {
            "symbol": symbol,
            "timestamp": datetime.now(SH_TZ).isoformat(),
            "predictions": {},
            "no_data_periods": [],
            "model_version": self.model_manager.versions.get("short_term", {}).get("version", "unknown"),
        }

        try:
            for p in targets:
                features = self._build_features(symbol, p)
                if features is not None:
                    features = self._apply_scaler(features, p)
                    preds = self.model_manager.predict_single(p, features)
                    result["predictions"][p] = preds
                else:
                    result["no_data_periods"].append(p)

        except Exception as e:
            result["error"] = str(e)

        return result

    def run_daily_prediction(self, symbols: Optional[List[str]] = None, period: Optional[str] = None) -> List[Dict]:
        """
        执行每日预测：遍历所有活跃股票，并发预测
        :param symbols: 股票列表，None 则使用活跃股票
        :param period: 指定预测周期；None 表示三周期全部预测
        :return: 所有预测结果列表
        """
        # 互斥锁：防止定时任务与 POST /run 并发触发
        with self._prediction_lock:
            symbols = symbols or self._get_active_stocks()
            print(f"[PredictionTask] Starting daily prediction for {len(symbols)} stocks at {datetime.now(SH_TZ)} (period={period or 'all'})")

            results = []
            with ThreadPoolExecutor(max_workers=self.max_workers) as executor:
                future_to_symbol = {
                    executor.submit(self.predict_single_stock, symbol, period): symbol
                    for symbol in symbols
                }

                for future in as_completed(future_to_symbol):
                    symbol = future_to_symbol[future]
                    try:
                        result = future.result()
                        results.append(result)
                    except Exception as e:
                        print(f"[PredictionTask] Error predicting {symbol}: {e}")
                        results.append({
                            "symbol": symbol,
                            "timestamp": datetime.now(SH_TZ).isoformat(),
                            "error": str(e),
                        })

            # 保存预测历史
            self.prediction_history.append({
                "timestamp": datetime.now(SH_TZ).isoformat(),
                "stock_count": len(symbols),
                "results": results,
            })

            print(f"[PredictionTask] Completed predictions for {len(results)} stocks")
            return results

    def run_weekly_training(self) -> List[str]:
        """
        每周重训练：加载真实数据并按真实管线训练全部模型（周六全量兜底）。
        返回训练成功的 period 列表；无足够数据时抛出异常并打印。
        """
        return self._run_training()

    def run_daily_light_training(self) -> List[str]:
        """
        每日轻量重训练：只重训 short_term 与 medium_term（跳过最耗时的 long_term），
        在收盘后快速跟上最新行情；long_term 由周六全量训练兜底。
        返回训练成功的 period 列表；无足够数据时抛出异常并打印。
        """
        return self._run_training(periods=["short_term", "medium_term"])

    def _run_training(self, periods: Optional[List[str]] = None) -> List[str]:
        """
        训练公共入口：加载真实数据并按真实管线训练指定周期模型。
        :param periods: 训练周期列表；None 表示全量（short/medium/long）。
        """
        import sys
        import train_models as tm

        ml_root = os.path.normpath(
            os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")
        )
        if ml_root not in sys.path:
            sys.path.insert(0, ml_root)

        # 复用互斥锁，避免训练与预测并发
        with self._prediction_lock:
            try:
                trained = tm.train_all_models(periods=periods)
                print(f"[PredictionTask] Training done, trained={trained}", flush=True)
                return trained
            except Exception as e:
                print(f"[PredictionTask] Training failed: {e}", flush=True)
                raise

    def run_model_evaluation(self, periods: Optional[List[str]] = None) -> Dict[str, float]:
        """
        每日评估：基于真实数据计算指定模型准确率估计。
        :param periods: 要评估的周期列表，默认 None 表示全部三个周期。
        返回 {period: accuracy}；无真实数据时返回空 dict。
        """
        import sys
        import train_models as tm

        ml_root = os.path.normpath(
            os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")
        )
        if ml_root not in sys.path:
            sys.path.insert(0, ml_root)

        if periods is None:
            periods = ["short_term", "medium_term", "long_term"]

        results: Dict[str, float] = {}
        try:
            frames = tm.load_real_frames(tm.get_symbols())
        except Exception as e:
            print(f"[PredictionTask] evaluation data load failed: {e}", flush=True)
            return results
        if not frames:
            print("[PredictionTask] no real data for evaluation", flush=True)
            return results

        for period in periods:
            try:
                data = tm.prepare_period_data(period, frames)
                if data is None:
                    continue
                X, y = data
                split = max(len(y) // 5, 1)
                Xv, yl = X[-split:], y[-split:]
                model = self.model_manager.models[period]
                labels = model.LABELS
                yv_str = [
                    labels[int(l)] if 0 <= int(l) < len(labels) else labels[0]
                    for l in yl
                ]
                acc = self.model_manager.evaluate(period, Xv, yv_str)
                results[period] = acc
                print(f"[PredictionTask] {period} eval accuracy: {acc:.4f}", flush=True)
            except Exception as e:
                print(f"[PredictionTask] evaluation {period} failed: {e}", flush=True)
        return results

    def save_predictions(self, results: List[Dict], output_path: str):
        """保存预测结果到文件"""
        import os
        os.makedirs(os.path.dirname(output_path), exist_ok=True)
        with open(output_path, "w") as f:
            json.dump(results, f, indent=2, ensure_ascii=False, default=str)
        print(f"[PredictionTask] Saved predictions to {output_path}")

    def get_summary(self, results: Optional[List[Dict]] = None) -> Dict[str, Any]:
        """生成预测摘要统计"""
        results = results or (self.prediction_history[-1]["results"] if self.prediction_history else [])

        summary = {
            "total_stocks": len(results),
            "timestamp": datetime.now(SH_TZ).isoformat(),
            "direction_distribution": {
                "short_term": {},
                "medium_term": {},
                "long_term": {},
            },
            "avg_confidence": {
                "short_term": 0.0,
                "medium_term": 0.0,
                "long_term": 0.0,
            },
            "errors": 0,
        }

        for r in results:
            if "error" in r:
                summary["errors"] += 1
                continue

            for period in ["short_term", "medium_term", "long_term"]:
                preds = r.get("predictions", {}).get(period, [])
                if preds:
                    direction = preds[0].get("direction", "unknown")
                    conf = preds[0].get("confidence", 0)
                    dist = summary["direction_distribution"][period]
                    dist[direction] = dist.get(direction, 0) + 1
                    summary["avg_confidence"][period] += conf

        valid = max(len(results) - summary["errors"], 1)
        for period in ["short_term", "medium_term", "long_term"]:
            summary["avg_confidence"][period] = round(
                summary["avg_confidence"][period] / valid, 4
            )

        return summary

    def get_historical_predictions(self, limit: int = 10) -> List[Dict]:
        """获取历史预测记录"""
        return self.prediction_history[-limit:]


# ──────────────────────────────────────────────
# 自测入口
# ──────────────────────────────────────────────

if __name__ == "__main__":
    print("=== Initializing PredictionTask ===")
    task = PredictionTask()

    print("\n=== Train models first ===")
    task.model_manager.train_all()

    print("\n=== Predict single stock ===")
    result = task.predict_single_stock("000001")
    print(json.dumps(result, indent=2, ensure_ascii=False, default=str))

    print("\n=== Run daily prediction ===")
    all_results = task.run_daily_prediction(symbols=["000001", "000002", "600519"])

    print("\n=== Summary ===")
    summary = task.get_summary(all_results)
    print(json.dumps(summary, indent=2, ensure_ascii=False))

    print("\n=== Save predictions ===")
    task.save_predictions(all_results, "/tmp/predictions.json")