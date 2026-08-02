# -*- mode: python ; coding: utf-8 -*-
"""PyInstaller onefile spec：单 EXE 打包（Lite 版，不含 torch/opencv）。

产出：dist/DigitalHuman-Lite.exe（~50MB，双击即用）

★ Lite 版包含：
  - 完整 Python 运行时 + fastapi/uvicorn/aiohttp/pyyaml
  - edge-tts + miniaudio（TTS 语音合成）
  - numpy（音频处理）
  - web 前端 + 配置 + 形象图
  - SQLite 持久化

★ Lite 版不含（运行时降级）：
  - torch / faster-whisper（ASR 降级 Mock）
  - opencv / MuseTalk（唇形降级占位）

需要 GPU 完整版时用 digitalhuman.spec（onedir + torch）。
"""
import os
from PyInstaller.utils.hooks import collect_submodules, collect_data_files, copy_metadata

_PROJECT_ROOT = os.environ.get("DH_PROJECT_ROOT", "")
if not _PROJECT_ROOT:
    _SPEC_DIR = os.path.dirname(os.path.abspath(SPECPATH))
    _PROJECT_ROOT = os.path.dirname(_SPEC_DIR)

def _p(rel):
    return os.path.join(_PROJECT_ROOT, rel)

hiddenimports = []
# 核心包
for pkg in ["aiohttp", "edge_tts", "miniaudio", "yaml", "websockets"]:
    try:
        hiddenimports += collect_submodules(pkg)
    except Exception:
        pass
# uvicorn 内部模块
hiddenimports += ["uvicorn.lifespan.on", "uvicorn.protocols.websockets.websockets_impl"]
# sqlite3 是 stdlib，无需 collect

datas = []
# 前端 + 脚本 + 配置 + 形象
datas += [(_p("web"), "web")]
datas += [(_p("scripts/musetalk_render.py"), "scripts")]
datas += [(_p("scripts/cosyvoice_tts.py"), "scripts")]
datas += [(_p("assets"), "assets")]
datas += [(_p("config.example.yaml"), ".")]
datas += [(_p("config.dev.yaml"), ".")]
# 包元数据
for pkg in ["edge-tts", "aiohttp", "uvicorn", "fastapi", "miniaudio"]:
    try:
        datas += copy_metadata(pkg)
    except Exception:
        pass

a = Analysis(
    [_p("digitalhuman/__main__.py")],
    pathex=[_PROJECT_ROOT],
    binaries=[],
    datas=datas,
    hiddenimports=hiddenimports,
    excludes=[
        # 排除重依赖（Lite 版不含）
        "torch", "torchvision", "torchaudio",
        "cv2", "opencv",
        "faster_whisper", "ctranslate2",
        "matplotlib", "scipy", "pandas", "IPython", "jupyter",
        "tensorboard", "tkinter", "unittest", "test",
        "transformers",  # faster-whisper 间接依赖
    ],
    noarchive=False,
)

pyz = PYZ(a.pure, a.zipped_data)

exe = EXE(
    pyz,
    a.scripts,
    a.binaries,
    a.datas,
    [],
    name="DigitalHuman-Lite",
    debug=False,
    bootloader_ignore_signals=False,
    strip=False,
    upx=True,            # Lite 版用 UPX 压缩（无 torch dll，安全）
    upx_exclude=[],
    runtime_tmpdir=None,  # 解压到系统 temp
    console=True,
    disable_windowed_traceback=False,
    target_arch=None,
    codesign_identity=None,
    entitlements_file=None,
    icon=_p("packaging/digitalhuman.ico"),
)
