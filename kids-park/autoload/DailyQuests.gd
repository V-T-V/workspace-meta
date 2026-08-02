#============================================================
# DailyQuests.gd — 每日任务系统（autoload 单例）
#============================================================
# 每天生成 3 个任务，奖励物品 + 贴纸：
#   - 收集类：收集 N 个指定物品（在已解锁区域随机选）
#   - 探索类：累计拾取任意物品 M 个
#   - 社交类：与 NPC 互动/送礼累计 K 次
# 完成所有 3 个任务额外给大奖励。
# 跨会话持久化（按日期键存 user://kidpark_quests.json），新的一天自动刷新。
# 进度通过 EventBus 信号实时反馈给 HUD/UI。
#============================================================
extends Node

const SAVE_PATH := "user://kidpark_quests.json"
const QUEST_COUNT: int = 3

# 任务定义模板（type → 生成器参数）
# type: collect_item / collect_any / social_interact
var quests: Array = []   # [{id, type, text, emoji, target, progress, done, reward}]
var quest_date: String = ""   # 当前任务所属日期 YYYY-MM-DD
var all_completed_rewarded: bool = false

# 任务 ID 计数（同一天内保证唯一）
var _id_counter: int = 0

func _ready() -> void:
	_load()
	_check_daily_refresh()
	# 监听收集事件推进任务
	EventBus.item_collected.connect(_on_item_collected)
	# 监听互动事件（NPC interact / give_gift 调用 EventBus.npc_interacted）
	EventBus.npc_interacted.connect(_on_npc_interacted)

func _notification(what: int) -> void:
	if what == NOTIFICATION_WM_CLOSE_REQUEST or what == NOTIFICATION_WM_GO_BACK_REQUEST:
		_save()
	elif what == MainLoop.NOTIFICATION_APPLICATION_PAUSED:
		_save()

func _exit_tree() -> void:
	_save()

## 今日日期字符串
func _today() -> String:
	return Time.get_datetime_string_from_system(false).substr(0, 10)

## 检查是否需要刷新（跨天 / 首次进入）
func _check_daily_refresh() -> void:
	var today = _today()
	if quest_date != today or quests.is_empty():
		quest_date = today
		all_completed_rewarded = false
		_generate_daily_quests()
		_save()
		call_deferred("_announce_new_quests")

## 生成今日 3 个任务（基于已解锁区域，难度自适应）
func _generate_daily_quests() -> void:
	quests.clear()
	_id_counter = 0
	# 收集候选：从已解锁区域的物品中选
	var item_pool: Array = []
	for zone_id in GameState.unlocked_zones:
		for it in GameState.get_zone_items(zone_id):
			item_pool.append(it)
	if item_pool.is_empty():
		item_pool = ["apple", "flower", "butterfly"]   # 兜底
	# 任务 1：收集指定物品
	var chosen_item = item_pool[randi() % item_pool.size()]
	var idef = GameState.ITEM_TYPES.get(chosen_item, {})
	var target1 = randi_range(3, 5)
	quests.append(_make_quest("collect_item", "收集 %d 个 %s" % [target1, idef.get("emoji", "❓")], idef.get("emoji", "🍎"), target1, chosen_item, _reward_for(target1)))
	# 任务 2：累计拾取任意物品（鼓励探索）
	var target2 = randi_range(8, 12)
	quests.append(_make_quest("collect_any", "拾取任意物品 %d 个" % target2, "🎁", target2, "", _reward_for(target2)))
	# 任务 3：社交互动（鼓励和 NPC 玩）
	var target3 = randi_range(2, 4)
	quests.append(_make_quest("social_interact", "和小动物互动 %d 次" % target3, "💬", target3, "", _reward_for(target3, true)))

## 根据目标量 + 是否稀有生成奖励（物品 + 贴纸标记）
func _reward_for(target: int, rare_reward: bool = false) -> Dictionary:
	var amount = target * 2
	var reward_item = "pearl" if rare_reward else "apple"
	return {"item": reward_item, "amount": amount, "coins_emoji": GameState.ITEM_TYPES.get(reward_item, {}).get("emoji", "🎁")}

## 构造单个任务对象
func _make_quest(type: String, text: String, emoji: String, target: int, param: String, reward: Dictionary) -> Dictionary:
	_id_counter += 1
	return {
		"id": "q%d" % _id_counter,
		"type": type,
		"text": text,
		"emoji": emoji,
		"target": target,
		"param": param,        # collect_item 时为 item_type
		"progress": 0,
		"done": false,
		"reward": reward,
	}

## 物品收集事件处理
func _on_item_collected(item_type: String, _count: int) -> void:
	if quests.is_empty():
		return
	var changed := false
	for q in quests:
		if q["done"]:
			continue
		match q["type"]:
			"collect_item":
				if item_type == q["param"]:
					q["progress"] = int(q["progress"]) + 1
					changed = true
			"collect_any":
				q["progress"] = int(q["progress"]) + 1
				changed = true
		if q["progress"] >= q["target"] and not q["done"]:
			_complete_quest(q)
			changed = true
	if changed:
		_save()
		EventBus.quests_updated.emit()

## NPC 互动事件处理
func _on_npc_interacted(_zone_id: String) -> void:
	if quests.is_empty():
		return
	var changed := false
	for q in quests:
		if q["done"] or q["type"] != "social_interact":
			continue
		q["progress"] = int(q["progress"]) + 1
		if q["progress"] >= q["target"]:
			_complete_quest(q)
		changed = true
	if changed:
		_save()
		EventBus.quests_updated.emit()

## 完成单个任务：发奖励 + Toast
func _complete_quest(q: Dictionary) -> void:
	q["done"] = true
	var reward: Dictionary = q["reward"]
	GameState.collect_item(reward["item"], reward["amount"])
	EventBus.toast_message.emit("每日任务完成！+%d %s" % [reward["amount"], reward["coins_emoji"]], q.get("emoji", "✅"))
	AudioBus.play_mission_complete()
	# 检查是否全部完成
	_check_all_completed()

## 全部 3 个任务完成 → 额外大奖
func _check_all_completed() -> void:
	if all_completed_rewarded:
		return
	var all_done := true
	for q in quests:
		if not q["done"]:
			all_done = false
			break
	if all_done:
		all_completed_rewarded = true
		# 大奖：3 个金星 + 贴纸
		GameState.collect_item("goldstar", 3)
		GameState.earn_sticker("📅日日精进")
		EventBus.toast_message.emit("全部每日任务完成！+3 🌟 + 贴纸！", "🏆")
		AudioBus.play_zone_unlock()

## 公开：获取今日任务列表（UI 用）
func get_quests() -> Array:
	return quests

## 公开：今日已完成数量
func get_completed_count() -> int:
	var n := 0
	for q in quests:
		if q["done"]:
			n += 1
	return n

func _announce_new_quests() -> void:
	EventBus.toast_message.emit("新一天！3 个每日任务已刷新", "📅")
	EventBus.quests_updated.emit()

# --- 持久化 ---
func _save() -> void:
	var data := {
		"quest_date": quest_date,
		"all_completed_rewarded": all_completed_rewarded,
		"quests": quests,
		"id_counter": _id_counter,
	}
	var f := FileAccess.open(SAVE_PATH, FileAccess.WRITE)
	if f == null:
		return
	f.store_string(JSON.stringify(data))
	f.close()

func _load() -> void:
	if not FileAccess.file_exists(SAVE_PATH):
		return
	var f := FileAccess.open(SAVE_PATH, FileAccess.READ)
	if f == null:
		return
	var parsed = JSON.parse_string(f.get_as_text())
	f.close()
	if typeof(parsed) != TYPE_DICTIONARY:
		return
	quest_date = String(parsed.get("quest_date", ""))
	all_completed_rewarded = bool(parsed.get("all_completed_rewarded", false))
	var q = parsed.get("quests", [])
	if typeof(q) == TYPE_ARRAY:
		quests = q
	_id_counter = int(parsed.get("id_counter", 0))
