"""PyInstaller hook：PyTorch（torch）。

torch 有大量动态导入的 ops/extensions，默认打包会漏。显式收集所有子模块 + 动态库。
"""
from PyInstaller.utils.hooks import collect_submodules, collect_dynamic_libs, collect_data_files

hiddenimports = collect_submodules("torch")
hiddenimports += collect_submodules("torchvision")
binaries = collect_dynamic_libs("torch")
datas = collect_data_files("torch")
