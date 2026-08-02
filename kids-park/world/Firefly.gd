#============================================================
# Firefly.gd — 萤火虫（夜间专属发光收集物）
#============================================================
# 只在夜晚出现（light_energy < 1.0）
# 发光的飞舞小球，飘浮随机轨迹
# 拾取后给予双倍计数 + 特殊"萤火"音效
# 白天隐藏（不渲染不碰撞），夜晚自动现身
#============================================================
extends Area3D

const Confetti = preload("res://world/Confetti.gd")
const FLOAT_SPEED: float = 1.5
const CATCH_RADIUS: float = 2.0

var _firefly: OmniLight3D
var _visual: MeshInstance3D
var _bob_phase: float = 0.0
var _move_target: Vector3
var _is_night: bool = false
var _sun: Node3D = null
var _player: CharacterBody3D = null
var _mat: StandardMaterial3D

func _ready() -> void:
	body_entered.connect(_on_body_entered)
	_bob_phase = randf() * TAU
	_pick_new_target()
	# 发光体（小球 + 点光源）
	_visual = MeshInstance3D.new()
	var sphere = SphereMesh.new()
	sphere.radius = 0.08
	sphere.height = 0.16
	_visual.mesh = sphere
	_mat = StandardMaterial3D.new()
	_mat.albedo_color = Color(1.0, 0.95, 0.4)
	_mat.emissive = Color(1.0, 0.9, 0.3)
	_mat.emissive_energy_multiplier = 3.0
	_mat.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	_visual.material_override = _mat
	add_child(_visual)
	# 点光源
	_firefly = OmniLight3D.new()
	_firefly.light_color = Color(1.0, 0.9, 0.4)
	_firefly.light_energy = 1.5
	_firefly.omni_range = 4.0
	add_child(_firefly)
	# 碰撞
	var col = CollisionShape3D.new()
	var shape = SphereShape3D.new()
	shape.radius = CATCH_RADIUS
	col.shape = shape
	add_child(col)
	# 隐藏（等待夜晚）
	visible = false
	monitoring = false

func _process(delta: float) -> void:
	# 检查是否夜晚
	_check_day_night()
	if not _is_night:
		return   # 白天不做任何处理
	# 飞舞移动
	_bob_phase += delta * FLOAT_SPEED
	global_position = global_position.move_toward(_move_target, delta * 0.8)
	if global_position.distance_to(_move_target) < 0.5:
		_pick_new_target()
	# 脉冲发光（忽明忽暗的萤火虫效果）
	var pulse = 0.5 + sin(_bob_phase * 2.0) * 0.5
	_mat.emissive_energy_multiplier = 1.5 + pulse * 2.0
	_firefly.light_energy = 0.8 + pulse * 1.2

func _check_day_night() -> void:
	if _sun == null:
		_sun = get_tree().current_scene.get_node_or_null("Sun")
	var was_night = _is_night
	if _sun and _sun.has_method("is_night"):
		_is_night = _sun.is_night()
	else:
		_is_night = false
	# 切换可见性
	if was_night != _is_night:
		visible = _is_night
		monitoring = _is_night
		if _is_night:
			_pick_new_target()

func _pick_new_target() -> void:
	# 在当前位置附近随机选一个飘浮目标
	var offset = Vector3(randf_range(-3, 3), randf_range(0.5, 3), randf_range(-3, 3))
	_move_target = global_position + offset

func _on_body_entered(body: Node) -> void:
	if not _is_night or not body.is_in_group("player"):
		return
	# 拾取萤火虫（双倍奖励）
	GameState.collect_item("firefly", 2)
	Confetti.burst(get_tree().current_scene, global_position, Color(1, 0.9, 0.3))
	EventBus.toast_message.emit("萤火虫 +2！✨", "✨")
	AudioBus.play_pickup()
	# 重新定位（飘到别处）
	global_position += Vector3(randf_range(-10, 10), 0, randf_range(-10, 10))
	_pick_new_target()
