"""
模型版本管理和训练调度
"""

import os
import json
import time
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

    def __init__(self, model_dir="/tmp/ml-models", tracking_uri=None):
        self.model_dir = model_dir
        os.makedirs(model_dir, exist_ok=True)

        # MLflow 设置（使用 SQLite 后端，避免 FileStore 维护模式问题）
        mlflow.set_tracking_uri(tracking_uri or f"sqlite:///{model_dir}/mlflow.db")
        mlflow.set_experiment("quant_trading")

        # 模型注册表
        self.models: Dict[str, Any] = {
            "short_term": ShortTermPredictor(),
            "medium_term": EnsemblePredictor(),
            "long_term": LongTermPredictor(),
        }

        # 准确率历史（用于自动重训练判断）
        self.accuracy_history: Dict[str, List[float]] = defaultdict(list)

        # 模型版本信息
        self.versions: Dict[str, Dict] = {
            "short_term": {"version": "v0.0.0", "path": "", "accuracy": 0.0},
            "medium_term": {"version": "v0.0.0", "path": "", "accuracy": 0.0},
            "long_term": {"version": "v0.0.0", "path": "", "accuracy": 0.0},
        }

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

    def _save_model(self, period: str):
        """保存模型到文件"""
        path = os.path.join(self.model_dir, f"{period}_model")
        if period == "short_term":
            path += ".pt"
        elif period == "medium_term":
            path += ".pkl"
        elif period == "long_term":
            path += ".pt"

        self.models[period].save(path)

        # 更新版本
        version_parts = self.versions[period]["version"].lstrip("v").split(".")
        version_parts[-1] = str(int(version_parts[-1]) + 1)
        new_version = "v" + ".".join(version_parts)
        self.versions[period]["version"] = new_version
        self.versions[period]["path"] = path

        # 保存版本元数据
        self._save_versions()

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
            status[period] = {
                "version": self.versions[period]["version"],
                "accuracy": self.versions[period]["accuracy"],
                "recent_accuracy": history[-5:] if len(history) >= 5 else history,
                "trained": self.versions[period]["path"] != "",
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