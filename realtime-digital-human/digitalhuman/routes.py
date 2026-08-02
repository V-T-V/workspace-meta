"""REST/WS 路由注册（从 server.create_app 拆出）。

把 10 个端点定义从 create_app 里抽出来，create_app 只做装配 + 调 register_routes。
"""
from __future__ import annotations

import logging
import os
import time
import uuid
from pathlib import Path

from fastapi import FastAPI, Request, Response, WebSocket
from fastapi.responses import FileResponse, JSONResponse, PlainTextResponse, StreamingResponse
from fastapi.staticfiles import StaticFiles

from .config import Config
from .engines.base import ASREngine, LLMEngine, LipSyncEngine, Message, Pusher, TTSEngine
from .session import SessionRegistry

log = logging.getLogger(__name__)


def register_routes(
    app: FastAPI,
    cfg: Config,
    asr: ASREngine,
    llm: LLMEngine,
    tts: TTSEngine,
    lipsync: LipSyncEngine,
    pusher: Pusher,
    portrait: bytes,
    registry: SessionRegistry,
    store,
    web_dir: str,
):
    """注册所有 REST + WS 路由。"""

    # --- 鉴权 middleware ---
    if cfg.server.auth_token:
        from fastapi.responses import JSONResponse as _JSON

        @app.middleware("http")
        async def auth_middleware(request: Request, call_next):
            public_paths = {"/", "/admin", "/admin.html", "/health", "/docs",
                            "/openapi.json", "/redoc", "/static", "/favicon.ico"}
            path = request.url.path
            if path in public_paths or path.startswith("/static"):
                return await call_next(request)
            token = request.query_params.get("token", "")
            if not token:
                auth = request.headers.get("Authorization", "")
                if auth.startswith("Bearer "):
                    token = auth[7:]
            if token != cfg.server.auth_token:
                log.warning("鉴权失败（401）: %s %s client=%s",
                            request.method, request.url.path,
                            request.client[0] if request.client else "?")
                return _JSON({"error": "token error"}, status_code=401)
            return await call_next(request)

    # --- 静态前端 ---
    if os.path.isdir(web_dir):
        app.mount("/static", StaticFiles(directory=web_dir), name="static")

    @app.get("/")
    async def index():
        idx = Path(web_dir) / "index.html"
        if idx.is_file():
            return FileResponse(str(idx))
        return JSONResponse({"status": "ok", "msg": "web dir not found, API alive"})

    @app.get("/admin")
    @app.get("/admin.html")
    async def admin_page():
        """管理控制台页面。"""
        adm = Path(web_dir) / "admin.html"
        if adm.is_file():
            return FileResponse(str(adm))
        return JSONResponse({"status": "ok", "msg": "admin page not found"}, status_code=404)

    @app.get("/favicon.ico")
    async def favicon():
        ico = Path(web_dir) / "favicon.ico"
        if ico.is_file():
            return FileResponse(str(ico), media_type="image/x-icon")
        # 返回打包内的 ico（避免 404）
        ico2 = Path(__file__).parent / "favicon.ico"
        if ico2.is_file():
            return FileResponse(str(ico2), media_type="image/x-icon")
        # ★ P0 修复：204 响应必须无 body！
        # 之前返回 JSONResponse(status_code=204) 带 18 字节 body，
        # h11 对 204/304 强制 0 body（ContentLengthWriter(0)），发送即抛
        # "Too much data for declared Content-Length" → uvicorn 关闭连接
        # → 同连接后续 /static/style.css、app.js 全部失败 → 页面残缺黑屏。
        return Response(status_code=204)

    @app.get("/health")
    async def health():
        return {"status": "ok", "sessions": len(registry)}

    @app.get("/api/dashboard")
    async def dashboard():
        from .metrics import get_metrics
        m = get_metrics()
        return {
            "sessions": len(registry),
            "max_sessions": cfg.server.max_sessions,
            "engine_status": {
                "asr": type(asr).__name__,
                "llm": type(llm).__name__,
                "tts": type(tts).__name__,
                "lipsync": type(lipsync).__name__,
            },
            "pipeline_total": m._pipeline_total,
            "pipeline_cancel_total": m._pipeline_cancel_total,
            "tts_fail_total": m._tts_fail_total,
            "barge_in_total": m._barge_in_total,
            "latency": {
                "first_token_p50": m._percentile(list(m._first_token_ms), 50),
                "first_token_p95": m._percentile(list(m._first_token_ms), 95),
                "first_frame_p50": m._percentile(list(m._first_frame_ms), 50),
                "first_frame_p95": m._percentile(list(m._first_frame_ms), 95),
            },
            "uptime_seconds": int(time.time() - m._start_time),
        }

    @app.get("/sessions/{session_id}/history")
    async def get_history(session_id: str):
        if store is None:
            return {"error": "persist disabled", "history": []}
        try:
            # ★ H-2：同步 sqlite 查询放 executor，避免大历史/磁盘抖动卡住事件循环
            import asyncio as _aio
            loop = _aio.get_running_loop()
            history = await loop.run_in_executor(None, store.get_session_history, session_id)
            return {"session_id": session_id, "history": history}
        except Exception as e:
            return {"error": str(e), "history": []}

    @app.get("/personas")
    async def list_personas():
        ps = cfg.personas.list if cfg.personas.list else []
        return {"default": cfg.personas.default_id,
                "personas": [{"id": p.id, "name": p.name, "voice": p.voice} for p in ps]}

    @app.get("/metrics")
    async def metrics_endpoint():
        from .metrics import get_metrics
        m = get_metrics()
        m.set_sessions_active(len(registry))
        return PlainTextResponse(m.render_prometheus(),
                                 media_type="text/plain; version=0.0.4")

    @app.post("/v1/chat/completions")
    async def openai_chat_completions(request: dict):
        import json as _json
        messages = request.get("messages", [])
        stream = request.get("stream", False)

        if not messages or not isinstance(messages, list):
            return JSONResponse({"error": {"message": "messages empty"}}, status_code=400)
        if len(messages) > 50:
            return JSONResponse({"error": {"message": "max 50 messages"}}, status_code=400)
        for m in messages:
            if not isinstance(m, dict) or "role" not in m or "content" not in m:
                return JSONResponse({"error": {"message": "need role+content"}}, status_code=400)
            if len(str(m["content"])) > 10000:
                return JSONResponse({"error": {"message": "max 10000 chars"}}, status_code=400)

        history = [Message(role=m["role"], content=m["content"])
                   for m in messages[:-1] if m.get("role") and m.get("content")]
        prompt = messages[-1]["content"] if messages else ""

        if stream:
            async def sse_stream():
                try:
                    async for token in llm.chat_stream(prompt, history):
                        chunk = {"choices": [{"delta": {"content": token}}]}
                        yield f"data: {_json.dumps(chunk)}\n\n"
                    yield "data: [DONE]\n\n"
                except Exception as e:
                    yield f"data: {_json.dumps({'error': {'message': str(e)}})}\n\n"
            return StreamingResponse(sse_stream(), media_type="text/event-stream")
        else:
            tokens = []
            try:
                async for token in llm.chat_stream(prompt, history):
                    tokens.append(token)
            except Exception as e:
                return JSONResponse({"error": {"message": str(e)}}, status_code=500)
            return {
                "id": f"chatcmpl-{os.getpid()}",
                "object": "chat.completion",
                "model": cfg.llm.model,
                "choices": [{"index": 0,
                             "message": {"role": "assistant", "content": "".join(tokens)},
                             "finish_reason": "stop"}],
            }

    @app.get("/v1/models")
    async def openai_list_models():
        return {"object": "list", "data": [{"id": cfg.llm.model, "object": "model"}]}

    @app.websocket("/ws/{session_id}")
    async def ws_endpoint(ws: WebSocket, session_id: str):
        from .ws_connection import WSConnection
        conn = WSConnection(ws, session_id, cfg, asr, llm, tts, lipsync, pusher,
                            portrait, registry, store)
        await conn.handle()

    @app.websocket("/ws")
    async def ws_default(ws: WebSocket):
        await ws_endpoint(ws, f"auto-{uuid.uuid4().hex[:8]}")
