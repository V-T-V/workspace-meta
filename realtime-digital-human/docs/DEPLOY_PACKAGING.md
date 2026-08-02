# Windows 单 EXE 部署指南

## 方案对比

| 方案 | 命令 | 产出 | 大小 | 功能 |
|------|------|------|------|------|
| **Lite 单 EXE** ★ | `build_exe.bat` | `DigitalHuman-Lite.exe` | ~38MB | 文字对话 + TTS 语音（ASR/唇形降级） |
| **完整 onedir** | `build_windows.bat` | `DigitalHuman/` 目录 | ~1.5GB | 完整 GPU（ASR + 唇形 + CUDA torch） |
| **Inno Setup 安装包** | `iscc installer.iss` | `DigitalHuman-Setup.exe` | ~1.2GB | 完整版 + 安装界面 |

## Lite 单 EXE（推荐：最简部署）

### 打包
```bat
build_exe.bat    :: 一键打包，产出 dist/DigitalHuman-Lite.exe（~38MB，3-5 分钟）
```

### 使用
双击 `DigitalHuman-Lite.exe` → 浏览器开 http://127.0.0.1:8000

**无需 Python 环境、无需装依赖、无需 config 文件**——真正的单 EXE 零配置启动。

### 自测（验证 EXE 是否正常）

```bat
:: 方法 1：EXE 内置自检（推荐，双击 EXE 加参数）
DigitalHuman-Lite.exe --self-test

:: 方法 2：脚本验证（全面，测 WS/REST/OpenAI 全链路）
python scripts/verify_exe.py
```

自检模式会检查 6 个维度：Python 环境 / 核心依赖 / 引擎装配 / Ollama 连通性 / 端口可用 / REST 端点。
输出彩色报告（✓ 通过 / ✗ 失败 / ⚠ 降级警告），30 秒内完成。

### 包含
- Python 运行时 + fastapi/uvicorn/aiohttp
- edge-tts + miniaudio（TTS 语音合成）
- web 前端 + 配置 + 形象图
- SQLite 持久化 + OpenAI 兼容 API + Prometheus metrics

### 不含（运行时降级）
- ASR（faster-whisper）→ 降级 Mock（文字输入模式）
- 唇形（MuseTalk）→ 降级占位渲染
- 需要这些时用完整版（`build_windows.bat`）

### 前置要求（目标机器）
- **Ollama + qwen2.5:3b 模型**（数字人的"大脑"）
  - 装 Ollama：https://ollama.com/download
  - 拉模型：`ollama pull qwen2.5:3b`

## 完整 GPU 版（onedir + CUDA）

见下方"完整离线包"章节。

---

## 完整离线包打包指南

### 三种打包方案对比

| 方案 | 命令 | 大小 | 含 torch | 含 faster-whisper | 适用 |
|------|------|------|---------|-------------------|------|
| **Lite 单 EXE** | `build_exe.bat` | ~38MB | ✗ | ✗ | 快速体验 |
| **完整 onedir** ★ | `build_full.bat` | ~400MB | ✗ | ✓ | 无需装依赖 |
| **GPU 完整** | `build_windows.bat` | ~1.5GB | ✓(CUDA) | ✓ | 4060 全功能 |

### 完整 onedir 包（推荐：~400MB，目标机无需装任何东西）

```bat
build_full.bat
```

产出 `dist/DigitalHuman/DigitalHuman.exe`（onedir 目录，双击即用）。

**包含**：faster-whisper + ctranslate2 + opencv + edge-tts + miniaudio + 全部源码 + 前端
**不含 torch**（faster-whisper 用 ctranslate2 后端，不需要 torch；若需 torch 功能运行时 `pip install torch`）

### GPU 完整版（CUDA 13.3）

打包机需具备：
- **Python 3.10+**（打包用）
- **NVIDIA GPU + CUDA 13.3**（打 GPU 包需要，CPU 包可省）
- **PyTorch CUDA 13.3**（打包脚本自动装）
- **Inno Setup 6**（可选，生成专业安装界面；下载 https://jrsoftware.org/isdl.php）

## 一键打包

在项目根目录（4060 + CUDA 13.3 机器上）：

```bat
build_windows.bat
```

脚本自动完成：
1. 建 clean venv（隔离打包环境）
2. 装 CUDA 13.3 PyTorch（`--index-url https://download.pytorch.org/whl/cu133`）
3. 装 faster-whisper / opencv / edge-tts / PyInstaller
4. PyInstaller 打包（`packaging/digitalhuman.spec`，onedir 模式）
5. Inno Setup 生成 `dist/DigitalHuman-Setup.exe`（若装了）

产出：
- `dist/DigitalHuman/` —— PyInstaller onedir（含 exe + 所有 dll + 资源）
- `dist/DigitalHuman-Setup.exe` —— Inno Setup 安装包（可选）

## 包含什么

| 组件 | 说明 |
|------|------|
| Python 运行时 | PyInstaller 内嵌 |
| torch（CUDA 13.3） | ~2.5GB（GPU 推理） |
| faster-whisper + ctranslate2 | ASR（~200MB） |
| opencv-python | 唇形渲染（~60MB） |
| edge-tts | TTS（在线，零本地） |
| 源码 + 前端 web/ | 数字人服务 |
| scripts/ | musetalk_render.py + cosyvoice_tts.py（subprocess 用） |
| assets/ + config | 默认形象图 + 配置模板 |

**包大小预估**：800MB - 1.5GB（主要被 torch + CUDA 库占据）

**不包含**：
- Ollama（外部依赖，安装后首次运行引导）
- MuseTalk 真实模型（用户按需 clone 官方 repo，见 docs/GPU_DEPLOY.md）

## 目标机器安装

1. 双击 `DigitalHuman-Setup.exe`
2. 选安装目录（默认 `C:\Program Files\DigitalHuman`）
3. 勾选"创建桌面快捷方式"
4. 安装完成自动运行**首次配置**：
   - 检测 Ollama → 未装则提示下载（打开 https://ollama.com/download）
   - 拉取模型 `qwen2.5:3b`（约 1.9GB）
5. 启动数字人 → 浏览器开 http://127.0.0.1:8000

## 直接运行（不装安装包）

PyInstaller 产出的 `dist/DigitalHuman/DigitalHuman.exe` 可直接双击运行，等价于 `python -m digitalhuman`。资源（web/、scripts/、assets/、config）在 exe 同级目录。

## 配置修改

安装后配置文件在安装目录（如 `C:\Program Files\DigitalHuman\config.gpu.yaml`）。修改后重启 exe 生效。零配置模式：删除 config 文件，exe 用内置默认。

## 打包排错

### PyInstaller 报"ModuleNotFoundError: No module named 'xxx'"

在 `packaging/digitalhuman.spec` 的 `hiddenimports` 里加该模块。常见漏的：
- `ctranslate2.libs`（ctranslate2 的运行时库）
- `transformers.models.whisper.tokenization_whisper`

### exe 运行报"找不到 ctranslate2.dll"

spec 已用 hook 收集，但某些版本路径不同。检查 `.venv/Lib/site-packages/ctranslate2/` 下的 `.dll`，手动加到 `binaries`：
```python
binaries += [("path/to/ctranslate2.dll", ".")]
```

### exe 运行报"torch CUDA 不可用"

打包机 torch 装的是 CPU 版。确认 `build_windows.bat` 第 3 步用 `--index-url cu133`，且 `python -c "import torch; print(torch.cuda.is_available())"` 输出 True 再打包。

### 包太大

- 排除 `matplotlib`/`scipy`（spec 已 exclude）
- 用 UPX 压缩（spec 当前关闭，因 torch dll 压缩易损坏；可对非 torch 的 dll 单独开）
- 拆分：torch 单独下载（首次运行拉取），exe 只含核心

### Inno Setup 找不到 ChineseSimplified.isl

Inno Setup 6 默认含中文。若没有，从 https://jrsoftware.org/files/istrans/ 下载 `ChineseSimplified.isl` 放到 Inno Setup 安装目录的 `Languages/`。

## PyInstaller spec 说明（packaging/digitalhuman.spec）

关键设计：
- **onedir 而非 onefile**：onefile 启动慢（每次解压到 temp），onedir 启动快、便于修改配置
- **collect_submodules(torch)**：torch 有 2000+ 动态子模块，必须全收集
- **collect_dynamic_libs(ctranslate2)**：faster-whisper 的后端 .dll
- **hookspath=["packaging/hooks"]**：自定义 hook 处理动态库（见 packaging/hooks/）
- **upx=False**：UPX 压缩 torch/ctranslate2 的 dll 会损坏

## 相关文件
- `packaging/digitalhuman.spec` —— PyInstaller 配置
- `packaging/hooks/` —— torch/ctranslate2/faster-whisper 的收集 hook
- `packaging/installer.iss` —— Inno Setup 脚本
- `packaging/run_after_install.bat` —— 首次运行引导（Ollama 检测）
- `build_windows.bat` —— 一键打包
