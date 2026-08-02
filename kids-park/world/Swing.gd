#============================================================
# Swing.gd — 秋千互动设施（荡漾+摆动+音效）
#============================================================
# 玩家靠近按 E 键开始荡秋千：
#   - 座椅自动前后摆动（sin 波驱动）
#   - 摆幅随时间增大（越荡越高）
#   - 5 秒后自动跳下 + 彩纸庆祝
# 每个区域 1 个，放在树下
#============================================================
extends Area3D

const Confetti = preload("res://world/Confetti.gd")
const SWING_DURATION: float = 5.0
const MAX_ANGLE: float = 55.0   # 最大摆角（度）

var _seat: Node3D = null
var _swing_active: bool = false
var _swing_timer: float = 0.0
var _swing_phase: float = 0.0
var _player_on_seat: CharacterBody3D = null
var _hint: Label3D = null

func _ready() -> void:
	body_entered.connect(_on_body_entered)
	body_exited.connect(_on_body_exited)
	_seat = _build_swing()
	add_child(_seat)
	# 碰撞区（座椅范围）
	var col = CollisionShape3D.new()
	var shape = BoxShape3D.new()
	shape.size = Vector3(1.0, 1.5, 1.0)
	col.shape = shape
	col.position = Vector3(0, 1.0, 0)
	add_child(col)
	# 交互提示
	_hint = Label3D.new()
	_hint.text = "荡秋千 🤸 按 E"
	_hint.font_size = 24
	_hint.position = Vector3(0, 3.5, 0)
	_hint.billboard = BaseMaterial3D.BILLBOARD_ENABLED
	_hint.outline_size = 6
	_hint.outline_modulate = Color(0, 0, 0, 0.5)
	_hint.visible = false
	add_child(_hint)

func _build_swing() -> Node3D:
	var node = Node3D.new()
	var frame_mat = ModelFactory.get_material(Color(0.4, 0.3, 0.15), {"shaded": true})
	var rope_mat = ModelFactory.get_material(Color(0.55, 0.4, 0.2))
	var seat_mat = ModelFactory.get_material(Color(0.8, 0.3, 0.3), {"emissive": Color(0.4, 0.1, 0.1), "emissive_energy": 0.2, "shaded": true})
	# A 字架（两根斜柱 + 横梁）
	for sx in [-1, 1]:
		var post = CSGCylinder3D.new()
		post.radius = 0.06; post.height = 3.5
		post.position = Vector3(sx * 1.0, 1.75, 0)
		post.rotation_degrees = Vector3(0, 0, sx * 15)
		post.material = frame_mat
		node.add_child(post)
	var beam = CSGCylinder3D.new()
	beam.radius = 0.06; beam.height = 2.2
	beam.position = Vector3(0, 3.3, 0)
	beam.rotation_degrees = Vector3(0, 0, 90)
	beam.material = frame_mat
	node.add_child(beam)
	# 摆动轴（看不见的 pivot）
	var pivot = Node3D.new()
	pivot.name = "Pivot"
	pivot.position = Vector3(0, 3.2, 0)
	node.add_child(pivot)
	# 绳子 + 座椅（挂在 pivot 下，随 pivot 摆动）
	var rope_l = CSGCylinder3D.new()
	rope_l.radius = 0.015; rope_l.height = 2.0
	rope_l.position = Vector3(-0.3, -1.0, 0)
	rope_l.material = rope_mat
	pivot.add_child(rope_l)
	var rope_r = CSGCylinder3D.new()
	rope_r.radius = 0.015; rope_r.height = 2.0
	rope_r.position = Vector3(0.3, -1.0, 0)
	rope_r.material = rope_mat
	pivot.add_child(rope_r)
	var seat = CSGBox3D.new()
	seat.size = Vector3(0.8, 0.08, 0.4)
	seat.position = Vector3(0, -2.0, 0)
	seat.material = seat_mat
	pivot.add_child(seat)
	return node

func _process(delta: float) -> void:
	if _swing_active:
		_swing_timer += delta
		_swing_phase += delta * 2.5
		# 摆幅随时间增大（越荡越高）
		var progress = min(_swing_timer / SWING_DURATION, 1.0)
		var amplitude = lerp(15.0, MAX_ANGLE, progress)
		var pivot = _seat.get_node_or_null("Pivot")
		if pivot:
			pivot.rotation_degrees.x = sin(_swing_phase) * amplitude
		# 玩家跟随座椅摆动（视觉上在荡）
		if _player_on_seat and is_instance_valid(_player_on_seat):
			_player_on_seat.global_position = global_position + Vector3(0, 1.2, 0) + Vector3(0, 0, sin(_swing_phase) * amplitude * 0.02)
		# 结束
		if _swing_timer >= SWING_DURATION:
			_end_swing()

func _on_body_entered(body: Node) -> void:
	if body.is_in_group("player") and not _swing_active:
		_hint.visible = true

func _on_body_exited(body: Node) -> void:
	if body.is_in_group("player"):
		_hint.visible = false

func _unhandled_input(event: InputEvent) -> void:
	if event is InputEventKey and event.pressed and event.keycode == KEY_E:
		if _hint.visible and not _swing_active:
			_start_swing()

func _start_swing() -> void:
	_swing_active = true
	_swing_timer = 0.0
	_swing_phase = 0.0
	_player_on_seat = get_tree().get_first_node_in_group("player")
	_hint.visible = false
	EventBus.toast_message.emit("荡起来啦！", "🤸")
	AudioBus.play_pickup()

func _end_swing() -> void:
	_swing_active = false
	# 复位摆动
	var pivot = _seat.get_node_or_null("Pivot")
	if pivot:
		pivot.rotation_degrees.x = 0
	# 彩纸庆祝
	Confetti.burst(get_tree().current_scene, global_position + Vector3(0, 1, 0), Color(0.9, 0.3, 0.5))
	EventBus.toast_message.emit("好开心！", "🎉")
	AudioBus.play_mission_complete()
