#============================================================
# FishingMinigame.gd — 钓鱼迷你游戏（沙滩专属）
#============================================================
# 沙滩区域的钓鱼点：玩家靠近按 E 开始
# 玩法：等待鱼上钩（1-5 秒随机）→ 提示"！" → 1.5 秒内按 E 收杆
# 成功：获得鱼类收集物 + 彩纸
# 失败/超时：鱼跑了，可重试
# 每次钓鱼有冷却 3 秒
#============================================================
extends Area3D

const Confetti = preload("res://world/Confetti.gd")
const HOOK_WAIT_MIN: float = 1.5
const HOOK_WAIT_MAX: float = 4.0
const REACTION_TIME: float = 1.5
const COOLDOWN: float = 3.0

enum State { IDLE, WAITING, HOOKED, COOLDOWN }

var _state: int = State.IDLE
var _timer: float = 0.0
var _player_near: bool = false
var _hint: Label3D = null
var _rod_tip: Node3D = null

func _ready() -> void:
	body_entered.connect(_on_body_entered)
	body_exited.connect(_on_body_exited)
	_build_dock()
	# 碰撞
	var col = CollisionShape3D.new()
	var shape = BoxShape3D.new()
	shape.size = Vector3(3.0, 1.0, 3.0)
	col.shape = shape
	col.position = Vector3(0, 0.5, 0)
	add_child(col)
	# 提示
	_hint = Label3D.new()
	_hint.text = "🎣 钓鱼点 按 E"
	_hint.font_size = 28
	_hint.position = Vector3(0, 2.5, 0)
	_hint.billboard = BaseMaterial3D.BILLBOARD_ENABLED
	_hint.outline_size = 6
	_hint.outline_modulate = Color(0, 0, 0, 0.6)
	_hint.visible = false
	add_child(_hint)

func _build_dock() -> void:
	var wood_mat = ModelFactory.get_material(Color(0.5, 0.35, 0.2), {"shaded": true})
	var water_mat = ModelFactory.get_material(Color(0.2, 0.5, 0.8), {"metallic": 0.7, "roughness": 0.15})
	# 木栈道
	var dock = CSGBox3D.new()
	dock.size = Vector3(3.0, 0.2, 3.0)
	dock.position = Vector3(0, 0.1, 0)
	dock.material = wood_mat
	add_child(dock)
	# 栈道支柱
	for sx in [-1, 1]:
		for sz in [-1, 1]:
			var post = CSGCylinder3D.new()
			post.radius = 0.08; post.height = 1.0
			post.position = Vector3(sx * 1.2, -0.4, sz * 1.2)
			post.material = wood_mat
			add_child(post)
	# 水面（栈道前方蓝色面）
	var water = CSGBox3D.new()
	water.size = Vector3(5.0, 0.05, 3.0)
	water.position = Vector3(0, 0.05, 3.0)
	water.material = water_mat
	add_child(water)
	# 钓鱼竿（斜插的细柱）
	var rod = CSGCylinder3D.new()
	rod.radius = 0.02; rod.height = 2.5
	rod.position = Vector3(0.5, 1.2, 0)
	rod.rotation_degrees = Vector3(-30, 0, 20)
	rod.material = wood_mat
	add_child(rod)
	# 鱼竿尖端（鱼线起点）
	_rod_tip = Node3D.new()
	_rod_tip.position = Vector3(0.8, 2.2, 0)
	add_child(_rod_tip)
	# 浮标（水面上的小球）
	var bobber = CSGSphere3D.new()
	bobber.radius = 0.08; bobber.position = Vector3(0.8, 0.2, 2.0)
	bobber.name = "Bobber"
	var bmat = ModelFactory.get_material(Color(0.9, 0.2, 0.2), {"emissive": Color(0.4, 0.1, 0.1), "emissive_energy": 0.5})
	bobber.material = bmat
	add_child(bobber)

func _process(delta: float) -> void:
	match _state:
		State.WAITING:
			_timer -= delta
			if _timer <= 0:
				# 鱼上钩！
				_state = State.HOOKED
				_timer = REACTION_TIME
				_hint.text = "❗ 鱼上钩！快按 E！"
				_hint.modulate = Color(1.3, 0.5, 0.3)
				AudioBus.play_pickup()
				# 浮标下沉动画
				var bobber = get_node_or_null("Bobber")
				if bobber:
					bobber.position.y = -0.1
		State.HOOKED:
			_timer -= delta
			if _timer <= 0:
				# 超时，鱼跑了
				_fail()
		State.COOLDOWN:
			_timer -= delta
			if _timer <= 0:
				_state = State.IDLE
				if _player_near:
					_hint.text = "🎣 钓鱼点 按 E"
					_hint.modulate = Color.WHITE
	# 浮标浮动
	var bobber = get_node_or_null("Bobber")
	if bobber and _state != State.HOOKED:
		var t = Time.get_ticks_msec() * 0.003
		bobber.position.y = 0.2 + sin(t) * 0.05

func _on_body_entered(body: Node) -> void:
	if body.is_in_group("player"):
		_player_near = true
		if _state == State.IDLE:
			_hint.visible = true

func _on_body_exited(body: Node) -> void:
	if body.is_in_group("player"):
		_player_near = false
		_hint.visible = false
		if _state == State.WAITING or _state == State.HOOKED:
			_cancel()

func _unhandled_input(event: InputEvent) -> void:
	if event is InputEventKey and event.pressed and event.keycode == KEY_E:
		if not _player_near:
			return
		match _state:
			State.IDLE:
				_start_fishing()
			State.HOOKED:
				_catch()

func _start_fishing() -> void:
	_state = State.WAITING
	_timer = randf_range(HOOK_WAIT_MIN, HOOK_WAIT_MAX)
	_hint.text = "🎣 等待鱼上钩..."
	_hint.modulate = Color(0.6, 0.8, 1.0)
	AudioBus.play_note(200.0, 0.1, 0.1)

func _catch() -> void:
	# 钓到了！
	_state = State.COOLDOWN
	_timer = COOLDOWN
	_hint.text = "🎣 钓到了！"
	_hint.modulate = Color(0.4, 1.0, 0.4)
	# 奖励：鱼类收集物
	GameState.collect_item("shell")
	GameState.collect_item("pearl")
	Confetti.burst(get_tree().current_scene, global_position + Vector3(0, 1, 0), Color(0.4, 0.7, 1.0))
	EventBus.toast_message.emit("钓到宝贝！+贝壳 +珍珠", "🎣")
	AudioBus.play_mission_complete()
	# 浮标复位
	var bobber = get_node_or_null("Bobber")
	if bobber:
		bobber.position.y = 0.2
	# 3 秒后重置提示
	get_tree().create_timer(COOLDOWN).timeout.connect(func():
		if _player_near and _state == State.IDLE:
			_hint.text = "🎣 钓鱼点 按 E"
			_hint.modulate = Color.WHITE
	)

func _fail() -> void:
	_state = State.COOLDOWN
	_timer = COOLDOWN
	_hint.text = "💨 鱼跑了..."
	_hint.modulate = Color(0.6, 0.6, 0.6)
	EventBus.toast_message.emit("鱼跑了，再试一次！", "💨")
	var bobber = get_node_or_null("Bobber")
	if bobber:
		bobber.position.y = 0.2

func _cancel() -> void:
	_state = State.IDLE
	_hint.visible = false
	_hint.modulate = Color.WHITE
	var bobber = get_node_or_null("Bobber")
	if bobber:
		bobber.position.y = 0.2
