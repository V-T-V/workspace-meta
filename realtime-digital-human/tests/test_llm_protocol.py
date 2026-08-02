"""LLM 引擎协议解析单测（不依赖真实 Ollama）。

用 mock 模拟 Ollama NDJSON 流 / gpu-mesh SSE 流，验证：
1. NDJSON 解析正确（含 M5：解析失败告警不中断）
2. SSE 解析正确（data: {json} / [DONE]）
3. session 连接复用（_get_session 返回同一实例）
4. close 清理 session
5. HTTP 错误状态码抛 RuntimeError
"""
import asyncio
import json
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from digitalhuman.engines.base import Message
from digitalhuman.engines.llm_ollama import (
    OllamaLLM, GpuMeshLLM, OpenAICompatLLM, CLOUD_PRESETS, build_llm,
)


# ---------- Mock aiohttp 工具 ----------

class MockResponse:
    """模拟 aiohttp.ClientResponse。"""
    def __init__(self, status=200, lines=None, body=None):
        self.status = status
        self._lines = lines or []
        self._body = body
        self._idx = 0

    async def __aenter__(self):
        return self

    async def __aexit__(self, *args):
        return False

    @property
    def content(self):
        return self

    def __aiter__(self):
        return self

    async def __anext__(self):
        if self._idx < len(self._lines):
            line = self._lines[self._idx]
            self._idx += 1
            return line
        raise StopAsyncIteration

    async def text(self):
        return self._body or ""

    async def read(self):
        return (self._body or "").encode()


class MockSession:
    """模拟 aiohttp.ClientSession，记录 post 调用。"""
    def __init__(self, response):
        self._response = response
        self.post_calls = []

    def post(self, url, **kwargs):
        self.post_calls.append((url, kwargs))
        return self._response

    @property
    def closed(self):
        return False

    async def close(self):
        pass


# ---------- OllamaLLM NDJSON 解析 ----------

@pytest.mark.asyncio
async def test_ollama_parses_ndjson():
    """NDJSON 每行一个 JSON，content 字段是 token。"""
    lines = [
        (json.dumps({"message": {"content": "你"}, "done": False}) + "\n").encode(),
        (json.dumps({"message": {"content": "好"}, "done": False}) + "\n").encode(),
        (json.dumps({"message": {"content": ""}, "done": True}) + "\n").encode(),
    ]
    resp = MockResponse(200, lines=lines)
    sess = MockSession(resp)

    llm = OllamaLLM("http://x", "m")
    llm._session = sess  # 注入 mock session，跳过 _get_session

    tokens = []
    async for t in llm.chat_stream("hi"):
        tokens.append(t)

    assert tokens == ["你", "好"]


@pytest.mark.asyncio
async def test_ollama_skips_invalid_json(caplog):
    """M5：NDJSON 解析失败应跳过且告警，不中断流。"""
    lines = [
        b"<html>502 Bad Gateway</html>\n",  # 非 JSON
        (json.dumps({"message": {"content": "ok"}, "done": True}) + "\n").encode(),
    ]
    resp = MockResponse(200, lines=lines)
    sess = MockSession(resp)

    llm = OllamaLLM("http://x", "m")
    llm._session = sess

    tokens = []
    with caplog.at_level("WARNING"):
        async for t in llm.chat_stream("hi"):
            tokens.append(t)

    assert tokens == ["ok"]  # 跳过了无效行，继续解析
    assert any("NDJSON 解析失败" in r.message for r in caplog.records)


@pytest.mark.asyncio
async def test_ollama_http_error_raises():
    """HTTP 非 200 应抛 RuntimeError。"""
    resp = MockResponse(500, body="internal error")
    sess = MockSession(resp)
    llm = OllamaLLM("http://x", "m")
    llm._session = sess

    with pytest.raises(RuntimeError, match="Ollama HTTP 500"):
        async for _ in llm.chat_stream("hi"):
            pass


@pytest.mark.asyncio
async def test_ollama_build_messages_with_history():
    """history 应正确拼入 messages（system + 历史 + 当前 user）。"""
    llm = OllamaLLM("http://x", "m", system_prompt="be nice")
    msgs = llm._build_messages("现在", [
        Message(role="user", content="之前"),
        Message(role="assistant", content="回复"),
    ])
    assert msgs[0] == {"role": "system", "content": "be nice"}
    assert msgs[1] == {"role": "user", "content": "之前"}
    assert msgs[2] == {"role": "assistant", "content": "回复"}
    assert msgs[3] == {"role": "user", "content": "现在"}


# ---------- session 复用 ----------

@pytest.mark.asyncio
async def test_ollama_session_reuse():
    """_get_session 应返回同一实例（连接池复用）。"""
    llm = OllamaLLM("http://x", "m")
    s1 = await llm._get_session()
    s2 = await llm._get_session()
    assert s1 is s2, "应复用同一 session"
    await llm.close()
    assert llm._session is None


@pytest.mark.asyncio
async def test_ollama_close_releases_session():
    llm = OllamaLLM("http://x", "m")
    await llm._get_session()
    assert llm._session is not None
    await llm.close()
    assert llm._session is None


# ---------- GpuMeshLLM SSE 解析 ----------

@pytest.mark.asyncio
async def test_gpumesh_parses_sse():
    """OpenAI SSE：data: {json}\n\n，结尾 data: [DONE]。"""
    lines = [
        b'data: {"choices":[{"delta":{"content":"hi"}}]}\n',
        b'data: {"choices":[{"delta":{"content":" there"}}]}\n',
        b'data: [DONE]\n',
    ]
    resp = MockResponse(200, lines=lines)
    sess = MockSession(resp)

    llm = GpuMeshLLM("http://x", "m")
    llm._session = sess

    tokens = []
    async for t in llm.chat_stream("hi"):
        tokens.append(t)

    assert tokens == ["hi", " there"]


@pytest.mark.asyncio
async def test_gpumesh_api_key_header():
    """api_key 应作为 Bearer 头发送。"""
    resp = MockResponse(200, lines=[b'data: [DONE]\n'])
    sess = MockSession(resp)
    llm = GpuMeshLLM("http://x", "m", api_key="sk-test")
    llm._session = sess

    async for _ in llm.chat_stream("hi"):
        pass

    assert len(sess.post_calls) == 1
    _, kwargs = sess.post_calls[0]
    assert kwargs["headers"]["Authorization"] == "Bearer sk-test"


# ---------- OpenAICompatLLM（线上云 API）----------

@pytest.mark.asyncio
async def test_openai_compat_parses_sse():
    """线上云 API SSE 流式解析（与 gpu_mesh 同协议，验证 OpenAICompatLLM 正确解析）。"""
    # bytes 字面量限 ASCII，中文用 encode 拼接
    lines = [
        'data: {"choices":[{"delta":{"content":"你"}}]}\n'.encode("utf-8"),
        'data: {"choices":[{"delta":{"content":"好"}}]}\n'.encode("utf-8"),
        b'data: [DONE]\n',
    ]
    resp = MockResponse(200, lines=lines)
    sess = MockSession(resp)
    llm = OpenAICompatLLM("https://api.deepseek.com", "deepseek-chat", backend_tag="deepseek")
    llm._session = sess

    tokens = []
    async for t in llm.chat_stream("hi"):
        tokens.append(t)
    assert tokens == ["你", "好"]


@pytest.mark.asyncio
async def test_openai_compat_api_key_header():
    """api_key 应作为 Bearer 头发送（线上密钥安全传输）。"""
    resp = MockResponse(200, lines=[b'data: [DONE]\n'])
    sess = MockSession(resp)
    llm = OpenAICompatLLM("https://api.deepseek.com", "m", api_key="sk-xxx", backend_tag="deepseek")
    llm._session = sess

    async for _ in llm.chat_stream("hi"):
        pass

    _, kwargs = sess.post_calls[0]
    assert kwargs["headers"]["Authorization"] == "Bearer sk-xxx"


@pytest.mark.asyncio
async def test_openai_compat_url_path_handling():
    """base_url 含 /v1 时不重复追加（openai 预设）；不含时追加 /v1（deepseek 根路径）。"""
    # deepseek: base_url 不含 /v1 → URL 应追加 /v1/chat/completions
    resp1 = MockResponse(200, lines=[b'data: [DONE]\n'])
    sess1 = MockSession(resp1)
    llm1 = OpenAICompatLLM("https://api.deepseek.com", "m", backend_tag="deepseek")
    llm1._session = sess1
    async for _ in llm1.chat_stream("hi"):
        pass
    assert sess1.post_calls[0][0] == "https://api.deepseek.com/v1/chat/completions"

    # openai: base_url 含 /v1 → URL 应是 /chat/completions（不重复）
    resp2 = MockResponse(200, lines=[b'data: [DONE]\n'])
    sess2 = MockSession(resp2)
    llm2 = OpenAICompatLLM("https://api.openai.com/v1", "gpt-4o", backend_tag="openai")
    llm2._session = sess2
    async for _ in llm2.chat_stream("hi"):
        pass
    assert sess2.post_calls[0][0] == "https://api.openai.com/v1/chat/completions"


@pytest.mark.asyncio
async def test_cloud_warmup_noop():
    """线上云 API 不预热（省 token + 省 1-3s 启动）：warmup 应立即返回不发请求。"""
    llm = OpenAICompatLLM("https://api.deepseek.com", "m", backend_tag="deepseek")
    # 注入一个会记录 post 调用的假 session
    fake_session = MagicMock()
    fake_session.closed = False
    fake_session.post = MagicMock(return_value=None)
    llm._session = fake_session
    await llm.warmup()  # 应不抛、不请求
    assert fake_session.post.call_count == 0


# ---------- build_llm 工厂 ----------

def test_build_llm_cloud_preset_auto_base_url():
    """backend=deepseek 且未显式配 base_url → 自动套预设 base_url。"""
    llm = build_llm("deepseek", base_url="", model="deepseek-chat", api_key="sk-x")
    assert isinstance(llm, OpenAICompatLLM)
    assert llm.base_url == CLOUD_PRESETS["deepseek"]
    assert llm.backend_tag == "deepseek"


def test_build_llm_api_key_env_priority(monkeypatch):
    """DH_LLM_API_KEY 环境变量应优先于 yaml 的 api_key（安全：key 不进 git）。"""
    monkeypatch.setenv("DH_LLM_API_KEY", "sk-from-env")
    llm = build_llm("deepseek", model="deepseek-chat", api_key="sk-from-yaml")
    assert llm.api_key == "sk-from-env"


def test_build_llm_cloud_explicit_base_url_wins():
    """用户在 yaml 显式配了非默认 base_url → 不被预设覆盖（支持自建/代理）。"""
    custom = "https://my-proxy.example.com"
    llm = build_llm("deepseek", base_url=custom, model="m", api_key="sk-x")
    assert llm.base_url == custom


def test_build_llm_gpu_mesh_backward_compat():
    """旧 config 的 gpu_mesh backend 仍可用（向后兼容，转 GpuMeshLLM）。"""
    llm = build_llm("gpu_mesh", base_url="http://gpu-mesh:8080", model="qwen2.5:7b", api_key="x")
    assert isinstance(llm, GpuMeshLLM)


def test_build_llm_unknown_backend_raises():
    """未知 backend 应抛 ValueError（而非静默用错引擎）。"""
    with pytest.raises(ValueError, match="未知 llm backend"):
        build_llm("nonexistent_backend")
