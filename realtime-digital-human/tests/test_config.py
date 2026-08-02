"""Config 加载与校验测试。"""
from digitalhuman.config import (
    ASRConfig,
    Config,
    ConfigError,
    from_dict,
    from_yaml,
    validate,
)
import os
import tempfile

import pytest


def test_default_config():
    cfg = Config()
    assert cfg.server.port == 8000
    assert cfg.asr.backend == "whisper"
    assert cfg.llm.model == "qwen2.5:3b"
    assert cfg.tts.backend == "edge"
    assert cfg.lipsync.backend == "musetalk"
    assert cfg.pusher.backend == "ws_mjpeg"


def test_from_dict_partial():
    """部分字段覆盖，其余用默认值。"""
    cfg = from_dict({
        "server": {"port": 9000},
        "llm": {"model": "qwen2.5:7b"},
    })
    assert cfg.server.port == 9000
    assert cfg.server.host == "127.0.0.1"   # 默认
    assert cfg.llm.model == "qwen2.5:7b"
    assert cfg.llm.temperature == 0.7        # 默认
    assert cfg.asr.backend == "whisper"      # 默认


def test_from_dict_ignores_unknown_keys():
    """未知字段被忽略而非报错（宽松策略），但会被收集到 unknown_keys 供报警（C12）。"""
    cfg = from_dict({
        "asr": {"backend": "whisper", "unknown_field": "xxx", "model": "tiny"},
    })
    assert cfg.asr.backend == "whisper"
    assert cfg.asr.model == "tiny"
    # ★ C12：未知键不再静默，被收集供 validate 报警
    assert "asr.unknown_field" in cfg.unknown_keys
    # validate 应包含该未知键的告警
    warns = validate(cfg)
    assert any("asr.unknown_field" in w for w in warns)


def test_from_dict_unknown_top_level_key():
    """顶层未知键也被收集。"""
    cfg = from_dict({"server": {"port": 8000}, "typo_section": {}})
    assert "typo_section" in cfg.unknown_keys


def test_config_clean_has_no_unknown_keys():
    """干净的配置不应有未知键。"""
    cfg = from_dict({
        "server": {"port": 8000},
        "asr": {"backend": "whisper", "model": "small"},
    })
    assert cfg.unknown_keys == []
    assert validate(cfg) == []


def test_personas_config():
    """#2：多角色配置加载。"""
    cfg = from_dict({
        "personas": {
            "default_id": "yunxi",
            "list": [
                {"id": "xiaoxiao", "name": "晓晓", "voice": "zh-CN-XiaoxiaoNeural",
                 "system_prompt": "温柔"},
                {"id": "yunxi", "name": "云希", "voice": "zh-CN-YunxiNeural",
                 "system_prompt": "活力"},
            ],
        }
    })
    assert cfg.personas.default_id == "yunxi"
    assert len(cfg.personas.list) == 2
    assert cfg.personas.list[0].id == "xiaoxiao"
    assert cfg.personas.list[1].voice == "zh-CN-YunxiNeural"
    assert cfg.personas.list[1].system_prompt == "活力"


def test_from_dict_empty():
    cfg = from_dict({})
    assert isinstance(cfg, Config)
    assert cfg.server.port == 8000


def test_from_dict_type_error():
    with pytest.raises(ConfigError):
        from_dict({"server": "not a dict"})
    with pytest.raises(ConfigError):
        from_dict("not a dict")


def test_from_yaml(tmp_path):
    p = tmp_path / "c.yaml"
    p.write_text(
        "server:\n  port: 7777\n  host: 0.0.0.0\n"
        "llm:\n  model: custom-model\n",
        encoding="utf-8",
    )
    cfg = from_yaml(str(p))
    assert cfg.server.port == 7777
    assert cfg.server.host == "0.0.0.0"
    assert cfg.llm.model == "custom-model"


def test_from_yaml_missing_file(tmp_path):
    with pytest.raises(ConfigError):
        from_yaml(str(tmp_path / "nope.yaml"))


def test_from_yaml_empty_file(tmp_path):
    p = tmp_path / "empty.yaml"
    p.write_text("", encoding="utf-8")
    cfg = from_yaml(str(p))
    assert cfg.server.port == 8000


def test_validate_clean():
    warns = validate(Config())
    assert warns == []


def test_validate_warns():
    cfg = Config()
    cfg.server.port = 99999
    cfg.asr.silence_ms = 50
    cfg.sentence_splitter.max_chars = 2
    cfg.lipsync.fps = 0
    warns = validate(cfg)
    assert len(warns) >= 4
    assert any("port" in w for w in warns)
    assert any("silence_ms" in w for w in warns)
