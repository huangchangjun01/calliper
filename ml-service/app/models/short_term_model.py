"""
短期预测模型: LSTM + Attention
预测目标: 未来1-3天涨跌方向
输入: 近30日分钟级数据 + 技术指标
"""

import os
import pickle

import numpy as np
import pandas as pd
import torch
import torch.nn as nn
import torch.nn.functional as F
from torch.utils.data import DataLoader, TensorDataset


# ──────────────────────────────────────────────
# 模型定义
# ──────────────────────────────────────────────

class LSTMAttentionModel(nn.Module):
    """LSTM + Multi-head Attention 短期预测模型"""

    def __init__(self, input_size=20, hidden_size=128, num_layers=2,
                 num_heads=4, num_classes=3, dropout=0.3):
        super().__init__()
        self.input_size = input_size
        self.hidden_size = hidden_size
        self.num_layers = num_layers
        self.num_classes = num_classes

        self.lstm = nn.LSTM(
            input_size=input_size,
            hidden_size=hidden_size,
            num_layers=num_layers,
            batch_first=True,
            dropout=dropout if num_layers > 1 else 0.0,
        )

        self.attention = nn.MultiheadAttention(
            embed_dim=hidden_size,
            num_heads=num_heads,
            dropout=dropout,
            batch_first=True,
        )

        self.dropout = nn.Dropout(dropout)
        self.fc1 = nn.Linear(hidden_size, hidden_size // 2)
        self.fc2 = nn.Linear(hidden_size // 2, num_classes)
        self.layer_norm = nn.LayerNorm(hidden_size)

    def forward(self, x):
        # x: (batch, seq_len, input_size)
        lstm_out, _ = self.lstm(x)  # (batch, seq_len, hidden_size)

        attn_out, _ = self.attention(lstm_out, lstm_out, lstm_out)
        attn_out = self.layer_norm(lstm_out + attn_out)

        # 取最后一个时间步
        out = attn_out[:, -1, :]  # (batch, hidden_size)
        out = self.dropout(out)
        out = F.relu(self.fc1(out))
        out = self.dropout(out)
        out = self.fc2(out)
        return out


# ──────────────────────────────────────────────
# 预测器封装
# ──────────────────────────────────────────────

class ShortTermPredictor:
    """短期预测器：封装训练、预测、持久化"""

    LABELS = ["下跌", "震荡", "上涨"]

    def __init__(self, input_size=20, hidden_size=128, num_layers=2,
                 num_heads=4, num_classes=3, dropout=0.3, device=None):
        self.device = device or torch.device("cuda" if torch.cuda.is_available() else "cpu")
        self.model = LSTMAttentionModel(
            input_size=input_size,
            hidden_size=hidden_size,
            num_layers=num_layers,
            num_heads=num_heads,
            num_classes=num_classes,
            dropout=dropout,
        ).to(self.device)
        self.input_size = input_size

    def train(self, df, y, epochs=50, batch_size=32, lr=0.001):
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
        optimizer = torch.optim.Adam(self.model.parameters(), lr=lr)

        self.model.train()
        for epoch in range(epochs):
            total_loss = 0.0
            for batch_X, batch_y in loader:
                optimizer.zero_grad()
                outputs = self.model(batch_X)
                loss = criterion(outputs, batch_y)
                loss.backward()
                optimizer.step()
                total_loss += loss.item()

            if (epoch + 1) % 10 == 0:
                avg_loss = total_loss / len(loader)
                print(f"[ShortTerm] Epoch {epoch+1}/{epochs} - Loss: {avg_loss:.4f}")

    def predict(self, df, current_price=None):
        """预测。df: 特征数据 (DataFrame 或 numpy array), current_price: 当前价格（用于计算目标价位）"""
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
            pass  # 已经是 (batch, seq_len, features)

        X_tensor = torch.tensor(X, dtype=torch.float32).to(self.device)
        with torch.no_grad():
            logits = self.model(X_tensor)
            probs = F.softmax(logits, dim=-1).cpu().numpy()

        predictions = probs.argmax(axis=-1)
        confidences = probs.max(axis=-1)

        # 基于真实当前价格计算目标价位
        if current_price is None:
            current_price = self._get_current_price_from_data(df)
        target_price_map = {0: current_price * 0.98, 1: current_price * 1.00, 2: current_price * 1.03}

        results = []
        for i, pred in enumerate(predictions):
            results.append({
                "direction": self.LABELS[pred],
                "confidence": float(confidences[i]),
                "target_price": round(target_price_map[pred], 2),
                "probabilities": {
                    self.LABELS[j]: round(float(probs[i][j]), 4)
                    for j in range(len(self.LABELS))
                },
            })
        return results

    def _get_current_price_from_data(self, df):
        """从特征数据中提取当前价格（取最后一行的 close 列，或最后一列）"""
        if hasattr(df, "columns") and "close" in df.columns:
            return float(df["close"].iloc[-1])
        if hasattr(df, "values"):
            arr = df.values
        else:
            arr = np.asarray(df)
        if arr.ndim == 3:
            arr = arr[0]  # 取第一个 batch
        if arr.ndim == 2:
            return float(arr[-1, -1]) if arr.shape[1] > 0 else 100.0
        return 100.0

    def save(self, path):
        """保存模型到文件"""
        os.makedirs(os.path.dirname(path), exist_ok=True)
        checkpoint = {
            "model_state_dict": self.model.state_dict(),
            "input_size": self.input_size,
            "model_class": "LSTMAttentionModel",
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
    predictor = ShortTermPredictor()
    # 使用真实格式的 DataFrame 进行训练
    dates = pd.date_range("2024-01-01", periods=60, freq="D")
    df = pd.DataFrame({
        "open":  100 + np.cumsum(np.random.randn(60) * 0.5),
        "high":  100 + np.cumsum(np.random.randn(60) * 0.5) + 1,
        "low":   100 + np.cumsum(np.random.randn(60) * 0.5) - 1,
        "close": 100 + np.cumsum(np.random.randn(60) * 0.5),
        "volume": np.random.randint(1000000, 10000000, 60),
    }, index=dates)
    y = np.zeros(60, dtype=np.int64)
    for i in range(30, 60):
        trend = df["close"].iloc[i] - df["close"].iloc[i-5]
        if trend > 1.0:
            y[i] = 2
        elif trend < -1.0:
            y[i] = 0
        else:
            y[i] = 1
    print("Training with real-format data...")
    predictor.train(df, y, epochs=20)
    results = predictor.predict(df)
    print("Prediction results:", results)
    predictor.save("/tmp/short_term_model.pt")
    print("Model saved.")