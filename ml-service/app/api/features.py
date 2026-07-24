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
# Endpoints
# ──────────────────────────────────────────────────────────────

@router.get("/{symbol}", response_model=FeatureSnapshot)
async def get_features(symbol: str, request: Request):
    """获取最新特征数据，使用真实特征管线"""
    feature_pipeline = request.app.state.feature_pipeline
    try:
        features = feature_pipeline.compute_features(symbol)
        import datetime
        items = []
        for name, value in features.items():
            category = "technical"
            if name.startswith("fund_"):
                category = "fundamental"
            elif name in ("volume_ratio", "money_flow", "vwap"):
                category = "sentiment"
            elif name in ("returns", "volatility"):
                category = "price"
            items.append(FeatureItem(name=name, value=float(value), category=category))
        return FeatureSnapshot(
            symbol=symbol,
            computed_at=datetime.datetime.utcnow().isoformat() + "Z",
            features=items,
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Feature retrieval failed: {str(e)}")


@router.post("/compute")
async def compute_features(request: Request, body: ComputeRequest):
    """计算并存储特征，使用真实特征管线"""
    feature_pipeline = request.app.state.feature_pipeline
    results = {}
    for symbol in body.symbols:
        try:
            features = feature_pipeline.compute_features(symbol)
            import datetime
            items = []
            for name, value in features.items():
                category = "technical"
                if name.startswith("fund_"):
                    category = "fundamental"
                elif name in ("volume_ratio", "money_flow", "vwap"):
                    category = "sentiment"
                elif name in ("returns", "volatility"):
                    category = "price"
                items.append(FeatureItem(name=name, value=float(value), category=category))
            results[symbol] = FeatureSnapshot(
                symbol=symbol,
                computed_at=datetime.datetime.utcnow().isoformat() + "Z",
                features=items,
            ).model_dump()
        except Exception as e:
            print(f"[Features] Error computing features for {symbol}: {e}")

    return {
        "status": "success",
        "computed": len(results),
        "results": list(results.values()),
    }


@router.get("/{symbol}/history", response_model=List[FeatureHistoryItem])
async def get_feature_history(
    symbol: str,
    request: Request,
    limit: int = Query(10, ge=1, le=100),
):
    """获取历史特征，从数据库读取"""
    feature_pipeline = request.app.state.feature_pipeline
    try:
        history = feature_pipeline.get_feature_history(symbol, limit=limit)
        return history
    except Exception:
        return []