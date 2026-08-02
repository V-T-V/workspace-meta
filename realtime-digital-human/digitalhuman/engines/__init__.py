"""四环节引擎抽象（ASR / LLM / TTS / 唇形）+ 工厂。

设计参照 gpu-mesh 的 engine.Engine 模式：每个环节定义抽象基类 + 多个具体实现 + 工厂分发。
所有方法都是 async generator（或返回 AsyncIterator），便于流式接力。
"""
from .base import (
    ASREngine,
    LLMEngine,
    LipSyncEngine,
    Message,
    Pusher,
    TTSEngine,
)

__all__ = [
    "ASREngine",
    "LLMEngine",
    "TTSEngine",
    "LipSyncEngine",
    "Pusher",
    "Message",
]
