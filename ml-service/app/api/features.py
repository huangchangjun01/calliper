from fastapi import APIRouter, HTTPException, Query, Request
from typing import List, Optional, Dict, Any
from pydantic import BaseModel

router = APIRouter()


# ──────────────────────────────────────────────────────────────
# Schemas
# ──────────────────────────────────────────────────────────────

class FeatureItem(BaseModel):
    name: str
    value: float
    category: str


class FeatureSnapshot(BaseModel):
    symbol: str
    computed_at: str
    features: List[FeatureItem]


class FeatureHistoryItem(BaseModel):
    symbol: str
    computed_at: str
    feature_count: int
    missing_count: int


class ComputeRequest(BaseModel):
    symbols: List[str]


# ──────────────────────────────────────────────────────────────
# Mock helpers
# ──────────────────────────────────────────────────────────────

def _mock_features(symbol: str) -> FeatureSnapshot:
    import datetime
    return FeatureSnapshot(
        symbol=symbol,
        computed_at=datetime.datetime.utcnow().isoformat() + "Z",
        features=[
            FeatureItem(name="rsi", value=55.3, category="technical"),
            FeatureItem(name="macd", value=1.25, category="technical"),
            FeatureItem(name="ma_5", value=178.20, category="technical"),
            FeatureItem(name="ma_20", value=175.80, category="technical"),
            FeatureItem(name="volume_ratio", value=1.35, category="sentiment"),
            FeatureItem(name="money_flow", value=1250000.0, category="sentiment"),
            FeatureItem(name="vwap", value=179.50, category="sentiment"),
            FeatureItem(name="returns", value=0.012, category="price"),
            FeatureItem(name="volatility_20", value=0.018, category="price"),
            FeatureItem(name="pe_ratio", value=22.5, category="fundamental"),
            FeatureItem(name="debt_equity", value=0.65, category="fundamental"),
            FeatureItem(name="revenue_growth", value=0.12, category="fundamental"),
        ],
    )


# ──────────────────────────────────────────────────────────────
# Endpoints
# ──────────────────────────────────────────────────────────────

@router.get("/{symbol}", response_model=FeatureSnapshot)
async def get_features(symbol: str, request: Request):
    """获取最新特征数据"""
    feature_pipeline = request.app.state.feature_pipeline
    try:
        return _mock_features(symbol)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Feature retrieval failed: {str(e)}")


@router.post("/compute")
async def compute_features(request: Request, body: ComputeRequest):
    """计算并存储特征"""
    feature_pipeline = request.app.state.feature_pipeline
    results = {}
    for symbol in body.symbols:
        results[symbol] = _mock_features(symbol)
    return {
        "status": "success",
        "computed": len(body.symbols),
        "results": [r.model_dump() for r in results.values()],
    }


@router.get("/{symbol}/history", response_model=List[FeatureHistoryItem])
async def get_feature_history(
    symbol: str,
    request: Request,
    limit: int = Query(10, ge=1, le=100),
):
    """获取历史特征"""
    import datetime
    history = []
    for i in range(limit):
        day = datetime.datetime.utcnow() - datetime.timedelta(days=i)
        history.append(
            FeatureHistoryItem(
                symbol=symbol,
                computed_at=day.isoformat() + "Z",
                feature_count=28,
                missing_count=i % 3,
            )
        )
    return history[:limit]