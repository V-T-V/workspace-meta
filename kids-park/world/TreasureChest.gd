#============================================================
# TreasureChest.gd — 宝箱（随机奖励：贴纸/收集物/成就）
#============================================================
# 宝箱散布在区域中，玩家靠近自动打开
# 奖励随机：3-5 个随机物品 + 概率获得贴纸
# 打开后 2 分钟重生（可重复开启）
#============================================================
extends Area3D

const Confetti = preload("res://world/Confetti.gd")
const RESPAWN_TIME: float = 120.0

var _opened: bool = false
var _respawn_timer: float = 0.0
var _visual: Node3D = null
var _initial_y: float = 0.0

func _ready() -> void:
	body_entered.connect(_on_body_entered)
	# 查找视觉节点
	for c in get_children():
		if c is CollisionShape3D:
			continue
		if c is Node3D:
			_visual = c
			break

func _process(delta: float) -> void:
	if _opened:
		_respawn_timer -= delta
		if _respawn_timer <= 0:
			_respawn()
		return
	# 浮动动画
	if _visual:
		var t = Time.get_ticks_msec() * 0.002
		_visual.position.y = _initial_y + sin(t) * 0.1
		_visual.rotate_y(delta * 0.5)

func _on_body_entered(body: Node) -> void:
	if _opened or not body.is_in_group("player"):
		return
	_open()

func _open() -> void:
	_opened = true
	_respawn_timer = RESPAWN_TIME
	# 打开动画：盖子上翻 + 缩放
	if _visual:
		var tw = create_tween()
		tw.tween_property(_visual, "scale", Vector3(1.3, 1.3, 1.3), 0.2)
		tw.tween_property(_visual, "scale", Vector3(0.8, 0.8, 0.8), 0.1)
	# 奖励：3-5 个随机物品（过滤锁定区 + 排除稀有 goldstar）
	var rng = RandomNumberGenerator.new()
	rng.randomize()
	var count = rng.randi_range(3, 5)
	# 只给已解锁区域的普通物品
	var valid_items: Array = []
	for item_type in GameState.ITEM_TYPES:
		var idef = GameState.ITEM_TYPES[item_type]
		var zone = idef.get("zone", "")
		if zone == "all":
			continue   # 跳过 goldstar
		if GameState.is_zone_unlocked(zone):
			valid_items.append(item_type)
	if valid_items.is_empty():
		valid_items = ["apple", "flower"]   # 安全兜底
	for i in count:
		var item = valid_items[rng.randi() % valid_items.size()]
		GameState.collect_item(item)
	# 概率获得贴纸（20%）
	if rng.randf() < 0.2:
		var sticker = "🎁宝箱惊喜"
		GameState.earn_sticker(sticker)
		EventBus.toast_message.emit("宝箱惊喜！获得贴纸！", "🎁")
		AudioBus.play_sticker()
	else:
		EventBus.toast_message.emit("宝箱打开！+%d 物品" % count, "🎁")
	# 彩纸爆发
	Confetti.burst(get_tree().current_scene, global_position + Vector3(0, 1, 0), Color(1, 0.8, 0.2))
	AudioBus.play_mission_complete()
	# 隐藏
	visible = false
	monitoring = false

func _respawn() -> void:
	_opened = false
	visible = true
	monitoring = true
	_initial_y = _visual.position.y if _visual else 0
	if _visual:
		_visual.scale = Vector3.ONE
	Confetti.burst(get_tree().current_scene, global_position, Color(0.4, 1.0, 0.6))
