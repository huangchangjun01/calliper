"""ML 服务鉴权助手。

从环境变量 ML_API_KEY 读取共享密钥（默认空）。
- 未配置 ML_API_KEY 时不强制鉴权（保持向后兼容）。
- 配置后，变更/训练/预测等写接口必须携带请求头 X-ML-API-Key 且与配置值一致，
  否则返回 401。
"""
import os
import secrets

from fastapi import HTTPException, Request

ML_API_KEY = os.getenv("ML_API_KEY", "").strip()


def require_api_key(request: Request):
    """FastAPI 依赖：校验 X-ML-API-Key 请求头。返回 None（未配置时不校验）。"""
    if not ML_API_KEY:
        return None

    provided = request.headers.get("X-ML-API-Key", "")
    if not provided or not secrets.compare_digest(provided, ML_API_KEY):
        raise HTTPException(
            status_code=401,
            detail="Invalid or missing API key",
            headers={"WWW-Authenticate": 'ApiKey realm="ml-service"'},
        )
    return None