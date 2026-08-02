#============================================================
# BouncyCastle.gd — 充气城堡（多色弹跳墙+随机弹射）
#============================================================
# 城堡造型：4 面彩色墙 + 城垛顶 + 内部弹跳地板
# 玩家进入 → 持续小弹跳（每 0.5s 自动起跳）+ 墙壁反弹
# 撞墙 → 随机方向弹飞 + 彩纸
# 花园专属大型设施，纯欢乐无目标
#============================================================
extends Area3D

const Confetti = preload("res://world/Confetti.gd")
const BOUNCE_INTERVAL: float = 0.5
const WALL_BOUNCE_FORCE: float = 8.0
const AUTO_BOUNCE_FORCE: float = 6.0

var _player_inside: CharacterBody3D = null
var _bounce_timer: float = 0.0
var _walls: Array = []   # 4 面墙的 Area3D

func _ready() -> void:
	body_entered.connect(_on_body_entered)
	body_exited.connect(_on_body_exited)
	_build_castle()
	# 主碰撞区（城堡内部地板）
	var col = CollisionShape3D.new()
	var shape = BoxShape3D.new()
	shape.size = Vector3(4.0, 0.3, 4.0)
	col.shape = shape
	col.position = Vector3(0, 0.15, 0)
	add_child(col)
	# 4 面弹力墙（独立 Area3D 检测碰撞反弹）
	_build_walls()

func _build_castle() -> void:
	var colors = [
		Color(0.9, 0.3, 0.4),   # 红
		Color(0.4, 0.6, 0.9),   # 蓝
		Color(0.9, 0.7, 0.3),   # 黄
		Color(0.5, 0.8, 0.4),   # 绿
	]
	var wall_mat_template = {"emissive": Color.WHITE, "emissive_energy": 0.2, "metallic": 0.1, "roughness": 0.4, "shaded": true}
	# 4 面墙
	for i in 4:
		var angle = i * PI / 2.0
		var wall = CSGBox3D.new()
		wall.size = Vector3(4.0, 1.5, 0.3)
		wall.position = Vector3(cos(angle) * 2.0, 0.75, sin(angle) * 2.0)
		wall.rotation_degrees.y = rad_to_deg(angle)
		var mat = ModelFactory.get_material(colors[i], wall_mat_template.duplicate())
		wall.material = mat
		add_child(wall)
		# 城垛（顶部锯齿装饰）
		for j in 3:
			var merlon = CSGBox3D.new()
			merlon.size = Vector3(0.6, 0.4, 0.35)
			var offset_x = (j - 1) * 1.2
			merlon.position = wall.position + Vector3(cos(angle), 0, sin(angle)) * 0.0 + Vector3(cos(angle + PI/2), 1.0, sin(angle + PI/2)) * offset_x
			merlon.position.y = 1.7
			merlon.rotation_degrees.y = rad_to_deg(angle)
			merlon.material = mat
			add_child(merlon)
	# 内部弹跳地板（彩色格子）
	for x in 2:
		for z in 2:
			var tile = CSGBox3D.new()
			tile.size = Vector3(1.9, 0.1, 1.9)
			tile.position = Vector3((x - 0.5) * 2.0, 0.05, (z - 0.5) * 2.0)
			var idx = (x + z) % 2
			tile.material = ModelFactory.get_material(colors[idx * 2], wall_mat_template)
			add_child(tile)

func _build_walls() -> void:
	# 4 面弹力墙检测区
	for i in 4:
		var angle = i * PI / 2.0
		var wall_area = Area3D.new()
		var col = CollisionShape3D.new()
		var shape = BoxShape3D.new()
		shape.size = Vector3(4.0, 2.0, 0.5)
		col.shape = shape
		col.position = Vector3(cos(angle) * 2.0, 1.0, sin(angle) * 2.0)
		wall_area.add_child(col)
		wall_area.collision_layer = 0
		wall_area.collision_mask = 1
		# 记录墙的法线方向（用于反弹）
		var normal = Vector3(-cos(angle), 0, -sin(angle))
		wall_area.set_meta("normal", normal)
		wall_area.body_entered.connect(func(body): _on_wall_hit(body, normal))
		add_child(wall_area)
		_walls.append(wall_area)

func _process(delta: float) -> void:
	if _player_inside and is_instance_valid(_player_inside):
		_bounce_timer -= delta
		if _bounce_timer <= 0 and _player_inside.is_on_floor():
			_bounce_timer = BOUNCE_INTERVAL
			_player_inside.velocity.y = AUTO_BOUNCE_FORCE
			# 随机水平微推（增加趣味）
			_player_inside.velocity.x += randf_range(-2, 2)
			_player_inside.velocity.z += randf_range(-2, 2)

func _on_wall_hit(body: Node, normal: Vector3) -> void:
	if not body.is_in_group("player"):
		return
	var player = body as CharacterBody3D
	if player == null:
		return
	# 沿墙法线反弹 + 额外向上弹
	player.velocity = normal * WALL_BOUNCE_FORCE
	player.velocity.y = AUTO_BOUNCE_FORCE * 1.5
	AudioBus.play_pickup()
	Confetti.burst(get_tree().current_scene, player.global_position, Color(0.9, 0.5, 0.7))

func _on_body_entered(body: Node) -> void:
	if body.is_in_group("player"):
		_player_inside = body as CharacterBody3D
		EventBus.toast_message.emit("充气城堡！蹦蹦蹦！", "🏰")
		AudioBus.play_zone_unlock()

func _on_body_exited(body: Node) -> void:
	if body.is_in_group("player"):
		_player_inside = null
