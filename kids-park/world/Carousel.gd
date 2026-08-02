#============================================================
# Carousel.gd — 旋转木马（标志性游乐设施）
#============================================================
# 红白条纹顶棚 + 中央柱 + 6 匹彩色木马环绕旋转
# 木马上下浮动（模拟骑乘颠簸）+ 整体旋转
# 玩家靠近时旋转加速 + 播放欢快旋律
# 每个区域 1 个，作为地标性设施
#============================================================
extends Area3D

const HORSE_COUNT: int = 6
const BASE_ROTATE_SPEED: float = 0.5
const BOOST_ROTATE_SPEED: float = 2.0

var _platform: Node3D = null
var _horses: Array = []   # [{node, phase}]
var _current_speed: float = BASE_ROTATE_SPEED
var _player_near: bool = false

func _ready() -> void:
	body_entered.connect(_on_body_entered)
	body_exited.connect(_on_body_exited)
	_build_carousel()
	# 碰撞区
	var col = CollisionShape3D.new()
	var shape = CylinderShape3D.new()
	shape.radius = 3.0
	shape.height = 0.5
	col.shape = shape
	col.position = Vector3(0, 0.25, 0)
	add_child(col)

func _build_carousel() -> void:
	var stripe_mat_a = ModelFactory.get_material(Color(0.9, 0.2, 0.2), {"emissive": Color(0.4, 0.1, 0.1), "emissive_energy": 0.2, "shaded": true})
	var stripe_mat_b = ModelFactory.get_material(Color(0.95, 0.95, 0.92), {"shaded": true})
	var pole_mat = ModelFactory.get_material(Color(0.85, 0.8, 0.2), {"metallic": 0.6, "roughness": 0.3})
	var horse_colors = [
		Color(0.9, 0.4, 0.5), Color(0.4, 0.6, 0.9), Color(0.9, 0.7, 0.3),
		Color(0.5, 0.8, 0.4), Color(0.8, 0.5, 0.9), Color(0.9, 0.6, 0.3),
	]
	# 底盘（大圆盘）
	var base = CSGCylinder3D.new()
	base.radius = 2.8; base.height = 0.3
	base.position = Vector3(0, 0.15, 0)
	base.material = stripe_mat_b
	add_child(base)
	# 旋转平台（所有木马挂在这个下面）
	_platform = Node3D.new()
	_platform.name = "Platform"
	_platform.position = Vector3(0, 0.3, 0)
	add_child(_platform)
	# 中央柱
	var center_pole = CSGCylinder3D.new()
	center_pole.radius = 0.15; center_pole.height = 3.0
	center_pole.position = Vector3(0, 1.5, 0)
	center_pole.material = pole_mat
	_platform.add_child(center_pole)
	# 顶棚（红白条纹圆锥=压扁圆柱 + 旋转条纹模拟）
	var roof = CSGCylinder3D.new()
	roof.radius = 3.2; roof.height = 0.6
	roof.position = Vector3(0, 3.2, 0)
	roof.scale = Vector3(1, 0.3, 1)
	roof.material = stripe_mat_a
	_platform.add_child(roof)
	# 顶棚尖端
	var spire = CSGCylinder3D.new()
	spire.radius = 0.05; spire.height = 0.5
	spire.position = Vector3(0, 3.7, 0)
	spire.material = pole_mat
	_platform.add_child(spire)
	# 6 匹木马（围成圆形）
	for i in HORSE_COUNT:
		var a = TAU * i / HORSE_COUNT
		var horse_holder = Node3D.new()
		horse_holder.position = Vector3(cos(a) * 2.0, 0, sin(a) * 2.0)
		horse_holder.name = "Horse%d" % i
		_platform.add_child(horse_holder)
		# 杆（金色素柱）
		var pole = CSGCylinder3D.new()
		pole.radius = 0.03; pole.height = 1.8
		pole.position = Vector3(0, 0.9, 0)
		pole.material = pole_mat
		horse_holder.add_child(pole)
		# 木马身体（简化：球+柱+头）
		var hmat = ModelFactory.get_material(horse_colors[i], {"emissive": horse_colors[i], "emissive_energy": 0.15, "shaded": true})
		var body = CSGBox3D.new()
		body.size = Vector3(0.3, 0.4, 0.7)
		body.position = Vector3(0, 0.8, 0)
		body.material = hmat
		body.name = "Body"
		horse_holder.add_child(body)
		# 马头
		var head = CSGSphere3D.new()
		head.radius = 0.15; head.position = Vector3(0, 1.05, 0.35)
		head.material = hmat
		horse_holder.add_child(head)
		# 马耳
		for sx in [-1, 1]:
			var ear = CSGCylinder3D.new()
			ear.radius = 0.03; ear.height = 0.1
			ear.position = Vector3(sx * 0.06, 1.2, 0.33)
			ear.scale = Vector3(0.5, 1, 0.5)
			ear.material = hmat
			horse_holder.add_child(ear)
		# 马尾
		var tail = CSGCylinder3D.new()
		tail.radius = 0.04; tail.height = 0.25
		tail.position = Vector3(0, 0.8, -0.45)
		tail.rotation_degrees = Vector3(30, 0, 0)
		tail.material = hmat
		horse_holder.add_child(tail)
		_horses.append({"node": horse_holder, "phase": i * (TAU / HORSE_COUNT)})

func _process(delta: float) -> void:
	# 平滑切换旋转速度
	var target_speed = BOOST_ROTATE_SPEED if _player_near else BASE_ROTATE_SPEED
	_current_speed = lerp(_current_speed, target_speed, delta * 3.0)
	# 平台旋转
	if _platform:
		_platform.rotation.y += delta * _current_speed
	# 木马上下浮动
	var t = Time.get_ticks_msec() * 0.003
	for h in _horses:
		var body = h.node.get_node_or_null("Body")
		if body:
			body.position.y = 0.8 + sin(t + h.phase) * 0.15

func _on_body_entered(body: Node) -> void:
	if body.is_in_group("player"):
		_player_near = true
		EventBus.toast_message.emit("旋转木马！", "🎠")
		AudioBus.play_mission_complete()

func _on_body_exited(body: Node) -> void:
	if body.is_in_group("player"):
		_player_near = false
