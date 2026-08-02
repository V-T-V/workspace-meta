#============================================================
# Slide.gd — 滑梯设施（攀爬+滑下+加速尾迹）
#============================================================
# 结构：阶梯（上）→ 平台 → 斜面（下滑）→ 落地缓冲区
# 玩家走上去自动攀爬阶梯 → 到平台后滑下斜面 → 加速冲出
# 滑行时持续生成彩色粒子尾迹 + 风声音效
# 每个区域 1 个，作为大型互动玩具
#============================================================
extends Area3D

const Confetti = preload("res://world/Confetti.gd")
const SLIDE_BOOST: float = 10.0    # 滑下时的水平加速
const PLATFORM_HEIGHT: float = 3.0  # 平台高度

var _player_on_slide: bool = false
var _visual: Node3D = null
var _slide_zone: Area3D = null
var _slide_color: Color

func _ready() -> void:
	_slide_color = Color(0.4, 0.7, 0.9)
	_visual = _build_slide()
	add_child(_visual)
	# 滑行触发区（斜面区域）
	_slide_zone = Area3D.new()
	var col = CollisionShape3D.new()
	var shape = BoxShape3D.new()
	shape.size = Vector3(2.0, 1.0, 4.0)
	col.shape = shape
	col.position = Vector3(0, PLATFORM_HEIGHT * 0.4, 3.0)
	_slide_zone.add_child(col)
	_slide_zone.body_entered.connect(_on_slide_entered)
	add_child(_slide_zone)
	# 攀爬触发区（整体碰撞）
	var climb_col = CollisionShape3D.new()
	var climb_shape = BoxShape3D.new()
	climb_shape.size = Vector3(3.0, PLATFORM_HEIGHT + 1, 2.0)
	climb_col.shape = climb_shape
	climb_col.position = Vector3(0, PLATFORM_HEIGHT * 0.5, -3.0)
	add_child(climb_col)

func _build_slide() -> Node3D:
	var node = Node3D.new()
	var step_mat = ModelFactory.get_material(Color(0.6, 0.5, 0.4), {"shaded": true})
	var slide_mat = ModelFactory.get_material(_slide_color, {"emissive": _slide_color, "emissive_energy": 0.2, "metallic": 0.3, "roughness": 0.1, "shaded": true})
	var rail_mat = ModelFactory.get_material(Color(0.9, 0.9, 0.9), {"metallic": 0.5, "roughness": 0.3})
	# 阶梯（4 级，背面）
	for i in 4:
		var step = CSGBox3D.new()
		step.size = Vector3(2.5, 0.15, 0.5)
		step.position = Vector3(0, i * (PLATFORM_HEIGHT / 4.0), -3.0 - i * 0.5)
		step.material = step_mat
		node.add_child(step)
	# 顶部平台
	var platform = CSGBox3D.new()
	platform.size = Vector3(3.0, 0.2, 2.5)
	platform.position = Vector3(0, PLATFORM_HEIGHT, -1.5)
	platform.material = step_mat
	node.add_child(platform)
	# 平台护栏（左右）
	for sx in [-1, 1]:
		var rail = CSGBox3D.new()
		rail.size = Vector3(0.1, 0.8, 2.5)
		rail.position = Vector3(sx * 1.45, PLATFORM_HEIGHT + 0.4, -1.5)
		rail.material = rail_mat
		node.add_child(rail)
	# 滑道斜面（从平台向下倾斜）
	var slide = CSGBox3D.new()
	slide.size = Vector3(2.0, 0.15, 5.0)
	slide.position = Vector3(0, PLATFORM_HEIGHT * 0.5, 3.0)
	slide.rotation_degrees = Vector3(-30, 0, 0)   # 向下倾斜 30°
	slide.material = slide_mat
	slide.name = "SlideSurface"
	node.add_child(slide)
	# 滑道两侧护栏
	for sx in [-1, 1]:
		var srail = CSGBox3D.new()
		srail.size = Vector3(0.08, 0.5, 5.0)
		srail.position = Vector3(sx * 1.0, PLATFORM_HEIGHT * 0.5 + 0.3, 3.0)
		srail.rotation_degrees = Vector3(-30, 0, 0)
		srail.material = rail_mat
		node.add_child(srail)
	return node

func _on_slide_entered(body: Node) -> void:
	if not body.is_in_group("player"):
		return
	if _player_on_slide:
		return
	_player_on_slide = true
	var player = body as CharacterBody3D
	if player:
		# 沿滑道方向加速（Z 正方向 = 滑下方向）
		player.velocity = Vector3(0, -2, SLIDE_BOOST)
	# 滑行音效 + 提示
	AudioBus.play_zone_unlock()
	EventBus.toast_message.emit("滑梯冲刺！", "🏂")
	# 滑行尾迹（0.5 秒内持续发射粒子）
	_spawn_slide_trail(player)
	# 冷却 1 秒后可再次触发
	get_tree().create_timer(1.5).timeout.connect(func():
		_player_on_slide = false
	)

func _spawn_slide_trail(player: Node3D) -> void:
	# 简易尾迹：在玩家身后持续生成彩色小粒子
	for i in 8:
		get_tree().create_timer(i * 0.08).timeout.connect(func():
			if is_instance_valid(player):
				Confetti.burst(get_tree().current_scene, player.global_position, Color(0.5, 0.8, 1.0))
		)
