"""PyInstaller hook：faster-whisper。

faster-whisper 内部用 transformers 的 WhisperModel，需确保相关子模块被打包。
"""
from PyInstaller.utils.hooks import collect_submodules, collect_data_files

hiddenimports = collect_submodules("faster_whisper")
datas = collect_data_files("faster_whisper")
