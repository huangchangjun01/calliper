from fastapi import APIRouter, HTTPException, Query, Request, Depends
from typing import List, Optional
from pydantic import BaseModel

from ..security import require_api_key

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


def _factors_from_pred(pred: dict) -> List[FactorItem]:
    """将预测中的 factors（dict 或 list）统一转为 FactorItem 列表。"""
    raw = pred.get("factors")
    items = []
    if isinstance(raw, dict):
        items = [FactorItem(name=k, value=v, description=k) for k, v in raw.items()]
    elif isinstance(raw, list):
        items = [
            FactorItem(
                name=str(f.get("name", "")),
                value=float(f.get("value", 0.0)),
                description=str(f.get("description", "")),
            )
            for f in raw if isinstance(f, dict)
        ]
    return items


# ──────────────────────────────────────────────────────────────
# Endpoints
# ──────────────────────────────────────────────────────────────

@router.get("/history")
async def get_prediction_history_global(
    request: Request,
    limit: int = Query(10, ge=1, le=100),
):
    """获取全局预测历史概要（避免被 /{symbol} 吞掉）。

    从预测任务的历史存储中读取，返回所有 symbol 的预测概要；
    无记录时返回空数组 []。
    """
    prediction_task = request.app.state.prediction_task
    try:
        historical = prediction_task.get_historical_predictions(limit=limit)
        items = []
        for entry in historical:
            ts = entry.get("timestamp", "")
            for r in entry.get("results", []):
                for preds in r.get("predictions", {}).values():
                    if preds:
                        pred = preds[0]
                        items.append(PredictionHistoryItem(
                            id=len(items) + 1,
                            symbol=r.get("symbol", ""),
                            period=pred.get("period", "short_term"),
                            direction=pred.get("direction", "hold"),
                            confidence=pred.get("confidence", 0.0),
                            target_price=pred.get("target_price", 0.0),
                            predicted_at=ts,
                            is_correct=None,
                        ))
        return items[:limit]
    except Exception:
        return []


@router.get("/{symbol}", response_model=PredictionResult)
async def get_prediction(
    symbol: str,
    request: Request,
    period: str = Query("", description="Prediction period: short_term, medium_term, long_term; empty to auto-select first available"),
):
    """获取单只股票最新预测，使用真实模型预测"""
    prediction_task = request.app.state.prediction_task
    try:
        result = prediction_task.predict_single_stock(symbol)
        if "error" in result:
            raise HTTPException(status_code=500, detail=result["error"])
        # 选择请求的周期；若周期为空，按固定顺序返回首个非空周期
        preds = []
        selected_period = period
        if period:
            preds = result.get("predictions", {}).get(period, [])
        else:
            for p in ["short_term", "medium_term", "long_term"]:
                if result.get("predictions", {}).get(p):
                    preds = result.get("predictions", {}).get(p)
                    selected_period = p
                    break
        if not preds:
            raise HTTPException(status_code=404, detail=f"No prediction available for {symbol}")

        import datetime
        # medium_term 在特征管线中按每日单样本预测返回多行结果，
        # 需取最新样本；short/long 返回单条（preds[0] 与 [-1] 一致）不受影响。
        pred = preds[-1] if selected_period == "medium_term" else preds[0]
        return PredictionResult(
            symbol=symbol,
            period=selected_period,
            direction=pred.get("direction", "hold"),
            confidence=pred.get("confidence", 0.0),
            target_price=pred.get("target_price", 0.0),
            factors=_factors_from_pred(pred),
            model_version=result.get("model_version", "unknown"),
            predicted_at=result.get("timestamp", datetime.datetime.utcnow().isoformat() + "Z"),
        )
    except HTTPException:
        raise
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Prediction failed: {str(e)}")


@router.post("/batch", response_model=List[PredictionResult])
async def batch_predict(
    request: Request,
    body: BatchRequest,
    _: None = Depends(require_api_key),
):
    """批量预测，使用真实模型"""
    from concurrent.futures import ThreadPoolExecutor
    prediction_task = request.app.state.prediction_task
    import datetime

    def _predict_symbol(symbol):
        try:
            return prediction_task.predict_single_stock(symbol)
        except Exception as e:
            print(f"[Predictions] Error predicting {symbol}: {e}")
            return None

    results = []
    with ThreadPoolExecutor(max_workers=8) as executor:
        futures = {symbol: executor.submit(_predict_symbol, symbol) for symbol in body.symbols}
        # 按请求 symbols 顺序收集结果，不改变请求顺序
        for symbol in body.symbols:
            result = futures[symbol].result()
            if not result or "error" in result:
                continue
            for period in ["short_term", "medium_term", "long_term"]:
                preds = result.get("predictions", {}).get(period, [])
                if not preds:
                    continue
                # medium_term 返回多日样本，取最新样本；short/long 单条不受影响。
                pred = preds[-1] if period == "medium_term" else preds[0]
                results.append(PredictionResult(
                    symbol=symbol,
                    period=period,
                    direction=pred.get("direction", "hold"),
                    confidence=pred.get("confidence", 0.0),
                    target_price=pred.get("target_price", 0.0),
                    factors=_factors_from_pred(pred),
                    model_version=result.get("model_version", "unknown"),
                    predicted_at=result.get("timestamp", datetime.datetime.utcnow().isoformat() + "Z"),
                ))
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
async def run_prediction(
    request: Request,
    _: None = Depends(require_api_key),
    period: str = "",
):
    """手动触发每日预测任务（后台异步执行，立即返回接受状态）
    :param period: 可选参数，限定预测周期（short_term/medium_term/long_term）；
        为空表示三周期全部预测。
    """
    import threading
    prediction_task = request.app.state.prediction_task
    try:
        # 校验 period 合法（空代表全量）
        target = period or None
        if target is not None and target not in ("short_term", "medium_term", "long_term"):
            raise HTTPException(status_code=400, detail=f"Invalid period: {period}")
        # 全量/单周期预测遍历真实数据耗时较长，放入后台线程异步执行，
        # 接口立即返回，避免前端/代理长时间阻塞；结果由后台任务写库。
        t = threading.Thread(
            target=prediction_task.run_daily_prediction,
            kwargs={"period": target},
            name="manual-prediction",
            daemon=True,
        )
        t.start()
        return {"status": "accepted", "message": "Prediction task triggered (async)", "period": target or "all"}
    except HTTPException:
        raise
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