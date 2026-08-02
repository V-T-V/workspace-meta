#============================================================
# MysteriousMerchant.gd — 神秘商人（夜晚出没，物物交换）
#============================================================
# 只在夜晚出现（联动 DayNightCycle.is_night()）
# 戴帽斗篷的神秘角色，站在中心广场喷泉旁
# 用 5 个普通物品换 1 个稀有金星（超值交换！）
# 白天消失（隐藏 + 关闭碰撞）
#============================================================
extends Area3D

const Confetti = preload("res://world/Confetti.gd")
const TRADE_COST: int = 5   # 5 个随机物品换 1 金星

var _visual: Node3D = null
var _is_night: bool = false
var _sun: Node3D = null
var _hint: Label3D = null
var _player_near: bool = false
var _trade_cooldown: float = 0.0

func _ready() -> void:
	body_entered.connect(_on_body_entered)
	body_exited.connect(_on_body_exited)
	_visual = _build_merchant()
	add_child(_visual)
	# 碰撞
	var col = CollisionShape3D.new()
	var shape = CapsuleShape3D.new()
	shape.height = 1.0; shape.radius = 0.4
	col.shape = shape
	col.position = Vector3(0, 0.5, 0)
	add_child(col)
	# 提示
	_hint = Label3D.new()
	_hint.text = "🧙 神秘商人\n按 E 交换"
	_hint.font_size = 24
	_hint.position = Vector3(0, 2.0, 0)
	_hint.billboard = BaseMaterial3D.BILLBOARD_ENABLED
	_hint.outline_size = 6
	_hint.outline_modulate = Color(0, 0, 0, 0.6)
	_hint.visible = false
	add_child(_hint)
	# 默认隐藏（等夜晚）
	visible = false
	monitoring = false

func _build_merchant() -> Node3D:
	var node = Node3D.new()
	var robe_mat = ModelFactory.get_material(Color(0.25, 0.15, 0.4), {"emissive": Color(0.1, 0.05, 0.2), "emissive_energy": 0.3, "shaded": true})
	var hat_mat = ModelFactory.get_material(Color(0.15, 0.1, 0.3), {"emissive": Color(0.05, 0.03, 0.1), "emissive_energy": 0.2})
	var gold_mat = ModelFactory.get_material(Color(1.0, 0.85, 0.2), {"metallic": 0.9, "roughness": 0.15, "emissive": Color(0.5, 0.4, 0), "emissive_energy": 0.5})
	# 斗篷身体（圆锥）
	var robe = CSGCylinder3D.new()
	robe.radius = 0.4; robe.height = 1.2
	robe.position = Vector3(0, 0.6, 0)
	robe.cone = true
	robe.material = robe_mat
	node.add_child(robe)
	# 头（隐藏在帽子下，只露一点）
	var head = CSGSphere3D.new()
	head.radius = 0.18; head.position = Vector3(0, 1.3, 0)
	head.material = robe_mat
	node.add_child(head)
	# 尖帽（大圆锥）
	var hat = CSGCylinder3D.new()
	hat.radius = 0.25; hat.height = 0.6
	hat.position = Vector3(0, 1.6, 0)
	hat.cone = true
	hat.material = hat_mat
	node.add_child(hat)
	# 帽尖星星
	var star = CSGSphere3D.new()
	star.radius = 0.06; star.position = Vector3(0, 1.95, 0)
	star.material = gold_mat
	node.add_child(star)
	# 发光眼睛（两个发光小球）
	for sx in [-1, 1]:
		var eye = CSGSphere3D.new()
		eye.radius = 0.03; eye.position = Vector3(sx * 0.06, 1.32, 0.15)
		eye.material = gold_mat
		node.add_child(eye)
	# 手持的发光水晶（交换物暗示）
	var crystal = CSGBox3D.new()
	crystal.size = Vector3(0.1, 0.15, 0.1)
	crystal.position = Vector3(0.3, 0.8, 0.2)
	crystal.rotation_degrees = Vector3(45, 45, 0)
	crystal.material = gold_mat
	crystal.name = "Crystal"
	node.add_child(crystal)
	# 自带光源（神秘紫光）
	var light = OmniLight3D.new()
	light.position = Vector3(0, 1.0, 0)
	light.light_color = Color(0.6, 0.3, 1.0)
	light.light_energy = 1.5
	light.omni_range = 5.0
	node.add_child(light)
	return node

func _process(delta: float) -> void:
	if _trade_cooldown > 0:
		_trade_cooldown -= delta
	# 检查昼夜
	if _sun == null:
		_sun = get_tree().current_scene.get_node_or_null("Sun")
	var was_night = _is_night
	if _sun and _sun.has_method("is_night"):
		_is_night = _sun.is_night()
	# 切换出现/消失
	if was_night != _is_night:
		visible = _is_night
		monitoring = _is_night
		if _is_night:
			EventBus.toast_message.emit("神秘商人出现了！", "🧙")
	# 水晶旋转
	if _visual and _is_night:
		var crystal = _visual.get_node_or_null("Crystal")
		if crystal:
			crystal.rotate_y(delta * 2.0)

func _on_body_entered(body: Node) -> void:
	if _is_night and body.is_in_group("player"):
		_player_near = true
		_hint.visible = true

func _on_body_exited(body: Node) -> void:
	if body.is_in_group("player"):
		_player_near = false
		_hint.visible = false

func _unhandled_input(event: InputEvent) -> void:
	if event is InputEventKey and event.pressed and event.keycode == KEY_E:
		if _player_near and _is_night and _trade_cooldown <= 0:
			_trade()

func _trade() -> void:
	# 需要至少 TRADE_COST 个物品
	if GameState.total_collected < TRADE_COST:
		EventBus.toast_message.emit("物品不够哦（需要 %d 个）" % TRADE_COST, "🧙")
		return
	_trade_cooldown = 3.0
	# 扣除 5 个随机物品（从已有收集中扣）
	var deducted = 0
	for item_type in GameState.collection.keys():
		while GameState.collection[item_type] > 0 and deducted < TRADE_COST:
			GameState.collection[item_type] -= 1
			GameState.total_collected -= 1
			deducted += 1
		if deducted >= TRADE_COST:
			break
	# 给稀有金星
	GameState.collect_item("goldstar", 1)
	Confetti.burst(get_tree().current_scene, global_position + Vector3(0, 1, 0), Color(0.6, 0.3, 1.0))
	EventBus.toast_message.emit("交换成功！获得稀有金星 🌟", "🧙")
	AudioBus.play_zone_unlock()
	EventBus.collection_updated.emit(GameState.total_collected)
