# -*- mode: python ; coding: utf-8 -*-
"""PyInstaller spec：实时数字人 Windows 打包配置。

★ 重依赖处理：
  - torch：用 collect_submodules 收集所有动态导入的 ops
  - faster-whisper + ctranslate2：收集 ctranslate2 的 .dll/.lib 数据文件
  - opencv（cv2）：PyInstaller 有内置 hook，但显式 collect 数据更稳
  - edge-tts / aiohttp / pyyaml：相对简单，collect_submodules 即可

★ 数据文件：
  - web/ 前端（HTML/CSS/JS）
  - scripts/musetalk_render.py + cosyvoice_tts.py（subprocess 调用的脚本）
  - assets/（默认形象图）
  - config.example.yaml / config.gpu.yaml（默认配置）

打包产出：dist/DigitalHuman/ （onedir 模式，比 onefile 启动快）
"""
import sys
import os
from PyInstaller.utils.hooks import collect_submodules, collect_data_files, copy_metadata

# 项目根目录：通过环境变量传入（最可靠，不依赖 spec 内部路径计算）。
# build_windows.bat 或命令行运行前需 set DH_PROJECT_ROOT=项目根绝对路径。
_PROJECT_ROOT = os.environ.get("DH_PROJECT_ROOT", "")
if not _PROJECT_ROOT:
    # fallback：尝试从 SPECPATH 推断（spec 在 packaging/ 下，根是上一级）
    _SPEC_DIR = os.path.dirname(os.path.abspath(SPECPATH))
    _PROJECT_ROOT = os.path.dirname(_SPEC_DIR)

block_cipher = None

# ---------- 收集动态导入的子模块 ----------
# ★ 不用 collect_submodules("torch")/("numpy")——PyInstaller 6.x 内置 hook 自动处理，
#   手动 collect 2414 个 torch 子模块要 10+ 分钟。只手动加真正需要的。
hiddenimports = []
# 只 collect 轻量包（fast）
hiddenimports += collect_submodules("faster_whisper")
hiddenimports += collect_submodules("edge_tts")
hiddenimports += collect_submodules("aiohttp")
hiddenimports += collect_submodules("yaml")
# ctranslate2 的动态库（通过 hook 自动处理，这里只补关键模块）
hiddenimports += ["ctranslate2", "ctranslate2.extensions"]
# cv2 的关键模块
hiddenimports += ["cv2", "cv2.data"]
# numpy/torch 由 PyInstaller 内置 hook 处理
# faster-whisper 可能用的 transformers 部分模块
hiddenimports += ["transformers.models.whisper", "transformers.models.whisper.modeling_whisper"]
# 常见遗漏
hiddenimports += ["pyyaml", "uvicorn.lifespan.on"]
# 常见遗漏
hiddenimports += ["ctranslate2", "pyyaml", "uvicorn.lifespan.on"]

# ---------- 收集数据文件（dll/资源/元数据）----------
datas = []
# torch / torchvision 的 dll 和 lib
datas += collect_data_files("torch", include_py_files=False)
datas += collect_data_files("torchvision", include_py_files=False)
# ctranslate2 的核心动态库（Windows 是 .dll）
datas += collect_data_files("ctranslate2", include_py_files=False)
# faster-whisper 的资源
datas += collect_data_files("faster_whisper", include_py_files=False)
# opencv 的 dll
datas += collect_data_files("cv2", include_py_files=False)
# 包元数据（某些库运行时需要 importlib.metadata）——容错：缺包不阻断 spec
for _pkg in ["torch", "faster-whisper", "ctranslate2", "edge-tts", "aiohttp", "uvicorn", "pyyaml"]:
    try:
        datas += copy_metadata(_pkg)
    except Exception:
        pass  # 当前环境未装该包（如 CPU 环境没 faster-whisper），打包机上有即可

# ---------- 项目自身的非 Python 数据 ----------
def _p(rel):
    """相对项目根的绝对路径。"""
    return os.path.join(_PROJECT_ROOT, rel)

# 前端 web/ 目录
datas += [(_p("web"), "web")]
# subprocess 脚本（musetalk_render.py / cosyvoice_tts.py）
datas += [(_p("scripts/musetalk_render.py"), "scripts")]
datas += [(_p("scripts/cosyvoice_tts.py"), "scripts")]
# 默认形象图 + 配置模板
datas += [(_p("assets"), "assets")]
datas += [(_p("config.example.yaml"), ".")]
datas += [(_p("config.gpu.yaml"), ".")]
datas += [(_p("config.dev.yaml"), ".")]

# ---------- 二进制（dll/so）----------
binaries = []
# ctranslate2 的 .dll 需显式收集（PyInstaller hook 不一定全）
binaries += collect_data_files("ctranslate2", include_py_files=False)


a = Analysis(
    [_p("digitalhuman/__main__.py")],
    pathex=[_PROJECT_ROOT],
    binaries=binaries,
    datas=datas,
    hiddenimports=hiddenimports,
    hookspath=[os.path.join(_PROJECT_ROOT, "packaging", "hooks")],
    hooksconfig={},
    runtime_hooks=[],
    excludes=[
        # 排除不需要的大模块（减小体积）
        "matplotlib", "scipy", "pandas", "IPython", "jupyter",
        "tensorboard", "tkinter", "unittest", "test",
    ],
    win_no_prefer_redirects=False,
    win_private_assemblies=False,
    cipher=block_cipher,
    noarchive=False,
)

pyz = PYZ(a.pure, a.zipped_data, cipher=block_cipher)

exe = EXE(
    pyz,
    a.scripts,
    [],
    exclude_binaries=True,
    name="DigitalHuman",
    debug=False,
    bootloader_ignore_signals=False,
    strip=False,
    upx=False,  # torch/ctranslate2 的 dll 不要 UPX 压缩（会损坏）
    console=True,  # console 模式（看日志；部署后可改 False）
    disable_windowed_traceback=False,
    target_arch=None,
    codesign_identity=None,
    entitlements_file=None,
    icon=_p("packaging/digitalhuman.ico"),
)

coll = COLLECT(
    exe,
    a.binaries,
    a.zipfiles,
    a.datas,
    strip=False,
    upx=False,
    upx_exclude=[],
    name="DigitalHuman",
)
