#============================================================
# Wardrobe.gd — 玩家换装系统（帽子/围巾/颜色定制）
#============================================================
# 按 H 键打开衣柜面板：
#   - 选择帽子类型（皇冠 🎪 / 兔耳 👰 / 圣诞帽 🎅 / 无）
#   - 选择围巾颜色（红/蓝/绿/黄/紫/无）
#   - 选择身体色调
# 装扮实时附加到玩家模型头顶/脖子
# 存档保存到 GameState
#============================================================
extends CanvasLayer

var _panel: Panel
var _active: bool = false
var _player: CharacterBody3D
var _hat_node: Node3D = null
var _scarf_node: Node3D = null
var _glasses_node: Node3D = null

# 当前装扮
var _hat_type: int = 0   # 0=无 1=皇冠 2=兔耳 3=圣诞帽
var _scarf_color: int = 0  # 0=无 1=红 2=蓝 3=绿 4=黄 5=紫
var _body_color: int = 0  # 身体色调索引（见 BODY_COLORS）
var _glasses: int = 0     # 0=无 1=普通 2=墨镜 3=星星

const HAT_NAMES := ["无帽", "👑皇冠", "🐰兔耳", "🎅圣诞"]
const SCARF_COLORS := [
	Color(0, 0, 0, 0),       # 无
	Color(0.9, 0.2, 0.2),    # 红
	Color(0.2, 0.4, 0.9),    # 蓝
	Color(0.2, 0.7, 0.3),    # 绿
	Color(0.95, 0.8, 0.2),   # 黄
	Color(0.7, 0.3, 0.9),    # 紫
]
# 身体色调调色板（索引 0=默认原色，其余为可染色）
# 第 0 项 color=null 表示"不改色，保留 Fox 原色"
const BODY_COLORS := [
	{"name": "原色", "color": null},
	{"name": "草莓", "color": Color(0.95, 0.55, 0.6)},
	{"name": "天空", "color": Color(0.55, 0.75, 0.95)},
	{"name": "薄荷", "color": Color(0.6, 0.92, 0.7)},
	{"name": "柠檬", "color": Color(0.98, 0.92, 0.55)},
	{"name": "薰衣草", "color": Color(0.78, 0.68, 0.95)},
	{"name": "蜜桃", "color": Color(1.0, 0.78, 0.6)},
]
const GLASSES_NAMES := ["无镜", "👓普通", "🕶️墨镜", "⭐星星"]

func _ready() -> void:
	# 从存档恢复装扮
	_hat_type = int(GameState.outfit.get("hat", 0))
	_scarf_color = int(GameState.outfit.get("scarf", 0))
	_body_color = int(GameState.outfit.get("body_color", 0))
	_glasses = int(GameState.outfit.get("glasses", 0))
	_build_ui()
	# 开局装扮由 _process 在玩家生成后一次性应用

func _unhandled_input(event: InputEvent) -> void:
	if event is InputEventKey and event.pressed and event.keycode == KEY_H:
		_toggle()

func _toggle() -> void:
	_active = not _active
	_panel.visible = _active
	_player = get_tree().get_first_node_in_group("player")

var _initial_applied: bool = false
func _process(_delta: float) -> void:
	# 玩家生成后一次性应用初始装扮（开局染色/戴帽）
	if _initial_applied:
		return
	if _player == null or not is_instance_valid(_player):
		_player = get_tree().get_first_node_in_group("player")
		return
	_apply_outfit()
	_initial_applied = true

func _build_ui() -> void:
	var root = Control.new()
	root.set_anchors_preset(Control.PRESET_FULL_RECT)
	root.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(root)
	# 遮罩
	var dim = ColorRect.new()
	dim.color = Color(0, 0, 0, 0.3)
	dim.set_anchors_preset(Control.PRESET_FULL_RECT)
	dim.visible = false
	root.add_child(dim)
	# 面板
	_panel = Panel.new()
	_panel.set_anchors_preset(Control.PRESET_CENTER)
	_panel.custom_minimum_size = Vector2(560, 520)
	_panel.position = Vector2(-280, -260)
	_panel.visible = false
	var bg = StyleBoxFlat.new()
	bg.bg_color = Color(0.98, 0.96, 0.9, 0.97)
	bg.corner_radius_top_left = 20
	bg.corner_radius_top_right = 20
	bg.corner_radius_bottom_left = 20
	bg.corner_radius_bottom_right = 20
	bg.border_width_top = 4
	bg.border_color = Color(0.9, 0.5, 0.8)
	_panel.add_theme_stylebox_override("panel", bg)
	root.add_child(_panel)
	# 标题
	var title = Label.new()
	title.text = "🎩 换装衣柜"
	title.add_theme_font_size_override("font_size", 30)
	title.position = Vector2(30, 20)
	_panel.add_child(title)
	# 帽子行
	var hat_label = Label.new()
	hat_label.text = "帽子："
	hat_label.add_theme_font_size_override("font_size", 20)
	hat_label.position = Vector2(30, 80)
	_panel.add_child(hat_label)
	var hat_row = HBoxContainer.new()
	hat_row.position = Vector2(100, 75)
	hat_row.add_theme_constant_override("separation", 8)
	_panel.add_child(hat_row)
	for i in HAT_NAMES.size():
		var idx = i
		var btn = Button.new()
		btn.text = HAT_NAMES[i]
		btn.add_theme_font_size_override("font_size", 18)
		btn.custom_minimum_size = Vector2(90, 50)
		btn.pressed.connect(func(): _select_hat(idx))
		hat_row.add_child(btn)
	# 围巾行
	var scarf_label = Label.new()
	scarf_label.text = "围巾："
	scarf_label.add_theme_font_size_override("font_size", 20)
	scarf_label.position = Vector2(30, 160)
	_panel.add_child(scarf_label)
	var scarf_row = HBoxContainer.new()
	scarf_row.position = Vector2(100, 155)
	scarf_row.add_theme_constant_override("separation", 6)
	_panel.add_child(scarf_row)
	var scarf_names = ["无", "红", "蓝", "绿", "黄", "紫"]
	for i in scarf_names.size():
		var idx = i
		var btn = Button.new()
		btn.text = scarf_names[i]
		btn.add_theme_font_size_override("font_size", 16)
		btn.custom_minimum_size = Vector2(50, 50)
		btn.pressed.connect(func(): _select_scarf(idx))
		scarf_row.add_child(btn)
	# 身体色调行（染色）
	var body_label = Label.new()
	body_label.text = "肤色："
	body_label.add_theme_font_size_override("font_size", 20)
	body_label.position = Vector2(30, 235)
	_panel.add_child(body_label)
	var body_row = HBoxContainer.new()
	body_row.position = Vector2(100, 230)
	body_row.add_theme_constant_override("separation", 5)
	_panel.add_child(body_row)
	for i in BODY_COLORS.size():
		var idx = i
		var btn = Button.new()
		btn.text = BODY_COLORS[i]["name"]
		btn.add_theme_font_size_override("font_size", 14)
		btn.custom_minimum_size = Vector2(60, 44)
		# 用按钮底色预览颜色
		if BODY_COLORS[i]["color"] != null:
			var sb = StyleBoxFlat.new()
			sb.bg_color = Color(0.98, 0.96, 0.9)
			sb.border_width_bottom = 6
			sb.border_color = BODY_COLORS[i]["color"]
			sb.corner_radius_top_left = 6
			sb.corner_radius_top_right = 6
			sb.corner_radius_bottom_left = 6
			sb.corner_radius_bottom_right = 6
			btn.add_theme_stylebox_override("normal", sb)
		btn.pressed.connect(func(): _select_body_color(idx))
		body_row.add_child(btn)
	# 眼镜行
	var glasses_label = Label.new()
	glasses_label.text = "眼镜："
	glasses_label.add_theme_font_size_override("font_size", 20)
	glasses_label.position = Vector2(30, 310)
	_panel.add_child(glasses_label)
	var glasses_row = HBoxContainer.new()
	glasses_row.position = Vector2(100, 305)
	glasses_row.add_theme_constant_override("separation", 6)
	_panel.add_child(glasses_row)
	for i in GLASSES_NAMES.size():
		var idx = i
		var btn = Button.new()
		btn.text = GLASSES_NAMES[i]
		btn.add_theme_font_size_override("font_size", 15)
		btn.custom_minimum_size = Vector2(72, 44)
		btn.pressed.connect(func(): _select_glasses(idx))
		glasses_row.add_child(btn)
	# 提示
	var hint = Label.new()
	hint.text = "按 H 关闭衣柜 · 装扮自动存档"
	hint.add_theme_font_size_override("font_size", 16)
	hint.add_theme_color_override("font_color", Color(0.5, 0.45, 0.4))
	hint.position = Vector2(30, 380)
	_panel.add_child(hint)

func _select_hat(idx: int) -> void:
	_hat_type = idx
	GameState.outfit["hat"] = idx
	GameState._save()
	_apply_outfit()
	AudioBus.play_pickup()

func _select_scarf(idx: int) -> void:
	_scarf_color = idx
	GameState.outfit["scarf"] = idx
	GameState._save()
	_apply_outfit()
	AudioBus.play_pickup()

func _select_body_color(idx: int) -> void:
	_body_color = idx
	GameState.outfit["body_color"] = idx
	GameState._save()
	_apply_outfit()
	AudioBus.play_pickup()

func _select_glasses(idx: int) -> void:
	_glasses = idx
	GameState.outfit["glasses"] = idx
	GameState._save()
	_apply_outfit()
	AudioBus.play_pickup()

func _apply_outfit() -> void:
	if _player == null:
		return
	# 找到视觉节点（Fox 模型），装扮附加到它上面（随走动动画一起动）
	var visual: Node3D = null
	for c in _player.get_children():
		if c is Node3D and c.name != "CollisionShape3D" and c.name != "CameraRig":
			visual = c
			break
	if visual == null:
		visual = _player   # 降级：附加到玩家本身
	# 应用身体染色（在清除旧装扮前，先染色本体）
	_apply_body_tint(visual)
	# 清除旧装扮
	if _hat_node:
		_hat_node.queue_free()
		_hat_node = null
	if _scarf_node:
		_scarf_node.queue_free()
		_scarf_node = null
	if _glasses_node:
		_glasses_node.queue_free()
		_glasses_node = null
	# 视觉节点头部位置（Fox 缩放后约 0.8m 高，相对视觉节点）
	var head_pos = Vector3(0, 0.85, 0)
	# 添加帽子（附加到视觉节点，随走动动画一起动）
	match _hat_type:
		1:  # 皇冠
			_hat_node = _make_crown()
			_hat_node.position = head_pos + Vector3(0, 0.15, 0)
			visual.add_child(_hat_node)
		2:  # 兔耳
			_hat_node = _make_bunny_ears()
			_hat_node.position = head_pos + Vector3(0, 0.1, 0)
			visual.add_child(_hat_node)
		3:  # 圣诞帽
			_hat_node = _make_santa_hat()
			_hat_node.position = head_pos + Vector3(0, 0.1, 0)
			visual.add_child(_hat_node)
	# 添加围巾
	if _scarf_color > 0:
		_scarf_node = _make_scarf(SCARF_COLORS[_scarf_color])
		_scarf_node.position = Vector3(0, 0.55, 0)
		visual.add_child(_scarf_node)
	# 添加眼镜（位于眼睛前方）
	if _glasses > 0:
		_glasses_node = _make_glasses(_glasses)
		_glasses_node.position = head_pos + Vector3(0, -0.05, 0.18)
		visual.add_child(_glasses_node)

## 给玩家身体模型染色（混入目标色，保留 PBR 纹理细节）
## _body_color == 0 时恢复原色（清除 override）
func _apply_body_tint(visual: Node3D) -> void:
	if _body_color <= 0 or _body_color >= BODY_COLORS.size():
		# 清除染色 override，恢复原色
		_clear_tint_override(visual)
		return
	var target: Color = BODY_COLORS[_body_color]["color"]
	_tint_recursive(visual, target)

func _tint_recursive(node: Node, tint: Color) -> void:
	if node is MeshInstance3D:
		var orig = node.get_active_material(0)
		if orig and orig is StandardMaterial3D:
			var mat = orig.duplicate() as StandardMaterial3D
			mat.albedo_color = orig.albedo_color.lerp(tint, 0.5)
			node.material_override = mat
	for c in node.get_children():
		_tint_recursive(c, tint)

func _clear_tint_override(node: Node) -> void:
	if node is MeshInstance3D:
		node.material_override = null
	for c in node.get_children():
		_clear_tint_override(c)

## 生成眼镜配件：1=普通圆框 2=墨镜 3=星星眼镜
func _make_glasses(gtype: int) -> Node3D:
	var node = Node3D.new()
	var frame_color = Color(0.2, 0.2, 0.2)
	var lens_color = Color(0.1, 0.1, 0.1, 0.6)
	match gtype:
		2:  # 墨镜
			frame_color = Color(0.1, 0.1, 0.1)
			lens_color = Color(0.05, 0.05, 0.1, 0.8)
		3:  # 星星眼镜
			frame_color = Color(0.95, 0.7, 0.1)
			lens_color = Color(1.0, 0.85, 0.3, 0.6)
	var frame_mat = ModelFactory.get_material(frame_color, {"metallic": 0.3, "roughness": 0.4})
	var lens_mat = ModelFactory.get_material(lens_color, {"emissive": lens_color, "emissive_energy": 0.2})
	# 两片镜片
	for sx in [-1, 1]:
		var lens = CSGCylinder3D.new()
		lens.radius = 0.09; lens.height = 0.02
		lens.position = Vector3(sx * 0.12, 0, 0)
		lens.rotation_degrees = Vector3(90, 0, 0)
		lens.material = lens_mat
		node.add_child(lens)
		# 镜框（环）
		var ring = CSGCylinder3D.new()
		ring.radius = 0.1; ring.height = 0.015
		ring.position = Vector3(sx * 0.12, 0, 0)
		ring.rotation_degrees = Vector3(90, 0, 0)
		ring.material = frame_mat
		node.add_child(ring)
	# 鼻梁（连接横条）
	var bridge = CSGBox3D.new()
	bridge.size = Vector3(0.06, 0.015, 0.015)
	bridge.position = Vector3(0, 0, 0)
	bridge.material = frame_mat
	node.add_child(bridge)
	# 星星眼镜额外加两个星标
	if gtype == 3:
		for sx in [-1, 1]:
			var star = CSGSphere3D.new()
			star.radius = 0.03; star.position = Vector3(sx * 0.12, 0.08, 0.01)
			star.material = frame_mat
			node.add_child(star)
	return node

func _make_crown() -> Node3D:
	var node = Node3D.new()
	var gold = ModelFactory.get_material(Color(1.0, 0.85, 0.1), {"metallic": 0.9, "roughness": 0.1, "emissive": Color(0.5, 0.4, 0), "emissive_energy": 0.3})
	# 环形底座
	var ring = CSGCylinder3D.new()
	ring.radius = 0.18; ring.height = 0.1
	ring.scale = Vector3(1, 1, 1)
	ring.material = gold
	node.add_child(ring)
	# 5 个尖
	for i in 5:
		var a = TAU * i / 5.0
		var spike = CSGCylinder3D.new()
		spike.radius = 0.03; spike.height = 0.12
		spike.position = Vector3(cos(a) * 0.15, 0.1, sin(a) * 0.15)
		spike.material = gold
		node.add_child(spike)
	return node

func _make_bunny_ears() -> Node3D:
	var node = Node3D.new()
	var pink = ModelFactory.get_material(Color(0.95, 0.7, 0.8))
	for sx in [-1, 1]:
		var ear = CSGCylinder3D.new()
		ear.radius = 0.04; ear.height = 0.3
		ear.position = Vector3(sx * 0.08, 0.2, 0)
		ear.scale = Vector3(0.6, 1, 0.6)
		ear.material = pink
		node.add_child(ear)
	return node

func _make_santa_hat() -> Node3D:
	var node = Node3D.new()
	var red = ModelFactory.get_material(Color(0.85, 0.1, 0.1))
	var white = ModelFactory.get_material(Color(0.95, 0.95, 0.95))
	# 锥体帽
	var cone = CSGCylinder3D.new()
	cone.radius = 0.15; cone.height = 0.35
	cone.position = Vector3(0, 0.15, 0)
	cone.rotation_degrees = Vector3(15, 0, 0)
	cone.scale = Vector3(1, 1, 0.3)
	cone.material = red
	node.add_child(cone)
	# 白色毛球
	var pom = CSGSphere3D.new()
	pom.radius = 0.06; pom.position = Vector3(0, 0.33, -0.05)
	pom.material = white
	node.add_child(pom)
	# 白色帽檐
	var brim = CSGCylinder3D.new()
	brim.radius = 0.17; brim.height = 0.05
	brim.position = Vector3(0, 0, 0)
	brim.material = white
	node.add_child(brim)
	return node

func _make_scarf(color: Color) -> Node3D:
	var node = Node3D.new()
	var mat = ModelFactory.get_material(color, {"emissive": color, "emissive_energy": 0.15})
	# 环绕脖子
	var ring = CSGCylinder3D.new()
	ring.radius = 0.22; ring.height = 0.1
	ring.scale = Vector3(1, 1, 0.6)
	ring.material = mat
	node.add_child(ring)
	# 飘带
	var tail = CSGBox3D.new()
	tail.size = Vector3(0.08, 0.25, 0.02)
	tail.position = Vector3(0.18, -0.1, 0.1)
	tail.rotation_degrees = Vector3(0, 0, 15)
	tail.material = mat
	node.add_child(tail)
	return node
