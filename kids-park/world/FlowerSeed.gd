#============================================================
# FlowerSeed.gd — 种花系统（种子→发芽→开花→收获）
#============================================================
# 花园专属花盆/花坛，玩家走近按 E 种下种子
# 4 阶段生长（每阶段 5 秒）：
#   0 种子（土堆）→ 1 嫩芽（小绿尖）→ 2 花苞 → 3 盛开
# 盛开后按 E 收获 → 获得 petal 收集物 + 重置为种子
# 儿童最爱：亲手种植→等待→收获的完整循环
#============================================================
extends Area3D

const Confetti = preload("res://world/Confetti.gd")
const GROW_TIME: float = 5.0   # 每阶段 5 秒（共 15 秒成熟）

enum Stage { EMPTY, SEED, SPROUT, BUD, BLOOM }

var _stage: int = Stage.EMPTY
var _grow_timer: float = 0.0
var _visual: Node3D = null
var _player_near: bool = false
var _hint: Label3D = null
var _flower_color: Color

func _ready() -> void:
	body_entered.connect(_on_body_entered)
	body_exited.connect(_on_body_exited)
	_flower_color = [Color(1,0.4,0.5), Color(1,0.8,0.3), Color(0.9,0.5,1), Color(0.4,0.8,0.5)][randi() % 4]
	_build_pot()
	_update_visual()
	# 碰撞
	var col = CollisionShape3D.new()
	var shape = CylinderShape3D.new()
	shape.radius = 0.6; shape.height = 0.8
	col.shape = shape
	col.position = Vector3(0, 0.4, 0)
	add_child(col)
	# 提示
	_hint = Label3D.new()
	_hint.font_size = 22
	_hint.position = Vector3(0, 1.5, 0)
	_hint.billboard = BaseMaterial3D.BILLBOARD_ENABLED
	_hint.outline_size = 5
	_hint.outline_modulate = Color(0, 0, 0, 0.5)
	_hint.visible = false
	add_child(_hint)
	# 默认已种（方便展示）
	if randf() < 0.5:
		_stage = Stage.SEED
		_grow_timer = GROW_TIME

func _build_pot() -> void:
	_visual = Node3D.new()
	add_child(_visual)
	# 花盆（陶罐=梯形圆柱）
	var pot = CSGCylinder3D.new()
	pot.radius = 0.3; pot.height = 0.3
	pot.position = Vector3(0, 0.15, 0)
	pot.scale = Vector3(1.2, 1, 1.2)
	var pot_mat = ModelFactory.get_material(Color(0.6, 0.4, 0.3), {"shaded": true})
	pot.material = pot_mat
	_visual.add_child(pot)
	# 土壤（深棕圆面）
	var soil = CSGCylinder3D.new()
	soil.radius = 0.28; soil.height = 0.05
	soil.position = Vector3(0, 0.3, 0)
	var soil_mat = ModelFactory.get_material(Color(0.3, 0.2, 0.1))
	soil.material = soil_mat
	soil.name = "Soil"
	_visual.add_child(soil)

func _process(delta: float) -> void:
	if _stage == Stage.EMPTY:
		if _player_near:
			_hint.text = "按 E 种花 🌱"
			_hint.visible = true
		return
	if _stage < Stage.BLOOM:
		_grow_timer -= delta
		if _grow_timer <= 0:
			_stage += 1
			_grow_timer = GROW_TIME
			_update_visual()
			AudioBus.play_note(500.0 + _stage * 100, 0.2, 0.1)
			if _stage == Stage.BLOOM:
				EventBus.toast_message.emit("花开了！", "🌸")
				Confetti.burst(get_tree().current_scene, global_position + Vector3(0, 0.8, 0), _flower_color)
	if _stage == Stage.BLOOM and _player_near:
		_hint.text = "按 E 收花 🌸"
		_hint.visible = true

func _update_visual() -> void:
	# 移除旧植物部分（保留花盆和土壤）
	for c in _visual.get_children():
		if c.name != "Soil" and c.name != "PotPart":
			c.queue_free()
	match _stage:
		Stage.EMPTY:
			pass   # 只有花盆
		Stage.SEED:
			# 土堆上小种子（深色小球）
			var seed_ball = CSGSphere3D.new()
			seed_ball.radius = 0.03; seed_ball.position = Vector3(0, 0.32, 0)
			seed_ball.material = ModelFactory.get_material(Color(0.4, 0.25, 0.1))
			_visual.add_child(seed_ball)
		Stage.SPROUT:
			# 嫩芽（绿色细尖）
			var sprout = CSGCylinder3D.new()
			sprout.radius = 0.02; sprout.height = 0.15
			sprout.position = Vector3(0, 0.4, 0)
			sprout.material = ModelFactory.get_material(Color(0.3, 0.6, 0.2))
			_visual.add_child(sprout)
			# 两片小叶
			for sx in [-1, 1]:
				var leaf = CSGBox3D.new()
				leaf.size = Vector3(0.06, 0.02, 0.03)
				leaf.position = Vector3(sx * 0.05, 0.45, 0)
				leaf.rotation_degrees = Vector3(0, 0, sx * 30)
				leaf.material = ModelFactory.get_material(Color(0.3, 0.6, 0.2))
				_visual.add_child(leaf)
		Stage.BUD:
			# 花苞（茎+绿色椭圆苞）
			var stem = CSGCylinder3D.new()
			stem.radius = 0.025; stem.height = 0.4
			stem.position = Vector3(0, 0.5, 0)
			stem.material = ModelFactory.get_material(Color(0.25, 0.5, 0.15))
			_visual.add_child(stem)
			var bud = CSGSphere3D.new()
			bud.radius = 0.08; bud.position = Vector3(0, 0.72, 0)
			bud.scale = Vector3(0.8, 1.2, 0.8)
			bud.material = ModelFactory.get_material(_flower_color.darkened(0.3), {"emissive": _flower_color.darkened(0.3), "emissive_energy": 0.1})
			_visual.add_child(bud)
		Stage.BLOOM:
			# 盛开（茎+花瓣展开）
			var stem = CSGCylinder3D.new()
			stem.radius = 0.025; stem.height = 0.4
			stem.position = Vector3(0, 0.5, 0)
			stem.material = ModelFactory.get_material(Color(0.25, 0.5, 0.15))
			_visual.add_child(stem)
			# 5 片花瓣
			for i in 5:
				var a = TAU * i / 5.0
				var petal = CSGSphere3D.new()
				petal.radius = 0.08
				petal.position = Vector3(cos(a) * 0.1, 0.78, sin(a) * 0.1)
				petal.scale = Vector3(1, 0.4, 1)
				petal.material = ModelFactory.get_material(_flower_color, {"emissive": _flower_color, "emissive_energy": 0.3})
				_visual.add_child(petal)
			# 花心
			var center = CSGSphere3D.new()
			center.radius = 0.05; center.position = Vector3(0, 0.78, 0)
			center.material = ModelFactory.get_material(Color(1, 0.85, 0.2), {"emissive": Color(1, 0.8, 0.2), "emissive_energy": 0.3})
			_visual.add_child(center)

func _on_body_entered(body: Node) -> void:
	if body.is_in_group("player"):
		_player_near = true

func _on_body_exited(body: Node) -> void:
	if body.is_in_group("player"):
		_player_near = false
		_hint.visible = false

func _unhandled_input(event: InputEvent) -> void:
	if event is InputEventKey and event.pressed and event.keycode == KEY_E:
		if _player_near:
			if _stage == Stage.EMPTY:
				_plant_seed()
			elif _stage == Stage.BLOOM:
				_harvest()

func _plant_seed() -> void:
	_stage = Stage.SEED
	_grow_timer = GROW_TIME
	_update_visual()
	_hint.visible = false
	EventBus.toast_message.emit("种下种子！等它长大～", "🌱")
	AudioBus.play_note(400.0, 0.15, 0.1)

func _harvest() -> void:
	GameState.collect_item("petal", 2)
	Confetti.burst(get_tree().current_scene, global_position + Vector3(0, 0.8, 0), _flower_color)
	EventBus.toast_message.emit("收获花瓣 ×2！", "🌸")
	AudioBus.play_mission_complete()
	# 重置为空盆
	_stage = Stage.EMPTY
	_update_visual()
