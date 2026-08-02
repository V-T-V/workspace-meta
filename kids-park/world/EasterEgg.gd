#============================================================
# EasterEgg.gd — 彩蛋系统（隐藏互动点，发现获稀有奖励）
#============================================================
# 每个区域藏 1 个隐蔽彩蛋（小型发光物，位置偏僻）
# 玩家靠近 2m 内自动发现 → 彩纸 + 稀有奖励 + "发现彩蛋"成就
# 全部 4 个彩蛋集齐 → 额外"探险家"贴纸
# 彩蛋一次性（发现后永久消失，存档记录）
#============================================================
extends Area3D

const Confetti = preload("res://world/Confetti.gd")
const DISCOVER_RADIUS: float = 2.5

@export var egg_id: String = "egg_grassland"
var _discovered: bool = false
var _visual: Node3D = null
var _hint: Label3D = null

func _ready() -> void:
	add_to_group("easter_egg")
	body_entered.connect(_on_body_entered)
	# 检查存档是否已发现
	if GameState.achievements.has("egg_all_found"):
		_discovered = true
	# 构建彩蛋外观（小型发光金蛋）
	_visual = _build_egg()
	_visual.visible = not _discovered
	add_child(_visual)
	# 碰撞
	var col = CollisionShape3D.new()
	var shape = SphereShape3D.new()
	shape.radius = DISCOVER_RADIUS
	col.shape = shape
	add_child(col)
	monitoring = not _discovered

func _build_egg() -> Node3D:
	var node = Node3D.new()
	var gold = ModelFactory.get_material(Color(1.0, 0.85, 0.2), {
		"emissive": Color(1.0, 0.7, 0.1),
		"emissive_energy": 1.0,
		"metallic": 0.8,
		"roughness": 0.15,
		"shaded": true,
	})
	var stripe = ModelFactory.get_material(Color(0.9, 0.3, 0.4), {"emissive": Color(0.4, 0.1, 0.15), "emissive_energy": 0.3})
	# 蛋体（椭圆球）
	var body = CSGSphere3D.new()
	body.radius = 0.18
	body.scale = Vector3(0.85, 1.15, 0.85)
	body.material = gold
	body.name = "EggBody"
	node.add_child(body)
	# 中间彩色条纹
	var band = CSGCylinder3D.new()
	band.radius = 0.19; band.height = 0.04
	band.position = Vector3(0, 0, 0)
	band.material = stripe
	node.add_child(band)
	# 发光光源
	var light = OmniLight3D.new()
	light.light_color = Color(1.0, 0.8, 0.3)
	light.light_energy = 1.5
	light.omni_range = 4.0
	node.add_child(light)
	return node

func _process(_delta: float) -> void:
	if _discovered:
		return
	# 旋转 + 上下浮动（吸引注意）
	if _visual:
		var t = Time.get_ticks_msec() * 0.002
		_visual.rotate_y(0.02)
		_visual.position.y = 0.3 + sin(t) * 0.1

func _on_body_entered(body: Node) -> void:
	if _discovered or not body.is_in_group("player"):
		return
	_discover()

func _discover() -> void:
	_discovered = true
	monitoring = false
	if _visual:
		_visual.visible = false
	# 奖励
	GameState.collect_item("goldstar", 1)
	Confetti.burst(get_tree().current_scene, global_position + Vector3(0, 1, 0), Color(1.0, 0.85, 0.2))
	EventBus.toast_message.emit("发现隐藏彩蛋！+稀有金星 🌟", "🥚")
	AudioBus.play_sticker()
	# 检查是否全部发现
	var found_count = 0
	for egg in get_tree().get_nodes_in_group("easter_egg"):
		if egg._discovered:
			found_count += 1
	if found_count >= 4:
		GameState.earn_sticker("🗺️乐园探险家")
		EventBus.toast_message.emit("全部彩蛋发现！探险家贴纸！", "🏆")
		AudioBus.play_zone_unlock()
