#============================================================
# GiftUI.gd — 赠礼界面（按 Q 键打开，选择物品送给最近 NPC）
#============================================================
# 玩家靠近 NPC 时按 Q 打开礼物选择面板
# 显示拥有的物品列表，点击送给 NPC
# 最爱物品给 +15 友谊，普通 +5
#============================================================
extends CanvasLayer

var _panel: Panel
var _nearby_npc: CharacterBody3D = null

func _ready() -> void:
	_build_ui()

func _build_ui() -> void:
	var root = Control.new()
	root.set_anchors_preset(Control.PRESET_FULL_RECT)
	root.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(root)
	_panel = Panel.new()
	_panel.set_anchors_preset(Control.PRESET_CENTER)
	_panel.custom_minimum_size = Vector2(380, 380)
	_panel.position = Vector2(-190, -190)
	_panel.visible = false
	var bg = StyleBoxFlat.new()
	bg.bg_color = Color(0.98, 0.9, 0.92, 0.97)
	bg.corner_radius_top_left = 16
	bg.corner_radius_top_right = 16
	bg.corner_radius_bottom_left = 16
	bg.corner_radius_bottom_right = 16
	bg.border_width_top = 4
	bg.border_color = Color(0.9, 0.4, 0.6)
	_panel.add_theme_stylebox_override("panel", bg)
	root.add_child(_panel)

func _unhandled_input(event: InputEvent) -> void:
	if event is InputEventKey and event.pressed and event.keycode == KEY_Q:
		if _panel.visible:
			_panel.visible = false
		else:
			_try_open()

func _try_open() -> void:
	# 找最近的 NPC
	var player = get_tree().get_first_node_in_group("player")
	if player == null:
		return
	var nearest = null
	var nearest_dist = 5.0
	for npc in get_tree().get_nodes_in_group("npc"):
		var d = player.global_position.distance_to(npc.global_position)
		if d < nearest_dist:
			nearest_dist = d
			nearest = npc
	if nearest == null:
		EventBus.toast_message.emit("附近没有可以送礼的朋友", "❓")
		return
	_nearby_npc = nearest
	_refresh_panel()
	_panel.visible = true

func _refresh_panel() -> void:
	for c in _panel.get_children():
		c.queue_free()
	var task = _nearby_npc.NPC_TASKS.get(_nearby_npc.zone_id, {})
	var title = Label.new()
	title.text = "🎁 送礼物给 %s" % task.get("emoji", "❓")
	title.add_theme_font_size_override("font_size", 24)
	title.add_theme_color_override("font_color", Color(0.8, 0.3, 0.5))
	title.position = Vector2(20, 15)
	_panel.add_child(title)
	# 最爱提示
	var fav_label = Label.new()
	fav_label.text = "最爱：%s（+15 友谊）" % task.get("fav_emoji", "🎁")
	fav_label.add_theme_font_size_override("font_size", 16)
	fav_label.add_theme_color_override("font_color", Color(0.9, 0.5, 0.7))
	fav_label.position = Vector2(20, 50)
	_panel.add_child(fav_label)
	# 物品列表
	var scroll = ScrollContainer.new()
	scroll.position = Vector2(20, 80)
	scroll.custom_minimum_size = Vector2(340, 260)
	_panel.add_child(scroll)
	var vbox = VBoxContainer.new()
	vbox.add_theme_constant_override("separation", 6)
	scroll.add_child(vbox)
	for item_type in GameState.ITEM_TYPES:
		var count = GameState.get_collection_count(item_type)
		if count <= 0:
			continue
		var idef = GameState.ITEM_TYPES[item_type]
		var is_fav = item_type == task.get("favorite", "")
		var row = HBoxContainer.new()
		row.add_theme_constant_override("separation", 8)
		vbox.add_child(row)
		var info = Label.new()
		info.text = "%s %s ×%d%s" % [idef["emoji"], item_type, count, " ❤️最爱" if is_fav else ""]
		info.add_theme_font_size_override("font_size", 18)
		info.custom_minimum_size = Vector2(220, 36)
		info.add_theme_color_override("font_color", Color(0.8, 0.2, 0.4) if is_fav else Color(0.3, 0.3, 0.3))
		row.add_child(info)
		var btn = Button.new()
		btn.text = "送"
		btn.add_theme_font_size_override("font_size", 16)
		btn.custom_minimum_size = Vector2(60, 36)
		var captured_type = item_type
		btn.pressed.connect(func():
			if _nearby_npc and _nearby_npc.has_method("give_gift"):
				_nearby_npc.give_gift(captured_type)
				_refresh_panel()
		)
		row.add_child(btn)
	# 关闭提示
	var hint = Label.new()
	hint.text = "按 Q 关闭"
	hint.add_theme_font_size_override("font_size", 14)
	hint.add_theme_color_override("font_color", Color(0.6, 0.5, 0.5))
	hint.position = Vector2(20, 350)
	_panel.add_child(hint)
