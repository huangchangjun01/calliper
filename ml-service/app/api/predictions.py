from fastapi import APIRouter, HTTPException, Query, Request
from typing import List, Optional
from pydantic import BaseModel

router = APIRouter()


# ──────────────────────────────────────────────────────────────
# Schemas
# ──────────────────────────────────────────────────────────────

class BatchRequest(BaseModel):
    symbols: List[str]


class FactorItem(BaseModel):
    name: str
    value: float
    description: str


class PredictionResult(BaseModel):
    symbol: str
    period: str
    direction: str
    confidence: float
    target_price: float
    factors: List[FactorItem]
    model_version: str
    predicted_at: str


class PredictionHistoryItem(BaseModel):
    id: int
    symbol: str
    period: str
    direction: str
    confidence: float
    target_price: float
    predicted_at: str
    is_correct: Optional[bool] = None


class AccuracyReport(BaseModel):
    symbol: str
    accuracy_7d: float
    accuracy_30d: float
    accuracy_total: float
    total_predictions: int


# ──────────────────────────────────────────────────────────────
# Mock helpers
# ──────────────────────────────────────────────────────────────

def _mock_factors(symbol: str, period: str) -> List[FactorItem]:
    base = {
        "short_term": [
            FactorItem(name="rsi", value=55.3, description="Relative Strength Index (14)"),
            FactorItem(name="macd", value=1.25, description="MACD signal line difference"),
            FactorItem(name="volume_ratio", value=1.35, description="Volume ratio vs 5-day average"),
            FactorItem(name="ma_alignment", value=1.0, description="MA5 vs MA20 alignment"),
        ],
        "medium_term": [
            FactorItem(name="momentum_20", value=3.2, description="20-day price momentum"),
            FactorItem(name="volatility_20", value=0.018, description="20-day volatility"),
            FactorItem(name="sector_relative", value=0.85, description="Relative strength vs sector"),
            FactorItem(name="money_flow", value=1250000.0, description="Aggregated money flow"),
        ],
        "long_term": [
            FactorItem(name="pe_ratio", value=22.5, description="Price-to-Earnings ratio"),
            FactorItem(name="market_cap", value=2.8e12, description="Market capitalization"),
            FactorItem(name="revenue_growth", value=0.12, description="YoY revenue growth"),
            FactorItem(name="debt_equity", value=0.65, description="Debt-to-Equity ratio"),
        ],
    }
    return base.get(period, base["short_term"])


def _mock_prediction(symbol: str, period: str) -> PredictionResult:
    import datetime
    mock_data = {
        "short_term": {"direction": "up", "confidence": 0.72, "target_price": 185.50},
        "medium_term": {"direction": "up", "confidence": 0.68, "target_price": 192.00},
        "long_term": {"direction": "up", "confidence": 0.75, "target_price": 210.00},
    }
    data = mock_data.get(period, mock_data["short_term"])
    return PredictionResult(
        symbol=symbol,
        period=period,
        direction=data["direction"],
        confidence=data["confidence"],
        target_price=data["target_price"],
        factors=_mock_factors(symbol, period),
        model_version="v1.2.0",
        predicted_at=datetime.datetime.utcnow().isoformat() + "Z",
    )


# ──────────────────────────────────────────────────────────────
# Endpoints
# ──────────────────────────────────────────────────────────────

@router.get("/{symbol}", response_model=PredictionResult)
async def get_prediction(symbol: str, request: Request):
    """获取单只股票最新预测，返回三个周期的预测结果"""
    prediction_task = request.app.state.prediction_task
    try:
        result = prediction_task.predict_single_stock(symbol)
        return _mock_prediction(symbol, "short_term")
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Prediction failed: {str(e)}")


@router.post("/batch", response_model=List[PredictionResult])
async def batch_predict(request: Request, body: BatchRequest):
    """批量预测"""
    prediction_task = request.app.state.prediction_task
    results = []
    for symbol in body.symbols:
        for period in ["short_term", "medium_term", "long_term"]:
            results.append(_mock_prediction(symbol, period))
    return results


@router.get("/{symbol}/history", response_model=List[PredictionHistoryItem])
async def get_prediction_history(
    symbol: str,
    request: Request,
    period: str = Query("short_term", description="Prediction period: short_term, medium_term, long_term"),
    limit: int = Query(10, ge=1, le=100),
):
    """获取历史预测"""
    import datetime
    history = []
    for i in range(limit):
        day = datetime.datetime.utcnow() - datetime.timedelta(days=i)
        history.append(
            PredictionHistoryItem(
                id=1000 + i,
                symbol=symbol,
                period=period,
                direction="up" if i % 3 != 0 else "down",
                confidence=round(0.65 + (i % 5) * 0.03, 2),
                target_price=150.0 + i * 2.5,
                predicted_at=day.isoformat() + "Z",
                is_correct=(i % 2 == 0),
            )
        )
    return history[:limit]


@router.post("/run")
async def run_prediction(request: Request):
    """手动触发每日预测任务"""
    prediction_task = request.app.state.prediction_task
    try:
        prediction_task.run_daily_prediction()
        return {"status": "success", "message": "Daily prediction task triggered"}
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Prediction run failed: {str(e)}")


@router.get("/accuracy/{symbol}", response_model=AccuracyReport)
async def get_prediction_accuracy(symbol: str, request: Request):
    """获取预测准确率"""
    return AccuracyReport(
        symbol=symbol,
        accuracy_7d=0.71,
        accuracy_30d=0.68,
        accuracy_total=0.66,
        total_predictions=245,
    )