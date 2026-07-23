# KidsPark · 儿童 3D 乐园探索

> Godot 4.7 · 3-8 岁 · 动物森友会简化版

## 一句话
儿童 3D 乐园探索：自由走动 → 拾取发光物品（水果/花/贝壳）→ 自动收集图鉴 → 帮 NPC 小动物找东西 → 获得贴纸奖励 → 解锁新区域。

## 设计原则（基于儿童游戏研究）
1. **无惩罚**：没有 Game Over / 生命值 / 时间限制 / 敌人攻击
2. **即时正反馈**：拾取 → Toast 消息 + 彩纸爆发
3. **零文字依赖**：大图标 emoji + 颜色区分
4. **简单操作**：左摇杆移动 + 右侧大按钮（跳/互动）

## 技术规格
| 项 | 值 |
|----|------|
| 引擎 | Godot 4.7（mobile 渲染器） |
| 玩家 | CharacterBody3D 第三人称 + SpringArm3D（pitch 收窄防眩晕） |
| 乐园 | 4 个彩色区域 2×2 排布（草地/沙滩/花园/冰雪） |
| 收集 | 12 种物品 × Area3D 自动拾取 + 旋转浮动动画 |
| NPC | 4 个小动物 + Label3D 气泡 + 互动任务 |
| 存档 | JSON（图鉴 + 贴纸 + 已解锁区域） |

## 操作
- **桌面**：WASD 移动，空格跳，鼠标看，ESC 退出
- **触屏**：左下虚拟摇杆移动，右下跳跃/互动大按钮（90px）

## 系统总结
- **采集**：Area3D body_entered → GameState.collect_item → EventBus 彩纸 + Toast → queue_free
- **NPC 任务**：4 个 NPC 各有"找 N 个 X"任务，玩家走近自动互动检查
- **区域解锁**：收集总数到阈值自动解锁新区域（10/20/35）
- **贴纸**：完成任务获得贴纸，存入 GameState.stickers

## 架构（复用 city-hunt 做减法+换皮）
```
kids-park/
├── project.godot              # mobile 渲染器
├── autoload/                  # EventBus/GameState(图鉴+贴纸)/ParkGen
├── player/                    # Player(悠闲移动)+PlayerCamera(收窄pitch)
├── world/                     # Collectible(Area3D拾取)+NPC(任务)
├── ui/                        # HUD(进度+Toast)+TouchControls(大按钮)
├── Main.gd / Main.tscn        # 运行时场景树构建
└── icon.svg                   # 笑脸图标
```

## 验证
- `cargo check` 零脚本错误 ✅（headless --quit-after 60）
- 截图验证：乐园地面/收集物/玩家可见 ✅
- 运行：`Godot.exe --path kids-park`
