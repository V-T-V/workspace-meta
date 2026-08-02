#============================================================
# BubbleMachine.gd — 泡泡机（持续发射漂浮泡泡，戳破获奖）
#============================================================
# 机器造型：底座+透明圆顶+喷嘴
# 每 0.3 秒发射一个泡泡（半透明球，向上漂浮+左右摇摆）
# 泡泡碰到玩家/飞太高 → 破裂（小粒子）+ 概率掉落收集物
# 沙滩专属趣味设施
#============================================================
extends Node3D

const Confetti = preload("res://world/Confetti.gd")
const SPAWN_INTERVAL: float = 0.4
const MAX_BUBBLES: int = 15
const FLOAT_SPEED: float = 1.5

var _spawn_timer: float = 0.0
var _bubbles: Array = []   # [{node, velocity, lifetime}]
var _bubble_mat: StandardMaterial3D

func _ready() -> void:
	_build_machine()
	# 泡泡材质（半透明+高光）
	_bubble_mat = StandardMaterial3D.new()
	_bubble_mat.albedo_color = Color(0.8, 0.9, 1.0, 0.3)
	_bubble_mat.transparency = BaseMaterial3D.TRANSPARENCY_ALPHA
	_bubble_mat.metallic = 0.3
	_bubble_mat.roughness = 0.0
	_bubble_mat.emissive = Color(0.5, 0.7, 1.0)
	_bubble_mat.emissive_energy_multiplier = 0.3

func _build_machine() -> void:
	var base_mat = ModelFactory.get_material(Color(0.3, 0.6, 0.8), {"emissive": Color(0.1, 0.3, 0.5), "emissive_energy": 0.3, "metallic": 0.5, "roughness": 0.3, "shaded": true})
	var gold_mat = ModelFactory.get_material(Color(1.0, 0.85, 0.2), {"metallic": 0.8, "roughness": 0.2})
	# 底座（圆柱）
	var base = CSGCylinder3D.new()
	base.radius = 0.4; base.height = 0.6
	base.position = Vector3(0, 0.3, 0)
	base.material = base_mat
	add_child(base)
	# 金色装饰环
	var ring = CSGCylinder3D.new()
	ring.radius = 0.42; ring.height = 0.08
	ring.position = Vector3(0, 0.6, 0)
	ring.material = gold_mat
	add_child(ring)
	# 透明圆顶（半球=压扁球）
	var dome = CSGSphere3D.new()
	dome.radius = 0.35; dome.scale = Vector3(1, 0.7, 1)
	dome.position = Vector3(0, 0.8, 0)
	dome.material = _bubble_mat
	add_child(dome)
	# 顶部喷嘴
	var nozzle = CSGCylinder3D.new()
	nozzle.radius = 0.08; nozzle.height = 0.3
	nozzle.position = Vector3(0, 1.1, 0)
	nozzle.material = gold_mat
	add_child(nozzle)
	# 发光（暗示在工作）
	var light = OmniLight3D.new()
	light.position = Vector3(0, 0.8, 0)
	light.light_color = Color(0.6, 0.85, 1.0)
	light.light_energy = 0.8
	light.omni_range = 3.0
	add_child(light)

func _process(delta: float) -> void:
	# 发射新泡泡
	_spawn_timer -= delta
	if _spawn_timer <= 0 and _bubbles.size() < MAX_BUBBLES:
		_spawn_timer = SPAWN_INTERVAL
		_spawn_bubble()
	# 更新所有泡泡
	var i = _bubbles.size() - 1
	while i >= 0:
		var b = _bubbles[i]
		b.lifetime -= delta
		var node = b.node
		if node:
			# 上浮 + 左右摇摆
			node.global_position += b.velocity * delta
			b.velocity.x = sin(Time.get_ticks_msec() * 0.002 + i) * 0.5
			node.rotate_y(delta * 1.0)
		# 破裂条件：飞太高 / 寿命到
		var should_pop = b.lifetime <= 0 or node.global_position.y > 8.0
		if should_pop:
			_pop_bubble(b)
			_bubbles.remove_at(i)
		i -= 1

func _spawn_bubble() -> void:
	var bubble = MeshInstance3D.new()
	var sphere = SphereMesh.new()
	sphere.radius = randf_range(0.1, 0.25)
	sphere.height = sphere.radius * 2
	bubble.mesh = sphere
	bubble.material_override = _bubble_mat
	bubble.global_position = global_position + Vector3(randf_range(-0.1, 0.1), 1.3, randf_range(-0.1, 0.1))
	add_child(bubble)
	_bubbles.append({
		"node": bubble,
		"velocity": Vector3(0, FLOAT_SPEED + randf_range(-0.3, 0.5), 0),
		"lifetime": randf_range(4.0, 7.0),
	})

func _pop_bubble(b: Dictionary) -> void:
	var node = b.node
	if node and is_instance_valid(node):
		# 小粒子爆发
		Confetti.burst(get_tree().current_scene, node.global_position, Color(0.7, 0.9, 1.0))
		# 20% 概率掉落收集物
		if randf() < 0.2:
			GameState.collect_item("pearl")
			EventBus.toast_message.emit("泡泡里有珍珠！", "🫧")
		node.queue_free()
