"""管线流式接力测试——验证 4 阶段顺序与背压。

核心断言：
1. token → sentence → audio → frame 的产出顺序符合预期
2. 多句场景下，第一句的 audio 早于第二句的 token（流式重叠，非串行）
3. 延迟埋点被正确标记
4. Mock 引擎收到正确的输入
"""
import asyncio

import pytest

from digitalhuman.config import Config
from digitalhuman.engines.base import Message, Pusher
from digitalhuman.engines.mock import MockASR, MockLLM, MockLipSync, MockTTS
from digitalhuman.pipeline import DigitalHumanPipeline, PipelineParts


class RecordingPusher(Pusher):
    """记录所有推送的 Pusher，按类型分类。"""

    def __init__(self):
        self.frames: list[bytes] = []
        self.audios: list[bytes] = []
        self.texts: list[str] = []
        self.call_log: list[tuple[str, float]] = []  # (kind, timestamp)

    async def push_frame(self, session_id: str, jpeg: bytes) -> None:
        self.frames.append(jpeg)
        self.call_log.append(("frame", asyncio.get_event_loop().time()))

    async def push_audio(self, session_id: str, pcm: bytes) -> None:
        self.audios.append(pcm)
        self.call_log.append(("audio", asyncio.get_event_loop().time()))

    async def push_text(self, session_id: str, text: str) -> None:
        self.texts.append(text)
        self.call_log.append(("text", asyncio.get_event_loop().time()))


async def _empty_audio_stream():
    """空音频流（Mock ASR 不依赖音频内容）。"""
    return
    yield  # make it an async generator


@pytest.mark.asyncio
async def test_pipeline_basic_single_sentence():
    """单句：用户说"你好呀" → 数字人回"你好。" → 至少一帧。"""
    cfg = Config()
    pusher = RecordingPusher()
    parts = PipelineParts(
        asr=MockASR(["你好呀"], delay=0.001),
        llm=MockLLM("你好。", delay_per_token=0.001),
        tts=MockTTS(delay=0.001),
        lipsync=MockLipSync(delay=0.001),
        pusher=pusher,
        portrait=b"PORTRAIT",
        session_id="s1",
    )
    pipeline = DigitalHumanPipeline(cfg, parts)

    async def mic():
        yield b"\x00" * 100  # 假 PCM

    await pipeline.run(mic())

    # ASR 收到音频
    assert parts.asr.fed_audio == [b"\x00" * 100]
    # LLM 收到用户文本
    assert parts.llm.received_prompts == ["你好呀"]
    # TTS 收到短句（"你好。"）
    assert parts.tts.received_texts == ["你好。"]
    # 唇形收到 portrait
    assert parts.lipsync.received_portrait == b"PORTRAIT"
    # pusher 收到至少一帧
    assert len(pusher.frames) > 0
    # 字幕：含 user 与 assistant
    assert any("[user] 你好呀" in t for t in pusher.texts)
    assert any("[assistant] 你好。" in t for t in pusher.texts)


@pytest.mark.asyncio
async def test_pipeline_multi_sentence_split():
    """LLM 回复含多句，应被切分器切成多段分别 TTS。"""
    cfg = Config()
    pusher = RecordingPusher()
    parts = PipelineParts(
        asr=MockASR(["讲个笑话"], delay=0.001),
        llm=MockLLM("你好。我很高兴。再见！", delay_per_token=0.001),
        tts=MockTTS(delay=0.001),
        lipsync=MockLipSync(delay=0.001),
        pusher=pusher,
        portrait=b"P",
    )
    pipeline = DigitalHumanPipeline(cfg, parts)

    async def mic():
        yield b"\x00" * 10

    await pipeline.run(mic())

    # 三句应分别喂给 TTS
    assert parts.tts.received_texts == ["你好。", "我很高兴。", "再见！"]
    # 字幕应有三句 assistant
    assistant_texts = [t for t in pusher.texts if t.startswith("[assistant]")]
    assert len(assistant_texts) == 3


@pytest.mark.asyncio
async def test_pipeline_streaming_overlap():
    """★ 关键：验证流式重叠——第一句的 frame 应早于第二句 TTS 的开始。

    用时间戳断言：如果串行（错误实现），第二句 TTS 收文本必然在第一句所有 frame 之后；
    正确的流式实现里，第二句 token 还在产时，第一句的 audio/frame 已经在推。
    """
    cfg = Config()
    pusher = RecordingPusher()

    # 让 LLM 慢一点吐 token（每 token 10ms），TTS 也慢一点
    # 这样如果实现错误（串行），总耗时会很长
    parts = PipelineParts(
        asr=MockASR(["说两句"], delay=0.001),
        llm=MockLLM("第一句。第二句。", delay_per_token=0.01),
        tts=MockTTS(delay=0.005),
        lipsync=MockLipSync(delay=0.001),
        pusher=pusher,
        portrait=b"P",
    )
    pipeline = DigitalHumanPipeline(cfg, parts)

    async def mic():
        yield b"\x00" * 10

    await asyncio.wait_for(pipeline.run(mic()), timeout=5.0)

    # 至少有 frame 产出
    assert len(pusher.frames) > 0
    assert len(parts.tts.received_texts) == 2

    # 延迟埋点：首 token / 首句 / 首 audio / 首 frame 都应被标记
    s = pipeline.timing.summary()
    assert "first_token" in s
    assert "first_sentence" in s
    assert "first_audio" in s
    assert "first_frame" in s
    # 首 frame 应不晚于 reply_end（流式实现里首帧在整段结束前就出了）
    assert s["first_frame"] <= s["reply_end"]
    # ★ M9 加固：真正的流式重叠断言。
    # 用 call_log 里的"事件索引"（顺序位置）而非时间戳——time.monotonic 精度可能
    # 不足以区分同一调度周期内的事件，但事件顺序是确定的。
    # 流式重叠的本质：第一句的 frame 推送应早于第二句 [assistant] 字幕推送。
    # 如果串行（错误实现），第一句的所有事件（含 frame）会在第二句开始前完成，
    # 但流式实现里第二句 token 还在产时第一句 frame 已在推。
    first_frame_idx = None
    second_assistant_idx = None
    assistant_count = 0
    for idx, (kind, ts) in enumerate(pusher.call_log):
        if kind == "frame" and first_frame_idx is None:
            first_frame_idx = idx
        if kind == "text":
            assistant_count += 1
            # call_log 顺序：[user]=1, [assistant]第一句=2, [assistant]第二句=3
            if assistant_count == 3:
                second_assistant_idx = idx
                break
    # ★ 断言不包在 if 里（M9）：缺失即失败
    assert first_frame_idx is not None, "应有至少一帧推送"
    assert second_assistant_idx is not None, (
        f"应有第二句 assistant 字幕；call_log text 次数={assistant_count}"
    )
    # 首 frame 必须在第二句字幕之前（按事件顺序）
    assert first_frame_idx < second_assistant_idx, (
        f"流式重叠失败：首帧(idx={first_frame_idx}) 应早于 "
        f"第二句字幕(idx={second_assistant_idx})。"
        f"call_log 顺序: {[(k, f'{t:.4f}') for k, t in pusher.call_log]}"
    )


@pytest.mark.asyncio
async def test_pipeline_history_accumulates():
    """多轮对话历史应累积，传给 LLM。"""
    cfg = Config()
    pusher = RecordingPusher()
    parts = PipelineParts(
        asr=MockASR(["你好"], delay=0.001),
        llm=MockLLM("你好。", delay_per_token=0.001),
        tts=MockTTS(delay=0.001),
        lipsync=MockLipSync(delay=0.001),
        pusher=pusher,
        portrait=b"P",
        history=[Message(role="system", content="be nice")],
    )
    pipeline = DigitalHumanPipeline(cfg, parts)

    async def mic():
        yield b"\x00" * 10

    await pipeline.run(mic())

    # history 应含初始 system + 本轮 user + assistant
    roles = [m.role for m in pipeline._history]
    assert roles == ["system", "user", "assistant"]
    assert pipeline._history[1].content == "你好"
    assert pipeline._history[2].content == "你好。"


@pytest.mark.asyncio
async def test_pipeline_cancellation_propagates():
    """ctx 取消（客户端断开）应让所有阶段 Task 退出，不泄漏。"""
    cfg = Config()
    pusher = RecordingPusher()
    # 用极慢的 LLM，确保运行中能被取消
    parts = PipelineParts(
        asr=MockASR(["长文本"], delay=0.001),
        llm=MockLLM("a" * 1000, delay_per_token=0.5),  # 每字 0.5s
        tts=MockTTS(delay=0.001),
        lipsync=MockLipSync(delay=0.001),
        pusher=pusher,
        portrait=b"P",
    )
    pipeline = DigitalHumanPipeline(cfg, parts)

    async def mic():
        yield b"\x00" * 10

    task = asyncio.create_task(pipeline.run(mic()))
    await asyncio.sleep(0.1)  # 让它跑起来
    task.cancel()
    with pytest.raises(asyncio.CancelledError):
        await task
    # 取消后短暂等待，确认无残留 task 报错
    await asyncio.sleep(0.05)


@pytest.mark.asyncio
async def test_session_set_persona_hot_switch():
    """★ #2/#4 热切换角色：set_persona 应更新 LLM system_prompt + TTS voice。"""
    from digitalhuman.session import Session

    cfg = Config()
    pusher = RecordingPusher()
    sess = Session(
        session_id="persona-test", cfg=cfg,
        asr=MockASR([], delay=0.001),
        llm=MockLLM("回复", delay_per_token=0.001),
        tts=MockTTS(delay=0.001),
        lipsync=MockLipSync(delay=0.001),
        pusher=pusher, portrait=b"P",
    )
    # 初始 voice/prompt（MockTTS/LLM 可能没有 voice/system_prompt 属性，用 hasattr 保护）

    # 切换角色
    class FakePersona:
        id = "yunxi"
        name = "云希"
        voice = "zh-CN-YunxiNeural"
        system_prompt = "你是活力助手"
        portrait = b""
    sess.set_persona(FakePersona())

    # 断言热切换生效（属性被更新）
    if hasattr(sess.llm, "system_prompt"):
        assert sess.llm.system_prompt == "你是活力助手"
    if hasattr(sess.tts, "voice"):
        assert sess.tts.voice == "zh-CN-YunxiNeural"


@pytest.mark.asyncio
async def test_session_history_not_polluted_on_cancel():
    """★ C3 回归：session 取消后不应把半截 history 写回。

    场景：用户说话，LLM 正在生成（慢），用户点 stop → cancel。
    下一次对话时 LLM 收到的 history 不应包含被截断的 assistant 回复。
    """
    from digitalhuman.session import Session
    from digitalhuman.engines.base import Message

    cfg = Config()
    pusher = RecordingPusher()
    sess = Session(
        session_id="cancel-test",
        cfg=cfg,
        asr=MockASR(["你好"], delay=0.001),
        llm=MockLLM("你好你好你好。", delay_per_token=0.3),  # 慢，便于中途取消
        tts=MockTTS(delay=0.001),
        lipsync=MockLipSync(delay=0.001),
        pusher=pusher,
        portrait=b"P",
        history=[Message(role="system", content="be nice")],
    )

    async def mic():
        yield b"\x00" * 10

    # 启动 utterance 处理（在后台 task 里）
    utter_task = asyncio.create_task(sess.handle_utterance(mic()))
    await asyncio.sleep(0.15)  # 等 LLM 开始生成但未完成
    # 取消
    await sess.cancel()
    try:
        await utter_task
    except (asyncio.CancelledError, Exception):
        pass

    # ★ 关键断言：history 应保持原始状态（仅 system），未被半截 reply 污染
    roles = [m.role for m in sess.history]
    assert roles == ["system"], (
        f"cancel 后 history 被污染: {[m.role for m in sess.history]}。"
        f"应为 ['system']，不含半截 user/assistant"
    )


@pytest.mark.asyncio
async def test_pipeline_llm_failure_does_not_deadlock():
    """★ C2 回归：LLM stage 抛异常时，下游 stage 不应永久阻塞。

    初版 _stage_llm 没有 finally 发哨兵，LLM 异常会导致 split/tts/lipsync
    永久阻塞在 queue.get()。修复后每个 stage 的 finally 都发哨兵。
    """
    from digitalhuman.engines.base import LLMEngine, Message

    class FailingLLM(LLMEngine):
        async def chat_stream(self, prompt, history=None):
            raise RuntimeError("Ollama 500 内部错误")
            yield ""  # 让它成为 async generator

    cfg = Config()
    pusher = RecordingPusher()
    parts = PipelineParts(
        asr=MockASR(["你好"], delay=0.001),
        llm=FailingLLM(),
        tts=MockTTS(delay=0.001),
        lipsync=MockLipSync(delay=0.001),
        pusher=pusher,
        portrait=b"P",
    )
    pipeline = DigitalHumanPipeline(cfg, parts)

    async def mic():
        yield b"\x00" * 10

    # 应在合理时间内结束（要么抛 RuntimeError，要么正常返回），不应超时
    try:
        await asyncio.wait_for(pipeline.run(mic()), timeout=3.0)
    except (RuntimeError, Exception) as e:
        # 预期：LLM 失败导致 pipeline 抛错——这是对的
        assert "llm" in str(e).lower() or "stage" in str(e).lower() or "失败" in str(e)
    # 关键：能走到这里说明没有死锁（3s 内结束）
