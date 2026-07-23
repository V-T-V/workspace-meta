#============================================================
# JumpPad.gd — 弹跳设施（弹簧垫/蘑菇蹦床）
#============================================================
# 玩家踩上去获得超大跳跃力（超级跳！）
# 儿童最爱：弹跳反馈 + 弹簧压缩动画 + 音效
# 每个区域 1-2 个，位置随机但避开 NPC/收集物
#============================================================
extends Area3D

const BOUNCE_FORCE: float = 18.0    # 超级跳跃力（普通跳 7）
const COOLDOWN: float = 0.5         # 防止连踩
const PAD_RADIUS: float = 1.2

var _cooldown: float = 0.0
var _visual: Node3D = null
var _compress: float = 0.0          # 压缩动画值（0=正常, 1=完全压缩）

func _ready() -> void:
	body_entered.connect(_on_body_entered)
	# 构建弹跳垫视觉（蘑菇造型）
	_visual = _build_visual()
	add_child(_visual)
	# 碰撞区
	var col = CollisionShape3D.new()
	var shape = CylinderShape3D.new()
	shape.radius = PAD_RADIUS
	shape.height = 0.5
	col.shape = shape
	col.position = Vector3(0, 0.25, 0)
	add_child(col)

func _build_visual() -> Node3D:
	var node = Node3D.new()
	# 蘑菇柄（白色短圆柱）
	var stem = CSGCylinder3D.new()
	stem.radius = 0.3
	stem.height = 0.4
	stem.position = Vector3(0, 0.2, 0)
	var smat = StandardMaterial3D.new()
	smat.albedo_color = Color(0.95, 0.92, 0.88)
	stem.material = smat
	node.add_child(stem)
	# 蘑菇帽（红色扁球，可压缩）
	var cap = CSGSphere3D.new()
	cap.radius = 1.0
	cap.scale = Vector3(1, 0.4, 1)
	cap.position = Vector3(0, 0.55, 0)
	cap.name = "MushroomCap"
	var cmat = StandardMaterial3D.new()
	cmat.albedo_color = Color(0.9, 0.2, 0.2)
	cmat.emissive = Color(0.5, 0.1, 0.1)
	cmat.emissive_energy_multiplier = 0.3
	cap.material = cmat
	node.add_child(cap)
	# 白色斑点（装饰）
	var rng = RandomNumberGenerator.new()
	rng.seed = randi()
	for i in 5:
		var dot = CSGSphere3D.new()
		dot.radius = 0.12
		var a = TAU * i / 5.0
		dot.position = Vector3(cos(a) * 0.6, 0.7, sin(a) * 0.6)
		var dmat = StandardMaterial3D.new()
		dmat.albedo_color = Color(0.98, 0.98, 0.95)
		dot.material = dmat
		node.add_child(dot)
	return node

func _process(delta: float) -> void:
	if _cooldown > 0:
		_cooldown -= delta
	# 压缩动画回弹
	if _compress > 0:
		_compress = lerp(_compress, 0.0, delta * 8.0)
		var cap = _visual.get_node_or_null("MushroomCap")
		if cap:
			var squash = 1.0 - _compress * 0.5   # 压缩时变扁
			cap.scale = Vector3(2 - squash, 0.4 * squash, 2 - squash)
			cap.position.y = 0.55 - _compress * 0.15

func _on_body_entered(body: Node) -> void:
	if _cooldown > 0:
		return
	if not body.is_in_group("player"):
		return
	# 超级弹跳！
	var player = body as CharacterBody3D
	player.velocity.y = BOUNCE_FORCE
	_cooldown = COOLDOWN
	_compress = 1.0   # 触发压缩动画
	AudioBus.play_jump()
	# 弹跳彩纸
	var Confetti = load("res://world/Confetti.gd")
	Confetti.burst(get_tree().current_scene, global_position, Color(0.9, 0.3, 0.3))
	EventBus.toast_message.emit("超级弹跳！", "🍄")
