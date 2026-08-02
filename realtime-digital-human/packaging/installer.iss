; 实时数字人 · Inno Setup 安装脚本
; 编译：iscc packaging\installer.iss
; 产出：dist\DigitalHuman-Setup.exe
;
; 功能：安装目录选择 / 桌面+开始菜单快捷方式 / 卸载 / 首次运行检测 Ollama

#define MyAppName "实时数字人"
#define MyAppNameEn "DigitalHuman"
#define MyAppVersion "0.1.0"
#define MyAppPublisher "M_X_M"
#define MyAppExeName "DigitalHuman.exe"

[Setup]
AppId={{B8F3D2E1-1234-5678-9ABC-DEF012345678}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={autopf}\{#MyAppNameEn}
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
OutputDir=dist
OutputBaseFilename=DigitalHuman-Setup
SetupIconFile=packaging\digitalhuman.ico
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
ArchitecturesAllowed=x64
ArchitecturesInstallIn64BitMode=x64
PrivilegesRequired=lowest
; 大包不内嵌（LZMA 压缩 torch/cv2 效果有限，但能省 10-20%）
DiskSpanning=no

[Languages]
Name: "chinesesimp"; MessagesFile: "compiler:Languages\ChineseSimplified.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "创建桌面快捷方式"; GroupDescription: "附加图标:"; Flags: checkedonce
Name: "startup"; Description: "开机自启（可选）"; GroupDescription: "附加图标:"; Flags: unchecked

[Files]
; 整个 PyInstaller 产出目录
Source: "dist\DigitalHuman\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs
; 启动器脚本（首次运行引导 Ollama）
Source: "packaging\run_after_install.bat"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{group}\卸载 {#MyAppName}"; Filename: "{uninstallexe}"
Name: "{group}\首次运行配置（Ollama）"; Filename: "{app}\run_after_install.bat"
Name: "{commondesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon
Name: "{userstartup}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: startup

[Run]
; 安装结束时自动启动配置向导（检测/引导 Ollama）
Filename: "{app}\run_after_install.bat"; Description: "运行首次配置（检测 Ollama）"; Flags: nowait postinstall skipifsilent

[UninstallDelete]
Type: filesandordirs; Name: "{app}"

[Code]
function InitializeSetup(): Boolean;
begin
    Result := True;
end;
