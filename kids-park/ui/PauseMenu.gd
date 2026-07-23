#============================================================
# PauseMenu.gd — 暂停菜单 + 设置面板
#============================================================
# 按 ESC（或手机返回键）打开暂停菜单：
#   - 继续游戏
#   - 音效开关
#   - 音乐开关
#   - 重新开始（清除存档）
#   - 退出
# 儿童友好：大按钮 + emoji 图标
#============================================================
extends CanvasLayer

var _panel: Panel
var _active: bool = false
var _sfx_on: bool = true
var _music_on: bool = true

func _ready() -> void:
	_build_ui()

func _unhandled_input(event: InputEvent) -> void:
	if event is InputEventKey and event.pressed and event.keycode == KEY_ESCAPE:
		_toggle_pause()

func _toggle_pause() -> void:
	_active = not _active
	_panel.visible = _active
	get_tree().paused = _active

func _build_ui() -> void:
	var root = Control.new()
	root.set_anchors_preset(Control.PRESET_FULL_RECT)
	root.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(root)
	# 半透明遮罩
	var dim = ColorRect.new()
	dim.color = Color(0, 0, 0, 0.4)
	dim.set_anchors_preset(Control.PRESET_FULL_RECT)
	dim.visible = false
	root.add_child(dim)
	# 主面板
	_panel = Panel.new()
	_panel.set_anchors_preset(Control.PRESET_CENTER)
	_panel.custom_minimum_size = Vector2(400, 450)
	_panel.position = Vector2(-200, -225)
	_panel.visible = false
	var bg = StyleBoxFlat.new()
	bg.bg_color = Color(0.98, 0.96, 0.9, 0.97)
	bg.corner_radius_top_left = 20
	bg.corner_radius_top_right = 20
	bg.corner_radius_bottom_left = 20
	bg.corner_radius_bottom_right = 20
	bg.border_width_top = 4
	bg.border_color = Color(0.95, 0.75, 0.3)
	_panel.add_theme_stylebox_override("panel", bg)
	root.add_child(_panel)
	# 标题
	var title = Label.new()
	title.text = "⚙️ 暂停菜单"
	title.add_theme_font_size_override("font_size", 32)
	title.set_anchors_preset(Control.PRESET_CENTER_TOP)
	title.position = Vector2(-90, 25)
	_panel.add_child(title)
	# 按钮容器
	var vbox = VBoxContainer.new()
	vbox.position = Vector2(60, 90)
	vbox.custom_minimum_size = Vector2(280, 330)
	vbox.add_theme_constant_override("separation", 12)
	_panel.add_child(vbox)
	# 继续按钮
	vbox.add_child(_make_btn("▶️ 继续游戏", Color(0.3, 0.8, 0.3), func(): _toggle_pause()))
	# 音效切换
	vbox.add_child(_make_btn("🔊 音效：开", Color(0.3, 0.6, 0.9), func(): _toggle_sfx()))
	# 音乐切换
	vbox.add_child(_make_btn("🎵 音乐：开", Color(0.6, 0.4, 0.9), func(): _toggle_music()))
	# 图鉴
	vbox.add_child(_make_btn("📖 查看图鉴", Color(0.9, 0.7, 0.3), func():
		_toggle_pause()
		var books = get_tree().current_scene.get_node_or_null("BooksUI")
		if books:
			books.set("_visible_flag", true)
			var panel = books.get_child(0).get_child(0)
			if panel:
				panel.visible = true
	))
	# 退出
	vbox.add_child(_make_btn("🚪 退出游戏", Color(0.9, 0.3, 0.3), func():
		get_tree().quit()
	))

func _make_btn(text: String, color: Color, on_press: Callable) -> Button:
	var btn = Button.new()
	btn.text = text
	btn.custom_minimum_size = Vector2(280, 60)
	btn.add_theme_font_size_override("font_size", 22)
	var sb = StyleBoxFlat.new()
	sb.bg_color = color.lightened(0.3)
	sb.corner_radius_top_left = 12
	sb.corner_radius_top_right = 12
	sb.corner_radius_bottom_left = 12
	sb.corner_radius_bottom_right = 12
	btn.add_theme_stylebox_override("normal", sb)
	btn.add_theme_stylebox_override("hover", sb)
	btn.add_theme_stylebox_override("pressed", sb)
	btn.pressed.connect(on_press)
	return btn

func _toggle_sfx() -> void:
	_sfx_on = not _sfx_on
	AudioServer.set_bus_mute(0, not _sfx_on)
	# 更新按钮文字
	var vbox = _panel.get_child(1)
	var btn = vbox.get_child(1) as Button
	btn.text = "🔊 音效：%s" % ("开" if _sfx_on else "关")

func _toggle_music() -> void:
	_music_on = not _music_on
	if _music_on:
		AudioBus.set_zone_music(AudioBus._current_zone)
	else:
		AudioBus.stop_music()
	var vbox = _panel.get_child(1)
	var btn = vbox.get_child(2) as Button
	btn.text = "🎵 音乐：%s" % ("开" if _music_on else "关")
