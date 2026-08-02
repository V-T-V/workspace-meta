"""自检模式（从 server.py 拆出）。

检查所有子系统，输出报告，退出。用户双击 EXE 加 --self-test 即可自检。
"""
from __future__ import annotations

import json
import os
import socket
import sys
import threading
import time

from .config import Config


def run_self_test(cfg: Config) -> int:
    """自检：Python / 依赖 / 引擎 / Ollama / 端口 / REST 端点。"""
    OK, FAIL, WARN = "[OK]", "[FAIL]", "[WARN]"
    passed = failed = warnings = 0

    def check(name, condition, detail=""):
        nonlocal passed, failed
        if condition:
            print(f"  {OK} {name}")
            passed += 1
        else:
            print(f"  {FAIL} {name}: {detail}")
            failed += 1

    def warn(name, detail=""):
        nonlocal warnings
        print(f"  {WARN} {name}: {detail}")
        warnings += 1

    print(f"\n{'='*50}\n  Digital Human Self-Test\n{'='*50}\n")

    # 1. Python
    print("[1/6] Python")
    check(f"Python {sys.version.split()[0]}", True)

    # 2. 核心依赖
    print("\n[2/6] Dependencies")
    for mod in ["fastapi", "uvicorn", "aiohttp", "yaml", "websockets"]:
        try:
            __import__(mod)
            check(mod, True)
        except ImportError:
            check(mod, False, "not installed")

    # 3. 引擎装配
    print("\n[3/6] Engines")
    try:
        from .server import _build_engines
        asr, llm, tts, lipsync = _build_engines(cfg)
        for name, eng in [("ASR", asr), ("LLM", llm), ("TTS", tts), ("LipSync", lipsync)]:
            cls = type(eng).__name__
            if cls.startswith("Mock"):
                warn(f"{name}({cls})", "degraded to Mock")
            else:
                check(f"{name}({cls})", True)
    except Exception as e:
        check("Engines", False, str(e))

    # 4. Ollama
    print("\n[4/6] Ollama")
    try:
        import urllib.request
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        base = cfg.llm.base_url.rstrip("/")
        opener.open(f"{base}/api/version", timeout=3)
        check("Ollama reachable", True)
        r2 = opener.open(f"{base}/api/tags", timeout=3)
        models = [m["name"] for m in json.loads(r2.read()).get("models", [])]
        ok = cfg.llm.model in models or any(m.startswith(cfg.llm.model.split(":")[0]) for m in models)
        check(f"Model {cfg.llm.model}", ok, f"available: {models[:5]}" if not ok else "")
    except Exception as e:
        check("Ollama reachable", False, str(e))

    # 5. 端口
    print("\n[5/6] Port")
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    try:
        sock.bind((cfg.server.host, cfg.server.port))
        sock.close()
        check(f"Port {cfg.server.port}", True)
    except OSError as e:
        check(f"Port {cfg.server.port}", False, str(e))
        sock.close()

    # 6. REST 端点
    print("\n[6/6] REST Endpoints")
    import uvicorn
    from .server import create_app

    app = create_app(cfg)
    config = uvicorn.Config(app, host=cfg.server.host, port=cfg.server.port, log_level="error")
    server = uvicorn.Server(config)
    thread = threading.Thread(target=server.run, daemon=True)
    thread.start()

    import urllib.request
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    base_url = f"http://{cfg.server.host}:{cfg.server.port}"
    ready = False
    for _ in range(20):
        try:
            opener.open(f"{base_url}/health", timeout=1)
            ready = True
            break
        except Exception:
            time.sleep(0.3)

    if ready:
        for ep, desc in [("/health", "Health"), ("/v1/models", "OpenAI"),
                         ("/personas", "Personas"), ("/api/dashboard", "Dashboard"),
                         ("/metrics", "Metrics")]:
            try:
                r = opener.open(f"{base_url}{ep}", timeout=2)
                check(f"{desc}({ep})", r.status == 200)
            except Exception as e:
                check(f"{desc}({ep})", False, str(e))
    else:
        check("Server start", False, "not ready in 6s")

    server.should_exit = True
    thread.join(timeout=3)

    # 汇总
    print(f"\n{'='*50}")
    print(f"  {OK} {passed} passed / {FAIL} {failed} failed / {WARN} {warnings} warnings")
    if failed > 0:
        print(f"\n  {FAIL} {failed} failures. Please fix before running.")
        return 1
    elif warnings > 0:
        print(f"\n  {WARN} {warnings} degraded warnings (functional but limited).")
        return 0
    else:
        print(f"\n  {OK} All passed! Run: python -m digitalhuman.server")
        return 0
