#============================================================
# Pet.gd — 宠物伴灵（跟随玩家的小动物）
#============================================================
# 完成 NPC 任务后获得宠物伴灵，跟随玩家在身后跑动
# 宠物会：跟在身后 2-3m / 跳跃跟随 / 拾取时欢呼 / 空闲时转圈
# 每个区域任务完成解锁对应宠物：🐰🐰🐱🐻🦊
#============================================================
extends CharacterBody3D

const FOLLOW_DISTANCE: float = 2.5     # 跟随距离
const CATCHUP_SPEED: float = 7.0       # 追赶速度
const IDLE_SPEED: float = 2.0          # 空闲漫游速度
const GRAVITY: float = 18.0

@export var pet_id: String = "bunny"   # 宠物类型标识
var _player: CharacterBody3D = null
var _hop_phase: float = 0.0
var _idle_angle: float = 0.0
var _visual: Node3D = null
var _emoji: Label3D = null
var _cheer_timer: float = 0.0          # 拾取时欢呼计时

const PET_CONFIG := {
	"bunny":   {"emoji": "🐰", "color": Color(0.95, 0.9, 0.85), "scale": 0.5},
	"cat":     {"emoji": "🐱", "color": Color(0.8, 0.6, 0.4), "scale": 0.5},
	"bear":    {"emoji": "🐻", "color": Color(0.6, 0.45, 0.3), "scale": 0.55},
	"fox":     {"emoji": "🦊", "color": Color(0.85, 0.5, 0.25), "scale": 0.5},
}

func _ready() -> void:
	add_to_group("pet")
	var cfg = PET_CONFIG.get(pet_id, PET_CONFIG["bunny"])
	# 用 CSG 简化动物（球身 + 球头 + 耳朵）
	_visual = _build_pet_visual(cfg["color"], cfg["scale"])
	add_child(_visual)
	# 头顶 emoji 气泡
	_emoji = Label3D.new()
	_emoji.text = cfg.get("emoji", "🐰")
	_emoji.font_size = 36
	_emoji.position = Vector3(0, 1.2, 0)
	_emoji.billboard = BaseMaterial3D.BILLBOARD_ENABLED
	_emoji.outline_size = 6
	_emoji.outline_modulate = Color(0, 0, 0, 0.5)
	add_child(_emoji)
	# 订阅拾取信号（宠物欢呼）
	EventBus.item_collected.connect(_on_item_collected)

func _build_pet_visual(color: Color, s: float) -> Node3D:
	var node = Node3D.new()
	var mat = ModelFactory.get_material(color, {"shaded": true})
	# 身体（球）
	var body = CSGSphere3D.new()
	body.radius = 0.3 * s
	body.position = Vector3(0, 0.3 * s, 0)
	body.material = mat
	node.add_child(body)
	# 头（球）
	var head = CSGSphere3D.new()
	head.radius = 0.22 * s
	head.position = Vector3(0, 0.55 * s, 0.2 * s)
	head.material = mat
	node.add_child(head)
	# 两只耳朵
	for sx in [-1, 1]:
		var ear = CSGSphere3D.new()
		ear.radius = 0.07 * s
		ear.scale = Vector3(0.5, 1.5, 0.5)
		ear.position = Vector3(sx * 0.1 * s, 0.75 * s, 0.15 * s)
		ear.material = mat
		node.add_child(ear)
	# 眼睛
	for sx in [-1, 1]:
		var eye = CSGSphere3D.new()
		eye.radius = 0.03 * s
		eye.position = Vector3(sx * 0.07 * s, 0.58 * s, 0.38 * s)
		var emat = ModelFactory.get_material(Color(0.05, 0.05, 0.1))
		eye.material = emat
		node.add_child(eye)
	return node

func _physics_process(delta: float) -> void:
	velocity.y -= GRAVITY * delta
	if _player == null or not is_instance_valid(_player):
		_player = get_tree().get_first_node_in_group("player")
		if _player == null:
			move_and_slide()
			return
	var to_player = _player.global_position - global_position
	to_player.y = 0
	var dist = to_player.length()
	# 水平移动
	if dist > FOLLOW_DISTANCE:
		# 追赶玩家
		var dir = to_player.normalized()
		velocity.x = dir.x * CATCHUP_SPEED
		velocity.z = dir.z * CATCHUP_SPEED
		# 面向移动方向
		var target_y = atan2(dir.x, dir.z)
		rotation.y = lerp_angle(rotation.y, target_y, delta * 8.0)
		# 跳跃动画（兔子式蹦跳）
		_hop_phase += delta * 8.0
		if _visual:
			_visual.position.y = max(0, sin(_hop_phase) * 0.2)
	else:
		# 空闲：减速 + 缓慢转圈
		velocity.x = lerp(velocity.x, 0.0, delta * 5.0)
		velocity.z = lerp(velocity.z, 0.0, delta * 5.0)
		_idle_angle += delta * 0.5
		rotation.y = lerp_angle(rotation.y, _idle_angle, delta * 2.0)
		if _visual:
			_visual.position.y = lerp(_visual.position.y, 0.0, delta * 5.0)
	# 欢呼弹跳（拾取时）
	if _cheer_timer > 0:
		_cheer_timer -= delta
		if _visual:
			_visual.position.y += abs(sin(_cheer_timer * 15.0)) * 0.15
	move_and_slide()

func _on_item_collected(_item_type: String, _count: int) -> void:
	# 玩家拾取时宠物欢呼
	_cheer_timer = 0.5
