# -*- mode: python ; coding: utf-8 -*-
"""PyInstaller spec：完整离线包（排除 torch，含 faster-whisper+opencv）。

产出：dist/DigitalHuman/（onedir，~400MB，目标机无需装任何东西）

★ 排除 torch（faster-whisper 用 ctranslate2 后端，不需 torch Python 包）
★ 不用 collect_submodules("torch")（极慢），只用 --collect-all 命令行参数收集其他包

用法：pyinstaller packaging/digitalhuman.full.spec --noconfirm --clean
或：build_full.bat（自动调用命令行参数版本）
"""
import os

_PROJECT_ROOT = os.environ.get("DH_PROJECT_ROOT", os.path.dirname(os.path.dirname(os.path.abspath(SPECPATH))))

def _p(rel):
    return os.path.join(_PROJECT_ROOT, rel)

a = Analysis(
    [_p("digitalhuman/__main__.py")],
    pathex=[_PROJECT_ROOT],
    binaries=[],
    datas=[
        (_p("web"), "web"),
        (_p("scripts/musetalk_render.py"), "scripts"),
        (_p("scripts/cosyvoice_tts.py"), "scripts"),
        (_p("assets"), "assets"),
        (_p("config.example.yaml"), "."),
        (_p("config.dev.yaml"), "."),
    ],
    hiddenimports=["uvicorn.lifespan.on", "pyyaml"],
    excludes=["torch", "torchvision", "matplotlib", "scipy", "pandas", "tkinter", "tensorboard"],
    noarchive=False,
)

pyz = PYZ(a.pure, a.zipped_data)

exe = EXE(
    pyz,
    a.scripts,
    [],
    exclude_binaries=True,
    name="DigitalHuman",
    console=True,
    icon=_p("packaging/digitalhuman.ico"),
)

coll = COLLECT(
    exe,
    a.binaries,
    a.zipfiles,
    a.datas,
    strip=False,
    upx=False,
    name="DigitalHuman",
)
