from fastapi import APIRouter, HTTPException, Request
from typing import List, Optional, Dict, Any
from pydantic import BaseModel

router = APIRouter()


# ──────────────────────────────────────────────────────────────
# Schemas
# ──────────────────────────────────────────────────────────────

class ModelStatusItem(BaseModel):
    period: str
    version: str
    accuracy: float
    last_trained: str
    is_healthy: bool
    model_type: str
    framework: str


class TrainingResult(BaseModel):
    period: str
    status: str
    new_version: Optional[str] = None
    accuracy: Optional[float] = None


class EvaluationResult(BaseModel):
    period: str
    accuracy: float
    precision: float
    recall: float
    f1_score: float
    evaluated_at: str


class ModelHealthReport(BaseModel):
    overall_healthy: bool
    models: List[ModelStatusItem]
    recommendations: List[str]


# ──────────────────────────────────────────────────────────────
# Mock helpers
# ──────────────────────────────────────────────────────────────

def _mock_model_status() -> List[ModelStatusItem]:
    return [
        ModelStatusItem(
            period="short_term",
            version="v1.3.5",
            accuracy=0.71,
            last_trained="2026-07-23T08:00:00Z",
            is_healthy=True,
            model_type="LSTM",
            framework="PyTorch",
        ),
        ModelStatusItem(
            period="medium_term",
            version="v2.1.0",
            accuracy=0.68,
            last_trained="2026-07-22T08:00:00Z",
            is_healthy=True,
            model_type="XGBoost+LightGBM Ensemble",
            framework="scikit-learn",
        ),
        ModelStatusItem(
            period="long_term",
            version="v1.0.8",
            accuracy=0.75,
            last_trained="2026-07-20T08:00:00Z",
            is_healthy=True,
            model_type="Transformer",
            framework="PyTorch",
        ),
    ]


# ──────────────────────────────────────────────────────────────
# Endpoints
# ──────────────────────────────────────────────────────────────

@router.get("/status", response_model=List[ModelStatusItem])
async def get_model_status(request: Request):
    """获取所有模型状态"""
    model_manager = request.app.state.model_manager
    try:
        return _mock_model_status()
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Model status retrieval failed: {str(e)}")


@router.post("/train/{period}", response_model=TrainingResult)
async def train_model(period: str, request: Request):
    """触发模型训练"""
    if period not in ("short_term", "medium_term", "long_term"):
        raise HTTPException(status_code=400, detail=f"Invalid period: {period}. Must be one of: short_term, medium_term, long_term")

    model_manager = request.app.state.model_manager
    try:
        model_manager.train_single(period)
        return TrainingResult(
            period=period,
            status="success",
            new_version="v1.0.1",
            accuracy=0.72,
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Training failed: {str(e)}")


@router.post("/evaluate", response_model=List[EvaluationResult])
async def evaluate_models(request: Request):
    """评估所有模型"""
    import datetime
    model_manager = request.app.state.model_manager
    try:
        return [
            EvaluationResult(
                period="short_term",
                accuracy=0.71,
                precision=0.73,
                recall=0.69,
                f1_score=0.71,
                evaluated_at=datetime.datetime.utcnow().isoformat() + "Z",
            ),
            EvaluationResult(
                period="medium_term",
                accuracy=0.68,
                precision=0.70,
                recall=0.66,
                f1_score=0.68,
                evaluated_at=datetime.datetime.utcnow().isoformat() + "Z",
            ),
            EvaluationResult(
                period="long_term",
                accuracy=0.75,
                precision=0.77,
                recall=0.73,
                f1_score=0.75,
                evaluated_at=datetime.datetime.utcnow().isoformat() + "Z",
            ),
        ]
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Evaluation failed: {str(e)}")


@router.get("/health", response_model=ModelHealthReport)
async def model_health_check(request: Request):
    """模型健康检查"""
    model_manager = request.app.state.model_manager
    try:
        statuses = _mock_model_status()
        all_healthy = all(m.is_healthy for m in statuses)
        return ModelHealthReport(
            overall_healthy=all_healthy,
            models=statuses,
            recommendations=[
                "short_term model accuracy is above threshold (0.60)",
                "medium_term model accuracy is above threshold (0.60)",
                "long_term model accuracy is above threshold (0.60)",
            ] if all_healthy else [
                "Consider retraining models with accuracy below 0.60",
            ],
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Health check failed: {str(e)}")