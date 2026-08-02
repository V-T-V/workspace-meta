"""实时数字人单机服务。

核心是流式管线重叠：ASR → LLM → 句切分 → TTS → 唇形 → 推流。
四个环节均为 asyncio 流，靠 asyncio.Queue 接力，绝不串行等待。
"""
try:
    from importlib.metadata import version as _version
    __version__ = _version("realtime-digital-human")
except Exception:
    __version__ = "0.0.0"  # 未 pip install 时（开发模式）
