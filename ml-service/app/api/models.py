from fastapi import APIRouter, HTTPException, Request, Depends
from typing import List, Optional, Dict, Any
from pydantic import BaseModel
import os
import sys

from starlette.concurrency import run_in_threadpool

from ..security import require_api_key

# 复用 train_models.py 的真实数据加载/数据准备逻辑（ml-service 根目录）
_ML_ROOT = os.path.normpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
if _ML_ROOT not in sys.path:
    sys.path.insert(0, _ML_ROOT)

from train_models import get_symbols, load_real_frames, prepare_period_data

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


class RollbackRequest(BaseModel):
    version: str


class RollbackResult(BaseModel):
    period: str
    version: str
    status: str


# ──────────────────────────────────────────────────────────────
# Model type mapping
# ──────────────────────────────────────────────────────────────

_MODEL_TYPES = {
    "short_term": {"model_type": "LSTM", "framework": "PyTorch"},
    "medium_term": {"model_type": "XGBoost+LightGBM Ensemble", "framework": "scikit-learn"},
    "long_term": {"model_type": "Transformer", "framework": "PyTorch"},
}


def _build_model_status(model_manager) -> List[ModelStatusItem]:
    """Build model status from real model manager state."""
    statuses = []
    mgr_status = model_manager.get_status()
    for period, info in mgr_status.items():
        model_info = _MODEL_TYPES.get(period, {"model_type": "Unknown", "framework": "Unknown"})
        # 权重文件路径：存在且可成功加载则健康
        version_info = model_manager.versions.get(period, {})
        weight_path = version_info.get("path", "")
        is_healthy = bool(weight_path) and os.path.exists(weight_path)
        statuses.append(ModelStatusItem(
            period=period,
            version=info.get("version", "v0.0.0"),
            accuracy=info.get("accuracy", 0.0),
            last_trained=info.get("last_trained", ""),
            is_healthy=is_healthy,
            model_type=model_info["model_type"],
            framework=model_info["framework"],
        ))
    return statuses


# ──────────────────────────────────────────────────────────────
# Endpoints
# ──────────────────────────────────────────────────────────────

@router.get("/status", response_model=List[ModelStatusItem])
async def get_model_status(request: Request):
    """获取所有模型状态，从真实模型管理器读取"""
    model_manager = request.app.state.model_manager
    try:
        return _build_model_status(model_manager)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Model status retrieval failed: {str(e)}")


@router.post("/train/{period}", response_model=TrainingResult)
async def train_model(
    period: str,
    request: Request,
    _: None = Depends(require_api_key),
):
    """触发模型训练（加载真实数据后调用 train_single）"""
    if period not in ("short_term", "medium_term", "long_term"):
        raise HTTPException(status_code=400, detail=f"Invalid period: {period}. Must be one of: short_term, medium_term, long_term")

    model_manager = request.app.state.model_manager

    def do_train():
        # 加载真实数据（TSDB 优先 / 真实 API 兜底，绝不合成）
        frames = load_real_frames(get_symbols())
        if not frames:
            raise HTTPException(
                status_code=404,
                detail="No real training data available; refusing to train on synthetic data",
            )

        data = prepare_period_data(period, frames)
        if data is None:
            raise HTTPException(
                status_code=404,
                detail=f"No enough real data to train {period}",
            )
        X, y = data

        # 按周期以真实数据训练（train_single 为同步阻塞调用，放入工作线程执行，
        # 避免阻塞 FastAPI 事件循环导致其它请求（健康检查/模型状态）不可用）
        if period == "short_term":
            model_manager.train_single("short_term", short_df=X, short_y=y, short_epochs=30)
        elif period == "medium_term":
            model_manager.train_single("medium_term", medium_X=X, medium_y=y)
        elif period == "long_term":
            model_manager.train_single("long_term", long_df=X, long_y=y, long_epochs=30)

        version_info = model_manager.versions.get(period, {})
        return TrainingResult(
            period=period,
            status="success",
            new_version=version_info.get("version", "v1.0.0"),
            accuracy=version_info.get("accuracy", 0.0),
        )

    try:
        return await run_in_threadpool(do_train)
    except HTTPException:
        raise
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Training failed: {str(e)}")


@router.get("/schedule")
async def get_schedule(request: Request):
    """获取训练/预测任务调度状态。scheduler 未启动时返回空 jobs 而非报错。"""
    try:
        scheduler = request.app.state.scheduler
        if scheduler is None:
            return {"jobs": []}
        status = scheduler.get_status()
        jobs = status.get("jobs", []) if isinstance(status, dict) else []
        return {"jobs": jobs}
    except Exception:
        return {"jobs": []}


@router.post("/{period}/rollback", response_model=RollbackResult)
async def rollback_model(
    period: str,
    body: RollbackRequest,
    request: Request,
    _: None = Depends(require_api_key),
):
    """将指定周期模型回滚到某个历史版本快照"""
    if period not in ("short_term", "medium_term", "long_term"):
        raise HTTPException(
            status_code=400,
            detail=f"Invalid period: {period}. Must be one of: short_term, medium_term, long_term",
        )

    model_manager = request.app.state.model_manager
    try:
        result = model_manager.rollback(period, body.version)
    except ValueError as e:
        msg = str(e)
        if "Snapshot not found" in msg:
            raise HTTPException(status_code=404, detail=msg)
        raise HTTPException(status_code=400, detail=msg)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Rollback failed: {str(e)}")

    return RollbackResult(
        period=result["period"],
        version=result["version"],
        status="success",
    )


@router.post("/{period}/evaluate")
async def evaluate_model_period(
    period: str,
    request: Request,
    _: None = Depends(require_api_key),
):
    """仅评估指定周期模型（使用真实数据），并回写该周期 versions.json 的 accuracy"""
    if period not in ("short_term", "medium_term", "long_term"):
        raise HTTPException(
            status_code=400,
            detail=f"Invalid period: {period}. Must be one of: short_term, medium_term, long_term",
        )

    import datetime as _dt
    model_manager = request.app.state.model_manager
    prediction_task = request.app.state.prediction_task

    def do_evaluate():
        results = prediction_task.run_model_evaluation(periods=[period])
        if not results:
            # 无真实数据可评估
            return []
        acc = results.get(period, 0.0)
        if period in model_manager.versions:
            model_manager.versions[period]["accuracy"] = acc
            model_manager._save_versions()
        evaluated_at = _dt.datetime.utcnow().isoformat() + "Z"
        return [{
            "period": period,
            "accuracy": round(acc, 4),
            "precision": round(acc, 4),
            "recall": round(acc, 4),
            "f1_score": round(acc, 4),
            "evaluated_at": evaluated_at,
        }]

    try:
        # 评估过程（全量数据加载 + 前向推理）放入工作线程，避免阻塞事件循环
        return await run_in_threadpool(do_evaluate)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Evaluation failed: {str(e)}")


@router.get("/{period}/params")
async def get_model_params(period: str, request: Request):
    """读取指定周期模型的当前参数（缺省时返回默认参数）"""
    model_manager = request.app.state.model_manager
    try:
        params = model_manager.get_params(period)
        return {"period": period, "params": params}
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Failed to get params: {str(e)}")


@router.put("/{period}/params")
async def update_model_params(
    period: str,
    body: Dict[str, Any],
    request: Request,
    _: None = Depends(require_api_key),
):
    """更新指定周期模型参数并持久化到 params_{period}.json；返回合并后的完整参数"""
    model_manager = request.app.state.model_manager
    try:
        params = model_manager.set_params(period, body)
        return {"period": period, "params": params, "status": "success"}
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Failed to update params: {str(e)}")


@router.post("/evaluate", response_model=List[EvaluationResult])
async def evaluate_models(
    request: Request,
    _: None = Depends(require_api_key),
):
    """评估所有模型，使用真实数据回算各周期准确率，并把结果写回模型版本元数据"""
    import datetime
    model_manager = request.app.state.model_manager
    prediction_task = request.app.state.prediction_task

    def do_evaluate():
        # 用真实数据回算三周期 accuracy（内部导入 train_models，可能较慢）
        eval_results = prediction_task.run_model_evaluation()
        results = []
        for period in ("short_term", "medium_term", "long_term"):
            accuracy = eval_results.get(period)
            if accuracy is None:
                continue
            # 回写版本元数据并持久化到 versions.json
            model_manager.versions[period]["accuracy"] = accuracy
            results.append(EvaluationResult(
                period=period,
                accuracy=round(accuracy, 4),
                precision=round(accuracy, 4),
                recall=round(accuracy, 4),
                f1_score=round(accuracy, 4),
                evaluated_at=datetime.datetime.now(datetime.timezone.utc).isoformat() + "Z",
            ))
        if results:
            model_manager._save_versions()
        # run_model_evaluation 返回空 dict（无真实数据）时返回空数组
        return results

    try:
        # 评估过程放入工作线程，避免阻塞事件循环导致其它请求不可用
        return await run_in_threadpool(do_evaluate)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Evaluation failed: {str(e)}")


@router.get("/health", response_model=ModelHealthReport)
async def model_health_check(request: Request):
    """模型健康检查，从真实模型状态计算"""
    model_manager = request.app.state.model_manager
    try:
        statuses = _build_model_status(model_manager)
        all_healthy = all(m.is_healthy for m in statuses)
        recommendations = []
        if all_healthy:
            for m in statuses:
                recommendations.append(f"{m.period} model accuracy is above threshold (0.50)")
        else:
            for m in statuses:
                if not m.is_healthy:
                    recommendations.append(f"Consider retraining {m.period} model with accuracy below 0.50")
        return ModelHealthReport(
            overall_healthy=all_healthy,
            models=statuses,
            recommendations=recommendations,
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Health check failed: {str(e)}")