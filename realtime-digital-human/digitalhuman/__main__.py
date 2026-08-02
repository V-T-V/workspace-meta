"""模块入口：python -m digitalhuman 等价于 python -m digitalhuman.server。

主要为 PyInstaller 打包提供干净的 console entry point。
PyInstaller 把 __main__.py 当顶层脚本执行，相对 import 会失败，所以这里用绝对 import。
"""
try:
    # 正常包模式（python -m digitalhuman）
    from digitalhuman.server import main
except ImportError:
    # PyInstaller 打包模式（__main__ 被当顶层脚本）
    import sys
    import os
    # 把 _internal 加入 path 找 digitalhuman 包
    base = getattr(sys, "_MEIPASS", os.path.dirname(sys.executable))
    if base not in sys.path:
        sys.path.insert(0, base)
    from digitalhuman.server import main

if __name__ == "__main__":
    main()
