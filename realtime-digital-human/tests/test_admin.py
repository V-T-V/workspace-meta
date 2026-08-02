"""管理控制台端点测试。"""
import os
import tempfile

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient

from digitalhuman.admin import register_admin_routes
from digitalhuman.config import Config, from_yaml, save_yaml


def _make_app(tmp_path):
    """构造带管理端点的测试 app。"""
    cfg = Config()
    log_path = str(tmp_path / "test.log")
    cfg_path = str(tmp_path / "config.yaml")
    # 写测试日志
    with open(log_path, "w", encoding="utf-8") as f:
        f.write("2026-01-01 [INFO] test: hello\n")
        f.write("2026-01-02 [ERROR] test: bad\n")
        f.write("2026-01-03 [WARNING] test: warn\n")
    save_yaml(cfg, cfg_path)

    app = FastAPI()
    register_admin_routes(app, cfg, log_path, cfg_path)
    return app, cfg, cfg_path


def test_get_config_sanitizes_api_key(tmp_path):
    """api_key 读取时脱敏（显示尾号，不回明文）。"""
    app, cfg, _ = _make_app(tmp_path)
    cfg.llm.api_key = "sk-secret-123456"
    cfg.asr.api_key = "sk-asr-secret-7890"
    client = TestClient(app)

    r = client.get("/api/admin/config")
    assert r.status_code == 200
    data = r.json()["config"]
    # 显示尾 4 位（sk-***3456 / sk-***7890）
    assert "3456" in data["llm"]["api_key"]
    assert "7890" in data["asr"]["api_key"]
    assert "secret" not in r.text  # 中间部分不能出现在响应里


def test_update_config_writes_yaml(tmp_path):
    """POST 更新配置后写回 config.yaml，重读一致。"""
    app, cfg, cfg_path = _make_app(tmp_path)
    client = TestClient(app)

    r = client.post("/api/admin/config", json={
        "llm": {"backend": "deepseek", "model": "deepseek-chat"}
    })
    assert r.status_code == 200
    body = r.json()
    assert "llm.backend" in body["updated"]
    assert "llm.model" in body["updated"]

    # 重读 config.yaml 确认写回
    cfg2 = from_yaml(cfg_path)
    assert cfg2.llm.backend == "deepseek"
    assert cfg2.llm.model == "deepseek-chat"


def test_logs_endpoint_returns_tail(tmp_path):
    """日志端点返回尾部行。"""
    app, _, _ = _make_app(tmp_path)
    client = TestClient(app)

    r = client.get("/api/admin/logs?tail=10")
    assert r.status_code == 200
    lines = r.json()["lines"]
    assert len(lines) == 3
    assert "hello" in lines[0]


def test_logs_level_filter(tmp_path):
    """level 过滤：只返回 ERROR 行。"""
    app, _, _ = _make_app(tmp_path)
    client = TestClient(app)

    r = client.get("/api/admin/logs?level=ERROR")
    lines = r.json()["lines"]
    assert len(lines) == 1
    assert "bad" in lines[0]


def test_test_llm_mock_backend(tmp_path):
    """LLM mock backend 测试直接返回 ok。"""
    app, cfg, _ = _make_app(tmp_path)
    cfg.llm.backend = "mock"
    client = TestClient(app)

    r = client.post("/api/admin/test/llm")
    data = r.json()
    assert data["ok"] is True
    assert "mock" in data["detail"].lower()


def test_selftest_endpoint(tmp_path):
    """环境自检返回结构化结果。"""
    app, _, _ = _make_app(tmp_path)
    client = TestClient(app)

    r = client.get("/api/admin/selftest")
    data = r.json()
    assert "total" in data
    assert "passed" in data
    assert isinstance(data["checks"], list)
    # 至少有 GPU 和端口检查
    names = [c["name"] for c in data["checks"]]
    assert any("CUDA" in n or "GPU" in n for n in names)


def test_update_config_skips_masked_api_key(tmp_path):
    """POST 传脱敏占位符（含 ***）不应覆盖真实 api_key。"""
    app, cfg, cfg_path = _make_app(tmp_path)
    cfg.llm.api_key = "sk-real-key-9999"
    save_yaml(cfg, cfg_path)  # 确保真实 key 已写盘
    client = TestClient(app)

    # 前端回显的脱敏值回传（不应覆盖）
    r = client.post("/api/admin/config", json={
        "llm": {"api_key": "sk-***9999"}
    })
    assert r.status_code == 200
    assert "llm.api_key" not in r.json()["updated"]  # 被跳过

    # 重读确认真实 key 没被覆盖
    cfg2 = from_yaml(cfg_path)
    assert cfg2.llm.api_key == "sk-real-key-9999"


def test_update_config_accepts_new_api_key(tmp_path):
    """POST 传真实新 api_key 应写入。"""
    app, cfg, cfg_path = _make_app(tmp_path)
    client = TestClient(app)

    r = client.post("/api/admin/config", json={
        "llm": {"api_key": "sk-new-real-key-1234"}
    })
    assert r.status_code == 200
    assert "llm.api_key" in r.json()["updated"]

    cfg2 = from_yaml(cfg_path)
    assert cfg2.llm.api_key == "sk-new-real-key-1234"


def test_list_sessions(tmp_path):
    """会话列表端点。"""
    app, cfg, _ = _make_app(tmp_path)
    # _make_app 不传 store，需直接测 store 逻辑
    from digitalhuman.store import SQLiteStore
    store = SQLiteStore(str(tmp_path / "test.db"))
    import asyncio
    asyncio.get_event_loop().run_until_complete(
        store.append_message("sess-a", "user", "你好"))
    asyncio.get_event_loop().run_until_complete(
        store.append_message("sess-a", "assistant", "你好呀"))

    from digitalhuman.admin import _list_sessions_sync
    rows = _list_sessions_sync(store)
    assert len(rows) == 1
    assert rows[0]["id"] == "sess-a"
    assert rows[0]["msg_count"] == 2


def test_get_session_memory(tmp_path):
    """查看会话记忆端点（历史 + 摘要提取）。"""
    from digitalhuman.store import SQLiteStore
    store = SQLiteStore(str(tmp_path / "test.db"))
    import asyncio
    loop = asyncio.new_event_loop()
    loop.run_until_complete(store.append_message("s1", "user", "我叫张三"))
    loop.run_until_complete(store.append_message("s1", "assistant", "你好张三"))
    loop.run_until_complete(store.append_message("s1", "system", "[对话摘要] 用户叫张三"))
    loop.close()

    history = store.get_session_history("s1")
    assert len(history) == 3
    summaries = [m for m in history if m["role"] == "system" and "对话摘要" in m["content"]]
    assert len(summaries) == 1
    assert "张三" in summaries[0]["content"]
