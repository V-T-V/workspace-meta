#============================================================
# WishingWell.gd — 许愿井（投物品获得奖励）
#============================================================
# 石砌井口 + 井水反光 + 木质顶架
# 玩家靠近按 E 键投入 1 个物品"许愿"
# 随机奖励：贴纸(10%) / 3 物品(40%) / 1 物品(50%)
# 每次许愿后井水闪光 + 上扬音效
# 每区域 1 个
#============================================================
extends Area3D

const Confetti = preload("res://world/Confetti.gd")

var _water: MeshInstance3D = null
var _player_near: bool = false
var _hint: Label3D = null
var _wish_cooldown: float = 0.0

func _ready() -> void:
	body_entered.connect(_on_body_entered)
	body_exited.connect(_on_body_exited)
	_build_well()
	# 碰撞
	var col = CollisionShape3D.new()
	var shape = CylinderShape3D.new()
	shape.radius = 1.2; shape.height = 1.5
	col.shape = shape
	col.position = Vector3(0, 0.75, 0)
	add_child(col)
	# 提示
	_hint = Label3D.new()
	_hint.text = "许愿井 🪙 按 E"
	_hint.font_size = 24
	_hint.position = Vector3(0, 2.5, 0)
	_hint.billboard = BaseMaterial3D.BILLBOARD_ENABLED
	_hint.outline_size = 6
	_hint.outline_modulate = Color(0, 0, 0, 0.5)
	_hint.visible = false
	add_child(_hint)

func _build_well() -> void:
	var stone_mat = ModelFactory.get_material(Color(0.55, 0.52, 0.48), {"shaded": true})
	var wood_mat = ModelFactory.get_material(Color(0.45, 0.3, 0.15), {"shaded": true})
	var water_mat = ModelFactory.get_material(Color(0.15, 0.4, 0.7), {"metallic": 0.8, "roughness": 0.1, "emissive": Color(0.1, 0.3, 0.5), "emissive_energy": 0.3})
	# 井壁（圆环 = 中空圆柱用两个圆柱模拟，简化为实心矮圆柱）
	var wall = CSGCylinder3D.new()
	wall.radius = 0.8; wall.height = 0.8
	wall.position = Vector3(0, 0.4, 0)
	wall.material = stone_mat
	add_child(wall)
	# 井水（内部蓝面）
	_water = MeshInstance3D.new()
	var water_mesh = CylinderMesh.new()
	water_mesh.top_radius = 0.6; water_mesh.bottom_radius = 0.6
	water_mesh.height = 0.05
	_water.mesh = water_mesh
	_water.position = Vector3(0, 0.6, 0)
	_water.material_override = water_mat
	add_child(_water)
	# 石头装饰圈（井口边缘小球）
	for i in 8:
		var a = TAU * i / 8.0
		var dot = CSGSphere3D.new()
		dot.radius = 0.12
		dot.position = Vector3(cos(a) * 0.85, 0.8, sin(a) * 0.85)
		dot.material = stone_mat
		add_child(dot)
	# 木质顶架（两根柱 + 横梁）
	for sx in [-1, 1]:
		var post = CSGCylinder3D.new()
		post.radius = 0.06; post.height = 2.0
		post.position = Vector3(sx * 0.9, 1.0, 0)
		post.material = wood_mat
		add_child(post)
	var beam = CSGCylinder3D.new()
	beam.radius = 0.06; beam.height = 2.2
	beam.position = Vector3(0, 2.0, 0)
	beam.rotation_degrees = Vector3(0, 0, 90)
	beam.material = wood_mat
	add_child(beam)
	# 小屋顶（横梁上方三角）
	var roof = CSGBox3D.new()
	roof.size = Vector3(2.2, 0.1, 0.8)
	roof.position = Vector3(0, 2.2, 0)
	roof.rotation_degrees = Vector3(15, 0, 0)
	roof.material = wood_mat
	add_child(roof)

func _process(delta: float) -> void:
	if _wish_cooldown > 0:
		_wish_cooldown -= delta
	# 井水微光波动
	if _water:
		var t = Time.get_ticks_msec() * 0.002
		var mat = _water.material_override as StandardMaterial3D
		if mat:
			mat.emissive_energy_multiplier = 0.3 + sin(t) * 0.15

func _on_body_entered(body: Node) -> void:
	if body.is_in_group("player"):
		_player_near = true
		_hint.visible = true

func _on_body_exited(body: Node) -> void:
	if body.is_in_group("player"):
		_player_near = false
		_hint.visible = false

func _unhandled_input(event: InputEvent) -> void:
	if event is InputEventKey and event.pressed and event.keycode == KEY_E:
		if _player_near and _wish_cooldown <= 0:
			_make_wish()

func _make_wish() -> void:
	# 需要至少 1 个物品
	if GameState.total_collected < 1:
		EventBus.toast_message.emit("还没有物品可以许愿哦～", "🪙")
		return
	_wish_cooldown = 2.0
	var rng = RandomNumberGenerator.new()
	rng.randomize()
	var roll = rng.randf()
	# 井水闪光
	var mat = _water.material_override as StandardMaterial3D
	if mat:
		var tw = create_tween()
		tw.tween_property(mat, "emissive_energy_multiplier", 2.0, 0.1)
		tw.tween_property(mat, "emissive_energy_multiplier", 0.3, 0.5)
	Confetti.burst(get_tree().current_scene, global_position + Vector3(0, 1, 0), Color(0.4, 0.7, 1.0))
	AudioBus.play_sticker()
	if roll < 0.1:
		# 贴纸大奖
		GameState.earn_sticker("🪙许愿井好运")
		EventBus.toast_message.emit("许愿成功！获得贴纸！", "🌟")
	elif roll < 0.5:
		# 3 物品
		var rewards = ["apple", "flower", "pearl", "butterfly"]
		for i in 3:
			GameState.collect_item(rewards[rng.randi() % rewards.size()])
		EventBus.toast_message.emit("许愿成真！+3 物品", "✨")
	else:
		# 1 物品
		GameState.collect_item("pearl")
		EventBus.toast_message.emit("井水闪光！+1 珍珠", "🪙")
