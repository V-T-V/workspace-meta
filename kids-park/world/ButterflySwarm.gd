#============================================================
# ButterflySwarm.gd — 蝴蝶群飞（草地专属动态生物群）
#============================================================
# 一群彩色蝴蝶在草地上空飞舞，跟随玩家但不吸附
# 每只蝴蝶独立随机轨迹（sin/cos 混合 + turbulence）
# 儿童视觉吸引力强，增强"活的乐园"感
#============================================================
extends Node3D

var _player: CharacterBody3D = null
var _butterflies: Array = []   # [{node, phase, speed, offset, color}]
var _center: Vector3

func _ready() -> void:
	_center = global_position
	_spawn_butterflies()

func _spawn_butterflies() -> void:
	var colors = [
		Color(0.9, 0.5, 0.7),   # 粉
		Color(0.5, 0.8, 1.0),   # 蓝
		Color(1.0, 0.85, 0.3),  # 黄
		Color(0.6, 0.9, 0.5),   # 绿
		Color(0.85, 0.5, 1.0),  # 紫
	]
	for i in 8:
		var bf = CSGBox3D.new()
		bf.size = Vector3(0.15, 0.01, 0.1)
		var mat = ModelFactory.get_material(colors[i % colors.size()], {
			"emissive": colors[i % colors.size()],
			"emissive_energy": 0.3,
			"shaded": true,
		})
		bf.material = mat
		add_child(bf)
		_butterflies.append({
			"node": bf,
			"phase": randf() * TAU,
			"speed": randf_range(1.5, 3.0),
			"offset": Vector3(randf_range(-8, 8), randf_range(1.5, 4), randf_range(-8, 8)),
			"radius": randf_range(2, 5),
		})

func _process(delta: float) -> void:
	if _player == null or not is_instance_valid(_player):
		_player = get_tree().get_first_node_in_group("player")
		if _player == null:
			return
	# 以玩家为中心飞舞（但保持一定距离，不吸附）
	var center = _player.global_position if _player else _center
	for bf in _butterflies:
		bf.phase += delta * bf.speed
		var t = bf.phase
		# 蝴蝶轨迹：8 字形 + 上下浮动
		var pos = center + bf.offset + Vector3(
			sin(t) * bf.radius,
			cos(t * 2.3) * 0.5,
			sin(t * 0.7) * bf.radius * 0.8
		)
		bf.node.global_position = pos
		# 蝴蝶朝向运动方向（简化：面向飞行切线）
		bf.node.rotation_degrees.y = rad_to_deg(t * 0.5)
		# 翅膀拍动（缩放 X 模拟翅膀开合）
		bf.node.scale.x = 0.8 + abs(sin(t * 8)) * 0.4
