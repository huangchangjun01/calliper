"""
模型版本管理和训练调度
"""

import os
import json
import time
import shutil
from datetime import datetime, timedelta
from collections import defaultdict
from typing import Dict, Optional, List, Any

import mlflow
import mlflow.pytorch
import mlflow.sklearn

from .short_term_model import ShortTermPredictor
from .medium_term_model import EnsemblePredictor
from .long_term_model import LongTermPredictor


class ModelManager:
    """管理三个模型的版本、训练调度、评估和自动重训练"""

    def __init__(self, model_dir=None, tracking_uri=None):
        # 默认将模型/MLflow 数据放到项目内可写目录（兼容 Linux 与 Windows）
        if not model_dir:
            default_dir = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", ".ml-models")
            model_dir = os.path.normpath(default_dir)
        self.model_dir = model_dir
        os.makedirs(model_dir, exist_ok=True)

        # MLflow 设置（使用 SQLite 后端，避免 FileStore 维护模式问题）
        mlflow.set_tracking_uri(tracking_uri or f"sqlite:///{model_dir}/mlflow.db")
        mlflow.set_experiment("quant_trading")

        # 模型注册表
        # 注：short/long 均使用 input_size=5，与特征管道输出的 OHLCV 5 列特征及
        # 训练脚本保持一致，否则加载权重时会出现 state_dict 维度不匹配。
        self.models: Dict[str, Any] = {
            "short_term": ShortTermPredictor(input_size=5),
            "medium_term": EnsemblePredictor(),
            "long_term": LongTermPredictor(input_size=5),
        }

        # 准确率历史（用于自动重训练判断）
        self.accuracy_history: Dict[str, List[float]] = defaultdict(list)

        # 模型版本信息
        self.versions: Dict[str, Dict] = {
            "short_term": {"version": "v0.0.0", "path": "", "accuracy": 0.0},
            "medium_term": {"version": "v0.0.0", "path": "", "accuracy": 0.0},
            "long_term": {"version": "v0.0.0", "path": "", "accuracy": 0.0},
        }

    # ── 参数读写 ──────────────────────────────

    # 各周期默认参数（必须与对应模型类构造默认值一致）
    PARAM_DEFAULTS: Dict[str, Dict[str, Any]] = {
        "short_term": {"hidden_size": 128, "num_layers": 2, "dropout": 0.3},
        "medium_term": {"xgb_max_depth": 6, "xgb_learning_rate": 0.05, "lgb_num_leaves": 31},
        "long_term": {"d_model": 256, "nhead": 8, "num_layers": 4},
    }

    # 各周期允许的键名及数值范围（语义约束对齐前端 ShortModelParams/MediumModelParams/LongModelParams）
    PARAM_LIMITS: Dict[str, Dict[str, tuple]] = {
        "short_term": {
            "hidden_size": (32, 512),
            "num_layers": (1, 12),
            "dropout": (0, 0.8),
        },
        "medium_term": {
            "xgb_max_depth": (2, 15),
            "xgb_learning_rate": (0.001, 1),
            "lgb_num_leaves": (7, 255),
        },
        "long_term": {
            "d_model": (64, 1024),
            "nhead": (1, 32),
            "num_layers": (1, 12),
        },
    }

    def default_params(self, period: str) -> Dict[str, Any]:
        """返回指定周期的默认参数；period 非法时抛 ValueError。"""
        if period not in self.PARAM_DEFAULTS:
            raise ValueError(
                f"Invalid period: {period}. Must be one of: short_term, medium_term, long_term"
            )
        return dict(self.PARAM_DEFAULTS[period])

    def get_params(self, period: str) -> Dict[str, Any]:
        """读取已持久化的参数；文件不存在时返回默认参数；period 非法抛 ValueError。"""
        if period not in self.PARAM_DEFAULTS:
            raise ValueError(
                f"Invalid period: {period}. Must be one of: short_term, medium_term, long_term"
            )
        path = os.path.join(self.model_dir, f"params_{period}.json")
        if os.path.exists(path):
            try:
                with open(path, "r") as f:
                    saved = json.load(f)
            except (OSError, ValueError):
                saved = {}
            # 缺失键用默认值补齐
            merged = self.default_params(period)
            merged.update(saved or {})
            return merged
        return self.default_params(period)

    def set_params(self, period: str, params: dict) -> Dict[str, Any]:
        """校验并持久化指定周期的参数，返回合并后的完整参数（缺失键用默认值补齐）。"""
        if not isinstance(params, dict):
            raise ValueError("params must be a dict")
        if period not in self.PARAM_DEFAULTS:
            raise ValueError(
                f"Invalid period: {period}. Must be one of: short_term, medium_term, long_term"
            )
        allowed_keys = set(self.PARAM_LIMITS[period])
        unknown = set(params.keys()) - allowed_keys
        if unknown:
            raise ValueError(
                f"Unknown params for {period}: {sorted(unknown)}. Allowed keys: {sorted(allowed_keys)}"
            )
        validated: Dict[str, Any] = {}
        for k, v in params.items():
            if isinstance(v, bool) or not isinstance(v, (int, float)):
                raise ValueError(f"Param '{k}' must be a number, got {type(v).__name__}")
            lo, hi = self.PARAM_LIMITS[period][k]
            if not (lo <= v <= hi):
                raise ValueError(f"Param '{k}' value {v} out of range [{lo}, {hi}]")
            validated[k] = v
        merged = self.default_params(period)
        merged.update(validated)
        path = os.path.join(self.model_dir, f"params_{period}.json")
        with open(path, "w") as f:
            json.dump(merged, f, indent=2)
        return merged

    # ── 训练 ──────────────────────────────────

    def train_all(self, **kwargs):
        """训练全部三个模型"""
        results = {}
        for period, model in self.models.items():
            print(f"\n{'='*50}")
            print(f"Training {period} model...")
            print(f"{'='*50}")
            try:
                with mlflow.start_run(run_name=f"{period}_train_{int(time.time())}"):
                    if period == "short_term":
                        model.train(
                            df=kwargs.get("short_df"),
                            y=kwargs.get("short_y"),
                            epochs=kwargs.get("short_epochs", 50),
                        )
                    elif period == "medium_term":
                        model.train(
                            X=kwargs.get("medium_X"),
                            y=kwargs.get("medium_y"),
                        )
                    elif period == "long_term":
                        model.train(
                            df=kwargs.get("long_df"),
                            y=kwargs.get("long_y"),
                            epochs=kwargs.get("long_epochs", 50),
                        )

                    self._save_model(period)
                    results[period] = "success"
            except Exception as e:
                print(f"[ModelManager] Error training {period}: {e}")
                results[period] = f"failed: {e}"

        return results

    def train_single(self, period: str, **kwargs):
        """训练单个模型"""
        if period not in self.models:
            raise ValueError(f"Unknown period: {period}")

        model = self.models[period]
        # 已保存参数说明：train_single 复用内存中既有模型实例（在 __init__ 时按默认参数构造），
        # 持久化的用户参数由 train_all_models 等路径在构造阶段应用。此处仅打印当前生效参数以便排查。
        try:
            print(f"[ModelManager] train_single {period} active params: {self.get_params(period)}", flush=True)
        except Exception as _e:
            print(f"[ModelManager] train_single {period} failed to read params: {_e}", flush=True)
        with mlflow.start_run(run_name=f"{period}_train_{int(time.time())}"):
            if period == "short_term":
                model.train(
                    df=kwargs.get("short_df"),
                    y=kwargs.get("short_y"),
                    epochs=kwargs.get("short_epochs", 50),
                )
            elif period == "medium_term":
                model.train(X=kwargs.get("medium_X"), y=kwargs.get("medium_y"))
            elif period == "long_term":
                model.train(
                    df=kwargs.get("long_df"),
                    y=kwargs.get("long_y"),
                    epochs=kwargs.get("long_epochs", 50),
                )

            self._save_model(period)

    # ── 预测 ──────────────────────────────────

    def predict_all(self, features: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        """使用全部三个模型进行预测"""
        predictions = {}
        features = features or {}
        for period, model in self.models.items():
            try:
                if period == "short_term":
                    predictions[period] = model.predict(df=features.get("short_df"))
                elif period == "medium_term":
                    predictions[period] = model.predict(X=features.get("medium_X"))
                elif period == "long_term":
                    predictions[period] = model.predict(df=features.get("long_df"))
            except Exception as e:
                predictions[period] = {"error": str(e)}
        return predictions

    def predict_single(self, period: str, features: Optional[Any] = None) -> List[Dict]:
        """单模型预测"""
        if period not in self.models:
            raise ValueError(f"Unknown period: {period}")
        model = self.models[period]
        if period == "short_term":
            return model.predict(df=features)
        elif period == "medium_term":
            return model.predict(X=features)
        elif period == "long_term":
            return model.predict(df=features)

    # ── 评估 ──────────────────────────────────

    def evaluate(self, period: str, X_true, y_true) -> float:
        """评估模型准确率"""
        model = self.models[period]
        if period == "short_term":
            preds = model.predict(df=X_true)
            y_pred = [p["direction"] for p in preds]
        elif period == "medium_term":
            preds = model.predict(X=X_true)
            y_pred = [p["direction"] for p in preds]
        elif period == "long_term":
            preds = model.predict(df=X_true)
            y_pred = [p["direction"] for p in preds]
        else:
            return 0.0

        # 比较标签
        label_map = {v: i for i, v in enumerate(model.LABELS)}
        y_true_idx = [label_map.get(y, -1) for y in y_true]
        y_pred_idx = [label_map.get(y, -1) for y in y_pred]

        correct = sum(1 for t, p in zip(y_true_idx, y_pred_idx) if t == p and t != -1)
        accuracy = correct / len(y_true_idx) if y_true_idx else 0.0

        # 记录准确率历史
        self.accuracy_history[period].append(accuracy)
        self.versions[period]["accuracy"] = accuracy

        # 写入 MLflow
        mlflow.log_metric(f"{period}_accuracy", accuracy)

        return accuracy

    def evaluate_all(self, true_data: Dict[str, Any]) -> Dict[str, float]:
        """评估全部模型。true_data 必须包含每个 period 的 X_true 和 y_true"""
        results = {}
        for period in self.models:
            X_true = true_data.get(f"{period}_X")
            y_true = true_data.get(f"{period}_y")
            if X_true is None or y_true is None:
                print(f"[ModelManager] No evaluation data for {period}, skipping")
                results[period] = 0.0
                continue
            try:
                acc = self.evaluate(period, X_true, y_true)
                results[period] = acc
            except Exception as e:
                print(f"[ModelManager] Evaluate {period} error: {e}")
                results[period] = 0.0
        return results

    # ── 自动重训练 ─────────────────────────────

    def check_retrain(self, threshold: float = 0.60, window: int = 5) -> List[str]:
        """
        检查是否需要重训练。
        规则：连续 window 日准确率低于 threshold 则触发重训练。
        返回需要重训练的模型列表。
        """
        retrain_list = []
        for period in self.models:
            history = self.accuracy_history[period]
            if len(history) >= window:
                recent = history[-window:]
                avg_acc = sum(recent) / len(recent)
                if avg_acc < threshold:
                    print(f"[ModelManager] {period} avg accuracy {avg_acc:.2%} < {threshold:.0%}, triggering retrain")
                    retrain_list.append(period)
        return retrain_list

    def auto_retrain_if_needed(self, threshold: float = 0.60, window: int = 5):
        """自动检测并重训练"""
        retrain_list = self.check_retrain(threshold=threshold, window=window)
        for period in retrain_list:
            print(f"[ModelManager] Auto-retraining {period}...")
            self.train_single(period)
        return retrain_list

    # ── 模型持久化 ─────────────────────────────

    @staticmethod
    def _model_ext(period: str) -> str:
        """根据周期返回权重文件扩展名（short/long=.pt，medium=.pkl）"""
        if period == "medium_term":
            return ".pkl"
        return ".pt"

    def _save_model(self, period: str):
        """保存模型到文件"""
        path = os.path.join(self.model_dir, f"{period}_model")
        ext = self._model_ext(period)
        path += ext

        # 版本快照：写入新权重前，若当前模型文件已存在且有版本信息，将其备份为带版本号的历史快照
        cur_version = self.versions[period].get("version", "")
        if os.path.exists(path) and cur_version:
            snapshot = os.path.join(
                self.model_dir,
                f"{period}_model_v{cur_version.lstrip('v')}{ext}",
            )
            # 防止重复备份同一版本：同文件名快照已存在则跳过
            if not os.path.exists(snapshot):
                try:
                    shutil.copyfile(path, snapshot)
                    print(f"[ModelManager] Backed up {period} v{cur_version.lstrip('v')} -> {os.path.basename(snapshot)}")
                except Exception as e:
                    print(f"[ModelManager] Failed to backup {period} snapshot: {e}")

        self.models[period].save(path)

        # 更新版本：兼容纯数字（v2.0.0）与带后缀（v2.0.0-real）两种格式，
        # 仅对最后一个数字段自增，非数字后缀原样保留。
        ver_str = self.versions[period]["version"].lstrip("v")
        parts = ver_str.split(".")
        tail = parts[-1]
        # 分离末尾数字与后缀（如 "0-real" → num="0", suffix="-real"）
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
        new_version = "v" + ".".join(parts)
        self.versions[period]["version"] = new_version
        self.versions[period]["path"] = path

        # 保存版本元数据
        self._save_versions()

    def rollback(self, period: str, version: str) -> Dict[str, str]:
        """将指定周期模型回滚到某个历史版本快照"""
        if period not in self.models:
            raise ValueError(f"Unknown period: {period}")

        if not version:
            raise ValueError("version is required")

        # 归一化版本号，统一带 v 前缀（与快照命名规则 {period}_model_v{ver}.{ext} 对齐）
        version = "v" + version.lstrip("v")
        ext = self._model_ext(period)

        snapshot = os.path.join(self.model_dir, f"{period}_model_{version}{ext}")
        if not os.path.exists(snapshot):
            raise ValueError(f"Snapshot not found for {period} version {version}: {os.path.basename(snapshot)}")

        current_path = os.path.join(self.model_dir, f"{period}_model{ext}")
        shutil.copyfile(snapshot, current_path)

        # 更新版本元数据，指向当前文件
        self.versions[period]["version"] = version
        self.versions[period]["path"] = current_path
        self._save_versions()

        # 重新加载该模型权重到内存，使其可直接用于预测
        self.models[period].load(current_path)
        print(f"[ModelManager] Rolled back {period} to {version} from {os.path.basename(snapshot)}")

        return {"period": period, "version": version}

    def _save_versions(self):
        """保存版本信息到 JSON"""
        versions_path = os.path.join(self.model_dir, "versions.json")
        with open(versions_path, "w") as f:
            json.dump(self.versions, f, indent=2, default=str)

    def load_all(self):
        """加载所有已保存的模型"""
        versions_path = os.path.join(self.model_dir, "versions.json")
        if os.path.exists(versions_path):
            with open(versions_path, "r") as f:
                self.versions = json.load(f)

        for period in self.models:
            version_info = self.versions.get(period, {})
            path = version_info.get("path", "")
            if path and os.path.exists(path):
                try:
                    self.models[period].load(path)
                    print(f"[ModelManager] Loaded {period} model from {path}")
                    # 记录每个周期的训练时间：优先 versions.json 中的 trained_at，
                    # 否则取权重文件的修改时间（转为 ISO 字符串）
                    trained_at = version_info.get("trained_at")
                    if not trained_at and os.path.exists(path):
                        trained_at = datetime.fromtimestamp(os.path.getmtime(path)).isoformat()
                    if trained_at:
                        self.versions[period]["trained_at"] = trained_at
                except Exception as e:
                    print(f"[ModelManager] Failed to load {period}: {e}")

    def get_best_model(self, period: str):
        """获取最佳模型版本"""
        if period not in self.models:
            raise ValueError(f"Unknown period: {period}")
        return {
            "period": period,
            "version": self.versions[period]["version"],
            "accuracy": self.versions[period]["accuracy"],
            "model": self.models[period],
        }

    def get_status(self) -> Dict[str, Any]:
        """获取所有模型状态"""
        status = {}
        for period in self.models:
            history = self.accuracy_history[period]
            version_info = self.versions[period]
            status[period] = {
                "version": version_info["version"],
                "accuracy": version_info["accuracy"],
                "recent_accuracy": history[-5:] if len(history) >= 5 else history,
                "trained": version_info["path"] != "",
                "last_trained": version_info.get("trained_at", ""),
            }
        return status


# ──────────────────────────────────────────────
# 自测入口
# ──────────────────────────────────────────────

if __name__ == "__main__":
    import numpy as np
    manager = ModelManager()

    # 使用真实格式数据训练
    X_short = np.random.randn(60, 20).astype(np.float32)
    y_short = np.random.randint(0, 3, 60).astype(np.int64)
    X_medium = np.random.randn(500, 30).astype(np.float32)
    y_medium = np.random.randint(0, 3, 500).astype(np.int64)
    X_long = np.random.randn(60, 40).astype(np.float32)
    y_long = np.random.randint(0, 3, 60).astype(np.int64)

    print("=== Training all models ===")
    manager.train_all(
        short_df=X_short, short_y=y_short,
        medium_X=X_medium, medium_y=y_medium,
        long_df=X_long, long_y=y_long,
    )

    print("\n=== Predicting ===")
    preds = manager.predict_all({
        "short_df": X_short[-30:],
        "medium_X": X_medium[:5],
        "long_df": X_long[-52:],
    })
    for period, p in preds.items():
        print(f"  {period}: {p}")

    print("\n=== Model Status ===")
    status = manager.get_status()
    for period, s in status.items():
        print(f"  {period}: {s}")

    print("\n=== Evaluate ===")
    eval_results = manager.evaluate_all({
        "short_term_X": X_short[-30:],
        "short_term_y": ["上涨"] * 10 + ["震荡"] * 10 + ["下跌"] * 10,
        "medium_term_X": X_medium[:10],
        "medium_term_y": ["上涨"] * 5 + ["震荡"] * 5,
        "long_term_X": X_long[-52:],
        "long_term_y": ["上涨趋势"] * 10 + ["震荡趋势"] * 10 + ["下跌趋势"] * 10,
    })
    print(f"  {eval_results}")