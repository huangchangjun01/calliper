#!/usr/bin/env python3
"""
量化交易系统 - 整合 Dashboard 服务
提供 K 线图 + 模拟交易预测的统一界面
"""
import json
import http.server
import os
import sys
import urllib.parse
from datetime import datetime

# 将 realtime_trading 模块导入路径
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

PORT = 8765

# ============================================================
# 预测 API 处理
# ============================================================
def run_prediction():
    """运行实时预测交易，返回结果"""
    from realtime_trading import (
        MarketDataFetcher, TechnicalPredictor, TradingEngine,
        AccuracyTracker, STOCK_POOL, INITIAL_CAPITAL, generate_report
    )
    
    fetcher = MarketDataFetcher()
    predictor = TechnicalPredictor()
    engine = TradingEngine(INITIAL_CAPITAL)
    tracker = AccuracyTracker()

    base_data = fetcher.fetch_all(STOCK_POOL)
    
    if not base_data:
        return {"error": "无法获取实时行情数据，请检查网络连接"}
    
    # 生成预测
    predictions = []
    for stock in base_data:
        for period in ["short", "medium", "long"]:
            pred = predictor.predict(stock, period)
            predictions.append(pred)
    
    # 执行交易
    engine.execute_trades(predictions, STOCK_POOL, 0)
    day_pnl = engine.settle_day(base_data)
    
    # 追踪预测准确率
    for stock in base_data:
        actual_direction = "neutral"
        if stock["change_pct"] > 0.3:
            actual_direction = "up"
        elif stock["change_pct"] < -0.3:
            actual_direction = "down"
        for pred in predictions:
            if pred["symbol"] == stock["code"]:
                tracker.log_prediction(pred, actual_direction)
    
    metrics = engine.get_metrics()
    accuracy = tracker.get_summary()
    
    # 构建行情概览
    market_snapshot = []
    for s in sorted(base_data, key=lambda x: abs(x.get("change_pct", 0)), reverse=True):
        market_snapshot.append({
            "code": s["code"],
            "name": s["name"],
            "price": s["price"],
            "change_pct": round(s.get("change_pct", 0), 2),
            "market": s.get("market", ""),
            "industry": s.get("industry", ""),
            "volume": s.get("volume", 0),
        })
    
    # 构建预测列表
    pred_list = []
    for p in sorted(predictions, key=lambda x: x["confidence"], reverse=True):
        if p["direction"] != "neutral":
            pred_list.append({
                "symbol": p["symbol"],
                "name": p["name"],
                "period": p["period"],
                "direction": p["direction"],
                "confidence": p["confidence"],
                "current_price": p["current_price"],
                "target_price": p["target_price"],
                "signals": p["signals"],
                "score": p["total_score"],
            })
    
    # 交易记录
    trade_list = []
    for t in engine.trades:
        trade_list.append({
            "symbol": t["symbol"],
            "name": t["name"],
            "direction": t["direction"],
            "price": t["price"],
            "quantity": t["quantity"],
            "cost": t["cost"],
            "confidence": t["confidence"],
            "period": t["period"],
            "pnl": t.get("pnl", None),
            "is_win": t.get("is_win", None),
        })
    
    return {
        "timestamp": datetime.now().strftime('%Y-%m-%d %H:%M:%S'),
        "market_snapshot": market_snapshot,
        "predictions": pred_list,
        "trades": trade_list,
        "metrics": {
            "initial_capital": metrics["initial_capital"],
            "total_assets": metrics["total_assets"],
            "total_pnl": metrics["total_pnl"],
            "total_return": metrics["total_return"],
            "sharpe_ratio": metrics["sharpe_ratio"],
            "max_drawdown": metrics["max_drawdown"],
            "trade_count": metrics["trade_count"],
            "win_rate": metrics["win_rate"],
            "win_count": metrics["win_count"],
        },
        "accuracy": {
            "overall_accuracy": accuracy.get("overall_accuracy", 0),
            "total_predictions": accuracy.get("total_predictions", 0),
            "correct_predictions": accuracy.get("correct_predictions", 0),
            "by_period": accuracy.get("by_period", {}),
            "by_stock": accuracy.get("by_stock", [])[:10],
        },
    }


class DashboardHandler(http.server.SimpleHTTPRequestHandler):
    """自定义 HTTP 请求处理器"""
    
    def do_GET(self):
        parsed = urllib.parse.urlparse(self.path)
        
        if parsed.path == "/api/predict":
            # 预测 API
            self.send_response(200)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Access-Control-Allow-Origin", "*")
            self.end_headers()
            try:
                result = run_prediction()
                self.wfile.write(json.dumps(result, ensure_ascii=False, default=str).encode("utf-8"))
            except Exception as e:
                self.wfile.write(json.dumps({"error": str(e)}, ensure_ascii=False).encode("utf-8"))
        elif parsed.path == "/api/health":
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Access-Control-Allow-Origin", "*")
            self.end_headers()
            self.wfile.write(json.dumps({"status": "ok"}).encode())
        elif parsed.path == "/" or parsed.path == "/dashboard":
            # 返回 Dashboard 页面
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.end_headers()
            with open(os.path.join(os.path.dirname(__file__), "dashboard.html"), "rb") as f:
                self.wfile.write(f.read())
        else:
            # 静态文件
            super().do_GET()
    
    def do_OPTIONS(self):
        self.send_response(200)
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Content-Type")
        self.end_headers()
    
    def log_message(self, format, *args):
        print(f"[{datetime.now().strftime('%H:%M:%S')}] {args[0]}")


if __name__ == "__main__":
    os.chdir(os.path.dirname(os.path.abspath(__file__)))
    server = http.server.HTTPServer(("0.0.0.0", PORT), DashboardHandler)
    print(f"Dashboard 服务启动: http://localhost:{PORT}/dashboard")
    print(f"预测 API: http://localhost:{PORT}/api/predict")
    print(f"K线图页面: http://localhost:{PORT}/kline_viewer.html")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\n服务已停止")
        server.shutdown()