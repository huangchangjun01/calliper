"""
预测任务执行
"""

import json
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime
from typing import Dict, List, Optional, Any

import numpy as np

from ..models.model_manager import ModelManager
from ..utils.data_loader import DataLoader


class PredictionTask:
    """预测任务执行器：每日预测、单股票预测、并发预测"""

    def __init__(self, model_manager: Optional[ModelManager] = None, max_workers: int = 8):
        self.model_manager = model_manager or ModelManager()
        self.max_workers = max_workers
        self.prediction_history: List[Dict] = []
        self.data_loader = DataLoader()

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

    def _build_features(self, symbol: str, period: str) -> Any:
        """构建特征数据，从真实数据源加载"""
        try:
            from datetime import timedelta
            end = datetime.now()
            start = end - timedelta(days=60)

            if period == "short_term":
                # Load minute-level data for last 30 days
                df = self.data_loader.load_stock_data(
                    symbol, start.strftime("%Y-%m-%d"), end.strftime("%Y-%m-%d"), interval="1h"
                )
            elif period == "medium_term":
                # Load daily data for last 60 days
                df = self.data_loader.load_stock_data(
                    symbol, start.strftime("%Y-%m-%d"), end.strftime("%Y-%m-%d"), interval="1d"
                )
            elif period == "long_term":
                # Load weekly data for last 52 weeks
                start = end - timedelta(days=365)
                df = self.data_loader.load_stock_data(
                    symbol, start.strftime("%Y-%m-%d"), end.strftime("%Y-%m-%d"), interval="1d"
                )
            else:
                return None

            if df.empty:
                return None

            # Convert to numpy array for model input
            feature_cols = ["open", "high", "low", "close", "volume"]
            available_cols = [c for c in feature_cols if c in df.columns]
            if not available_cols:
                return None

            features = df[available_cols].values.astype(np.float32)
            return features
        except Exception as e:
            print(f"[PredictionTask] Error building features for {symbol}/{period}: {e}")
            return None

    def predict_single_stock(self, symbol: str) -> Dict[str, Any]:
        """
        对单只股票进行三个周期的预测
        :param symbol: 股票代码
        :return: 预测结果字典
        """
        result = {
            "symbol": symbol,
            "timestamp": datetime.now().isoformat(),
            "predictions": {},
            "model_version": self.model_manager.versions.get("short_term", {}).get("version", "unknown"),
        }

        try:
            # 短期预测
            short_features = self._build_features(symbol, "short_term")
            if short_features is not None:
                short_preds = self.model_manager.predict_single("short_term", short_features)
                result["predictions"]["short_term"] = short_preds

            # 中期预测
            medium_features = self._build_features(symbol, "medium_term")
            if medium_features is not None:
                medium_preds = self.model_manager.predict_single("medium_term", medium_features)
                result["predictions"]["medium_term"] = medium_preds

            # 长期预测
            long_features = self._build_features(symbol, "long_term")
            if long_features is not None:
                long_preds = self.model_manager.predict_single("long_term", long_features)
                result["predictions"]["long_term"] = long_preds

        except Exception as e:
            result["error"] = str(e)

        return result

    def run_daily_prediction(self, symbols: Optional[List[str]] = None) -> List[Dict]:
        """
        执行每日预测：遍历所有活跃股票，并发预测
        :param symbols: 股票列表，None 则使用活跃股票
        :return: 所有预测结果列表
        """
        symbols = symbols or self._get_active_stocks()
        print(f"[PredictionTask] Starting daily prediction for {len(symbols)} stocks at {datetime.now()}")

        results = []
        with ThreadPoolExecutor(max_workers=self.max_workers) as executor:
            future_to_symbol = {
                executor.submit(self.predict_single_stock, symbol): symbol
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
                        "timestamp": datetime.now().isoformat(),
                        "error": str(e),
                    })

        # 保存预测历史
        self.prediction_history.append({
            "timestamp": datetime.now().isoformat(),
            "stock_count": len(symbols),
            "results": results,
        })

        print(f"[PredictionTask] Completed predictions for {len(results)} stocks")
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
            "timestamp": datetime.now().isoformat(),
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