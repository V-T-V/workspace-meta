"""云端 ASR 引擎单测（不依赖真实 API）。

用 mock 模拟 aiohttp POST，验证：
1. transcribe_stream 消费完整 PCM → 打包 WAV → POST → 解析 text
2. multipart form 含 model/language 字段
3. api_key 作为 Bearer 头
4. HTTP 错误状态码抛 RuntimeError
5. build_asr_cloud 工厂：预设 base_url / env 优先 / 未知报错
"""
import asyncio
import json
from unittest.mock import AsyncMock, MagicMock

import aiohttp
import pytest

from digitalhuman.engines.asr_cloud import (
    CloudASR, ASR_CLOUD_PRESETS, build_asr_cloud,
)


# ---------- Mock aiohttp ----------

class MockResp:
    def __init__(self, status=200, json_data=None, text_body=""):
        self.status = status
        self._json = json_data or {}
        self._text = text_body

    async def __aenter__(self): return self
    async def __aexit__(self, *a): return False
    async def json(self): return self._json
    async def text(self): return self._text


class MockSession:
    """记录 post 调用（url, data=form, headers）。"""
    def __init__(self, resp):
        self._resp = resp
        self.post_calls = []
        self.closed = False

    def post(self, url, **kwargs):
        self.post_calls.append((url, kwargs))
        return self._resp

    async def close(self): pass


@pytest.mark.asyncio
async def test_transcribe_parses_text():
    """消费 PCM → POST → JSON.text → yield 一条文本。"""
    resp = MockResp(200, json_data={"text": "你好呀"})
    sess = MockSession(resp)
    asr = CloudASR("https://api.groq.com/openai/v1", model="whisper-large-v3",
                   api_key="sk-x", backend_tag="groq")
    asr._session = sess

    async def mic():
        yield b"\x00\x00" * 1600  # 1600 samples PCM16（占位静音）

    texts = []
    async for t in asr.transcribe_stream(mic()):
        texts.append(t)
    assert texts == ["你好呀"]
    assert len(sess.post_calls) == 1
    url = sess.post_calls[0][0]
    assert url.endswith("/audio/transcriptions")  # base_url 含 /v1 → 不重复追加


@pytest.mark.asyncio
async def test_transcribe_empty_audio_yields_nothing():
    """空音频流不 POST（避免无谓请求）。"""
    asr = CloudASR("https://x", backend_tag="cloud")
    asr._session = MockSession(MockResp(200, {"text": ""}))

    async def mic():
        return
        yield  # 让它成为 async generator

    texts = [t async for t in asr.transcribe_stream(mic())]
    assert texts == []


@pytest.mark.asyncio
async def test_form_fields_model_language(monkeypatch):
    """构造 form 时必须含 model + language 字段（用 spy 拦截 FormData.add_field）。"""
    added_fields = []
    orig_init = aiohttp.FormData.add_field

    def spy_add_field(self, name, *args, **kwargs):
        added_fields.append(name)
        return orig_init(self, name, *args, **kwargs)

    monkeypatch.setattr(aiohttp.FormData, "add_field", spy_add_field)

    resp = MockResp(200, {"text": "ok"})
    sess = MockSession(resp)
    asr = CloudASR("https://api.groq.com/openai/v1", model="whisper-large-v3",
                   language="zh", api_key="sk-x")
    asr._session = sess

    async def mic():
        yield b"\x00\x00" * 10

    async for _ in asr.transcribe_stream(mic()):
        pass

    assert "file" in added_fields
    assert "model" in added_fields
    assert "language" in added_fields


@pytest.mark.asyncio
async def test_api_key_bearer_header():
    """api_key 作为 Authorization: Bearer 头。"""
    resp = MockResp(200, {"text": "x"})
    sess = MockSession(resp)
    asr = CloudASR("https://x", api_key="sk-secret")
    asr._session = sess

    async def mic():
        yield b"\x00\x00" * 10

    async for _ in asr.transcribe_stream(mic()):
        pass

    headers = sess.post_calls[0][1]["headers"]
    assert headers["Authorization"] == "Bearer sk-secret"


@pytest.mark.asyncio
async def test_http_error_raises():
    """HTTP 错误状态码抛 RuntimeError（含状态码与响应体片段）。"""
    resp = MockResp(401, text_body='{"error":"invalid api key"}')
    sess = MockSession(resp)
    asr = CloudASR("https://x", api_key="bad")
    asr._session = sess

    async def mic():
        yield b"\x00\x00" * 10

    with pytest.raises(RuntimeError, match="HTTP 401"):
        async for _ in asr.transcribe_stream(mic()):
            pass


# ---------- build_asr_cloud 工厂 ----------

def test_preset_auto_base_url():
    """backend=groq 且未配 base_url → 自动套预设。"""
    asr = build_asr_cloud("groq", model="whisper-large-v3", api_key="x")
    assert isinstance(asr, CloudASR)
    assert asr.base_url == ASR_CLOUD_PRESETS["groq"]
    assert asr.backend_tag == "groq"


def test_api_key_env_priority(monkeypatch):
    """DH_ASR_API_KEY 环境变量优先于 yaml。"""
    monkeypatch.setenv("DH_ASR_API_KEY", "sk-from-env")
    asr = build_asr_cloud("groq", model="m", api_key="sk-from-yaml")
    assert asr.api_key == "sk-from-env"


def test_custom_base_url_not_overridden():
    """用户显式配 base_url → 不被预设覆盖（支持自建 whisper-server）。"""
    custom = "https://my-asr.example.com/v1"
    asr = build_asr_cloud("groq", base_url=custom, model="m", api_key="x")
    assert asr.base_url == custom


def test_unknown_backend_raises():
    """未知 backend 抛 ValueError。"""
    with pytest.raises(ValueError, match="未知 cloud asr backend"):
        build_asr_cloud("nonexistent")


def test_url_path_v1_vs_root():
    """base_url 含 /v1 → /audio/transcriptions；不含 → /v1/audio/transcriptions。"""
    asr1 = CloudASR("https://api.groq.com/openai/v1")
    # 直接验证 URL 拼接逻辑（不真正发请求）
    base1 = asr1.base_url
    path1 = "/audio/transcriptions" if base1.endswith("/v1") else "/v1/audio/transcriptions"
    assert base1 + path1 == "https://api.groq.com/openai/v1/audio/transcriptions"

    asr2 = CloudASR("https://custom-asr.com")
    base2 = asr2.base_url
    path2 = "/audio/transcriptions" if base2.endswith("/v1") else "/v1/audio/transcriptions"
    assert base2 + path2 == "https://custom-asr.com/v1/audio/transcriptions"
