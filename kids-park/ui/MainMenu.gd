#============================================================
# MainMenu.gd — 主菜单/开始界面
#============================================================
# 游戏启动时显示，提供：
#   - 标题 + 副标题
#   - 开始游戏按钮
#   - 继续游戏（有存档时显示进度）
#   - 操作说明
#   - 退出按钮
# 按开始后淡出，进入游戏
#============================================================
extends CanvasLayer

var _overlay: ColorRect
var _panel: Panel
var _started: bool = false

func _ready() -> void:
	_build_ui()
	# E2E 测试或截图模式下自动跳过菜单
	if OS.get_environment("KIDS_PARK_E2E") == "1" or OS.get_environment("PARK_SCREENSHOT") == "1":
		_start_game()
		return
	# 暂停游戏逻辑直到开始
	get_tree().paused = true
	process_mode = Node.PROCESS_MODE_ALWAYS

func _build_ui() -> void:
	var root = Control.new()
	root.set_anchors_preset(Control.PRESET_FULL_RECT)
	root.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(root)
	# 全屏遮罩（渐变背景）
	_overlay = ColorRect.new()
	_overlay.color = Color(0.4, 0.6, 0.9, 0.95)
	_overlay.set_anchors_preset(Control.PRESET_FULL_RECT)
	root.add_child(_overlay)
	# 主面板
	_panel = Panel.new()
	_panel.set_anchors_preset(Control.PRESET_CENTER)
	_panel.custom_minimum_size = Vector2(600, 500)
	_panel.position = Vector2(-300, -250)
	var bg = StyleBoxFlat.new()
	bg.bg_color = Color(0.98, 0.96, 0.88, 0.0)   # 透明（让遮罩色透出）
	_panel.add_theme_stylebox_override("panel", bg)
	root.add_child(_panel)
	# 大标题
	var title = Label.new()
	title.text = "🎪 儿童乐园"
	title.add_theme_font_size_override("font_size", 64)
	title.add_theme_color_override("font_color", Color(1.0, 0.95, 0.7))
	title.set_anchors_preset(Control.PRESET_CENTER_TOP)
	title.position = Vector2(-180, 30)
	title.custom_minimum_size = Vector2(360, 80)
	title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_panel.add_child(title)
	# 副标题
	var subtitle = Label.new()
	subtitle.text = "✨ 探索 · 收集 · 交朋友 ✨"
	subtitle.add_theme_font_size_override("font_size", 24)
	subtitle.add_theme_color_override("font_color", Color(1.0, 1.0, 0.95))
	subtitle.set_anchors_preset(Control.PRESET_CENTER_TOP)
	subtitle.position = Vector2(-150, 120)
	subtitle.custom_minimum_size = Vector2(300, 30)
	subtitle.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_panel.add_child(subtitle)
	# 存档进度（有存档时显示）
	if GameState.total_collected > 0:
		var progress = Label.new()
		progress.text = "上次进度：⭐ %d  🏅 %d  🗺️ %d/4 区域" % [GameState.total_collected, GameState.stickers.size(), GameState.unlocked_zones.size()]
		progress.add_theme_font_size_override("font_size", 18)
		progress.add_theme_color_override("font_color", Color(0.95, 0.9, 0.7))
		progress.set_anchors_preset(Control.PRESET_CENTER_TOP)
		progress.position = Vector2(-160, 165)
		progress.custom_minimum_size = Vector2(320, 25)
		progress.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
		_panel.add_child(progress)
	# 开始按钮（大）
	var start_btn = Button.new()
	start_btn.text = "▶️  开始游戏"
	start_btn.add_theme_font_size_override("font_size", 32)
	start_btn.custom_minimum_size = Vector2(300, 80)
	start_btn.set_anchors_preset(Control.PRESET_CENTER)
	start_btn.position = Vector2(-150, 30)
	start_btn.process_mode = Node.PROCESS_MODE_ALWAYS
	var sb = StyleBoxFlat.new()
	sb.bg_color = Color(0.3, 0.8, 0.4)
	sb.corner_radius_top_left = 20
	sb.corner_radius_top_right = 20
	sb.corner_radius_bottom_left = 20
	sb.corner_radius_bottom_right = 20
	start_btn.add_theme_stylebox_override("normal", sb)
	start_btn.add_theme_stylebox_override("hover", sb)
	start_btn.add_theme_stylebox_override("pressed", sb)
	start_btn.pressed.connect(_start_game)
	_panel.add_child(start_btn)
	# 操作说明
	var controls = Label.new()
	controls.text = "🎮 操作说明\nWASD 移动 · 空格跳跃（可二段跳）\nE 互动 · P 拍照 · Tab 图鉴\nESC 暂停 · M 魔法书 · H 换装"
	controls.add_theme_font_size_override("font_size", 16)
	controls.add_theme_color_override("font_color", Color(0.95, 0.95, 0.9))
	controls.set_anchors_preset(Control.PRESET_CENTER_BOTTOM)
	controls.position = Vector2(-160, -130)
	controls.custom_minimum_size = Vector2(320, 100)
	controls.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_panel.add_child(controls)

func _start_game() -> void:
	if _started:
		return
	_started = true
	# 恢复游戏
	get_tree().paused = false
	# 淡出动画
	var tw = create_tween()
	tw.tween_property(_overlay, "color:a", 0.0, 0.5)
	tw.tween_property(_panel, "modulate:a", 0.0, 0.3)
	tw.tween_callback(func():
		queue_free()
	)
	AudioBus.play_zone_unlock()
