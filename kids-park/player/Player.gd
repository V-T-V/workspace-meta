#============================================================
# Player.gd — 玩家角色（第三人称，悠闲移动+跳跃，无伤害）
#============================================================
extends CharacterBody3D

const GRAVITY: float = 18.0
const MOVE_SPEED: float = 4.0      # 悠闲速度（比 city-hunt 低）
const JUMP_VELOCITY: float = 7.0

var touch_move_x: float = 0.0
var touch_move_y: float = 0.0
var touch_jump: bool = false
var yaw: float = 0.0
var pitch: float = 0.0

var _visual: Node3D = null        # 视觉节点（Fox.glb 或 CSG）—— 做走动动画
var _visual_base_rot: Vector3 = Vector3.ZERO   # 视觉节点初始旋转（保留模型朝向）
var _walk_phase: float = 0.0      # 走动相位（驱动上下颠簸 + 摇摆）
var _is_moving: bool = false
var _dust_timer: float = 0.0      # 走动尘土发射计时
var _last_zone: String = ""       # 上次所在区域（驱动背景音乐切换）

func _ready() -> void:
	add_to_group("player")
	global_position = ParkGen.get_spawn()
	# 查找视觉子节点（Main.gd 把 Fox 模型作为唯一子节点挂进来）
	for c in get_children():
		if c is Node3D and c.name != "CollisionShape3D" and c.name != "CameraRig":
			_visual = c
			_visual_base_rot = c.rotation_degrees
			break

func _physics_process(delta: float) -> void:
	velocity.y -= GRAVITY * delta
	if _is_jump_pressed() and is_on_floor():
		velocity.y = JUMP_VELOCITY
		AudioBus.play_jump()
	# 移动（相机相对方向）
	var input_dir = _get_move_input()
	var cam = get_viewport().get_camera_3d()
	_is_moving = false
	if cam:
		var cb = cam.global_transform.basis
		var forward = -cb.z
		forward.y = 0
		forward = forward.normalized()
		var right = cb.x
		right.y = 0
		right = right.normalized()
		var move_dir = (forward * -input_dir.y + right * input_dir.x).normalized()
		velocity.x = move_dir.x * MOVE_SPEED
		velocity.z = move_dir.z * MOVE_SPEED
		if move_dir != Vector3.ZERO:
			_is_moving = true
			var target_angle = atan2(move_dir.x, move_dir.z)
			rotation.y = lerp_angle(rotation.y, target_angle, delta * 10.0)
	else:
		velocity.x = 0
		velocity.z = 0
	move_and_slide()
	_animate_walk(delta)
	_update_zone_music()

# 根据玩家位置切换背景音乐
func _update_zone_music() -> void:
	var zone = _get_current_zone()
	if zone != _last_zone:
		_last_zone = zone
		AudioBus.set_zone_music(zone)
		# 同时切换天气
		var weather = get_tree().current_scene.get_node_or_null("Weather")
		if weather and weather.has_method("set_zone_weather"):
			weather.set_zone_weather(zone)

# 判断玩家当前所在区域
func _get_current_zone() -> String:
	var pos = global_position
	var best_zone = ""
	# 找最近已解锁的区域中心
	var best_dist = INF
	for zone_id in ParkGen.ZONE_CENTERS:
		if not GameState.is_zone_unlocked(zone_id):
			continue
		var center = ParkGen.ZONE_CENTERS[zone_id]
		var d = pos.distance_to(center)
		if d < best_dist:
			best_dist = d
			best_zone = zone_id
	return best_zone

func _unhandled_input(event: InputEvent) -> void:
	# E 键互动（键盘玩家也需要能和 NPC 对话）
	if event is InputEventKey and event.pressed and event.keycode == KEY_E:
		_try_interact_nearby()

func _try_interact_nearby() -> void:
	for npc in get_tree().get_nodes_in_group("npc"):
		if npc.has_method("interact") and global_position.distance_to(npc.global_position) < 4.0:
			npc.interact()
			return

# 走动动画：上下小颠簸 + 左右轻微摇摆，静止时回归
func _animate_walk(delta: float) -> void:
	if _visual == null:
		return
	if _is_moving and is_on_floor():
		_walk_phase += delta * 12.0   # 步频
		var bounce = sin(_walk_phase) * 0.06
		var sway = sin(_walk_phase * 0.5) * 2.0   # 度
		_visual.position = Vector3(0, bounce, 0)
		# 保留基础朝向，只在 Z 轴叠加摇摆
		_visual.rotation_degrees = Vector3(_visual_base_rot.x, _visual_base_rot.y, sway)
		# 每步落地发尘土（步频一半周期 = 每步一次）
		_dust_timer -= delta
		if _dust_timer <= 0.0:
			_dust_timer = 0.35   # 约 3 步/秒
			_spawn_dust()
			AudioBus.play_step()
	else:
		# 平滑回归静止
		_walk_phase = 0.0
		_visual.position = _visual.position.lerp(Vector3.ZERO, delta * 8.0)
		var rd = _visual.rotation_degrees
		_visual.rotation_degrees = rd.lerp(_visual_base_rot, delta * 8.0)

# 走动尘土：脚下小粒子爆发（即时视觉反馈，强化"在动"的感觉）
func _spawn_dust() -> void:
	var p := GPUParticles3D.new()
	p.global_position = global_position + Vector3(0, 0.05, 0)
	p.amount = 6
	p.lifetime = 0.4
	p.one_shot = true
	p.emitting = true
	p.explosiveness = 0.5
	var mat := ParticleProcessMaterial.new()
	mat.direction = Vector3(0, 1, 0)
	mat.spread = 25.0
	mat.initial_velocity_min = 0.5
	mat.initial_velocity_max = 1.5
	mat.gravity = Vector3(0, -3, 0)
	mat.scale_min = 0.08
	mat.scale_max = 0.15
	mat.color = Color(0.85, 0.8, 0.7, 0.6)
	p.process_material = mat
	var sphere := SphereMesh.new()
	sphere.radius = 0.08
	sphere.height = 0.16
	p.draw_pass_1 = sphere
	get_tree().current_scene.add_child(p)
	get_tree().create_timer(0.6).timeout.connect(func():
		if is_instance_valid(p):
			p.queue_free()
	)

func _get_move_input() -> Vector2:
	var x := 0.0
	var y := 0.0
	if Input.is_physical_key_pressed(KEY_W): y -= 1.0
	if Input.is_physical_key_pressed(KEY_S): y += 1.0
	if Input.is_physical_key_pressed(KEY_A): x -= 1.0
	if Input.is_physical_key_pressed(KEY_D): x += 1.0
	x += touch_move_x
	y += touch_move_y
	return Vector2(clamp(x, -1.0, 1.0), clamp(y, -1.0, 1.0))

func _is_jump_pressed() -> bool:
	return Input.is_physical_key_pressed(KEY_SPACE) or touch_jump
