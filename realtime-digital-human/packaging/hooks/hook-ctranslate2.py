"""PyInstaller hook：ctranslate2（faster-whisper 的后端）。

ctranslate2 加载 .dll/.so 时用动态路径查找，PyInstaller 默认可能漏。
显式收集包内所有二进制 + 数据文件。
"""
from PyInstaller.utils.hooks import collect_dynamic_libs, collect_data_files

# ctranslate2 的核心计算库（Windows: ctranslate2.dll, cpu_features.dll 等）
binaries = collect_dynamic_libs("ctranslate2")
datas = collect_data_files("ctranslate2")
