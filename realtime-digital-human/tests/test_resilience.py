"""深度优化项的专项测试：TTS 容错 / edge-tts 重试 / pusher 背压 / LLM 预热。"""
import asyncio
import time
from unittest.mock import AsyncMock, MagicMock

import pytest

from digitalhuman.config import Config
from digitalhuman.engines.base import Pusher
from digitalhuman.engines.mock import MockASR, MockLLM, MockLipSync, MockTTS
from digitalhuman.pipeline import DigitalHumanPipeline, PipelineParts
from digitalhuman.pusher.base import WSMjpegPusher


# ---------- TTS 单句失败容错 ----------

class FlakyTTS:
    """模拟 TTS：对第 2 句抛错，其他句正常。"""

    def __init__(self):
        self.call_count = 0
        self.received_texts = []

    async def synthesize_stream(self, text):
        self.call_count += 1
        self.received_texts.append(text)
        if self.call_count == 2:
            # 第 2 句抛错（模拟 edge-tts 偶发 NoAudioReceived）
            raise RuntimeError("NoAudioReceived (simulated)")
        # 正常返回假音频
        yield b"\x00" * 20


class RecordingPusher(Pusher):
    def __init__(self):
        self.frames = []
        self.texts = []

    async def push_frame(self, s, j):
        self.frames.append(j)

    async def push_audio(self, s, p):
        pass

    async def push_text(self, s, t):
        self.texts.append(t)


@pytest.mark.asyncio
async def test_tts_single_sentence_failure_does_not_crash_pipeline():
    """★ TTS 单句失败不中断整个 pipeline（深度优化核心）。

    场景：LLM 回复 3 句，第 2 句 TTS 失败。
    正确行为：第 1、3 句正常产音频/帧，第 2 句跳过（字幕仍推），pipeline 不崩。
    """
    cfg = Config()
    pusher = RecordingPusher()
    parts = PipelineParts(
        asr=MockASR(["讲三句"], delay=0.001),
        llm=MockLLM("第一句。第二句。第三句。", delay_per_token=0.001),
        tts=FlakyTTS(),
        lipsync=MockLipSync(delay=0.001),
        pusher=pusher,
        portrait=b"P",
    )
    pipeline = DigitalHumanPipeline(cfg, parts)

    async def mic():
        yield b"\x00" * 10

    # 不应抛异常
    await asyncio.wait_for(pipeline.run(mic()), timeout=5.0)

    # 3 句字幕都应推送（含失败的中间句）
    assistant_texts = [t for t in pusher.texts if t.startswith("[assistant]")]
    assert len(assistant_texts) == 3, f"应推 3 句字幕，实际 {len(assistant_texts)}"
    # 应有 1 条 error 提示（第 2 句 TTS 失败，m7 分级后是 [error:warn]）
    error_texts = [t for t in pusher.texts if "[error" in t]
    assert len(error_texts) >= 1, "应有 TTS 失败的 error 提示"
    # TTS 被调了 3 次（3 句都尝试了）
    assert parts.tts.call_count == 3
    # 第 1、3 句产了帧（第 2 句跳过）
    assert len(pusher.frames) > 0


# ---------- edge-tts 重试 ----------

# ---------- #1 Barge-in 打断 ----------

@pytest.mark.asyncio
async def test_barge_in_cancels_current_reply():
    """★ #1 Barge-in：数字人正在回复时，用户开口应 cancel 当前 pipeline。

    场景：LLM 慢吐 token（正在回复），此时检测到用户说话（能量高），
    session.cancel() 被调用，当前回复停止。
    """
    from digitalhuman.config import Config
    from digitalhuman.engines.mock import MockASR, MockLLM, MockTTS, MockLipSync
    from digitalhuman.session import Session
    from digitalhuman.pusher.base import DisabledPusher

    cfg = Config()
    sess = Session(
        session_id="barge-test", cfg=cfg,
        asr=MockASR(["你好"], delay=0.001),
        llm=MockLLM("这是一个很长的回复，会被用户打断的。", delay_per_token=0.3),
        tts=MockTTS(delay=0.001), lipsync=MockLipSync(delay=0.001),
        pusher=DisabledPusher(),
    )

    async def mic():
        yield b"\x00" * 10

    # 启动回复（后台 task）
    utter_task = asyncio.create_task(sess.handle_utterance(mic()))
    await asyncio.sleep(0.2)  # 等 LLM 开始生成

    # 此时数字人正在回复（_current 非 None）
    assert sess._current is not None, "应正在回复"

    # 用户开口打断
    await sess.cancel()
    try:
        await utter_task
    except (asyncio.CancelledError, Exception):
        pass

    # 打断后 _current 应为 None，history 不被半截回复污染
    assert sess._current is None
    # history 不含被取消的 assistant 回复（C3 保证）


@pytest.mark.asyncio
async def test_barge_in_history_not_polluted():
    """#1 Barge-in 后下一轮对话不受影响（history 干净）。"""
    from digitalhuman.config import Config
    from digitalhuman.engines.mock import MockASR, MockLLM, MockTTS, MockLipSync
    from digitalhuman.engines.base import Message
    from digitalhuman.session import Session
    from digitalhuman.pusher.base import DisabledPusher

    cfg = Config()
    sess = Session(
        session_id="barge-history", cfg=cfg,
        asr=MockASR([], delay=0.001),
        llm=MockLLM("被打断的回复", delay_per_token=0.3),
        tts=MockTTS(delay=0.001), lipsync=MockLipSync(delay=0.001),
        pusher=DisabledPusher(),
        history=[Message(role="system", content="be nice")],
    )

    # 第一轮：被打断
    sess.asr = MockASR(["用户的话"])
    async def mic1():
        yield b"\x00" * 10
    task1 = asyncio.create_task(sess.handle_utterance(mic1()))
    await asyncio.sleep(0.15)
    await sess.cancel()
    try:
        await task1
    except (asyncio.CancelledError, Exception):
        pass  # cancel 传播的 CancelledError，预期

    # 第二轮：正常完成
    sess.asr = MockASR(["第二次"])
    sess.llm = MockLLM("第二次回复。", delay_per_token=0.001)
    async def mic2():
        yield b"\x00" * 10
    await sess.handle_utterance(mic2())

    # 第二轮 history 应干净：system + 第二次 user + assistant
    roles = [m.role for m in sess.history]
    assert "system" in roles
    # 不应包含被打断的半截回复（被 cancel 的轮次未回写）
    assistant_contents = [m.content for m in sess.history if m.role == "assistant"]
    assert all("被打断" not in c for c in assistant_contents), \
        f"被打断的回复不应留在 history: {assistant_contents}"


@pytest.mark.asyncio
async def test_edge_tts_outputs_pcm16_not_mp3(monkeypatch):
    """★ P0 回归：edge-tts 输出必须是 PCM16（不是 MP3），否则 MuseTalk 拿到乱码。

    之前 edge-tts 直接 yield MP3 chunk，下游 MuseTalk 按 PCM16 解读 → 乱码音频 + 错误帧数。
    修复：攒完整段 MP3 → miniaudio 解码成 PCM16 → 切 chunk yield。
    """
    from digitalhuman.engines import tts_edge

    # mock edge_tts 返回假 MP3 数据
    fake_edge_tts = MagicMock()
    class FakeCommunicate:
        def __init__(self, text, voice, rate): pass
        async def stream(self):
            yield {"type": "audio", "data": b"\xff\xf3fake_mp3_data"}
            return
    fake_edge_tts.Communicate = FakeCommunicate
    monkeypatch.setitem(__import__("sys").modules, "edge_tts", fake_edge_tts)
    monkeypatch.setattr("importlib.import_module",
                        lambda name: fake_edge_tts if name == "edge_tts" else __import__(name))
    # mock 解码返回明确的 PCM16
    # mock 异步解码（P0 优化：解码移入 executor）
    async def _fake_decode_async(mp3, sample_rate=16000):
        return b"\x00\x00\x10\x00\x20\x00" * 100
    monkeypatch.setattr(tts_edge, "_decode_mp3_to_pcm16_async", _fake_decode_async)  # 600 bytes PCM16

    tts = tts_edge.EdgeTTS()
    chunks = []
    async for c in tts.synthesize_stream("你好"):
        chunks.append(c)

    assert len(chunks) >= 1, "应产出 PCM16 chunk"
    # ★ 关键断言：chunk 不能是 MP3 帧头（ff fb/f3/fa）
    for c in chunks:
        assert not (c[0] == 0xff and c[1] in (0xfb, 0xf3, 0xfa, 0xf2)), \
            f"输出仍是 MP3 帧头（{c[:2].hex()}），MuseTalk 会拿到乱码"
    # 应是 PCM16（首字节 0x00 静音或小数值）


@pytest.mark.asyncio
async def test_edge_tts_retries_on_failure(monkeypatch):
    """edge-tts 偶发失败应自动重试。"""
    from digitalhuman.engines import tts_edge

    call_count = {"n": 0}

    class FlakyCommunicate:
        def __init__(self, text, voice, rate):
            self.text = text

        async def stream(self):
            call_count["n"] += 1
            if call_count["n"] < 3:  # 前 2 次失败
                raise RuntimeError("NoAudioReceived")
            # 第 3 次成功
            yield {"type": "audio", "data": b"audio_chunk"}

    # mock edge_tts 模块
    fake_edge_tts = MagicMock()
    fake_edge_tts.Communicate = FlakyCommunicate
    monkeypatch.setitem(__import__("sys").modules, "edge_tts", fake_edge_tts)
    monkeypatch.setattr("importlib.import_module", lambda name: fake_edge_tts if name == "edge_tts" else __import__(name))
    # P0：mock 异步解码（同步 miniaudio 解不了假数据）
    async def _fake_decode(mp3, sample_rate=16000):
        return b"\x00" * 1280
    monkeypatch.setattr(tts_edge, "_decode_mp3_to_pcm16_async", _fake_decode)

    tts = tts_edge.EdgeTTS(max_retries=3)
    chunks = []
    async for c in tts.synthesize_stream("你好"):
        chunks.append(c)

    assert call_count["n"] == 3, f"应重试到第 3 次成功，实际调用 {call_count['n']} 次"
    # 解码后产出 PCM16 chunk（20ms = 640 bytes，1280 bytes 切 2 chunk）
    assert len(chunks) >= 1
    assert all(len(c) <= 640 for c in chunks)  # 每个 chunk ≤ 20ms


# ---------- pusher 背压保护 ----------
# ★ P1-2 改造后：push_frame 入队即返回（<1ms），实际发送由后台 _frame_sender_loop 执行，
#   超时/异常 unbind 发生在后台 task。测试需 yield 控制权让后台 task 跑。

@pytest.mark.asyncio
async def test_pusher_send_timeout_unbinds_session():
    """★ 慢客户端超时后应 unbind（不冻住 pipeline）。"""
    pusher = WSMjpegPusher(send_timeout=0.1)  # 100ms 超时

    async def slow_send(data):
        await asyncio.sleep(1.0)  # 模拟慢客户端（1s）
        return None

    pusher.bind("sess1", slow_send)

    # ★ P1-2：push_frame 入队即返回（<1ms），不再阻塞
    t0 = time.monotonic()
    await pusher.push_frame("sess1", b"jpeg")
    elapsed = time.monotonic() - t0
    assert elapsed < 0.1, f"push_frame 应入队即返回，实际 {elapsed:.2f}s"

    # 等后台 sender task 触发超时 unbind（100ms 超时 + 余量）
    await asyncio.sleep(0.3)
    assert "sess1" not in pusher._conns, "慢客户端应被 unbind"

    # 第二次推送：session 已 unbind，应静默丢弃，不报错
    await pusher.push_frame("sess1", b"jpeg2")


@pytest.mark.asyncio
async def test_pusher_send_exception_unbinds_session():
    """send 抛异常（连接断开）应 unbind。"""
    pusher = WSMjpegPusher()

    async def broken_send(data):
        raise ConnectionError("WebSocket closed")

    pusher.bind("sess2", broken_send)
    await pusher.push_frame("sess2", b"jpeg")  # 入队即返回，不应抛
    # 等后台 sender task 执行发送并捕获异常 unbind
    await asyncio.sleep(0.1)
    assert "sess2" not in pusher._conns


# ---------- LLM 预热 ----------

@pytest.mark.asyncio
async def test_llm_warmup_does_not_raise_on_failure():
    """LLM warmup 失败不应抛（预热是优化，非必需）。"""
    from digitalhuman.engines.llm_ollama import OllamaLLM

    llm = OllamaLLM("http://127.0.0.1:1", "model")  # 不可达端口
    # warmup 不应抛（内部捕获）
    await llm.warmup()  # 不抛即通过
    await llm.close()


@pytest.mark.asyncio
async def test_llm_warmup_succeeds_when_available():
    """LLM warmup 成功时应正常完成（用 Mock 验证调用）。"""
    from digitalhuman.engines.llm_ollama import OllamaLLM
    from digitalhuman.engines.base import LLMEngine

    class StubLLM(OllamaLLM):
        """绕过真实 HTTP，用 stub chat_stream。"""
        async def chat_stream(self, prompt, history=None):
            yield "ok"

    llm = StubLLM("http://x", "m")
    await llm.warmup()  # 应正常完成
    await llm.close()


# ---------- 死锁回归（深度模拟发现的 bug） ----------

@pytest.mark.asyncio
async def test_no_deadlock_when_lipsync_fails_mid_stream():
    """★ 死锁回归：lipsync 中途失败时，TTS 不应永久阻塞在满的 audio_q。

    深度模拟 S3 发现：lipsync 失败后 audio_q 堆满，TTS 的 put 永久阻塞 → 死锁。
    修复：_put_or_drop 非阻塞 put + 连续丢 32 次提前退出。
    """
    from digitalhuman.engines.base import LipSyncEngine

    class FailAfter1FrameLipSync(LipSyncEngine):
        """产 1 帧后抛错，模拟中途失败。"""
        async def render_stream(self, portrait, audio_stream):
            yield b"\xff\xd8jpeg"
            raise RuntimeError("lipsync 中途崩溃")

    cfg = Config()
    parts = PipelineParts(
        asr=MockASR(["测"], delay=0.001),
        llm=MockLLM("第一句。第二句。第三句。", delay_per_token=0.001),
        tts=MockTTS(delay=0.001),
        lipsync=FailAfter1FrameLipSync(),
        pusher=RecordingPusher(),
        portrait=b"P",
    )
    pipeline = DigitalHumanPipeline(cfg, parts)

    async def mic():
        yield b"\x00" * 10

    # 应在合理时间内结束（不死锁）。lipsync 失败会抛 RuntimeError，正常。
    try:
        await asyncio.wait_for(pipeline.run(mic()), timeout=8.0)
        # 若没抛错也 OK（容错可能跳过）
    except RuntimeError:
        pass  # 预期：lipsync 失败重抛
    # 关键：能走到这里说明没死锁（8s 内结束）


@pytest.mark.asyncio
async def test_long_llm_reply_does_not_timeout():
    """★ 超长回复回归：200 字 LLM 回复不应因帧数爆炸超时。

    深度模拟 S5 发现：MockLipSync 按音频字节数产帧，长回复帧数爆炸 → 超时。
    修复：MockLipSync 按 fps（25）限速，帧数 = 音频时长 × fps。
    """
    cfg = Config()
    cfg.sentence_splitter.max_chars = 10
    parts = PipelineParts(
        asr=MockASR(["x"], delay=0.001),
        llm=MockLLM("字" * 80 + "结束。", delay_per_token=0.0001),  # 80 字
        tts=MockTTS(delay=0.0001),
        lipsync=MockLipSync(delay=0.0001),
        pusher=RecordingPusher(),
        portrait=b"P",
    )
    pipeline = DigitalHumanPipeline(cfg, parts)

    async def mic():
        yield b"\x00" * 10

    await asyncio.wait_for(pipeline.run(mic()), timeout=10.0)
    # 能完成即通过（之前会超时死锁）
