#============================================================
# PirateShip.gd — 海盗船（大型摆动游乐设施）
#============================================================
# 弧形船体挂在支架上，大幅钟摆式摆动
# 摆动幅度随时间递增（越荡越高），周期性复位
# 玩家靠近时摆动加速 + 欢呼音效
# 冰雪区域的地标设施
#============================================================
extends Area3D

const BASE_AMPLITUDE: float = 20.0
const MAX_AMPLITUDE: float = 55.0
const CYCLE_TIME: float = 12.0   # 12 秒一个完整渐强周期

var _ship: Node3D = null
var _swing_phase: float = 0.0
var _cycle_timer: float = 0.0
var _player_near: bool = false
var _current_amplitude: float = BASE_AMPLITUDE

func _ready() -> void:
	body_entered.connect(_on_body_entered)
	body_exited.connect(_on_body_exited)
	_build_ship()
	# 碰撞区（船体范围）
	var col = CollisionShape3D.new()
	var shape = BoxShape3D.new()
	shape.size = Vector3(2.0, 1.0, 4.0)
	col.shape = shape
	col.position = Vector3(0, 1.5, 0)
	add_child(col)

func _build_ship() -> void:
	var wood_mat = ModelFactory.get_material(Color(0.45, 0.3, 0.15), {"shaded": true})
	var dark_wood = ModelFactory.get_material(Color(0.3, 0.18, 0.1), {"shaded": true})
	var sail_mat = ModelFactory.get_material(Color(0.95, 0.92, 0.85), {"shaded": true})
	var flag_mat = ModelFactory.get_material(Color(0.15, 0.15, 0.15), {"emissive": Color(0.05, 0.05, 0.05), "emissive_energy": 0.2})
	# A 字支架（固定）
	for sx in [-1, 1]:
		var post = CSGCylinder3D.new()
		post.radius = 0.1; post.height = 3.5
		post.position = Vector3(sx * 1.5, 1.75, 0)
		post.rotation_degrees = Vector3(0, 0, sx * 12)
		post.material = dark_wood
		add_child(post)
	# 横梁
	var beam = CSGCylinder3D.new()
	beam.radius = 0.1; beam.height = 3.5
	beam.position = Vector3(0, 3.3, 0)
	beam.rotation_degrees = Vector3(0, 0, 90)
	beam.material = dark_wood
	add_child(beam)
	# 船体（会摆动的节点）
	_ship = Node3D.new()
	_ship.name = "ShipBody"
	_ship.position = Vector3(0, 2.8, 0)
	add_child(_ship)
	# 弧形船底（压扁圆柱模拟）
	var hull = CSGCylinder3D.new()
	hull.radius = 1.0; hull.height = 3.5
	hull.position = Vector3(0, -0.3, 0)
	hull.rotation_degrees = Vector3(90, 0, 0)
	hull.scale = Vector3(1, 1, 0.5)
	hull.material = wood_mat
	_ship.add_child(hull)
	# 船舷围栏（两侧矮柱）
	for sz in [-1, 1]:
		for i in 4:
			var post = CSGCylinder3D.new()
			post.radius = 0.03; post.height = 0.4
			post.position = Vector3(0, 0.1, sz * 0.8)
			post.position.x = (i - 1.5) * 0.8
			post.material = wood_mat
			_ship.add_child(post)
	# 桅杆
	var mast = CSGCylinder3D.new()
	mast.radius = 0.04; mast.height = 2.5
	mast.position = Vector3(0, 1.2, 0)
	mast.material = dark_wood
	_ship.add_child(mast)
	# 船帆（白色方块）
	var sail = CSGBox3D.new()
	sail.size = Vector3(0.02, 1.2, 1.0)
	sail.position = Vector3(0, 1.8, 0)
	sail.material = sail_mat
	_ship.add_child(sail)
	# 海盗旗（黑色小方块）
	var flag = CSGBox3D.new()
	flag.size = Vector3(0.01, 0.3, 0.4)
	flag.position = Vector3(0, 2.6, 0)
	flag.material = flag_mat
	_ship.add_child(flag)

func _process(delta: float) -> void:
	# 周期计时（渐强→复位循环）
	_cycle_timer += delta
	if _cycle_timer >= CYCLE_TIME:
		_cycle_timer = 0.0
	# 振幅随周期进度变化（前 70% 渐强，后 30% 减弱回基线）
	var cycle_progress = _cycle_timer / CYCLE_TIME
	if cycle_progress < 0.7:
		_current_amplitude = lerp(BASE_AMPLITUDE, MAX_AMPLITUDE, cycle_progress / 0.7)
	else:
		_current_amplitude = lerp(MAX_AMPLITUDE, BASE_AMPLITUDE, (cycle_progress - 0.7) / 0.3)
	# 玩家靠近时摆动更快
	var speed = 1.5 if _player_near else 1.0
	_swing_phase += delta * speed
	# 应用摆动（绕 X 轴钟摆）
	if _ship:
		_ship.rotation_degrees.x = sin(_swing_phase) * _current_amplitude

func _on_body_entered(body: Node) -> void:
	if body.is_in_group("player"):
		_player_near = true
		EventBus.toast_message.emit("海盗船！抓紧啦！", "🏴‍☠️")
		AudioBus.play_zone_unlock()

func _on_body_exited(body: Node) -> void:
	if body.is_in_group("player"):
		_player_near = false
