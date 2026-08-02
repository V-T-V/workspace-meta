#============================================================
# Balloon.gd — 气球装饰（可戳破+随机奖励）
#============================================================
# 飘浮在空中的彩色气球，玩家跳起来碰到就戳破
# 戳破后：气球碎片彩纸 + 随机奖励（1-3 个物品）
# 5 秒后在随机位置重新生成（永远有气球可戳）
#============================================================
extends Area3D

const Confetti = preload("res://world/Confetti.gd")

var _visual: Node3D = null
var _balloon_color: Color
var _respawn_timer: float = 0.0
var _active: bool = true

func _ready() -> void:
	body_entered.connect(_on_body_entered)
	_balloon_color = Color(randf(), randf(), randf())
	_visual = _build_balloon()
	add_child(_visual)
	# 碰撞区
	var col = CollisionShape3D.new()
	var shape = SphereShape3D.new()
	shape.radius = 0.6
	col.shape = shape
	add_child(col)

func _build_balloon() -> Node3D:
	var node = Node3D.new()
	var mat = ModelFactory.get_material(_balloon_color, {
		"emissive": _balloon_color,
		"emissive_energy": 0.2,
		"metallic": 0.2,
		"roughness": 0.3,
		"shaded": true,
	})
	# 气球本体（椭球）
	var body = CSGSphere3D.new()
	body.radius = 0.35
	body.scale = Vector3(1, 1.3, 1)
	body.material = mat
	body.name = "BalloonBody"
	node.add_child(body)
	# 气球结
	var knot = CSGSphere3D.new()
	knot.radius = 0.04; knot.position = Vector3(0, -0.45, 0)
	knot.material = mat
	node.add_child(knot)
	# 细绳
	var rope = CSGCylinder3D.new()
	rope.radius = 0.005; rope.height = 1.5
	rope.position = Vector3(0, -1.2, 0)
	var rmat = ModelFactory.get_material(Color(0.9, 0.9, 0.9))
	rope.material = rmat
	node.add_child(rope)
	return node

func _process(delta: float) -> void:
	if not _active:
		_respawn_timer -= delta
		if _respawn_timer <= 0:
			_respawn()
		return
	# 飘浮动画（左右摇摆 + 上下浮动）
	if _visual:
		var t = Time.get_ticks_msec() * 0.001
		_visual.position.x = sin(t * 1.5) * 0.15
		_visual.position.y = 0.5 + sin(t * 2.0) * 0.1
		_visual.rotation_degrees.z = sin(t * 1.5) * 5

func _on_body_entered(body: Node) -> void:
	if not _active or not body.is_in_group("player"):
		return
	# 戳破！
	_active = false
	_respawn_timer = 5.0
	visible = false
	monitoring = false
	# 气球碎片彩纸
	Confetti.burst(get_tree().current_scene, global_position, _balloon_color)
	# 随机奖励 1-3 个物品
	var rng = RandomNumberGenerator.new()
	rng.randomize()
	var reward_count = rng.randi_range(1, 3)
	var rewards = ["apple", "flower", "butterfly", "pearl", "starfish", "snowflake"]
	for i in reward_count:
		GameState.collect_item(rewards[rng.randi() % rewards.size()])
	EventBus.toast_message.emit("气球戳破！+%d 物品" % reward_count, "🎈")
	AudioBus.play_pickup()

func _respawn() -> void:
	_active = true
	visible = true
	monitoring = true
	_balloon_color = Color(randf(), randf(), randf())
	# 重建视觉（换色）
	if _visual:
		_visual.queue_free()
	_visual = _build_balloon()
	add_child(_visual)
	# 移到新位置
	var rng = RandomNumberGenerator.new()
	rng.randomize()
	global_position += Vector3(rng.randf_range(-5, 5), 0, rng.randf_range(-5, 5))
