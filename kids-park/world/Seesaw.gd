#============================================================
# Seesaw.gd — 跷跷板（玩家站上去自动倾斜上下）
#============================================================
# 长木板架在三角支点上
# 玩家走到一端 → 该端下沉 + 另一端翘起
# 离开 → 平衡复位
# 物理用程序化旋转模拟（非 RigidBody，避免复杂物理）
#============================================================
extends Area3D

const TILT_SPEED: float = 3.0
const MAX_TILT: float = 25.0   # 最大倾斜角度

var _board: Node3D = null
var _current_tilt: float = 0.0
var _target_tilt: float = 0.0
var _player_on: CharacterBody3D = null

func _ready() -> void:
	body_entered.connect(_on_body_entered)
	body_exited.connect(_on_body_exited)
	_build_seesaw()
	# 碰撞区（长板范围）
	var col = CollisionShape3D.new()
	var shape = BoxShape3D.new()
	shape.size = Vector3(1.0, 0.5, 4.0)
	col.shape = shape
	col.position = Vector3(0, 0.5, 0)
	add_child(col)

func _build_seesaw() -> void:
	var wood_mat = ModelFactory.get_material(Color(0.55, 0.38, 0.22), {"shaded": true})
	var paint_mat = ModelFactory.get_material(Color(0.9, 0.4, 0.3), {"emissive": Color(0.4, 0.15, 0.1), "emissive_energy": 0.2, "shaded": true})
	var metal_mat = ModelFactory.get_material(Color(0.7, 0.7, 0.72), {"metallic": 0.7, "roughness": 0.3})
	# 三角支点（固定不动）
	var pivot_base = CSGBox3D.new()
	pivot_base.size = Vector3(0.8, 0.4, 0.8)
	pivot_base.position = Vector3(0, 0.2, 0)
	pivot_base.material = metal_mat
	add_child(pivot_base)
	# 木板（会倾斜的节点）
	_board = Node3D.new()
	_board.name = "Board"
	_board.position = Vector3(0, 0.5, 0)
	add_child(_board)
	# 长木板主体
	var plank = CSGBox3D.new()
	plank.size = Vector3(0.8, 0.1, 4.0)
	plank.position = Vector3(0, 0, 0)
	plank.material = wood_mat
	_board.add_child(plank)
	# 两端彩色把手
	for sz in [-1, 1]:
		var handle = CSGBox3D.new()
		handle.size = Vector3(0.85, 0.3, 0.1)
		handle.position = Vector3(0, 0.15, sz * 1.9)
		handle.material = paint_mat
		_board.add_child(handle)
	# 手柄立柱
	for sz in [-1, 1]:
		for sx in [-1, 1]:
			var post = CSGCylinder3D.new()
			post.radius = 0.02; post.height = 0.3
			post.position = Vector3(sx * 0.35, 0.2, sz * 1.9)
			post.material = metal_mat
			_board.add_child(post)

func _process(delta: float) -> void:
	# 计算目标倾斜
	if _player_on and is_instance_valid(_player_on):
		# 根据玩家在板上的 Z 位置决定倾斜方向和幅度
		var local_z = _player_on.global_position.z - global_position.z
		var normalized = clamp(local_z / 2.0, -1.0, 1.0)
		_target_tilt = -normalized * MAX_TILT
	else:
		_target_tilt = 0.0
	# 平滑倾斜
	_current_tilt = lerp(_current_tilt, _target_tilt, delta * TILT_SPEED)
	if _board:
		_board.rotation_degrees.x = _current_tilt

func _on_body_entered(body: Node) -> void:
	if body.is_in_group("player"):
		_player_on = body as CharacterBody3D
		EventBus.toast_message.emit("跷跷板！", "⚖️")
		AudioBus.play_pickup()

func _on_body_exited(body: Node) -> void:
	if body.is_in_group("player"):
		_player_on = null
