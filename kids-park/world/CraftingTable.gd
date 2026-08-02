#============================================================
# CraftingTable.gd — 收集物合成台（3 个同类→1 个高级）
#============================================================
# 每个区域 1 个合成台（彩色工作台造型）
# 玩家走近按 E 打开合成界面：
#   显示当前拥有的物品 + 合成配方
#   3 个普通物品 → 1 个稀有/高级物品
# 配方示例：
#   3 apple → 1 honey（蜂蜜罐）
#   3 shell → 1 pearl（珍珠）
#   3 flower → 1 petal（花瓣）
#   3 snowflake → 1 icecrystal（冰晶）
#============================================================
extends Area3D

const RECIPES := {
	"apple": {"output": "honey", "emoji_out": "🍯", "name_out": "蜂蜜"},
	"shell": {"output": "pearl", "emoji_out": "🤍", "name_out": "珍珠"},
	"flower": {"output": "petal", "emoji_out": "🌷", "name_out": "花瓣"},
	"snowflake": {"output": "icecrystal", "emoji_out": "💎", "name_out": "冰晶"},
}
const COST: int = 3

var _player_near: bool = false
var _panel: Panel
var _hint: Label3D

func _ready() -> void:
	body_entered.connect(_on_body_entered)
	body_exited.connect(_on_body_exited)
	_build_table()
	# 碰撞
	var col = CollisionShape3D.new()
	var shape = BoxShape3D.new()
	shape.size = Vector3(2.0, 1.0, 1.5)
	col.shape = shape
	col.position = Vector3(0, 0.5, 0)
	add_child(col)
	_hint = Label3D.new()
	_hint.text = "🔨 合成台\n按 E 合成"
	_hint.font_size = 22
	_hint.position = Vector3(0, 2.0, 0)
	_hint.billboard = BaseMaterial3D.BILLBOARD_ENABLED
	_hint.outline_size = 5
	_hint.outline_modulate = Color(0,0,0,0.5)
	_hint.visible = false
	add_child(_hint)
	_build_ui()

func _build_table() -> void:
	var wood = ModelFactory.get_material(Color(0.5, 0.35, 0.2), {"shaded": true})
	var metal = ModelFactory.get_material(Color(0.7, 0.7, 0.72), {"metallic": 0.7, "roughness": 0.3})
	var top = ModelFactory.get_material(Color(0.4, 0.5, 0.6), {"emissive": Color(0.2,0.25,0.3), "emissive_energy": 0.2, "shaded": true})
	# 桌面
	var desk = CSGBox3D.new()
	desk.size = Vector3(1.8, 0.1, 1.2)
	desk.position = Vector3(0, 0.8, 0)
	desk.material = top
	add_child(desk)
	# 4 条桌腿
	for sx in [-1, 1]:
		for sz in [-1, 1]:
			var leg = CSGCylinder3D.new()
			leg.radius = 0.04; leg.height = 0.8
			leg.position = Vector3(sx * 0.75, 0.4, sz * 0.45)
			leg.material = metal
			add_child(leg)
	# 桌面上的工具（小锤子=圆柱+球）
	var hammer_handle = CSGCylinder3D.new()
	hammer_handle.radius = 0.02; hammer_handle.height = 0.3
	hammer_handle.position = Vector3(0.3, 0.95, 0)
	hammer_handle.rotation_degrees = Vector3(0, 0, 30)
	hammer_handle.material = wood
	add_child(hammer_handle)
	var hammer_head = CSGBox3D.new()
	hammer_head.size = Vector3(0.15, 0.08, 0.06)
	hammer_head.position = Vector3(0.45, 1.05, 0)
	hammer_head.material = metal
	add_child(hammer_head)

func _build_ui() -> void:
	var root_layer = CanvasLayer.new()
	add_child(root_layer)
	var root = Control.new()
	root.set_anchors_preset(Control.PRESET_FULL_RECT)
	root.mouse_filter = Control.MOUSE_FILTER_IGNORE
	root_layer.add_child(root)
	_panel = Panel.new()
	_panel.set_anchors_preset(Control.PRESET_CENTER)
	_panel.custom_minimum_size = Vector2(420, 350)
	_panel.position = Vector2(-210, -175)
	_panel.visible = false
	var bg = StyleBoxFlat.new()
	bg.bg_color = Color(0.2, 0.18, 0.25, 0.97)
	bg.corner_radius_top_left = 16
	bg.corner_radius_top_right = 16
	bg.corner_radius_bottom_left = 16
	bg.corner_radius_bottom_right = 16
	bg.border_width_top = 4
	bg.border_color = Color(0.6, 0.5, 0.3)
	_panel.add_theme_stylebox_override("panel", bg)
	root.add_child(_panel)

func _unhandled_input(event: InputEvent) -> void:
	if event is InputEventKey and event.pressed and event.keycode == KEY_E:
		if _player_near:
			if _panel.visible:
				_close_panel()
			else:
				_open_panel()

func _open_panel() -> void:
	_refresh_panel()
	_panel.visible = true

func _close_panel() -> void:
	_panel.visible = false

func _refresh_panel() -> void:
	for c in _panel.get_children():
		c.queue_free()
	var title = Label.new()
	title.text = "🔨 合成台（3→1）"
	title.add_theme_font_size_override("font_size", 26)
	title.add_theme_color_override("font_color", Color(0.9, 0.8, 0.4))
	title.position = Vector2(20, 15)
	_panel.add_child(title)
	var vbox = VBoxContainer.new()
	vbox.position = Vector2(20, 60)
	vbox.custom_minimum_size = Vector2(380, 270)
	vbox.add_theme_constant_override("separation", 8)
	_panel.add_child(vbox)
	for input_type in RECIPES:
		var recipe = RECIPES[input_type]
		var have = GameState.get_collection_count(input_type)
		var idef_in = GameState.ITEM_TYPES[input_type]
		var idef_out = GameState.ITEM_TYPES[recipe["output"]]
		var can_craft = have >= COST
		var row = HBoxContainer.new()
		row.add_theme_constant_override("separation", 10)
		vbox.add_child(row)
		var info = Label.new()
		info.text = "%s ×%d (%d)  →  %s %s" % [idef_in["emoji"], COST, have, recipe["emoji_out"], recipe["name_out"]]
		info.add_theme_font_size_override("font_size", 18)
		info.add_theme_color_override("font_color", Color.WHITE if can_craft else Color(0.5, 0.5, 0.5))
		info.custom_minimum_size = Vector2(260, 40)
		row.add_child(info)
		var btn = Button.new()
		btn.text = "合成" if can_craft else "不足"
		btn.disabled = not can_craft
		btn.add_theme_font_size_override("font_size", 16)
		btn.custom_minimum_size = Vector2(80, 40)
		var in_type = input_type
		btn.pressed.connect(func(): _craft(in_type))
		row.add_child(btn)
	var hint = Label.new()
	hint.text = "按 E 关闭"
	hint.add_theme_font_size_override("font_size", 14)
	hint.add_theme_color_override("font_color", Color(0.5, 0.45, 0.5))
	hint.position = Vector2(20, 320)
	_panel.add_child(hint)

func _craft(input_type: String) -> void:
	var recipe = RECIPES[input_type]
	if GameState.get_collection_count(input_type) < COST:
		return
	# 扣除
	GameState.collection[input_type] -= COST
	GameState.total_collected -= COST
	# 给产出
	GameState.collect_item(recipe["output"])
	_refresh_panel()
	EventBus.toast_message.emit("合成成功！获得 %s" % recipe["name_out"], recipe["emoji_out"])
	AudioBus.play_mission_complete()

func _process(_delta: float) -> void:
	if _player_near and not _panel.visible:
		var t = Time.get_ticks_msec() * 0.003
		_hint.position.y = 2.0 + sin(t) * 0.08

func _on_body_entered(body: Node) -> void:
	if body.is_in_group("player"):
		_player_near = true
		_hint.visible = true

func _on_body_exited(body: Node) -> void:
	if body.is_in_group("player"):
		_player_near = false
		_hint.visible = false
		_panel.visible = false
