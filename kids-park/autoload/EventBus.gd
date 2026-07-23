#============================================================
# EventBus.gd — 信号总线（autoload 单例）
#============================================================
extends Node

# --- 采集 ---
signal item_collected(item_type: String, count: int)
signal collection_updated(total: int)

# --- 任务 ---
signal mission_accepted(mission: Dictionary)
signal mission_progress_updated(mission_id: String, progress: int, target: int)
signal mission_completed(mission_id: String, sticker: String)

# --- 贴纸 ---
signal sticker_earned(sticker_name: String)

# --- 区域 ---
signal zone_unlocked(zone_name: String)

# --- 反馈 ---
signal confetti_burst(pos: Vector3, color: Color)
signal toast_message(text: String, emoji: String)
