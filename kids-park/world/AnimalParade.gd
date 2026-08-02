#============================================================
# AnimalParade.gd — 动物朋友游行（定期 NPC 游行队伍）
#============================================================
# 每 3 分钟触发一次：4 只彩色小动物排成一列
# 沿区域外围环形路径行进，头顶飘彩旗
# 玩家加入队伍（靠近）获得"游行参与者"贴纸
# 游行持续 30 秒后动物散开
#============================================================
extends Node3D

const PARADE_INTERVAL: float = 180.0   # 3 分钟一次
const PARADE_DURATION: float = 30.0
const PARADE_SPEED: float = 2.0

var _timer: float = 60.0   # 首次 60 秒后开始
var _active: bool = false
var _active_timer: float = 0.0
var _animals: Array = []   # [{node, offset}]
var _player_joined: bool = false

func _ready() -> void:
	pass   # 动物在触发时才创建

func _process(delta: float) -> void:
	if _active:
		_active_timer -= delta
		_update_parade(delta)
		if _active_timer <= 0:
			_end_parade()
	else:
		_timer -= delta
		if _timer <= 0:
			_start_parade()

func _start_parade() -> void:
	_active = true
	_active_timer = PARADE_DURATION
	_player_joined = false
	_animals.clear()
	# 4 只彩色小动物（球身+头+耳朵）
	var colors = [Color(0.9,0.4,0.5), Color(0.4,0.7,0.9), Color(0.9,0.7,0.3), Color(0.5,0.8,0.4)]
	var center = ParkGen.ZONE_CENTERS["grassland"]
	for i in 4:
		var animal = _make_parade_animal(colors[i])
		animal.global_position = center + Vector3(-i * 1.2, 0, 0)
		add_child(animal)
		_animals.append({"node": animal, "offset": -i * 1.2})
	EventBus.toast_message.emit("动物游行开始！快来看！", "🎪")
	AudioBus.play_zone_unlock()

func _make_parade_animal(color: Color) -> Node3D:
	var node = Node3D.new()
	var mat = ModelFactory.get_material(color, {"emissive": color, "emissive_energy": 0.2, "shaded": true})
	# 身体
	var body = CSGSphere3D.new()
	body.radius = 0.3; body.position = Vector3(0, 0.3, 0)
	body.material = mat
	node.add_child(body)
	# 头
	var head = CSGSphere3D.new()
	head.radius = 0.2; head.position = Vector3(0, 0.6, 0.2)
	head.material = mat
	node.add_child(head)
	# 耳朵
	for sx in [-1, 1]:
		var ear = CSGSphere3D.new()
		ear.radius = 0.06; ear.scale = Vector3(0.5, 1.5, 0.5)
		ear.position = Vector3(sx * 0.08, 0.78, 0.18)
		ear.material = mat
		node.add_child(ear)
	# 眼睛
	for sx in [-1, 1]:
		var eye = CSGSphere3D.new()
		eye.radius = 0.025; eye.position = Vector3(sx * 0.06, 0.62, 0.36)
		eye.material = ModelFactory.get_material(Color(0.05, 0.05, 0.1))
		node.add_child(eye)
	# 头顶小彩旗
	var flag_pole = CSGCylinder3D.new()
	flag_pole.radius = 0.01; flag_pole.height = 0.4
	flag_pole.position = Vector3(0, 0.95, 0)
	flag_pole.material = ModelFactory.get_material(Color(0.8, 0.8, 0.8))
	node.add_child(flag_pole)
	var flag = CSGBox3D.new()
	flag.size = Vector3(0.15, 0.1, 0.01)
	flag.position = Vector3(0.08, 1.1, 0)
	flag.material = ModelFactory.get_material(color.lightened(0.3), {"emissive": color, "emissive_energy": 0.3})
	node.add_child(flag)
	return node

func _update_parade(delta: float) -> void:
	# 沿草地外围环形行进
	var center = ParkGen.ZONE_CENTERS["grassland"]
	var radius = ParkGen.ZONE_SIZE * 0.4
	# 计算行进角度
	var base_angle = Time.get_ticks_msec() * 0.0005
	for i in _animals.size():
		var a = base_angle + i * 0.15   # 队列间隔
		var pos = center + Vector3(cos(a) * radius, 0, sin(a) * radius)
		_animals[i].node.global_position = pos
		_animals[i].node.rotation_degrees.y = rad_to_deg(a + PI / 2)
		# 走动弹跳
		var bounce = sin(Time.get_ticks_msec() * 0.01 + i) * 0.05
		_animals[i].node.position.y = bounce
	# 检查玩家是否加入
	var player = get_tree().get_first_node_in_group("player")
	if player and not _player_joined:
		# 检查玩家是否靠近任一游行动物
		for an in _animals:
			if an.node.global_position.distance_to(player.global_position) < 3.0:
				_player_joined = true
				GameState.earn_sticker("🎪游行参与者")
				EventBus.toast_message.emit("加入游行队伍！", "🎪")
				AudioBus.play_sticker()
				break

func _end_parade() -> void:
	_active = false
	_timer = PARADE_INTERVAL
	# 动物散开（渐隐）
	for an in _animals:
		var node = an.node
		if node:
			var tw = create_tween()
			tw.tween_property(node, "scale", Vector3.ZERO, 0.5)
			tw.tween_callback(func(): node.queue_free())
	_animals.clear()
