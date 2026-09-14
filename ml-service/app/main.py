from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
import os

from dotenv import load_dotenv

# 加载 .env（必须在读取环境变量的模块导入之前执行）
load_dotenv()

from app.api.predictions import router as predictions_router
from app.api.features import router as features_router
from app.api.models import router as models_router
from app.models.model_manager import ModelManager
from app.features.feature_pipeline import FeaturePipeline
from app.tasks.prediction_task import PredictionTask
from app.tasks.scheduler import TaskScheduler


def create_app() -> FastAPI:
    app = FastAPI(title="Quant Trading ML Service", version="0.1.0")

    # CORS：仅允许配置的来源（默认本地前端），不使用通配符与凭据组合
    cors_origins = [
        o.strip()
        for o in os.getenv("ML_CORS_ORIGINS", "http://localhost:5173").split(",")
        if o.strip()
    ]
    app.add_middleware(
        CORSMiddleware,
        allow_origins=cors_origins,
        allow_methods=["*"],
        allow_headers=["*"],
    )

    # Initialize core components
    model_manager = ModelManager()
    feature_pipeline = FeaturePipeline()
    prediction_task = PredictionTask(model_manager=model_manager)
    scheduler = TaskScheduler()

    # 注册每日收盘后预测任务（15:30 Asia/Shanghai）
    scheduler.schedule_daily_prediction(
        lambda: prediction_task.run_daily_prediction(),
        hour=15,
        minute=30,
    )
    print("[Main] Registered daily prediction job (15:30 Asia/Shanghai)")

    # 注册盘前预测任务（8:30 Asia/Shanghai）：开盘前生成当日最新预测，
    # 供盘中模拟交易使用（交易日 9:30 开盘即按最新信号交易）
    scheduler.schedule_pre_market_prediction(
        lambda: prediction_task.run_daily_prediction(),
    )
    print("[Main] Registered pre-market prediction job (08:30 Asia/Shanghai)")

    # 注册每周全量重训练任务（真实数据管线，默认周六 02:00，兜底长期模型与全量刷新）
    scheduler.schedule_weekly_training(
        lambda: prediction_task.run_weekly_training(),
    )
    print("[Main] Registered weekly full training job")

    # 注册每日轻量重训练任务（默认 17:00，收盘后重训 short/medium 快速跟进最新行情；
    # 与 15:30 预测、16:00 评估错峰，long_term 由周六全量训练兜底）
    scheduler.schedule_daily_training(
        lambda: prediction_task.run_daily_light_training(),
    )
    print("[Main] Registered daily light training job")

    # 注册每日模型评估任务（默认 16:00）
    scheduler.schedule_model_evaluation(
        lambda: prediction_task.run_model_evaluation(),
    )
    print("[Main] Registered daily model evaluation job")

    # Attach to app state for route access
    app.state.model_manager = model_manager
    app.state.feature_pipeline = feature_pipeline
    app.state.prediction_task = prediction_task
    app.state.scheduler = scheduler

    # Register routers
    app.include_router(predictions_router, prefix="/api/v1/predictions", tags=["predictions"])
    app.include_router(features_router, prefix="/api/v1/features", tags=["features"])
    app.include_router(models_router, prefix="/api/v1/models", tags=["models"])

    @app.get("/health")
    async def health_check():
        return {"status": "ok", "service": "ml-service"}

    @app.on_event("startup")
    async def startup():
        model_manager.load_all()
        scheduler.start()

    @app.on_event("shutdown")
    async def shutdown():
        scheduler.stop()

    return app


app = create_app()

if __name__ == "__main__":
    import uvicorn
    uvicorn.run("app.main:app", host="0.0.0.0", port=8000, reload=True)