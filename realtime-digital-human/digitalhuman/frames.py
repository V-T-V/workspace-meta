"""帧编码协议。

MuseTalk subprocess 通过 stdin/stdout 与主进程通信。
没有标准协议——用户原方案的 `proc.stdout.read(1024*768)` 读"一帧"是伪代码，
无法可靠分帧（JPEG 长度不固定）。

本模块定义两种简单二进制分帧协议：

1. 长度前缀帧（LengthPrefix）：4 字节 big-endian 无符号长度 + payload
   - 用于 MuseTalk stdout 读 JPEG 帧、TTS stdout 读 PCM 片段
2. MJPEG over WS：每帧独立 JPEG，WS message 天然分界，无需额外协议

写帧格式（subprocess → 主进程 stdout）：
    [len:4B big-endian][payload:len bytes]

控制帧：len == 0 表示"一段音频的结束标记"（MuseTalk 据此 flush）；
       len == 0xFFFFFFFF 表示"流结束 EOF"。
"""
from __future__ import annotations

import asyncio
import logging
import struct
from collections.abc import AsyncIterator

_log = logging.getLogger(__name__)

# 4 字节无符号 int（big-endian）
_HEADER = struct.Struct(">I")
HEADER_SIZE = _HEADER.size  # 4
# 流结束哨兵
EOF_MARKER = 0xFFFFFFFF
# 段结束哨兵（一组音频/一帧序列结束，下游据此 flush）
SEGMENT_END = 0


def encode_frame(payload: bytes) -> bytes:
    """编码一帧：长度前缀 + payload。"""
    if len(payload) >= EOF_MARKER:
        raise ValueError(f"payload 过大（{len(payload)} >= {EOF_MARKER}），无法编码")
    return _HEADER.pack(len(payload)) + payload


def encode_eof() -> bytes:
    """编码流结束标记。"""
    return _HEADER.pack(EOF_MARKER)


def encode_segment_end() -> bytes:
    """编码段结束标记（payload 为空但非 EOF）。"""
    return _HEADER.pack(SEGMENT_END)


def _is_special(n: int) -> bool:
    return n == EOF_MARKER or n == SEGMENT_END


async def read_frame(reader) -> bytes | None:
    """从 asyncio.StreamReader 读一帧。

    返回:
        bytes: 帧 payload
        b"":   段结束标记
        None:  EOF
    """
    header = await reader.readexactly(HEADER_SIZE)
    (n,) = _HEADER.unpack(header)
    if n == EOF_MARKER:
        return None
    if n == SEGMENT_END:
        return b""
    payload = await reader.readexactly(n)
    return payload


async def iter_frames(reader) -> AsyncIterator[bytes]:
    """持续读帧直到 EOF，逐帧 yield payload（段结束标记 yield 为 b""）。

    ★ 捕获 IncompleteReadError（C4）：subprocess 崩溃时 stdout 断流，
    readexactly 抛 IncompleteReadError。视为异常 EOF，log 后正常结束，
    而非让异常冒泡导致整个管线报错且诊断信息弱。
    """
    while True:
        try:
            frame = await read_frame(reader)
        except asyncio.IncompleteReadError as e:
            # subprocess 提前关闭 stdout（崩溃/被 kill）
            _log.warning(
                "frame 流提前结束（读到 %d/%d 字节头），可能 subprocess 崩溃",
                len(e.partial), HEADER_SIZE,
            )
            return
        if frame is None:
            return
        yield frame


# ---------- WS MJPEG 帧封装 ----------

# WS 消息类型标识（与前端约定）。首字节为类型：
WS_MSG_FRAME = 0x01     # 视频帧（JPEG）
WS_MSG_AUDIO = 0x02     # 音频片段（PCM）—— 浏览器 → 服务端 也用这个
WS_MSG_TEXT = 0x03      # 文字（字幕/识别结果）
WS_MSG_END = 0x04       # 一段回复结束（服务端 → 浏览器）
WS_MSG_LATENCY = 0x05   # 延迟统计（JSON，服务端 → 浏览器）
WS_MSG_UTTERANCE_END = 0x10  # 用户说完一句（浏览器 → 服务端，触发 pipeline）
WS_MSG_INTERRUPT = 0x11      # 打断（服务端 → 浏览器，用户开口时停止当前回复）
WS_MSG_SET_PERSONA = 0x12    # 切换角色（浏览器 → 服务端，payload=persona_id utf8）
WS_MSG_SET_TTS_RATE = 0x13   # 设置语速（浏览器 → 服务端，payload=rate utf8 如 "+20%"）
WS_MSG_ENGINE_STATUS = 0x14  # 引擎状态（服务端 → 浏览器，JSON：哪些引擎降级了）
WS_MSG_TEXT_INPUT = 0x15     # 文字输入（浏览器 → 服务端，payload=utf8 文本，直接走 LLM）
WS_MSG_CLEAR_HISTORY = 0x16  # 清空对话（浏览器 → 服务端，遗忘历史）


def ws_pack(msg_type: int, payload: bytes) -> bytes:
    """打包 WS 二进制消息：1 字节类型 + payload。

    P1-3：用 bytearray 预分配，省一次 bytes 拼接的全量拷贝（高频帧收益）。
    """
    out = bytearray(len(payload) + 1)
    out[0] = msg_type
    out[1:] = payload
    return bytes(out)


def ws_unpack(data: bytes) -> tuple[int, bytes]:
    """解包 WS 二进制消息。"""
    if not data:
        raise ValueError("空 WS 消息")
    return data[0], data[1:]


def ws_pack_text(text: str) -> bytes:
    return ws_pack(WS_MSG_TEXT, text.encode("utf-8"))


def ws_pack_frame(jpeg: bytes) -> bytes:
    return ws_pack(WS_MSG_FRAME, jpeg)


def ws_pack_audio(pcm: bytes) -> bytes:
    return ws_pack(WS_MSG_AUDIO, pcm)
