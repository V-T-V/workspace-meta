#============================================================
# Collectible.gd — 可拾取物（Area3D + 旋转浮动 + 自动拾取 + 收集后重生）
#============================================================
extends Area3D

const Confetti = preload("res://world/Confetti.gd")
const RESPAWN_TIME: float = 30.0  # 拾取后 30 秒重生（保持持续可玩）
const CULL_DISTANCE: float = 45.0 # 超过此距离隐藏视觉（移动端省渲染）

@export var item_type: String = "apple"
var _bob_phase: float = 0.0
var _collected: bool = false
var _respawn_timer: float = 0.0
var _initial_pos: Vector3
var _visual: Node3D = null   # 缓存视觉节点引用，避免每帧遍历子节点

func _ready() -> void:
	body_entered.connect(_on_body_entered)
	_bob_phase = randf() * TAU
	_initial_pos = global_position
	# 查找视觉节点（跳过 CollisionShape3D，找承载模型的 Node3D）
	for c in get_children():
		if c is CollisionShape3D:
			continue
		if c is Node3D:
			_visual = c
			break

func _process(delta: float) -> void:
	if _collected:
		# 重生计时
		_respawn_timer -= delta
		if _respawn_timer <= 0.0:
			_respawn()
		return
	# 距离剔除 + 靠近高亮
	var player = get_tree().get_first_node_in_group("player")
	var proximity_boost := 1.0   # 靠近玩家时放大（引导注意）
	if player:
		var d = global_position.distance_to(player.global_position)
		if _visual:
			_visual.visible = d < CULL_DISTANCE
		if d >= CULL_DISTANCE:
			return   # 远处不跑旋转/脉冲动画
		# 5m 内加速旋转 + 放大（引导儿童走向拾取物）
		if d < 5.0:
			proximity_boost = 1.0 + (1.0 - d / 5.0) * 2.0   # 1x → 3x
	# 旋转 + 上下浮动 + 脉冲缩放（靠近时更快更大）
	_bob_phase += delta * 2.0 * proximity_boost
	if _visual:
		_visual.rotate_y(delta * 2.0 * proximity_boost)
		_visual.position.y = 0.5 + sin(_bob_phase) * 0.15
		var pulse_base = 1.0 + sin(_bob_phase * 1.5) * 0.08
		var pulse = pulse_base * (1.0 + (proximity_boost - 1.0) * 0.3)
		_visual.scale = Vector3(pulse, pulse, pulse)

func _on_body_entered(body: Node) -> void:
	if _collected or not body.is_in_group("player"):
		return
	# 拾取！
	var idef = GameState.ITEM_TYPES.get(item_type, {})
	var color: Color = idef.get("color", Color.WHITE)
	GameState.collect_item(item_type)
	Confetti.burst(get_tree().current_scene, global_position, color)
	EventBus.toast_message.emit("+1 %s" % idef.get("emoji", "⭐"), idef.get("emoji", "⭐"))
	# 音效反馈（即时正反馈，儿童游戏核心）
	AudioBus.play_pickup()
	# 隐藏而非销毁（30 秒后重生）
	_collected = true
	_respawn_timer = RESPAWN_TIME
	visible = false
	# 禁用碰撞
	monitoring = false

func _respawn() -> void:
	_collected = false
	visible = true
	monitoring = true
	# 重生彩纸（告诉儿童"又有新的了"）
	var idef = GameState.ITEM_TYPES.get(item_type, {})
	Confetti.burst(get_tree().current_scene, global_position, idef.get("color", Color.WHITE))
	AudioBus.play_respawn()
