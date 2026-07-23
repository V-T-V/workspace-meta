#============================================================
# GameState.gd — 收集图鉴 + 贴纸 + 存档（无生命/弹药/通缉）
#============================================================
extends Node

const SAVE_PATH := "user://kidspark_state.json"

# 收集图鉴：item_type -> 已收集数量
var collection: Dictionary = {}
var total_collected: int = 0

# 贴纸册
var stickers: Array = []

# 已解锁区域
var unlocked_zones: Array = ["grassland"]  # 默认开放草地

# 任务进度
var mission_progress: Dictionary = {}

# 成就系统
var achievements: Array = []   # 已解锁的成就 ID 列表

# 成就定义（ID → {name, emoji, condition_desc, check_fn_name}）
const ACHIEVEMENTS := {
	"first_step":    {"name": "初次探索", "emoji": "👣", "desc": "收集第 1 个物品", "threshold": 1},
	"collector_10":  {"name": "小收藏家", "emoji": "📦", "desc": "收集 10 个物品", "threshold": 10},
	"collector_50":  {"name": "收藏大师", "emoji": "🏆", "desc": "收集 50 个物品", "threshold": 50},
	"collector_100": {"name": "乐园之星", "emoji": "⭐", "desc": "收集 100 个物品", "threshold": 100},
	"zone_2":        {"name": "双区探险", "emoji": "🗺️", "desc": "解锁 2 个区域", "threshold": 2},
	"zone_4":        {"name": "全境探索", "emoji": "🌍", "desc": "解锁全部 4 个区域", "threshold": 4},
	"sticker_4":     {"name": "交友达人", "emoji": "❤️", "desc": "获得全部 4 个贴纸", "threshold": 4},
	"variety_6":     {"name": "多样收集", "emoji": "🌈", "desc": "收集 6 种不同物品", "threshold": 6},
	"variety_12":    {"name": "百科全书", "emoji": "📖", "desc": "收集全部 12 种物品", "threshold": 12},
}

# 收集物类型定义（名称/颜色/图标/所属区域）
const ITEM_TYPES := {
	"apple":      {"emoji": "🍎", "color": Color(0.9, 0.3, 0.2), "zone": "grassland"},
	"flower":     {"emoji": "🌸", "color": Color(0.9, 0.5, 0.7), "zone": "grassland"},
	"butterfly":  {"emoji": "🦋", "color": Color(0.6, 0.8, 1.0), "zone": "grassland"},
	"shell":      {"emoji": "🐚", "color": Color(0.95, 0.85, 0.7), "zone": "beach"},
	"starfish":   {"emoji": "⭐", "color": Color(1.0, 0.8, 0.2), "zone": "beach"},
	"pearl":      {"emoji": "🤍", "color": Color(0.9, 0.9, 1.0), "zone": "beach"},
	"petal":      {"emoji": "🌷", "color": Color(0.95, 0.4, 0.5), "zone": "garden"},
	"honey":      {"emoji": "🍯", "color": Color(1.0, 0.75, 0.2), "zone": "garden"},
	"ladybug":    {"emoji": "🐞", "color": Color(0.9, 0.2, 0.15), "zone": "garden"},
	"snowflake":  {"emoji": "❄️", "color": Color(0.7, 0.9, 1.0), "zone": "ice"},
	"icecrystal": {"emoji": "💎", "color": Color(0.5, 0.8, 1.0), "zone": "ice"},
	"egg":        {"emoji": "🥚", "color": Color(1.0, 0.95, 0.8), "zone": "ice"},
	"goldstar":   {"emoji": "🌟", "color": Color(1.0, 0.85, 0.1), "zone": "all", "value": 10},
}

# 区域配置
const ZONES := {
	"grassland": {"name": "草地乐园", "emoji": "🌱", "color": Color(0.55, 0.85, 0.4), "unlock_total": 0},
	"beach":     {"name": "沙滩海湾", "emoji": "🏖️", "color": Color(0.95, 0.88, 0.6), "unlock_total": 10},
	"garden":    {"name": "花卉花园", "emoji": "🌷", "color": Color(0.9, 0.6, 0.75), "unlock_total": 20},
	"ice":       {"name": "冰雪世界", "emoji": "❄️", "color": Color(0.65, 0.85, 0.95), "unlock_total": 35},
}

func _ready() -> void:
	_load()

func _exit_tree() -> void:
	_save()

func collect_item(item_type: String, amount: int = 1) -> void:
	if not ITEM_TYPES.has(item_type):
		return
	# 稀有物品给予额外计数
	var idef = ITEM_TYPES[item_type]
	var value = idef.get("value", 1)
	var actual_amount = amount * value
	if not collection.has(item_type):
		collection[item_type] = 0
	collection[item_type] += amount
	total_collected += actual_amount
	EventBus.item_collected.emit(item_type, collection[item_type])
	EventBus.collection_updated.emit(total_collected)
	_check_zone_unlocks()
	_check_achievements()

## 检查并解锁里程碑成就
func _check_achievements() -> void:
	# 按收集总数解锁
	var total_checks := {
		"first_step": total_collected,
		"collector_10": total_collected,
		"collector_50": total_collected,
		"collector_100": total_collected,
	}
	# 按区域数解锁
	var zone_checks := {
		"zone_2": unlocked_zones.size(),
		"zone_4": unlocked_zones.size(),
	}
	# 按贴纸数解锁
	var sticker_checks := {
		"sticker_4": stickers.size(),
	}
	# 按物品种类解锁
	var variety = 0
	for k in collection:
		if collection[k] > 0:
			variety += 1
	var variety_checks := {
		"variety_6": variety,
		"variety_12": variety,
	}
	# 检查所有成就
	for ach_id in ACHIEVEMENTS:
		if ach_id in achievements:
			continue
		var adef = ACHIEVEMENTS[ach_id]
		var current_val = 0
		if total_checks.has(ach_id):
			current_val = total_checks[ach_id]
		elif zone_checks.has(ach_id):
			current_val = zone_checks[ach_id]
		elif sticker_checks.has(ach_id):
			current_val = sticker_checks[ach_id]
		elif variety_checks.has(ach_id):
			current_val = variety_checks[ach_id]
		if current_val >= adef["threshold"]:
			achievements.append(ach_id)
			EventBus.toast_message.emit("成就解锁：%s %s" % [adef["emoji"], adef["name"]], adef["emoji"])
			AudioBus.play_sticker()

func _check_zone_unlocks() -> void:
	for zone_id in ZONES:
		if zone_id in unlocked_zones:
			continue
		var threshold: int = ZONES[zone_id]["unlock_total"]
		if total_collected >= threshold:
			unlocked_zones.append(zone_id)
			EventBus.zone_unlocked.emit(ZONES[zone_id]["name"])
			EventBus.toast_message.emit("新区域解锁！", "🎉")

func earn_sticker(sticker_name: String) -> void:
	if sticker_name in stickers:
		return
	stickers.append(sticker_name)
	EventBus.sticker_earned.emit(sticker_name)

func get_zone_items(zone_id: String) -> Array:
	var result: Array = []
	for item_type in ITEM_TYPES:
		if ITEM_TYPES[item_type]["zone"] == zone_id:
			result.append(item_type)
	return result

func get_collection_count(item_type: String) -> int:
	return collection.get(item_type, 0)

func is_zone_unlocked(zone_id: String) -> bool:
	return zone_id in unlocked_zones

# --- 存档 ---
func _save() -> void:
	var data := {
		"collection": collection,
		"total_collected": total_collected,
		"stickers": stickers,
		"unlocked_zones": unlocked_zones,
		"mission_progress": mission_progress,
		"achievements": achievements,
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
	collection = parsed.get("collection", {})
	total_collected = int(parsed.get("total_collected", 0))
	stickers = parsed.get("stickers", [])
	unlocked_zones = parsed.get("unlocked_zones", ["grassland"])
	mission_progress = parsed.get("mission_progress", {})
	achievements = parsed.get("achievements", [])
	print("[KidsPark] 载入：已收集 %d 贴纸 %d 区域 %d 成就 %d" % [total_collected, stickers.size(), unlocked_zones.size(), achievements.size()])
