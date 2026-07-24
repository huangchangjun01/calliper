from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.api.predictions import router as predictions_router
from app.api.features import router as features_router
from app.api.models import router as models_router
from app.models.model_manager import ModelManager
from app.features.feature_pipeline import FeaturePipeline
from app.tasks.prediction_task import PredictionTask
from app.tasks.scheduler import TaskScheduler


def create_app() -> FastAPI:
    app = FastAPI(title="Quant Trading ML Service", version="0.1.0")

    app.add_middleware(
        CORSMiddleware,
        allow_origins=["*"],
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )

    # Initialize core components
    model_manager = ModelManager()
    feature_pipeline = FeaturePipeline()
    prediction_task = PredictionTask(model_manager=model_manager)
    scheduler = TaskScheduler()

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