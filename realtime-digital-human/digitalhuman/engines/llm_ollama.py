"""LLM 引擎：本地 Ollama / 线上云 API（OpenAI 兼容）流式实现。

支持三类后端：
  - ollama    本地 Ollama /api/chat（NDJSON）
  - openai    通用 OpenAI 兼容云 API /v1/chat/completions（SSE）
  - 厂商快捷项 deepseek / glm / qwen / kimi（自动套预设 base_url，走 OpenAI 兼容 SSE）

设计：
★ C1 连接复用：ClientSession 引擎级常驻，省 TCP 握手。
★ C1 模型常驻：keep_alive 让 Ollama 把模型留在显存。
★ 线上不预热：云 API 模型常驻免加载，预热白白消耗 token + 拖慢启动 1-3s（仅本地 Ollama 预热）。
★ api_key 安全：环境变量 DH_LLM_API_KEY 优先于 yaml（key 不进 git/EXE）。
★ 重构：OllamaLLM 与 OpenAICompatLLM 抽 _BaseHTTPLLM 基类，省 ~60 行重复。
"""
from __future__ import annotations

import asyncio
import json
import logging
import os
import time
from collections.abc import AsyncIterator

import aiohttp

from .base import LLMEngine, Message

log = logging.getLogger(__name__)

# 预设云厂商 → 默认 base_url（写 backend: deepseek 即自动套用，用户无需查文档）。
# 均为 OpenAI 兼容（/v1/chat/completions + SSE）。
CLOUD_PRESETS: dict[str, str] = {
    "deepseek": "https://api.deepseek.com",
    "glm":      "https://open.bigmodel.cn/api/paas/v4",   # 智谱 GLM
    "openai":   "https://api.openai.com/v1",
    "qwen":     "https://dashscope.aliyuncs.com/compatible-mode/v1",  # 通义千问
    "kimi":     "https://api.moonshot.cn/v1",
}


class _BaseHTTPLLM(LLMEngine):
    """OllamaLLM 与 GpuMeshLLM 的共同基类。

    承载：连接池管理（_get_session/close）、消息构造（_build_messages）、预热（warmup）。
    子类只需实现 chat_stream（协议不同：NDJSON vs SSE）。
    """

    def __init__(self, base_url: str, model: str, system_prompt: str = "",
                 temperature: float = 0.7, max_tokens: int = 256,
                 timeout: float = 120.0, keep_alive: str = "30m",
                 num_ctx: int = 2048):
        self.base_url = base_url.rstrip("/")
        self.model = model
        self.system_prompt = system_prompt
        self.temperature = temperature
        self.max_tokens = max_tokens
        self.timeout = timeout
        self.keep_alive = keep_alive
        self.num_ctx = num_ctx
        self._session: aiohttp.ClientSession | None = None
        # ★ H1：保护懒初始化的并发安全（max_sessions>1 时多 pipeline 并发首请求会竞态）
        self._session_lock = asyncio.Lock()

    async def _get_session(self) -> aiohttp.ClientSession:
        """获取或创建常驻 ClientSession（连接池复用）。

        ★ H1：double-check + Lock，避免并发首请求各自建 session 导致旧 session 泄漏。
        """
        if self._session is None or self._session.closed:
            async with self._session_lock:
                if self._session is None or self._session.closed:
                    timeout = aiohttp.ClientTimeout(
                        total=self.timeout,
                        sock_read=30,  # ★ H3：防流式响应半开连接导致读永久挂起
                    )
                    connector = aiohttp.TCPConnector(
                        limit=10, force_close=False, enable_cleanup_closed=True,
                    )
                    self._session = aiohttp.ClientSession(
                        timeout=timeout, trust_env=False, connector=connector,
                    )
        return self._session

    async def close(self) -> None:
        """关闭常驻 session（服务停止时调）。"""
        if self._session and not self._session.closed:
            await self._session.close()
            self._session = None

    def _build_messages(self, prompt: str, history: list[Message]) -> list[dict]:
        """构造 OpenAI/Ollama 通用 messages 列表。"""
        msgs: list[dict] = []
        if self.system_prompt:
            msgs.append({"role": "system", "content": self.system_prompt})
        for m in history:
            if m.role in ("system", "user", "assistant"):
                msgs.append({"role": m.role, "content": m.content})
        msgs.append({"role": "user", "content": prompt})
        return msgs

    async def warmup(self) -> None:
        """预热模型：启动时发一个极小请求，触发模型加载到显存。失败不抛。"""
        try:
            log.info("预热 LLM 模型 %s（加载到显存）...", self.model)
            t0 = time.monotonic()
            async for _ in self.chat_stream("hi"):
                break
            log.info("LLM 预热完成，耗时 %.2fs", time.monotonic() - t0)
        except Exception as e:
            log.warning("LLM 预热失败（不影响启动）: %s", e)


class OllamaLLM(_BaseHTTPLLM):
    """直连本机 Ollama /api/chat 流式（NDJSON）。"""

    async def chat_stream(
        self, prompt: str, history: list[Message] | None = None
    ) -> AsyncIterator[str]:
        messages = self._build_messages(prompt, history or [])
        payload = {
            "model": self.model,
            "messages": messages,
            "stream": True,
            "keep_alive": self.keep_alive,
            "think": False,  # 关闭 qwen3 思维链（实时对话不需要，省 5-20s 延迟）
            "options": {
                "temperature": self.temperature,
                "num_predict": self.max_tokens,
                "num_ctx": self.num_ctx,
            },
        }
        url = f"{self.base_url}/api/chat"
        log.debug("Ollama LLM POST %s model=%s", url, self.model)
        sess = await self._get_session()
        async with sess.post(url, json=payload) as resp:
            if resp.status != 200:
                body = await resp.text()
                raise RuntimeError(f"Ollama HTTP {resp.status}: {body[:200]}")
            # NDJSON：每行一个 JSON
            async for raw_line in resp.content:
                line = raw_line.strip()
                if not line:
                    continue
                try:
                    chunk = json.loads(line)
                except json.JSONDecodeError:
                    log.warning("Ollama NDJSON 解析失败，跳过: %r", line[:200])
                    continue
                content = (chunk.get("message") or {}).get("content", "")
                if content:
                    yield content
                if chunk.get("done"):
                    return


class GpuMeshLLM(_BaseHTTPLLM):
    """走 gpu-mesh OpenAI 兼容网关 /v1/chat/completions 流式（SSE）。

    与 OllamaLLM 的区别：请求/响应格式走 OpenAI 标准，需 api_key。
    """

    def __init__(self, base_url: str, model: str, system_prompt: str = "",
                 temperature: float = 0.7, max_tokens: int = 256,
                 api_key: str | None = None, timeout: float = 120.0,
                 keep_alive: str = "30m", num_ctx: int = 2048):
        super().__init__(base_url, model, system_prompt, temperature, max_tokens,
                         timeout, keep_alive, num_ctx)
        self.api_key = api_key

    async def chat_stream(
        self, prompt: str, history: list[Message] | None = None
    ) -> AsyncIterator[str]:
        messages = self._build_messages(prompt, history or [])
        payload = {
            "model": self.model,
            "messages": messages,
            "stream": True,
            "temperature": self.temperature,
            "max_tokens": self.max_tokens,
        }
        headers = {"Content-Type": "application/json"}
        if self.api_key:
            headers["Authorization"] = f"Bearer {self.api_key}"
        url = f"{self.base_url}/v1/chat/completions"
        sess = await self._get_session()
        async with sess.post(url, json=payload, headers=headers) as resp:
            if resp.status != 200:
                body = await resp.text()
                raise RuntimeError(f"gpu-mesh HTTP {resp.status}: {body[:200]}")
            # OpenAI SSE：data: {json}\n\n，结尾 data: [DONE]
            async for raw_line in resp.content:
                line = raw_line.decode("utf-8", errors="ignore").strip()
                if not line or not line.startswith("data:"):
                    continue
                data = line[5:].strip()
                if data == "[DONE]":
                    return
                try:
                    chunk = json.loads(data)
                except json.JSONDecodeError:
                    log.warning("gpu-mesh SSE 解析失败，跳过: %r", data[:200])
                    continue
                choices = chunk.get("choices") or []
                if choices:
                    delta = choices[0].get("delta") or {}
                    content = delta.get("content", "")
                    if content:
                        yield content


class OpenAICompatLLM(_BaseHTTPLLM):
    """通用 OpenAI 兼容云 API（DeepSeek/智谱GLM/OpenAI/通义/Kimi 等）流式（SSE）。

    与 GpuMeshLLM 同走 /v1/chat/completions，区别：
    - 加请求级 DEBUG 日志（线上故障排查必备）
    - 线上模型常驻，warmup 重写为 no-op（省 token + 省 1-3s 启动）
    - 由 build_llm 根据 backend（openai/预设厂商/gpu_mesh）统一构造，自动套预设 base_url
    """

    def __init__(self, base_url: str, model: str, system_prompt: str = "",
                 temperature: float = 0.7, max_tokens: int = 256,
                 api_key: str | None = None, timeout: float = 120.0,
                 keep_alive: str = "30m", num_ctx: int = 2048,
                 backend_tag: str = "openai"):
        super().__init__(base_url, model, system_prompt, temperature, max_tokens,
                         timeout, keep_alive, num_ctx)
        self.api_key = api_key
        self.backend_tag = backend_tag  # 仅用于日志区分（openai/deepseek/glm/...）

    async def chat_stream(
        self, prompt: str, history: list[Message] | None = None
    ) -> AsyncIterator[str]:
        messages = self._build_messages(prompt, history or [])
        payload = {
            "model": self.model,
            "messages": messages,
            "stream": True,
            "temperature": self.temperature,
            "max_tokens": self.max_tokens,
        }
        headers = {"Content-Type": "application/json"}
        if self.api_key:
            headers["Authorization"] = f"Bearer {self.api_key}"
        # base_url 可能已含 /v1（预设厂商如 openai），也可能不含（deepseek 根路径）
        path = "/chat/completions" if self.base_url.endswith("/v1") else "/v1/chat/completions"
        url = f"{self.base_url}{path}"
        log.debug("[%s] LLM POST %s model=%s", self.backend_tag, url, self.model)
        sess = await self._get_session()
        async with sess.post(url, json=payload, headers=headers) as resp:
            if resp.status != 200:
                body = await resp.text()
                raise RuntimeError(f"[{self.backend_tag}] HTTP {resp.status}: {body[:200]}")
            async for raw_line in resp.content:
                line = raw_line.decode("utf-8", errors="ignore").strip()
                if not line or not line.startswith("data:"):
                    continue
                data = line[5:].strip()
                if data == "[DONE]":
                    return
                try:
                    chunk = json.loads(data)
                except json.JSONDecodeError:
                    log.warning("[%s] SSE 解析失败，跳过: %r", self.backend_tag, data[:200])
                    continue
                choices = chunk.get("choices") or []
                if choices:
                    delta = choices[0].get("delta") or {}
                    content = delta.get("content", "")
                    if content:
                        yield content

    async def warmup(self) -> None:
        """线上云 API 不预热：模型服务端常驻，预热只浪费 token + 拖慢启动。"""
        log.info("[%s] 线上云 API，跳过预热（模型已常驻）", self.backend_tag)


def build_llm(backend: str, **kwargs) -> LLMEngine:
    """工厂：按 backend 构造 LLM 引擎。

    backend 取值：
      - ollama         本地 Ollama（NDJSON）
      - openai         通用 OpenAI 兼容（base_url 必须在 kwargs 给）
      - deepseek/glm/qwen/kimi  预设云厂商（自动套 CLOUD_PRESETS 的 base_url）
      - gpu_mesh       兼容旧 config（走 OpenAI 兼容网关，转 GpuMeshLLM）
      - disabled/none  降级 MockLLM

    api_key 解析：kwargs 里的值（来自 config.yaml）会被 DH_LLM_API_KEY 环境变量覆盖（安全优先）。
    """
    # ★ api_key：环境变量优先于 yaml（key 不进 git/EXE）
    env_key = os.environ.get("DH_LLM_API_KEY")
    if env_key:
        kwargs["api_key"] = env_key

    if backend == "ollama":
        # OllamaLLM（本地）不接受 api_key 参数，剔除避免 TypeError
        kwargs.pop("api_key", None)
        return OllamaLLM(**kwargs)
    if backend in CLOUD_PRESETS:
        # 预设云厂商：用户没写 base_url 时自动套预设
        if not kwargs.get("base_url") or kwargs["base_url"] == "http://127.0.0.1:11434":
            kwargs["base_url"] = CLOUD_PRESETS[backend]
            log.info("[%s] 使用预设 base_url: %s", backend, CLOUD_PRESETS[backend])
        return OpenAICompatLLM(backend_tag=backend, **kwargs)
    if backend == "openai":
        return OpenAICompatLLM(backend_tag="openai", **kwargs)
    if backend == "gpu_mesh":
        return GpuMeshLLM(**kwargs)
    if backend in ("mock", "disabled", "none"):
        from .mock import MockLLM
        reply = "（mock 模式预置回复）" if backend == "mock" else "[LLM disabled]"
        return MockLLM(reply)
    raise ValueError(f"未知 llm backend: {backend}（合法值: ollama/openai/deepseek/glm/qwen/kimi/gpu_mesh/mock/disabled）")
