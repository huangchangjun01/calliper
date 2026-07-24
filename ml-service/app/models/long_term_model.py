"""
长期预测模型: Transformer
预测目标: 未来1-6个月趋势
输入: 周线数据 + 财务数据 + 行业趋势 + 宏观经济指标
"""

import os
import math

import numpy as np
import torch
import torch.nn as nn
import torch.nn.functional as F
from torch.utils.data import DataLoader, TensorDataset


# ──────────────────────────────────────────────
# Positional Encoding
# ──────────────────────────────────────────────

class PositionalEncoding(nn.Module):
    def __init__(self, d_model, max_len=500, dropout=0.1):
        super().__init__()
        self.dropout = nn.Dropout(p=dropout)

        pe = torch.zeros(max_len, d_model)
        position = torch.arange(0, max_len, dtype=torch.float32).unsqueeze(1)
        div_term = torch.exp(
            torch.arange(0, d_model, 2, dtype=torch.float32)
            * (-math.log(10000.0) / d_model)
        )
        pe[:, 0::2] = torch.sin(position * div_term)
        pe[:, 1::2] = torch.cos(position * div_term)
        pe = pe.unsqueeze(0)  # (1, max_len, d_model)
        self.register_buffer("pe", pe)

    def forward(self, x):
        # x: (batch, seq_len, d_model)
        x = x + self.pe[:, :x.size(1), :]
        return self.dropout(x)


# ──────────────────────────────────────────────
# Transformer 预测模型
# ──────────────────────────────────────────────

class TransformerPredictorModel(nn.Module):
    """基于 Transformer Encoder 的长期趋势预测"""

    def __init__(self, input_size=40, d_model=256, nhead=8, num_layers=4,
                 dim_feedforward=512, num_classes=3, dropout=0.2):
        super().__init__()
        self.input_size = input_size
        self.d_model = d_model
        self.num_classes = num_classes

        self.input_proj = nn.Linear(input_size, d_model)
        self.pos_encoder = PositionalEncoding(d_model, dropout=dropout)

        encoder_layer = nn.TransformerEncoderLayer(
            d_model=d_model,
            nhead=nhead,
            dim_feedforward=dim_feedforward,
            dropout=dropout,
            batch_first=True,
        )
        self.transformer_encoder = nn.TransformerEncoder(
            encoder_layer, num_layers=num_layers
        )

        self.dropout = nn.Dropout(dropout)
        self.fc1 = nn.Linear(d_model, d_model // 2)
        self.fc2 = nn.Linear(d_model // 2, num_classes)
        self.layer_norm = nn.LayerNorm(d_model)

    def forward(self, x):
        # x: (batch, seq_len, input_size)
        x = self.input_proj(x)  # (batch, seq_len, d_model)
        x = self.pos_encoder(x)
        x = self.transformer_encoder(x)  # (batch, seq_len, d_model)

        # 全局平均池化
        x = x.mean(dim=1)  # (batch, d_model)
        x = self.layer_norm(x)
        x = self.dropout(x)
        x = F.relu(self.fc1(x))
        x = self.dropout(x)
        x = self.fc2(x)
        return x


# ──────────────────────────────────────────────
# 预测器封装
# ──────────────────────────────────────────────

class LongTermPredictor:
    """长期预测器：封装训练、预测、持久化"""

    LABELS = ["下跌趋势", "震荡趋势", "上涨趋势"]

    def __init__(self, input_size=40, d_model=256, nhead=8, num_layers=4,
                 num_classes=3, dropout=0.2, device=None):
        self.device = device or torch.device("cuda" if torch.cuda.is_available() else "cpu")
        self.model = TransformerPredictorModel(
            input_size=input_size,
            d_model=d_model,
            nhead=nhead,
            num_layers=num_layers,
            num_classes=num_classes,
            dropout=dropout,
        ).to(self.device)
        self.input_size = input_size

    def train(self, df, y, epochs=50, batch_size=16, lr=0.0005):
        """训练模型。df: 特征数据 (DataFrame 或 numpy array), y: 标签数据 (numpy array)"""
        if df is None or y is None:
            raise ValueError("训练数据不能为空，必须提供真实市场数据")

        if hasattr(df, "values"):
            X = df.values.astype(np.float32)
        else:
            X = np.asarray(df, dtype=np.float32)

        y = np.asarray(y, dtype=np.int64)
        if X.ndim == 2:
            seq_len = X.shape[0]
            X = X.reshape(-1, seq_len, X.shape[1])

        X_tensor = torch.tensor(X, dtype=torch.float32).to(self.device)
        y_tensor = torch.tensor(y, dtype=torch.long).to(self.device)

        dataset = TensorDataset(X_tensor, y_tensor)
        loader = DataLoader(dataset, batch_size=batch_size, shuffle=True)

        criterion = nn.CrossEntropyLoss()
        optimizer = torch.optim.AdamW(self.model.parameters(), lr=lr)

        self.model.train()
        for epoch in range(epochs):
            total_loss = 0.0
            for batch_X, batch_y in loader:
                optimizer.zero_grad()
                outputs = self.model(batch_X)
                loss = criterion(outputs, batch_y)
                loss.backward()
                torch.nn.utils.clip_grad_norm_(self.model.parameters(), 1.0)
                optimizer.step()
                total_loss += loss.item()

            if (epoch + 1) % 10 == 0:
                avg_loss = total_loss / len(loader)
                print(f"[LongTerm] Epoch {epoch+1}/{epochs} - Loss: {avg_loss:.4f}")

    def predict(self, df, current_price=None):
        """预测。df: 特征数据 (DataFrame 或 numpy array), current_price: 当前价格"""
        if df is None:
            raise ValueError("预测数据不能为空，必须提供真实市场数据")

        self.model.eval()
        if hasattr(df, "values"):
            X = df.values.astype(np.float32)
        else:
            X = np.asarray(df, dtype=np.float32)
        if X.ndim == 2:
            X = X.reshape(1, X.shape[0], X.shape[1])
        elif X.ndim == 3:
            pass

        X_tensor = torch.tensor(X, dtype=torch.float32).to(self.device)
        with torch.no_grad():
            logits = self.model(X_tensor)
            probs = F.softmax(logits, dim=-1).cpu().numpy()

        predictions = probs.argmax(axis=-1)
        confidences = probs.max(axis=-1)

        # 基于真实当前价格计算月度目标价位
        if current_price is None:
            current_price = self._get_current_price_from_data(df)
        target_price_map = {0: current_price * 0.90, 1: current_price * 1.02, 2: current_price * 1.15}

        results = []
        for i, pred in enumerate(predictions):
            results.append({
                "direction": self.LABELS[pred],
                "confidence": round(float(confidences[i]), 4),
                "target_price": round(target_price_map[pred], 2),
                "probabilities": {
                    self.LABELS[j]: round(float(probs[i][j]), 4)
                    for j in range(len(self.LABELS))
                },
            })
        return results

    def _get_current_price_from_data(self, df):
        """从特征数据中提取当前价格"""
        if hasattr(df, "columns") and "close" in df.columns:
            return float(df["close"].iloc[-1])
        if hasattr(df, "values"):
            arr = df.values
        else:
            arr = np.asarray(df)
        if arr.ndim == 3:
            arr = arr[0]
        if arr.ndim == 2:
            return float(arr[-1, -1]) if arr.shape[1] > 0 else 100.0
        return 100.0

    def save(self, path):
        """保存模型到文件"""
        os.makedirs(os.path.dirname(path), exist_ok=True)
        checkpoint = {
            "model_state_dict": self.model.state_dict(),
            "input_size": self.input_size,
            "model_class": "TransformerPredictorModel",
        }
        torch.save(checkpoint, path)

    def load(self, path):
        """从文件加载模型"""
        checkpoint = torch.load(path, map_location=self.device)
        self.model.load_state_dict(checkpoint["model_state_dict"])
        self.model.to(self.device)
        self.model.eval()


# ──────────────────────────────────────────────
# 自测入口
# ──────────────────────────────────────────────

if __name__ == "__main__":
    import pandas as pd
    predictor = LongTermPredictor()
    dates = pd.date_range("2024-01-01", periods=60, freq="W")
    df = pd.DataFrame({
        "open":  100 + np.cumsum(np.random.randn(60) * 0.5),
        "high":  100 + np.cumsum(np.random.randn(60) * 0.5) + 1,
        "low":   100 + np.cumsum(np.random.randn(60) * 0.5) - 1,
        "close": 100 + np.cumsum(np.random.randn(60) * 0.5),
        "volume": np.random.randint(1000000, 10000000, 60),
    }, index=dates)
    y = np.zeros(60, dtype=np.int64)
    for i in range(30, 60):
        trend = df["close"].iloc[i] - df["close"].iloc[i-13]
        if trend > 2.0:
            y[i] = 2
        elif trend < -2.0:
            y[i] = 0
        else:
            y[i] = 1
    print("Training with real-format data...")
    predictor.train(df, y, epochs=20)
    results = predictor.predict(df)
    print("Prediction results:", results)
    predictor.save("/tmp/long_term_model.pt")
    print("Model saved.")