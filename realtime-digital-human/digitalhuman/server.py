"""FastAPI 服务 + WebSocket 端点。

WS 协议（二进制消息，首字节类型，见 frames.py）：
    浏览器 → 服务端：
        0x02 audio   - PCM 音频片段
        0x10 utterance_end - 用户说完一句，触发 pipeline
    服务端 → 浏览器：
        0x01 frame   - JPEG 视频帧
        0x02 audio   - 音频片段（MP3 from edge-tts）
        0x03 text    - 字幕（[user] / [assistant]）
        0x04 end     - 一段回复结束
        0x05 latency - 延迟统计 JSON

启动：python -m digitalhuman.server -c config.yaml
"""
from __future__ import annotations

import argparse
import logging
import os
import time
import sys
import uuid
from contextlib import asynccontextmanager
from pathlib import Path

from fastapi import FastAPI, WebSocket
from fastapi.responses import FileResponse, JSONResponse, PlainTextResponse, StreamingResponse
from fastapi.staticfiles import StaticFiles

from .config import Config, from_yaml, validate
from .engines.llm_ollama import build_llm
from .engines.mock import MockASR, MockLLM, MockLipSync
from .pusher.base import build_pusher
from .session import SessionRegistry


def _resource_root() -> str:
    """只读资源基准目录：web/、scripts/、assets/（PyInstaller 打包后数据在 _internal）。

    PyInstaller onedir 模式：打包的数据文件在 exe 同级的 `_internal/` 目录，
    运行时 sys._MEIPASS 指向该目录。开发模式用 cwd。
    """
    if getattr(sys, "frozen", False):
        # PyInstaller 打包：_MEIPASS 是打包数据的根目录
        return getattr(sys, "_MEIPASS", os.path.dirname(sys.executable))
    return os.getcwd()


def _config_root() -> str:
    """用户配置基准目录：config*.yaml（让用户能编辑，放 exe 同级而非 _internal）。

    打包后用户需要能修改 config，所以 config 查找用 exe 同级目录（可写）。
    开发模式用 cwd。
    """
    if getattr(sys, "frozen", False):
        return os.path.dirname(sys.executable)
    return os.getcwd()
# 显式 logger 名：python -m digitalhuman.server 时 __name__ 会变成 "__main__"，丢失模块名
log = logging.getLogger("digitalhuman.server")


def _build_engines(cfg: Config):
    """根据 config 装配四环节引擎。未就绪的环节降级为 Mock。"""
    import threading

    # ASR（在子线程里加载，超时则降级 Mock——避免 huggingface 网络卡死阻塞启动）
    if cfg.asr.backend == "whisper":
        asr_result: list = [None]  # [asr_engine or None]
        asr_error: list = [None]

        def _load_asr():
            try:
                from .engines.asr_whisper import build_asr
                asr_result[0] = build_asr(
                    model=cfg.asr.model,
                    language=cfg.asr.language,
                    device=cfg.asr.device,
                    compute_type=cfg.asr.compute_type or None,
                    silence_ms=cfg.asr.silence_ms,
                )
            except Exception as e:
                asr_error[0] = e

        t = threading.Thread(target=_load_asr, daemon=True)
        t.start()
        t0_asr = time.monotonic()
        t.join(timeout=30)  # 最多等 30 秒
        if t.is_alive() or asr_result[0] is None:
            err = asr_error[0] or "ASR 加载超时（30s），可能网络不可达或模型下载中"
            log.warning("ASR whisper 不可用，降级 Mock（%.1fs）: %s",
                        time.monotonic() - t0_asr, err)
            if asr_error[0]:
                log.debug("ASR 加载失败详情", exc_info=asr_error[0])
            asr = MockASR(["（ASR 未就绪，请检查 faster-whisper 安装或网络）"])
        else:
            asr = asr_result[0]
            log.info("ASR 引擎就绪（%.1fs）: %s", time.monotonic() - t0_asr,
                     type(asr).__name__)
    elif cfg.asr.backend in ("cloud", "openai", "groq", "deepinfra", "siliconflow"):
        # ★ 云端 ASR：把语音识别上云，省 ~1GB 本地显存给 MuseTalk 唇形
        try:
            from .engines.asr_cloud import build_asr_cloud
            asr = build_asr_cloud(
                cfg.asr.backend,
                base_url=cfg.asr.base_url,
                model=cfg.asr.model or "whisper-large-v3",
                language=cfg.asr.language,
                api_key=cfg.asr.api_key,
                timeout=cfg.asr.timeout,
            )
            log.info("ASR 云端引擎就绪: %s（base_url=%s, model=%s）",
                     type(asr).__name__, getattr(asr, "base_url", ""),
                     getattr(asr, "model", ""))
        except Exception as e:
            log.warning("ASR 云端引擎装配失败，降级 Mock: %s", e)
            asr = MockASR(["（云端 ASR 未就绪）"])
    elif cfg.asr.backend == "mock":
        # 显式 mock：纯演示/无 GPU 无网络，预置固定回复（与 disabled 的空列表区分）
        asr = MockASR(["（这是 ASR mock 模式的预置回复）"])
        log.info("ASR 使用 mock 模式（预置回复，不依赖任何模型/网络）")
    elif cfg.asr.backend in ("disabled", "none"):
        asr = MockASR([])
        log.info("ASR 已禁用（MockASR 空列表）")
    else:
        raise ValueError(f"未知 asr backend: {cfg.asr.backend}")

    # LLM
    from .engines.llm_ollama import CLOUD_PRESETS
    needs_http = cfg.llm.backend in ("ollama", "gpu_mesh", "openai") or cfg.llm.backend in CLOUD_PRESETS
    if needs_http:
        llm = build_llm(
            cfg.llm.backend,
            base_url=cfg.llm.base_url,
            model=cfg.llm.model,
            system_prompt=cfg.llm.system_prompt,
            temperature=cfg.llm.temperature,
            max_tokens=cfg.llm.max_tokens,
            num_ctx=cfg.llm.num_ctx,
            api_key=cfg.llm.api_key,
        )
        # 可达性探测：不可达则降级 MockLLM（避免 pipeline 网络异常）
        if not _probe_llm(llm, cfg.llm.backend):
            log.warning("LLM %s 不可达（%s），降级 Mock", cfg.llm.backend, getattr(llm, "base_url", ""))
            llm = MockLLM("[LLM 未就绪，请检查 Ollama/云 API 配置]")
    elif cfg.llm.backend == "mock":
        # 显式 mock：纯演示，预置固定回复（不依赖 Ollama/云 API/网络）
        llm = MockLLM("这是 mock 模式的预置回复。我不依赖任何模型，适合纯演示或无网络环境。")
        log.info("LLM 使用 mock 模式（预置回复，不依赖任何模型/网络）")
    else:
        llm = build_llm(cfg.llm.backend)  # disabled/none → build_llm 内部降级 MockLLM

    # TTS（edge=微软云零显存 / cosyvoice=本地克隆音色占 ~1.5GB）
    # ★ H2：cosyvoice 失败（脚本缺失/依赖未装）降级 edge，与 ASR/LLM/lipsync 降级哲学一致，
    #   避免一个语音子系统未就绪就让整个服务起不来
    from .engines.tts_edge import build_tts
    try:
        tts = build_tts(
            cfg.tts.backend,
            voice=cfg.tts.voice,
            rate=cfg.tts.rate,
            script=cfg.tts.cosyvoice_script,
            voice_sample=cfg.tts.voice_sample,
        )
    except Exception as e:
        log.warning("TTS %s 装配失败，降级 edge: %s", cfg.tts.backend, e)
        tts = build_tts("edge", voice=cfg.tts.voice, rate=cfg.tts.rate)

    # 唇形
    if cfg.lipsync.backend == "musetalk":
        try:
            from .engines.lipsync_musetalk import build_lipsync
            # 资源路径：相对路径基于 _resource_root（支持 PyInstaller 打包）
            musetalk_path = cfg.lipsync.musetalk_script
            if not os.path.isabs(musetalk_path):
                musetalk_path = os.path.join(_resource_root(), musetalk_path)
            lipsync = build_lipsync(
                musetalk_script=musetalk_path,
                fps=cfg.lipsync.fps,
                width=cfg.lipsync.output_width,
                height=cfg.lipsync.output_height,
            )
        except (ImportError, RuntimeError) as e:
            log.warning("唇形 musetalk 不可用，降级 Mock: %s", e)
            lipsync = MockLipSync()
    elif cfg.lipsync.backend == "mock":
        # 显式 mock：纯演示，按 fps 产占位帧（不渲染真帧，0 GPU 占用）
        lipsync = MockLipSync(fps=cfg.lipsync.fps)
        log.info("唇形使用 mock 模式（占位帧，0 GPU 占用，纯演示）")
    elif cfg.lipsync.backend in ("disabled", "none"):
        lipsync = MockLipSync()
        log.info("唇形已禁用（MockLipSync 占位）")
    else:
        raise ValueError(f"未知 lipsync backend: {cfg.lipsync.backend}")

    return asr, llm, tts, lipsync


def _probe_llm(llm, backend: str, timeout: float = 2.0) -> bool:
    """探测 LLM 后端 TCP 端口是否可达（仅启动期用，同步）。

    M7：用同步 socket + settimeout（不用 asyncio.run，避免启动期创建临时事件循环
    影响后续 uvicorn 的 loop）。本函数仅在 create_app（uvicorn 启动前）调用，
    阻塞最多 timeout 秒，可接受。运行时若需探测应另写异步版本。
    """
    import socket
    from urllib.parse import urlparse

    base = getattr(llm, "base_url", "")
    if not base:
        return True  # Mock 等无 base_url 的视为可用
    try:
        u = urlparse(base)
        host = u.hostname or "127.0.0.1"
        port = u.port or (443 if u.scheme == "https" else 80)
    except Exception:
        return False
    try:
        s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        s.settimeout(timeout)
        try:
            s.connect((host, port))
            return True
        finally:
            s.close()
    except OSError:
        return False


def _load_portrait(cfg: Config) -> bytes:
    """读取数字人形象图。不存在则返回空 bytes（Mock 唇形不依赖）。"""
    p = Path(cfg.lipsync.portrait)
    if not p.is_absolute():
        p = Path(_resource_root()) / p
    if p.is_file():
        return p.read_bytes()
    log.warning("形象图不存在: %s（将使用 Mock 唇形或默认）", p)
    return b""


def _init_store(cfg: Config):
    """初始化持久化存储（config.server.history_db 为空则禁用）。"""
    if not cfg.server.history_db:
        return None
    try:
        from .store import SQLiteStore
        path = cfg.server.history_db
        if not os.path.isabs(path):
            path = os.path.join(_config_root(), path)
        store = SQLiteStore(path)
        log.info("对话历史持久化已启用: %s", path)
        return store
    except Exception as e:
        log.warning("持久化存储初始化失败，回退纯内存: %s", e)
        return None


def _make_lifespan(cfg, asr, llm, tts, lipsync, pusher, registry):
    """创建 lifespan 上下文管理器（启动预热 + 关闭清理）。"""
    @asynccontextmanager
    async def lifespan(app: FastAPI):
        log.info("数字人服务启动 server=%s:%d", cfg.server.host, cfg.server.port)
        log.info("引擎: asr=%s llm=%s tts=%s lipsync=%s pusher=%s",
                 type(asr).__name__, type(llm).__name__,
                 type(tts).__name__, type(lipsync).__name__,
                 type(pusher).__name__)
        if hasattr(llm, "warmup") and not os.environ.get("DH_SKIP_WARMUP"):
            await llm.warmup()
        yield
        log.info("数字人服务关闭，清理 %d 个会话", len(registry))
        for sess in registry.all():
            await sess.cancel()
        # ★ 关闭所有可能持有 ClientSession/subprocess 的引擎（C1：原仅 [lipsync,llm]，
        #   漏了 asr/tts —— CloudASR 持有 aiohttp.ClientSession，不关闭会泄漏连接池）
        for engine in [asr, llm, tts, lipsync]:
            if hasattr(engine, "close"):
                try:
                    await engine.close()
                except Exception as e:
                    log.warning("%s 关闭异常: %s", type(engine).__name__, e)
    return lifespan


def create_app(cfg: Config, cfg_path: str = "", log_path: str = "") -> FastAPI:
    """工厂：装配引擎 + 注册路由。

    cfg_path/log_path 供管理台读写配置/日志（空串则管理台相关功能降级）。
    """
    warns = validate(cfg)
    for w in warns:
        log.warning("配置告警: %s", w)

    # ★ 装配阶段详细日志（目标机器诊断：每步结果 + 耗时）
    t_asm = time.monotonic()
    asr, llm, tts, lipsync = _build_engines(cfg)
    portrait = _load_portrait(cfg)
    pusher = build_pusher(cfg.pusher.backend)
    registry = SessionRegistry(max_sessions=cfg.server.max_sessions)
    store = _init_store(cfg)

    # web 目录
    web_dir = cfg.server.web_dir or "web"
    if not os.path.isabs(web_dir):
        web_dir = os.path.join(_resource_root(), web_dir)

    log.info("装配完成（%.1fs）: asr=%s llm=%s tts=%s lipsync=%s pusher=%s",
             time.monotonic() - t_asm,
             type(asr).__name__, type(llm).__name__, type(tts).__name__,
             type(lipsync).__name__, type(pusher).__name__)
    log.info("资源根目录: %s | web 目录: %s（%s）| 形象图: %d 字节",
             _resource_root(), web_dir,
             "存在" if os.path.isdir(web_dir) else "不存在（前端页面将不可用！）",
             len(portrait))
    if not os.path.isdir(web_dir):
        log.error("web 目录不存在: %s —— 浏览器打开将黑屏/空白，请检查打包是否包含 web/", web_dir)

    lifespan = _make_lifespan(cfg, asr, llm, tts, lipsync, pusher, registry)

    app = FastAPI(title="realtime-digital-human", lifespan=lifespan)
    app.state.cfg = cfg
    app.state.registry = registry
    app.state.pusher = pusher

    # 路由注册（全部委托给 routes.py）
    from .routes import register_routes
    register_routes(app, cfg, asr, llm, tts, lipsync, pusher,
                    portrait, registry, store, web_dir)

    # 管理控制台端点（/api/admin/*）
    from .admin import register_admin_routes
    register_admin_routes(app, cfg, log_path, cfg_path, store)

    return app



def _run_self_test(cfg: Config) -> int:
    """自检委托到 selftest.py。"""
    from .selftest import run_self_test
    return run_self_test(cfg)

def _setup_logging(verbose: bool) -> str:
    """配置日志：控制台 + 文件（data/digitalhuman.log，5MB×3 轮转）。

    文件日志是诊断问题的关键——控制台窗口关闭后日志就丢了，
    目标机器上黑屏/无声等问题都靠 digitalhuman.log 回溯。
    返回日志文件路径（空串表示文件日志不可用）。
    """
    import logging.handlers
    level = logging.DEBUG if verbose else logging.INFO
    fmt = "%(asctime)s [%(levelname)s] %(name)s: %(message)s"
    logging.basicConfig(level=level, format=fmt)

    try:
        data_dir = os.path.join(_config_root(), "data")
        os.makedirs(data_dir, exist_ok=True)
        path = os.path.join(data_dir, "digitalhuman.log")
        fh = logging.handlers.RotatingFileHandler(
            path, maxBytes=5 * 1024 * 1024, backupCount=3, encoding="utf-8")
        fh.setLevel(level)
        fh.setFormatter(logging.Formatter(fmt))
        logging.getLogger().addHandler(fh)
        return path
    except Exception as e:
        logging.getLogger(__name__).warning("文件日志不可用（仅控制台）: %s", e)
        return ""


def main():
    parser = argparse.ArgumentParser(
        description="实时数字人单机服务",
        # ★ 简化部署：不传 -c 也能跑（零配置模式）
        epilog="快速启动：python -m digitalhuman.server  （零配置，自动探测 Ollama）",
    )
    parser.add_argument("-c", "--config", default=None,
                        help="配置文件路径。不传则用内置默认（零配置模式）")
    parser.add_argument("--host", default=None, help="覆盖 host（默认 127.0.0.1）")
    parser.add_argument("--port", type=int, default=None, help="覆盖 port（默认 8000）")
    parser.add_argument("--quick", action="store_true",
                        help="零配置模式：无 config 文件时用内置默认，不报错")
    parser.add_argument("--self-test", action="store_true",
                        help="自检模式：启动服务 → 检查所有子系统 → 输出报告 → 退出")
    parser.add_argument("-v", "--verbose", action="store_true", help="DEBUG 日志")
    args = parser.parse_args()

    log_path = _setup_logging(args.verbose)
    if log_path:
        log.info("运行日志文件: %s（5MB×3 轮转）", log_path)

    # ★ 加载配置：有文件用文件，没有则内置默认（零配置启动）
    cfg = None
    used_cfg_path = ""
    if args.config:
        # 显式指定了 config 文件
        cfg = from_yaml(args.config)
        used_cfg_path = args.config
    else:
        # 尝试默认 config.yaml / config.dev.yaml，都不在则用内置默认
        # config 用 _config_root（exe 同级，用户可编辑）；不是 _resource_root（_internal 只读）
        for candidate in ("config.yaml", "config.dev.yaml", "config.gpu.yaml"):
            full = candidate if os.path.isabs(candidate) else os.path.join(_config_root(), candidate)
            if os.path.isfile(full):
                cfg = from_yaml(full)
                used_cfg_path = full
                log.info("使用配置文件: %s", full)
                break
        if cfg is None:
            log.info("未找到 config 文件，使用内置默认配置（零配置模式）")
            cfg = Config()
            cfg.server.host = "127.0.0.1"
            cfg.server.port = 8000
            # ★ 零配置模式也写一份 config.yaml 供管理台编辑（exe 同级，可写）
            used_cfg_path = os.path.join(_config_root(), "config.yaml")

    if args.host:
        cfg.server.host = args.host
    if args.port:
        cfg.server.port = args.port

    # ★ self-test 模式：自检后退出（不启动常驻服务）
    if args.self_test:
        return _run_self_test(cfg)

    # 友好的启动 banner（EXE 用户看到的第一个输出）
    try:
        from . import __version__
    except Exception:
        __version__ = "?"
    is_frozen = getattr(sys, "frozen", False)
    mode = "单EXE" if is_frozen else "开发模式"
    print(f"""
╔══════════════════════════════════════════════════╗
║  实时数字人 v{__version__}（{mode}）
║
║  浏览器打开: http://{cfg.server.host}:{cfg.server.port}
║  API 文档:   http://{cfg.server.host}:{cfg.server.port}/docs
║  按 Ctrl+C 停止
╚══════════════════════════════════════════════════╝
  提示：首次对话较慢（模型预热），后续会快。
  若数字人无响应，请检查 Ollama 是否已启动（ollama serve）。""")

    # ★ onefile 首次启动提示（解压 150MB 到 temp 需要时间）
    if is_frozen and not os.environ.get("DH_SKIP_WARMUP"):
        print("  [INFO] 首次启动正在初始化（约 30-60 秒），请耐心等待...\n", flush=True)

    import uvicorn
    app = create_app(cfg, cfg_path=used_cfg_path, log_path=log_path)
    uvicorn.run(app, host=cfg.server.host, port=cfg.server.port, log_level="info")


if __name__ == "__main__":
    main()
