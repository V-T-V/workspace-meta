"""MuseTalk 唇形引擎测试（subprocess 协议闭环）。

验证：
1. MuseTalkLipSync 能启动 scripts/musetalk_render.py subprocess
2. 通过 stdin 喂音频，从 stdout 收到合法 JPEG 帧
3. 帧数与音频时长/fps 匹配
4. 缺脚本时抛 RuntimeError

注意：此测试依赖 opencv-python（musetalk_render.py 占位渲染用）。
缺 cv2 时跳过。
"""
import asyncio

import pytest

from digitalhuman.engines.lipsync_musetalk import MuseTalkLipSync, build_lipsync


def _has_cv2() -> bool:
    try:
        import cv2  # noqa: F401
        return True
    except ImportError:
        return False


@pytest.mark.skipif(not _has_cv2(), reason="缺 opencv-python，跳过 MuseTalk 渲染测试")
@pytest.mark.asyncio
async def test_musetalk_renders_jpeg_frames():
    """喂 1 秒音频 @5fps，应收到 5 帧 JPEG（单次模式）。"""
    import numpy as np
    from digitalhuman.engines.lipsync_musetalk import MuseTalkLipSync
    ls = MuseTalkLipSync(
        musetalk_script="scripts/musetalk_render.py",
        fps=5, width=64, height=64, persistent=False,  # 单测用单次模式，避免跨 loop
    )
    audio = (np.sin(np.linspace(0, 440 * 2 * np.pi, 16000))
             * 0.5 * 32767).astype("<i2").tobytes()

    async def audio_gen():
        yield audio

    frames = []
    async for frame in ls.render_stream(b"", audio_gen()):
        frames.append(frame)

    assert len(frames) == 5, f"应收到 5 帧（1s @5fps），实际 {len(frames)}"
    for f in frames:
        assert f[:2] == b"\xff\xd8", f"JPEG 应以 FFD8 开头，实际 {f[:2].hex()}"
        assert f[-2:] == b"\xff\xd9", "JPEG 应以 FFD9 结尾"


@pytest.mark.skipif(not _has_cv2(), reason="缺 opencv-python")
@pytest.mark.asyncio
async def test_musetalk_with_portrait():
    """喂 portrait（假数据）+ 音频，应仍输出帧。"""
    import numpy as np
    from digitalhuman.engines.lipsync_musetalk import MuseTalkLipSync
    ls = MuseTalkLipSync(
        musetalk_script="scripts/musetalk_render.py",
        fps=10, width=32, height=32, persistent=False,
    )
    audio = (np.random.rand(8000) * 0.5 * 32767).astype("<i2").tobytes()
    portrait = b"\x89PNG\r\n\x1a\n" + b"\x00" * 100

    async def audio_gen():
        yield audio

    frames = []
    async for frame in ls.render_stream(portrait, audio_gen()):
        frames.append(frame)
    assert len(frames) >= 1
    assert all(f[:2] == b"\xff\xd8" for f in frames)


@pytest.mark.skipif(not _has_cv2(), reason="缺 opencv-python")
@pytest.mark.asyncio
async def test_musetalk_persistent_reuses_process():
    """★ 常驻模式：多段音频复用同一 subprocess（省启动开销）。

    验证：连续 render_stream 两次，第二次的首帧延迟应远小于第一次（省了 Python 启动+import）。
    """
    import time
    import numpy as np
    from digitalhuman.engines.lipsync_musetalk import MuseTalkLipSync
    ls = MuseTalkLipSync(
        musetalk_script="scripts/musetalk_render.py",
        fps=5, width=32, height=32, persistent=True,
    )
    try:
        audio = (np.sin(np.linspace(0, 440 * 2 * np.pi, 8000))
                 * 0.5 * 32767).astype("<i2").tobytes()  # 0.5s

        async def gen():
            yield audio

        # 第一段：含 subprocess 启动开销
        t0 = time.monotonic()
        n1 = 0
        async for f in ls.render_stream(b"", gen()):
            n1 += 1
        first_seg_latency = time.monotonic() - t0

        # 第二段：应复用 proc，省启动开销
        t1 = time.monotonic()
        n2 = 0
        first2 = None
        async for f in ls.render_stream(b"", gen()):
            if first2 is None:
                first2 = time.monotonic() - t1
            n2 += 1
        second_seg_latency = time.monotonic() - t1

        assert n1 > 0 and n2 > 0, f"两段都应有帧: n1={n1} n2={n2}"
        # ★ 核心断言：第二段延迟应明显小于第一段（省了 ~1.5s 启动）
        assert second_seg_latency < first_seg_latency * 0.7, (
            f"常驻复用应更快：第一段 {first_seg_latency:.2f}s，"
            f"第二段 {second_seg_latency:.2f}s"
        )
    finally:
        await ls.close()


@pytest.mark.skipif(not _has_cv2(), reason="缺 opencv-python")
@pytest.mark.asyncio
async def test_musetalk_persistent_close_terminates_proc():
    """常驻模式 close() 应终止 subprocess。"""
    from digitalhuman.engines.lipsync_musetalk import MuseTalkLipSync
    ls = MuseTalkLipSync(
        musetalk_script="scripts/musetalk_render.py",
        fps=5, width=32, height=32, persistent=True,
    )
    await ls._get_or_start_proc()
    assert ls._proc is not None
    assert ls._proc.returncode is None  # 运行中
    await ls.close()
    assert ls._proc is None


def test_musetalk_missing_script_raises():
    """缺脚本时应抛 RuntimeError。"""
    with pytest.raises(RuntimeError):
        MuseTalkLipSync(musetalk_script="scripts/nonexistent.py")
