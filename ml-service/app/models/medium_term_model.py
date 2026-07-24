"""
中短期预测模型: XGBoost + LightGBM 集成
预测目标: 未来1-4周涨跌方向
输入: 日线技术指标 + 基本面因子 + 资金流向
"""

import os
import pickle

import numpy as np
import pandas as pd

import xgboost as xgb
import lightgbm as lgb
from sklearn.model_selection import train_test_split
from sklearn.metrics import accuracy_score


class EnsemblePredictor:
    """XGBoost + LightGBM 集成预测器"""

    LABELS = ["下跌", "震荡", "上涨"]

    def __init__(self, xgb_params=None, lgb_params=None):
        self.xgb_params = xgb_params or {
            "objective": "multi:softprob",
            "num_class": 3,
            "max_depth": 6,
            "learning_rate": 0.05,
            "n_estimators": 200,
            "subsample": 0.8,
            "colsample_bytree": 0.8,
            "random_state": 42,
            "verbosity": 0,
        }
        self.lgb_params = lgb_params or {
            "objective": "multiclass",
            "num_class": 3,
            "max_depth": 6,
            "learning_rate": 0.05,
            "n_estimators": 200,
            "subsample": 0.8,
            "colsample_bytree": 0.8,
            "random_state": 42,
            "verbose": -1,
        }

        self.xgb_model = None
        self.lgb_model = None
        self.xgb_weight = 0.5
        self.lgb_weight = 0.5
        self.feature_names = []

    def _generate_mock_data(self, num_samples=500, num_features=30):
        """生成 mock 训练数据"""
        X = np.random.randn(num_samples, num_features).astype(np.float32)
        y = np.zeros(num_samples, dtype=np.int64)
        for i in range(num_samples):
            proxy = X[i, 0] * 0.5 + X[i, 1] * 0.3 + X[i, 2] * 0.2
            if proxy > 0.3:
                y[i] = 2
            elif proxy < -0.3:
                y[i] = 0
            else:
                y[i] = 1
        self.feature_names = [f"feature_{j}" for j in range(num_features)]
        return X, y

    def _extract_n_estimators(self, params):
        """提取 n_estimators 参数"""
        return params.pop("n_estimators", 100)

    def train(self, X=None, y=None):
        """训练 XGBoost 和 LightGBM 两个模型。X/y 为 None 时使用 mock 数据"""
        if X is None or y is None:
            X, y = self._generate_mock_data()

        X_train, X_val, y_train, y_val = train_test_split(
            X, y, test_size=0.2, random_state=42
        )

        # ── XGBoost ──
        xgb_params = dict(self.xgb_params)
        n_estimators_xgb = xgb_params.pop("n_estimators", 200)
        self.xgb_model = xgb.XGBClassifier(**xgb_params, n_estimators=n_estimators_xgb)
        self.xgb_model.fit(X_train, y_train)
        xgb_acc = accuracy_score(y_val, self.xgb_model.predict(X_val))
        print(f"[Ensemble] XGBoost val accuracy: {xgb_acc:.4f}")

        # ── LightGBM ──
        lgb_params = dict(self.lgb_params)
        n_estimators_lgb = lgb_params.pop("n_estimators", 200)
        self.lgb_model = lgb.LGBMClassifier(**lgb_params, n_estimators=n_estimators_lgb)
        self.lgb_model.fit(X_train, y_train)
        lgb_acc = accuracy_score(y_val, self.lgb_model.predict(X_val))
        print(f"[Ensemble] LightGBM val accuracy: {lgb_acc:.4f}")

        # ── 加权权重 ──
        total = xgb_acc + lgb_acc
        if total > 0:
            self.xgb_weight = xgb_acc / total
            self.lgb_weight = lgb_acc / total
        else:
            self.xgb_weight = 0.5
            self.lgb_weight = 0.5

        print(f"[Ensemble] Weights - XGB: {self.xgb_weight:.3f}, LGB: {self.lgb_weight:.3f}")

    def predict(self, X=None):
        """集成预测。X 为 None 时使用 mock 数据"""
        if X is None:
            X = np.random.randn(5, 30).astype(np.float32)

        if self.xgb_model is None or self.lgb_model is None:
            raise RuntimeError("模型尚未训练，请先调用 train()")

        xgb_proba = self.xgb_model.predict_proba(X)
        lgb_proba = self.lgb_model.predict_proba(X)

        # 加权集成
        ensemble_proba = (
            self.xgb_weight * xgb_proba + self.lgb_weight * lgb_proba
        )
        predictions = ensemble_proba.argmax(axis=-1)
        confidences = ensemble_proba.max(axis=-1)

        results = []
        for i, pred in enumerate(predictions):
            results.append({
                "direction": self.LABELS[pred],
                "confidence": round(float(confidences[i]), 4),
                "probabilities": {
                    self.LABELS[j]: round(float(ensemble_proba[i][j]), 4)
                    for j in range(len(self.LABELS))
                },
            })
        return results

    def get_feature_importance(self, top_n=20):
        """获取特征重要性（XGBoost 和 LightGBM 平均）"""
        if self.xgb_model is None or self.lgb_model is None:
            raise RuntimeError("模型尚未训练，请先调用 train()")

        xgb_imp = self.xgb_model.feature_importances_
        lgb_imp = self.lgb_model.feature_importances_

        avg_imp = (xgb_imp + lgb_imp) / 2.0
        indices = np.argsort(avg_imp)[::-1][:top_n]

        importance = []
        for idx in indices:
            name = self.feature_names[idx] if idx < len(self.feature_names) else f"f_{idx}"
            importance.append({
                "feature": name,
                "xgb_importance": round(float(xgb_imp[idx]), 6),
                "lgb_importance": round(float(lgb_imp[idx]), 6),
                "avg_importance": round(float(avg_imp[idx]), 6),
            })
        return importance

    def save(self, path):
        """保存模型到文件"""
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "wb") as f:
            pickle.dump({
                "xgb_model": self.xgb_model,
                "lgb_model": self.lgb_model,
                "xgb_weight": self.xgb_weight,
                "lgb_weight": self.lgb_weight,
                "feature_names": self.feature_names,
            }, f)

    def load(self, path):
        """从文件加载模型"""
        with open(path, "rb") as f:
            data = pickle.load(f)
        self.xgb_model = data["xgb_model"]
        self.lgb_model = data["lgb_model"]
        self.xgb_weight = data["xgb_weight"]
        self.lgb_weight = data["lgb_weight"]
        self.feature_names = data.get("feature_names", [])


# ──────────────────────────────────────────────
# 自测入口
# ──────────────────────────────────────────────

if __name__ == "__main__":
    predictor = EnsemblePredictor()
    print("Training with mock data...")
    predictor.train()
    results = predictor.predict()
    print("Prediction results:", results)
    imp = predictor.get_feature_importance(top_n=5)
    print("Top 5 feature importance:", imp)
    predictor.save("/tmp/medium_term_model.pkl")
    print("Model saved.")