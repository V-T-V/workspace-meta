#============================================================
# NPC.gd — NPC 小动物（站立 + 气泡 + 任务触发 + 完成持久化）
#============================================================
extends CharacterBody3D

const Confetti = preload("res://world/Confetti.gd")

@export var zone_id: String = "grassland"
var _player_near: bool = false
var _bubble: Label3D = null
var _progress_label: Label3D = null   # 头顶任务进度 "🍎 2/5"
var _task_done: bool = false    # 本局是否已完成（避免重复触发）
var _last_progress: int = -1    # 上次显示的进度（避免每帧重设文字）

const NPC_TASKS := {
	"grassland": {"emoji": "🐰", "task": "找5个苹果 🍎", "target_type": "apple", "target_count": 5, "target_emoji": "🍎", "sticker": "🐰小兔的朋友"},
	"beach":     {"emoji": "🐱", "task": "收集3个贝壳 🐚", "target_type": "shell", "target_count": 3, "target_emoji": "🐚", "sticker": "🐱小猫的宝藏"},
	"garden":    {"emoji": "🐻", "task": "找2个蜂蜜 🍯", "target_type": "honey", "target_count": 2, "target_emoji": "🍯", "sticker": "🐻小熊的甜点"},
	"ice":       {"emoji": "🦊", "task": "找3个雪花 ❄️", "target_type": "snowflake", "target_count": 3, "target_emoji": "❄️", "sticker": "🦊小狐的冬日"},
}

func _ready() -> void:
	add_to_group("npc")
	var task = NPC_TASKS.get(zone_id, {})
	# 已领过贴纸则显示完成态
	if task and GameState.stickers.has(task.get("sticker", "")):
		_task_done = true
	# 浮动气泡（3D 文字）—— emoji
	_bubble = Label3D.new()
	if _task_done:
		_bubble.text = "❤️"
	else:
		_bubble.text = task.get("emoji", "❓")
	_bubble.font_size = 64
	_bubble.position = Vector3(0, 2.0, 0)
	_bubble.billboard = BaseMaterial3D.BILLBOARD_ENABLED
	_bubble.outline_size = 8
	_bubble.outline_modulate = Color(0, 0, 0, 0.6)
	add_child(_bubble)
	# 任务进度标签（emoji + 数字，如 "🍎 0/5"）
	if not _task_done and task:
		_progress_label = Label3D.new()
		_progress_label.font_size = 36
		_progress_label.position = Vector3(0, 1.55, 0)
		_progress_label.billboard = BaseMaterial3D.BILLBOARD_ENABLED
		_progress_label.outline_size = 6
		_progress_label.outline_modulate = Color(0, 0, 0, 0.5)
		add_child(_progress_label)
		_update_progress_text()

func _update_progress_text() -> void:
	if _progress_label == null:
		return
	var task = NPC_TASKS.get(zone_id, {})
	if task.is_empty():
		return
	var have = GameState.get_collection_count(task["target_type"])
	if have == _last_progress:
		return   # 进度未变，不重设
	_last_progress = have
	var emoji = task.get("target_emoji", "⭐")
	# 用目标物品 emoji + 进度数字（儿童友好，无需识字）
	_progress_label.text = "%s %d/%d" % [emoji, min(have, task["target_count"]), task["target_count"]]
	# 接近完成时变绿色鼓励
	if have >= int(task["target_count"]):
		_progress_label.modulate = Color(0.4, 1.0, 0.4)
	elif have >= int(task["target_count"]) * 0.5:
		_progress_label.modulate = Color(1.0, 0.9, 0.4)
	else:
		_progress_label.modulate = Color.WHITE

func _process(delta: float) -> void:
	# 轻微弹跳
	var t = Time.get_ticks_msec() * 0.003
	_bubble.position.y = 2.0 + sin(t) * 0.1
	if _progress_label:
		_progress_label.position.y = 1.55 + sin(t + 1.0) * 0.08
		_update_progress_text()   # 实时刷新进度（玩家拾取时立即更新）
	# 玩家靠近时面向玩家 + 气泡高亮
	var player = get_tree().get_first_node_in_group("player")
	if player:
		var dist = global_position.distance_to(player.global_position)
		_player_near = dist < 3.0
		if _player_near:
			_bubble.modulate = Color(1.3, 1.3, 1.0)
			# 面向玩家
			var dir = (player.global_position - global_position)
			dir.y = 0
			if dir.length() > 0.1:
				var target_y = atan2(dir.x, dir.z)
				rotation.y = lerp_angle(rotation.y, target_y, delta * 5.0)
		else:
			_bubble.modulate = Color.WHITE
			# 空闲时缓慢转向（可爱的小动作）
			rotation.y += sin(t * 0.3) * delta * 0.5

func interact() -> void:
	"""玩家互动：检查任务完成"""
	if _task_done:
		# 已完成，给鼓励反馈
		EventBus.toast_message.emit("我们是好朋友啦！", "❤️")
		return
	var task = NPC_TASKS.get(zone_id, {})
	if task.is_empty():
		return
	var target_type: String = task["target_type"]
	var target_count: int = task["target_count"]
	var sticker: String = task["sticker"]
	# 已领过贴纸（跨局持久）
	if GameState.stickers.has(sticker):
		_task_done = true
		_bubble.text = "❤️"
		return
	var have = GameState.get_collection_count(target_type)
	if have >= target_count:
		# 完成！
		_task_done = true
		GameState.earn_sticker(sticker)
		Confetti.burst(get_tree().current_scene, global_position + Vector3(0, 1, 0), Color(1, 0.8, 0.2))
		EventBus.toast_message.emit("任务完成！", "🎉")
		AudioBus.play_mission_complete()
		_bubble.text = "❤️"
	else:
		# 进度提示（儿童友好：显示还差几个 + emoji）
		EventBus.toast_message.emit("还需要 %d 个 %s" % [target_count - have, task.get("emoji", "❓")], task.get("emoji", "❓"))
