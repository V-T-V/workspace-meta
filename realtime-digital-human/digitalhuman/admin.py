"""管理控制台后端端点（admin）。

提供：配置读取/修改（写回 config.yaml）、日志查看、连接测试（LLM/ASR/TTS）、环境自检、优雅重启。

★ 设计：引擎一次性装配常驻，切换靠"改配置+重启"（最稳，单机可接受 5-10s 中断）。
★ 鉴权：复用 cfg.server.auth_token，/api/admin/* 天然受 routes 的 middleware 保护。
★ 安全：api_key 读取时脱敏（只返回是否已设置，不回明文）。
"""
from __future__ import annotations

import asyncio
import logging
import os
import socket
import sys
from collections.abc import Awaitable, Callable
from urllib.parse import urlparse

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse

from .config import Config, save_yaml, validate, from_yaml

log = logging.getLogger(__name__)

# 含敏感信息的字段路径（读取时脱敏）
_SENSITIVE_FIELDS = {("llm", "api_key"), ("asr", "api_key")}


def _sanitize_config(cfg: Config) -> dict:
    """Config → dict，敏感字段脱敏（显示尾号，便于用户确认是哪个 key）。"""
    from .config import to_yaml_dict
    data = to_yaml_dict(cfg)
    for section, key in _SENSITIVE_FIELDS:
        if section in data and key in data[section]:
            val = data[section][key]
            # 优先看环境变量是否设了 key（运行时实际用的是 env）
            import os as _os
            env_key_name = f"DH_{section.upper()}_API_KEY"
            env_val = _os.environ.get(env_key_name, "")
            actual = env_val or val
            if actual:
                # 显示尾 4 位（sk-...abcd），用户能确认是哪个 key
                masked = actual[:3] + "***" + actual[-4:] if len(actual) > 8 else "(已设置)"
                source = "（环境变量）" if env_val and not val else ""
                data[section][key] = f"{masked}{source}"
            else:
                data[section][key] = "(未设置)"
    return data


async def _probe_tcp(host: str, port: int, timeout: float = 3.0) -> bool:
    """异步 TCP 端口探测。"""
    try:
        _, writer = await asyncio.wait_for(
            asyncio.open_connection(host, port), timeout=timeout)
        writer.close()
        try:
            await writer.wait_closed()
        except Exception:
            pass
        return True
    except (OSError, asyncio.TimeoutError):
        return False


def _parse_host_port(base_url: str) -> tuple[str, int]:
    """从 base_url 解析 host:port。"""
    u = urlparse(base_url)
    host = u.hostname or "127.0.0.1"
    port = u.port or (443 if u.scheme == "https" else 80)
    return host, port


async def _test_llm_async(cfg: Config) -> dict:
    """测试 LLM 连接：TCP 探测 + （Ollama）列模型 + （云）验证 key。

    返回 {ok, detail, models?}。
    """
    backend = cfg.llm.backend
    base_url = cfg.llm.base_url
    result = {"backend": backend, "base_url": base_url, "ok": False, "detail": ""}

    if backend in ("mock", "disabled", "none"):
        result["ok"] = True
        result["detail"] = f"{backend} 模式无需连接测试"
        return result

    host, port = _parse_host_port(base_url)
    reachable = await _probe_tcp(host, port)
    if not reachable:
        result["detail"] = f"TCP 不可达 {host}:{port}"
        return result

    # Ollama：列模型验证（复用 selftest 思路）
    if backend == "ollama":
        try:
            import aiohttp
            async with aiohttp.ClientSession(timeout=aiohttp.ClientTimeout(total=5)) as sess:
                async with sess.get(f"{base_url.rstrip('/')}/api/tags") as resp:
                    if resp.status == 200:
                        data = await resp.json()
                        models = [m["name"] for m in data.get("models", [])]
                        result["ok"] = True
                        result["models"] = models[:10]
                        result["detail"] = f"可达，可用模型 {len(models)} 个"
                        model_ok = cfg.llm.model in models
                        if not model_ok:
                            result["detail"] += f"（⚠ 配置的 {cfg.llm.model} 不在列表）"
                    else:
                        result["detail"] = f"/api/tags 返回 {resp.status}"
        except Exception as e:
            result["detail"] = f"Ollama 探测异常: {e}"
        return result

    # 云端 LLM：发最小 chat 验证 key 有效
    api_key = os.environ.get("DH_LLM_API_KEY") or cfg.llm.api_key
    if not api_key:
        result["ok"] = True  # TCP 通了就算，key 缺失由 validate 告警
        result["detail"] = "TCP 可达（⚠ 缺 api_key，运行时会 401）"
        return result
    try:
        import aiohttp
        path = "/chat/completions" if base_url.rstrip("/").endswith("/v1") else "/v1/chat/completions"
        url = base_url.rstrip("/") + path
        payload = {"model": cfg.llm.model, "messages": [{"role": "user", "content": "hi"}],
                   "max_tokens": 5, "stream": False}
        headers = {"Authorization": f"Bearer {api_key}", "Content-Type": "application/json"}
        async with aiohttp.ClientSession(timeout=aiohttp.ClientTimeout(total=10)) as sess:
            async with sess.post(url, json=payload, headers=headers) as resp:
                if resp.status == 200:
                    result["ok"] = True
                    result["detail"] = "连接成功，api_key 有效"
                else:
                    body = await resp.text()
                    result["detail"] = f"HTTP {resp.status}: {body[:100]}"
    except Exception as e:
        result["detail"] = f"云 LLM 探测异常: {e}"
    return result


async def _test_asr_async(cfg: Config) -> dict:
    """测试 ASR：本地测模型文件存在，云端 TCP 探测。"""
    backend = cfg.asr.backend
    result = {"backend": backend, "ok": False, "detail": ""}

    if backend in ("mock", "disabled", "none"):
        result["ok"] = True
        result["detail"] = f"{backend} 模式无需测试"
        return result

    if backend == "whisper":
        # 本地：检查模型目录完整性
        roots = [os.environ.get("DH_PROJECT_ROOT"), os.getcwd(),
                 os.path.dirname(os.getcwd())]
        found = False
        for root in roots:
            if not root:
                continue
            for cand in [os.path.join(root, "models", cfg.asr.model),
                         os.path.join(root, "models", f"whisper-{cfg.asr.model}")]:
                if os.path.isdir(cand) and all(
                    os.path.isfile(os.path.join(cand, f)) for f in
                        ("model.bin", "config.json", "tokenizer.json")):
                    found = True
                    break
            if found:
                break
        result["ok"] = found
        result["detail"] = "本地模型完整" if found else f"未找到 {cfg.asr.model} 完整 snapshot（需 model.bin+config.json+tokenizer.json）"
        return result

    # 云端 ASR：TCP 探测
    from .engines.asr_cloud import ASR_CLOUD_PRESETS
    base_url = cfg.asr.base_url or ASR_CLOUD_PRESETS.get(backend, "")
    if not base_url:
        result["detail"] = f"无法确定 {backend} 的 base_url"
        return result
    host, port = _parse_host_port(base_url)
    reachable = await _probe_tcp(host, port)
    result["ok"] = reachable
    result["detail"] = f"TCP {'可达' if reachable else '不可达'} {host}:{port}"
    return result


async def _test_tts_async(cfg: Config) -> dict:
    """测试 TTS：edge 发一句测试合成。"""
    backend = cfg.tts.backend
    result = {"backend": backend, "ok": False, "detail": ""}

    if backend in ("mock", "disabled", "none"):
        result["ok"] = True
        result["detail"] = f"{backend} 模式无需测试"
        return result

    if backend == "edge":
        try:
            import edge_tts
            communicate = edge_tts.Communicate("测试", cfg.tts.voice, rate=cfg.tts.rate)
            got = False
            async for chunk in communicate.stream():
                if chunk["type"] == "audio" and chunk["data"]:
                    got = True
                    break
            result["ok"] = got
            result["detail"] = "合成成功" if got else "edge-tts 返回空音频"
        except Exception as e:
            result["detail"] = f"edge-tts 异常: {e}"
        return result

    if backend == "cosyvoice":
        script_ok = os.path.isfile(cfg.tts.cosyvoice_script)
        result["ok"] = script_ok
        result["detail"] = "脚本存在" if script_ok else f"脚本缺失: {cfg.tts.cosyvoice_script}"
        return result

    result["detail"] = f"未知 tts backend: {backend}"
    return result


def _list_sessions_sync(store) -> list[dict]:
    """同步查所有会话（消息数 + 最后活跃时间）。"""
    import sqlite3, time as _time
    conn = sqlite3.connect(store.db_path)
    try:
        rows = conn.execute(
            "SELECT s.id, s.persona, s.created_at, s.last_active, "
            "(SELECT COUNT(*) FROM messages m WHERE m.session_id = s.id) as msg_count "
            "FROM sessions s ORDER BY s.last_active DESC LIMIT 100"
        ).fetchall()
        return [{"id": r[0], "persona": r[1], "created_at": r[2],
                 "last_active": r[3], "msg_count": r[4]} for r in rows]
    finally:
        conn.close()


def register_admin_routes(
    app: FastAPI,
    cfg: Config,
    log_path: str,
    cfg_path: str,
    store=None,
) -> None:
    """注册管理控制台端点（/api/admin/*）。"""

    # ---------- 配置读写 ----------

    @app.get("/api/admin/config")
    async def get_config():
        """返回当前配置（敏感字段脱敏）。"""
        return {
            "config": _sanitize_config(cfg),
            "config_path": cfg_path,
            "warnings": validate(cfg),
        }

    @app.post("/api/admin/config")
    async def update_config(request: Request):
        """更新配置（部分字段），写回 config.yaml。

        ★ 不立即生效——需点"应用并重启"重新装配引擎。
        请求体：{"asr": {...}, "llm": {...}, ...}（与 config.yaml 结构一致）
        """
        import dataclasses
        try:
            body = await request.json()
        except Exception as e:
            return JSONResponse({"error": f"请求体非 JSON: {e}"}, status_code=400)

        # merge 到 cfg（按段更新，只改 body 里出现的字段）
        updated = []
        # ★ api_key 特殊处理：脱敏占位符（含 ***）不覆盖原值，避免刷新页面后误清空 key
        _SENSITIVE_VALUES = {"api_key"}
        section_map = {"server": cfg.server, "asr": cfg.asr, "llm": cfg.llm,
                       "tts": cfg.tts, "lipsync": cfg.lipsync, "pusher": cfg.pusher,
                       "sentence_splitter": cfg.sentence_splitter, "queue": cfg.queue}
        for section, sub_cfg in section_map.items():
            if section not in body:
                continue
            valid = {f.name for f in dataclasses.fields(sub_cfg)}
            for k, v in body[section].items():
                if k not in valid:
                    continue
                # api_key 是脱敏占位符（含 *** 或括号说明）→ 不覆盖
                if k in _SENSITIVE_VALUES and isinstance(v, str) and ("***" in v or v.startswith("(")):
                    continue
                setattr(sub_cfg, k, v)
                updated.append(f"{section}.{k}")

        # 校验
        warns = validate(cfg)

        # 写回 config.yaml
        try:
            save_yaml(cfg, cfg_path)
            log.info("管理台更新配置，写回 %s（字段: %s）", cfg_path, updated)
        except Exception as e:
            return JSONResponse({"error": f"写回 config.yaml 失败: {e}",
                                 "updated": updated, "warnings": warns}, status_code=500)

        return {"updated": updated, "warnings": warns,
                "msg": "配置已保存，需重启生效（点'应用并重启'）"}

    # ---------- 日志 ----------

    @app.get("/api/admin/logs")
    async def get_logs(tail: int = 200, level: str = ""):
        """读取日志尾部 N 行，支持 level 过滤（ERROR/WARN/INFO）。"""
        if not log_path or not os.path.isfile(log_path):
            return {"lines": [], "path": log_path, "msg": "日志文件不存在"}
        try:
            with open(log_path, encoding="utf-8", errors="ignore") as f:
                all_lines = f.readlines()[-tail * 3:]  # 多读些以备 level 过滤
            if level:
                level_upper = level.upper()
                all_lines = [l for l in all_lines if f"[{level_upper}]" in l]
            return {"lines": all_lines[-tail:], "path": log_path}
        except Exception as e:
            return JSONResponse({"error": f"读日志失败: {e}"}, status_code=500)

    # ---------- 测试连接 ----------

    # ---------- 会话历史与记忆（★ 可视化查看数字人记住了什么）----------

    @app.get("/api/admin/sessions")
    async def list_sessions():
        """列出所有会话（含消息数、最后活跃时间）。"""
        if store is None:
            return {"sessions": [], "msg": "持久化未启用"}
        try:
            loop = __import__("asyncio").get_event_loop()
            rows = await loop.run_in_executor(None, _list_sessions_sync, store)
            return {"sessions": rows}
        except Exception as e:
            return JSONResponse({"error": str(e)}, status_code=500)

    @app.get("/api/admin/sessions/{session_id}/memory")
    async def get_session_memory(session_id: str, limit: int = 50):
        """查看某会话的完整历史 + 提取出的摘要（长期记忆）。

        返回 {history: [...], summaries: [...]} —— 让用户看到数字人记住了什么。
        """
        if store is None:
            return {"history": [], "summaries": [], "msg": "持久化未启用"}
        try:
            loop = __import__("asyncio").get_event_loop()
            history = await loop.run_in_executor(
                None, store.get_session_history, session_id, limit)
            # 从历史里提取摘要消息（长期记忆）
            summaries = [m for m in history
                         if m.get("role") == "system" and "对话摘要" in m.get("content", "")]
            return {"session_id": session_id,
                    "history": history,
                    "summaries": summaries,
                    "total": len(history)}
        except Exception as e:
            return JSONResponse({"error": str(e)}, status_code=500)

    @app.delete("/api/admin/sessions/{session_id}")
    async def clear_session(session_id: str):
        """清空某会话（遗忘）。"""
        if store is None:
            return {"msg": "持久化未启用，无需清理"}
        try:
            loop = __import__("asyncio").get_event_loop()
            await loop.run_in_executor(None, store.clear_session, session_id)
            log.info("管理台清空会话: %s", session_id)
            return {"msg": f"会话 {session_id} 已清空"}
        except Exception as e:
            return JSONResponse({"error": str(e)}, status_code=500)

    # ---------- 原有测试连接端点 ----------

    @app.post("/api/admin/test/llm")
    async def test_llm():
        return await _test_llm_async(cfg)

    @app.post("/api/admin/test/asr")
    async def test_asr():
        return await _test_asr_async(cfg)

    @app.post("/api/admin/test/tts")
    async def test_tts():
        return await _test_tts_async(cfg)

    @app.get("/api/admin/selftest")
    async def selftest():
        """环境自检（异步包装 selftest.run_self_test 的核心检查）。"""
        checks = []

        # GPU/CUDA
        try:
            import ctranslate2
            n = ctranslate2.get_cuda_device_count()
            checks.append({"name": "CUDA GPU", "ok": n > 0,
                           "detail": f"检测到 {n} 个 CUDA 设备" if n > 0 else "未检测到 CUDA（CPU 模式）"})
        except Exception:
            checks.append({"name": "CUDA GPU", "ok": False, "detail": "ctranslate2 未安装"})

        # 端口占用
        s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        try:
            s.bind((cfg.server.host, cfg.server.port))
            checks.append({"name": f"端口 {cfg.server.port}", "ok": True, "detail": "可用"})
        except OSError:
            checks.append({"name": f"端口 {cfg.server.port}", "ok": False, "detail": "被占用（当前服务运行中属正常）"})
        finally:
            s.close()

        # 核心依赖
        for mod in ["fastapi", "uvicorn", "aiohttp", "yaml", "websockets"]:
            try:
                __import__(mod)
                checks.append({"name": f"依赖 {mod}", "ok": True, "detail": ""})
            except ImportError:
                checks.append({"name": f"依赖 {mod}", "ok": False, "detail": "未安装"})

        passed = sum(1 for c in checks if c["ok"])
        return {"total": len(checks), "passed": passed, "checks": checks}

    # ---------- 重启 ----------

    @app.post("/api/admin/restart")
    async def restart():
        """优雅重启：让当前进程退出，配合外部 supervisor/Start.bat 重启。

        ★ 单机部署靠 Start.bat（有 pause）或 nssm 等进程守护重启。
        """
        log.warning("管理台触发重启，进程即将退出（等待外部重启）...")

        async def _delayed_exit():
            await asyncio.sleep(1.0)  # 让响应先返回
            # 先触发 lifespan 清理（fastapi 的 shutdown 事件）
            os._exit(0)

        asyncio.create_task(_delayed_exit())
        return {"msg": "正在重启，5 秒后刷新页面"}
