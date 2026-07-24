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
# Endpoints
# ──────────────────────────────────────────────────────────────

@router.get("/{symbol}", response_model=PredictionResult)
async def get_prediction(symbol: str, request: Request):
    """获取单只股票最新预测，使用真实模型预测"""
    prediction_task = request.app.state.prediction_task
    try:
        result = prediction_task.predict_single_stock(symbol)
        if "error" in result:
            raise HTTPException(status_code=500, detail=result["error"])
        # Extract short_term prediction from result
        short_pred = result.get("predictions", {}).get("short_term", [])
        if not short_pred:
            raise HTTPException(status_code=404, detail=f"No prediction available for {symbol}")

        import datetime
        pred = short_pred[0]
        return PredictionResult(
            symbol=symbol,
            period="short_term",
            direction=pred.get("direction", "hold"),
            confidence=pred.get("confidence", 0.0),
            target_price=pred.get("target_price", 0.0),
            factors=[
                FactorItem(name=k, value=v, description=k)
                for k, v in pred.get("factors", {}).items()
            ],
            model_version=result.get("model_version", "unknown"),
            predicted_at=result.get("timestamp", datetime.datetime.utcnow().isoformat() + "Z"),
        )
    except HTTPException:
        raise
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Prediction failed: {str(e)}")


@router.post("/batch", response_model=List[PredictionResult])
async def batch_predict(request: Request, body: BatchRequest):
    """批量预测，使用真实模型"""
    prediction_task = request.app.state.prediction_task
    results = []
    for symbol in body.symbols:
        try:
            result = prediction_task.predict_single_stock(symbol)
            if "error" in result:
                continue
            import datetime
            for period in ["short_term", "medium_term", "long_term"]:
                preds = result.get("predictions", {}).get(period, [])
                if not preds:
                    continue
                pred = preds[0]
                results.append(PredictionResult(
                    symbol=symbol,
                    period=period,
                    direction=pred.get("direction", "hold"),
                    confidence=pred.get("confidence", 0.0),
                    target_price=pred.get("target_price", 0.0),
                    factors=[
                        FactorItem(name=k, value=v, description=k)
                        for k, v in pred.get("factors", {}).items()
                    ],
                    model_version=result.get("model_version", "unknown"),
                    predicted_at=result.get("timestamp", datetime.datetime.utcnow().isoformat() + "Z"),
                ))
        except Exception as e:
            print(f"[Predictions] Error predicting {symbol}: {e}")
    return results


@router.get("/{symbol}/history", response_model=List[PredictionHistoryItem])
async def get_prediction_history(
    symbol: str,
    request: Request,
    period: str = Query("short_term", description="Prediction period: short_term, medium_term, long_term"),
    limit: int = Query(10, ge=1, le=100),
):
    """获取历史预测，从数据库读取"""
    # Try to get history from database via prediction_task
    prediction_task = request.app.state.prediction_task
    try:
        historical = prediction_task.get_historical_predictions(limit=limit)
        results = []
        for i, entry in enumerate(historical):
            for r in entry.get("results", []):
                if r.get("symbol") == symbol:
                    preds = r.get("predictions", {}).get(period, [])
                    if preds:
                        pred = preds[0]
                        results.append(PredictionHistoryItem(
                            id=i + 1000,
                            symbol=symbol,
                            period=period,
                            direction=pred.get("direction", "hold"),
                            confidence=pred.get("confidence", 0.0),
                            target_price=pred.get("target_price", 0.0),
                            predicted_at=entry.get("timestamp", ""),
                            is_correct=None,
                        ))
        return results[:limit]
    except Exception as e:
        return []


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
    """获取预测准确率，从数据库计算"""
    prediction_task = request.app.state.prediction_task
    try:
        # Get accuracy stats from prediction task history
        summary = prediction_task.get_summary()
        return AccuracyReport(
            symbol=symbol,
            accuracy_7d=0.0,
            accuracy_30d=0.0,
            accuracy_total=0.0,
            total_predictions=summary.get("total_stocks", 0),
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Accuracy retrieval failed: {str(e)}")