#============================================================
# MagicBook.gd — 魔法书 UI（查看/追踪技能解锁进度）
#============================================================
# 按 M 键打开：5 个技能槽，已解锁高亮+描述，未解锁显示进度条
# 实时刷新（监听 collection_updated）
#============================================================
extends CanvasLayer

var _panel: Panel
var _content: VBoxContainer
var _visible_flag: bool = false

func _ready() -> void:
	_build_ui()
	EventBus.collection_updated.connect(func(_t): if _visible_flag: _refresh())

func _unhandled_input(event: InputEvent) -> void:
	if event is InputEventKey and event.pressed and event.keycode == KEY_M:
		_visible_flag = not _visible_flag
		_panel.visible = _visible_flag
		if _visible_flag:
			_refresh()

func _build_ui() -> void:
	var root = Control.new()
	root.set_anchors_preset(Control.PRESET_FULL_RECT)
	root.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(root)
	_panel = Panel.new()
	_panel.set_anchors_preset(Control.PRESET_CENTER)
	_panel.custom_minimum_size = Vector2(450, 400)
	_panel.position = Vector2(-225, -200)
	_panel.visible = false
	var bg = StyleBoxFlat.new()
	bg.bg_color = Color(0.15, 0.1, 0.25, 0.95)
	bg.corner_radius_top_left = 16
	bg.corner_radius_top_right = 16
	bg.corner_radius_bottom_left = 16
	bg.corner_radius_bottom_right = 16
	bg.border_width_top = 4
	bg.border_color = Color(0.5, 0.3, 0.9)
	_panel.add_theme_stylebox_override("panel", bg)
	root.add_child(_panel)
	var title = Label.new()
	title.text = "📖 魔法书"
	title.add_theme_font_size_override("font_size", 30)
	title.add_theme_color_override("font_color", Color(0.8, 0.6, 1.0))
	title.position = Vector2(20, 15)
	_panel.add_child(title)
	_content = VBoxContainer.new()
	_content.position = Vector2(20, 65)
	_content.custom_minimum_size = Vector2(410, 315)
	_content.add_theme_constant_override("separation", 10)
	_panel.add_child(_content)
	var hint = Label.new()
	hint.text = "按 M 关闭 · 收集物品解锁新魔法"
	hint.add_theme_font_size_override("font_size", 14)
	hint.add_theme_color_override("font_color", Color(0.6, 0.5, 0.7))
	hint.position = Vector2(20, 370)
	_panel.add_child(hint)

func _refresh() -> void:
	for c in _content.get_children():
		c.queue_free()
	var unlocked_count = 0
	for skill_id in GameState.SKILLS:
		var sdef = GameState.SKILLS[skill_id]
		var unlocked = skill_id in GameState.unlocked_skills
		if unlocked:
			unlocked_count += 1
		var row = Panel.new()
		row.custom_minimum_size = Vector2(410, 60)
		var sb = StyleBoxFlat.new()
		if unlocked:
			sb.bg_color = Color(0.3, 0.2, 0.5, 0.8)
			sb.border_color = Color(0.7, 0.5, 1.0)
		else:
			sb.bg_color = Color(0.2, 0.15, 0.3, 0.6)
			sb.border_color = Color(0.4, 0.3, 0.5)
		sb.border_width_left = 3
		sb.border_width_right = 3
		sb.border_width_top = 3
		sb.border_width_bottom = 3
		sb.corner_radius_top_left = 10
		sb.corner_radius_top_right = 10
		sb.corner_radius_bottom_left = 10
		sb.corner_radius_bottom_right = 10
		row.add_theme_stylebox_override("panel", sb)
		var lbl = Label.new()
		var status = "✅" if unlocked else "🔒"
		var progress = ""
		if not unlocked:
			progress = " (%d/%d)" % [GameState.total_collected, sdef["threshold"]]
		lbl.text = "%s %s %s\n   %s%s" % [status, sdef["emoji"], sdef["name"], sdef["desc"], progress]
		lbl.position = Vector2(12, 8)
		lbl.add_theme_font_size_override("font_size", 16)
		lbl.add_theme_color_override("font_color", Color(0.95, 0.9, 1.0) if unlocked else Color(0.6, 0.55, 0.65))
		row.add_child(lbl)
		_content.add_child(row)
	# 汇总
	var summary = Label.new()
	summary.text = "\n已掌握 %d / %d 种魔法" % [unlocked_count, GameState.SKILLS.size()]
	summary.add_theme_font_size_override("font_size", 20)
	summary.add_theme_color_override("font_color", Color(0.8, 0.6, 1.0))
	summary.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_content.add_child(summary)
