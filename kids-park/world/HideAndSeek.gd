#============================================================
# HideAndSeek.gd — 捉迷藏迷你游戏
#============================================================
# 按 G 键启动：一个 NPC 会躲到区域边缘隐蔽处
# 玩家需要在 60 秒内找到它（靠近 3m 内自动触发）
# 找到后获得 "🎯捉迷藏高手" 贴纸 + 5 个随机物品
# 每局每区域只能玩一次
#============================================================
extends CanvasLayer

const Confetti = preload("res://world/Confetti.gd")
const SEEK_TIME: float = 60.0
const FIND_DISTANCE: float = 3.5

var _active: bool = false
var _timer: float = 0.0
var _hiding_npc: CharacterBody3D = null
var _timer_label: Label
var _played_zones: Array = []   # 本局已玩过的区域

func _ready() -> void:
	_build_timer_label()

func _build_timer_label() -> void:
	var root = Control.new()
	root.set_anchors_preset(Control.PRESET_FULL_RECT)
	root.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(root)
	_timer_label = Label.new()
	_timer_label.text = ""
	_timer_label.add_theme_font_size_override("font_size", 40)
	_timer_label.add_theme_color_override("font_color", Color(0.9, 0.3, 0.5))
	_timer_label.set_anchors_preset(Control.PRESET_CENTER_TOP)
	_timer_label.position = Vector2(-100, 10)
	_timer_label.visible = false
	root.add_child(_timer_label)

func _unhandled_input(event: InputEvent) -> void:
	if event is InputEventKey and event.pressed and event.keycode == KEY_G:
		if not _active:
			_start_hide_and_seek()

func _start_hide_and_seek() -> void:
	var player = get_tree().get_first_node_in_group("player")
	if player == null:
		return
	# 找一个该区域尚未玩过的 NPC
	var candidates: Array = []
	for npc in get_tree().get_nodes_in_group("npc"):
		if npc.zone_id not in _played_zones:
			candidates.append(npc)
	if candidates.is_empty():
		EventBus.toast_message.emit("所有区域都玩过了！", "🎯")
		return
	# 随机选一个 NPC
	_hiding_npc = candidates[randi() % candidates.size()]
	var zone_id = _hiding_npc.zone_id
	_played_zones.append(zone_id)
	# 把 NPC 移到区域边缘隐蔽位置
	var center = ParkGen.ZONE_CENTERS[zone_id]
	var rng = RandomNumberGenerator.new()
	rng.randomize()
	var a = rng.randf() * TAU
	var d = ParkGen.ZONE_SIZE * 0.42
	_hiding_npc.global_position = center + Vector3(cos(a) * d, 0, sin(a) * d)
	# 隐藏 NPC 头顶气泡（让它"躲起来"）
	var bubble = _hiding_npc.get_node_or_null("Label3D")
	if bubble:
		bubble.visible = false
	# 开始计时
	_active = true
	_timer = SEEK_TIME
	_timer_label.visible = true
	EventBus.toast_message.emit("捉迷藏开始！找到隐藏的动物！", "🎯")
	AudioBus.play_zone_unlock()

func _process(delta: float) -> void:
	if not _active:
		return
	_timer -= delta
	var player = get_tree().get_first_node_in_group("player")
	if player == null:
		return
	# 更新计时显示
	_timer_label.text = "🎯 %.0f 秒" % max(0, _timer)
	# 检查是否找到
	if _hiding_npc and is_instance_valid(_hiding_npc):
		var dist = player.global_position.distance_to(_hiding_npc.global_position)
		if dist < FIND_DISTANCE:
			_found()
			return
	# 超时
	if _timer <= 0:
		_timeout()

func _found() -> void:
	_active = false
	_timer_label.visible = false
	# 恢复 NPC 气泡
	if _hiding_npc and is_instance_valid(_hiding_npc):
		var bubble = _hiding_npc.get_node_or_null("Label3D")
		if bubble:
			bubble.visible = true
	# 奖励
	GameState.earn_sticker("🎯捉迷藏高手")
	var reward_items = ["apple", "flower", "pearl", "butterfly", "starfish"]
	for i in 5:
		GameState.collect_item(reward_items[i % reward_items.size()])
	EventBus.toast_message.emit("找到了！获得贴纸 + 5 物品！", "🎉")
	AudioBus.play_mission_complete()
	Confetti.burst(get_tree().current_scene, _hiding_npc.global_position + Vector3(0, 1, 0), Color(1, 0.8, 0.2))

func _timeout() -> void:
	_active = false
	_timer_label.visible = false
	# 恢复 NPC 气泡 + 位置
	if _hiding_npc and is_instance_valid(_hiding_npc):
		var bubble = _hiding_npc.get_node_or_null("Label3D")
		if bubble:
			bubble.visible = true
		# NPC 回到原位
		var zone_id = _hiding_npc.zone_id
		var center = ParkGen.ZONE_CENTERS[zone_id]
		_hiding_npc.global_position = center + Vector3(6, 0, 6)
	EventBus.toast_message.emit("时间到！下次再试吧～", "⏰")
