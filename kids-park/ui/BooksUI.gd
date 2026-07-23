#============================================================
# BooksUI.gd — 图鉴 + 贴纸册（按 Tab 键切换显示）
#============================================================
# 两个标签页：
#   📖 图鉴：12 种收集物，已收集显示 emoji+数量，未收集显示 ❓
#   🏅 贴纸册：获得的任务贴纸列表
# 儿童友好：大图标 + 鲜艳背景，零文字依赖（用 emoji 区分）
#============================================================
extends CanvasLayer

var _panel: Panel
var _content: VBoxContainer
var _tab_book: Button
var _tab_sticker: Button
var _current_tab: int = 0   # 0=图鉴 1=贴纸
var _visible_flag: bool = false

func _ready() -> void:
	_build_ui()
	_refresh()
	# 订阅信号实时刷新
	EventBus.collection_updated.connect(func(_t): _refresh())
	EventBus.sticker_earned.connect(func(_s): _refresh())
	EventBus.zone_unlocked.connect(func(_z): _refresh())

func _unhandled_input(event: InputEvent) -> void:
	# Tab 键切换显示/隐藏
	if event is InputEventKey and event.pressed and event.keycode == KEY_TAB:
		_visible_flag = not _visible_flag
		_panel.visible = _visible_flag
	# 触屏无 Tab，也可用 B 键
	if event is InputEventKey and event.pressed and event.keycode == KEY_B:
		_visible_flag = not _visible_flag
		_panel.visible = _visible_flag

func _build_ui() -> void:
	var root = Control.new()
	root.set_anchors_preset(Control.PRESET_FULL_RECT)
	root.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(root)
	# 主面板（居中）
	_panel = Panel.new()
	_panel.set_anchors_preset(Control.PRESET_CENTER)
	_panel.custom_minimum_size = Vector2(600, 420)
	_panel.position = Vector2(-300, -210)
	_panel.visible = false
	var bg = StyleBoxFlat.new()
	bg.bg_color = Color(0.98, 0.96, 0.9, 0.95)
	bg.corner_radius_top_left = 16
	bg.corner_radius_top_right = 16
	bg.corner_radius_bottom_left = 16
	bg.corner_radius_bottom_right = 16
	bg.border_width_top = 4
	bg.border_color = Color(0.95, 0.75, 0.3)
	_panel.add_theme_stylebox_override("panel", bg)
	root.add_child(_panel)
	# 标题栏（两个切换按钮）
	var title_bar = HBoxContainer.new()
	title_bar.position = Vector2(20, 15)
	title_bar.size = Vector2(560, 60)
	_panel.add_child(title_bar)
	_tab_book = _make_tab_btn("📖 图鉴", 0)
	title_bar.add_child(_tab_book)
	_tab_sticker = _make_tab_btn("🏅 贴纸", 1)
	title_bar.add_child(_tab_sticker)
	# 内容容器
	_content = VBoxContainer.new()
	_content.position = Vector2(20, 85)
	_content.custom_minimum_size = Vector2(560, 315)
	_content.add_theme_constant_override("separation", 8)
	_panel.add_child(_content)
	# 底部提示
	var hint = Label.new()
	hint.text = "按 Tab 切换 · 再按 Tab 关闭"
	hint.position = Vector2(20, 390)
	hint.add_theme_font_size_override("font_size", 16)
	hint.add_theme_color_override("font_color", Color(0.5, 0.45, 0.4))
	_panel.add_child(hint)

func _make_tab_btn(text: String, tab_id: int) -> Button:
	var btn = Button.new()
	btn.text = text
	btn.custom_minimum_size = Vector2(270, 55)
	btn.add_theme_font_size_override("font_size", 24)
	btn.pressed.connect(func():
		_current_tab = tab_id
		_refresh()
	)
	return btn

func _refresh() -> void:
	# 清空内容
	for c in _content.get_children():
		c.queue_free()
	if _current_tab == 0:
		_refresh_book()
		_tab_book.add_theme_color_override("font_color", Color(0.95, 0.5, 0.1))
		_tab_sticker.add_theme_color_override("font_color", Color(0.5, 0.45, 0.4))
	else:
		_refresh_sticker()
		_tab_sticker.add_theme_color_override("font_color", Color(0.95, 0.5, 0.1))
		_tab_book.add_theme_color_override("font_color", Color(0.5, 0.45, 0.4))

func _refresh_book() -> void:
	# 按区域分组展示收集物
	var by_zone: Dictionary = {}
	for item_type in GameState.ITEM_TYPES:
		var idef = GameState.ITEM_TYPES[item_type]
		var zone = idef["zone"]
		if not by_zone.has(zone):
			by_zone[zone] = []
		by_zone[zone].append(item_type)
	for zone_id in by_zone:
		var zdef = GameState.ZONES.get(zone_id, {})
		var zone_label = Label.new()
		zone_label.text = "%s %s" % [zdef.get("emoji", "🗺️"), zdef.get("name", zone_id)]
		zone_label.add_theme_font_size_override("font_size", 22)
		zone_label.add_theme_color_override("font_color", zdef.get("color", Color.WHITE).darkened(0.3))
		_content.add_child(zone_label)
		# 网格行（每个物品一个 emoji 大方块）
		var row = HBoxContainer.new()
		row.add_theme_constant_override("separation", 12)
		_content.add_child(row)
		for item_type in by_zone[zone_id]:
			var idef = GameState.ITEM_TYPES[item_type]
			var count = GameState.get_collection_count(item_type)
			var cell = _make_item_cell(idef, count)
			row.add_child(cell)

func _make_item_cell(idef: Dictionary, count: int) -> Panel:
	var cell = Panel.new()
	cell.custom_minimum_size = Vector2(110, 90)
	var sb = StyleBoxFlat.new()
	if count > 0:
		sb.bg_color = Color(1, 0.98, 0.85)
		sb.border_color = Color(0.4, 0.8, 0.3)
	else:
		sb.bg_color = Color(0.85, 0.82, 0.78)
		sb.border_color = Color(0.6, 0.55, 0.5)
	sb.border_width_left = 2
	sb.border_width_right = 2
	sb.border_width_top = 2
	sb.border_width_bottom = 2
	sb.corner_radius_top_left = 10
	sb.corner_radius_top_right = 10
	sb.corner_radius_bottom_left = 10
	sb.corner_radius_bottom_right = 10
	cell.add_theme_stylebox_override("panel", sb)
	# emoji（已收集显示真实 emoji，未收集显示问号）
	var emoji_label = Label.new()
	emoji_label.text = idef.get("emoji", "❓") if count > 0 else "❓"
	emoji_label.position = Vector2(35, 8)
	emoji_label.add_theme_font_size_override("font_size", 36)
	cell.add_child(emoji_label)
	# 数量
	var count_label = Label.new()
	count_label.text = "×%d" % count if count > 0 else "未发现"
	count_label.position = Vector2(15, 55)
	count_label.add_theme_font_size_override("font_size", 16)
	count_label.add_theme_color_override("font_color", Color(0.4, 0.35, 0.3) if count > 0 else Color(0.7, 0.65, 0.6))
	cell.add_child(count_label)
	return cell

func _refresh_sticker() -> void:
	if GameState.stickers.is_empty():
		var empty = Label.new()
		empty.text = "还没有贴纸 🎁\n完成 NPC 任务获得贴纸！"
		empty.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
		empty.add_theme_font_size_override("font_size", 26)
		empty.custom_minimum_size = Vector2(560, 200)
		_content.add_child(empty)
		return
	# 网格展示贴纸
	var grid = GridContainer.new()
	grid.columns = 2
	grid.add_theme_constant_override("h_separation", 20)
	grid.add_theme_constant_override("v_separation", 16)
	_content.add_child(grid)
	for sticker in GameState.stickers:
		var cell = Panel.new()
		cell.custom_minimum_size = Vector2(265, 70)
		var sb = StyleBoxFlat.new()
		sb.bg_color = Color(1, 0.95, 0.75)
		sb.border_color = Color(0.95, 0.7, 0.2)
		sb.border_width_left = 3
		sb.border_width_right = 3
		sb.border_width_top = 3
		sb.border_width_bottom = 3
		sb.corner_radius_top_left = 12
		sb.corner_radius_top_right = 12
		sb.corner_radius_bottom_left = 12
		sb.corner_radius_bottom_right = 12
		cell.add_theme_stylebox_override("panel", sb)
		var lbl = Label.new()
		lbl.text = "🏅  %s" % sticker
		lbl.position = Vector2(15, 20)
		lbl.add_theme_font_size_override("font_size", 20)
		cell.add_child(lbl)
		grid.add_child(cell)
