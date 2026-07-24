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

    def _generate_mock_data(self, num_samples=500, seq_len=30):
        """生成 mock 训练数据"""
        X = np.random.randn(num_samples, seq_len, self.input_size).astype(np.float32)
        # 模拟标签：添加一些可学习的模式
        y = np.zeros(num_samples, dtype=np.int64)
        for i in range(num_samples):
            trend = X[i, -5:, 0].mean() - X[i, :5, 0].mean()
            if trend > 0.3:
                y[i] = 2  # 上涨
            elif trend < -0.3:
                y[i] = 0  # 下跌
            else:
                y[i] = 1  # 震荡
        return X, y

    def train(self, df=None, epochs=50, batch_size=32, lr=0.001):
        """训练模型。df 为 None 时使用 mock 数据，支持 DataFrame 或 numpy array"""
        if df is not None:
            if hasattr(df, "values"):
                X = df.values.astype(np.float32)
            else:
                X = np.asarray(df, dtype=np.float32)
            y = np.random.randint(0, 3, len(X))
            if X.ndim == 2:
                seq_len = X.shape[0]
                X = X.reshape(-1, seq_len, X.shape[1])
        else:
            X, y = self._generate_mock_data()

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

    def predict(self, df=None):
        """预测。df 为 None 时使用 mock 数据，支持 DataFrame 或 numpy array"""
        self.model.eval()
        if df is not None:
            if hasattr(df, "values"):
                X = df.values.astype(np.float32)
            else:
                X = np.asarray(df, dtype=np.float32)
            if X.ndim == 2:
                X = X.reshape(1, X.shape[0], X.shape[1])
            elif X.ndim == 3:
                pass  # 已经是 (batch, seq_len, features)
        else:
            X = np.random.randn(1, 30, self.input_size).astype(np.float32)

        X_tensor = torch.tensor(X, dtype=torch.float32).to(self.device)
        with torch.no_grad():
            logits = self.model(X_tensor)
            probs = F.softmax(logits, dim=-1).cpu().numpy()

        predictions = probs.argmax(axis=-1)
        confidences = probs.max(axis=-1)

        # 模拟目标价位
        current_price = 100.0
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
    predictor = ShortTermPredictor()
    print("Training with mock data...")
    predictor.train(epochs=20)
    results = predictor.predict()
    print("Prediction results:", results)
    predictor.save("/tmp/short_term_model.pt")
    print("Model saved.")