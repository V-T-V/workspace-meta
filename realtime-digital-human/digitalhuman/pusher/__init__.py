"""推流层：把视频帧/音频推送到客户端。

预留 WebRTC 升级：MVP 用 ws_mjpeg（实现简单、首帧 <1s），
后续可新增 webrtc_aiortc 实现替换，业务层无感。
"""
from ..engines.base import Pusher
from .base import build_pusher

__all__ = ["Pusher", "build_pusher"]
