# -*- mode: python ; coding: utf-8 -*-
"""最小验证 spec：不含 torch，仅验证打包链路（路径解析 + 数据文件 + exe 生成）。

用于 CPU 环境快速验证 spec 架构正确性。真实打包用 digitalhuman.spec（含 torch）。
"""
import os
_PROJECT_ROOT = os.environ.get("DH_PROJECT_ROOT", os.path.dirname(os.path.dirname(os.path.abspath(SPECPATH))))

datas = [
    (os.path.join(_PROJECT_ROOT, "web"), "web"),
    (os.path.join(_PROJECT_ROOT, "scripts/musetalk_render.py"), "scripts"),
    (os.path.join(_PROJECT_ROOT, "assets"), "assets"),
    (os.path.join(_PROJECT_ROOT, "config.example.yaml"), "."),
]

a = Analysis(
    [os.path.join(_PROJECT_ROOT, "digitalhuman/__main__.py")],
    pathex=[_PROJECT_ROOT],
    binaries=[],
    datas=datas,
    hiddenimports=["uvicorn.lifespan.on"],
    excludes=["torch", "cv2", "faster_whisper", "ctranslate2", "numpy",
              "matplotlib", "scipy", "pandas"],
    noarchive=False,
)
pyz = PYZ(a.pure)
exe = EXE(pyz, a.scripts, [], exclude_binaries=True, name="DigitalHuman-min",
          console=True, debug=False, strip=False, upx=False)
coll = COLLECT(exe, a.binaries, a.zipfiles, a.datas, strip=False, upx=False,
               name="DigitalHuman-min")
