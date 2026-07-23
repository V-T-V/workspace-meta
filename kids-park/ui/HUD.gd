#============================================================
# HUD.gd — 收集进度 + 任务提示 + 彩纸 + Toast（儿童友好大图标）
#============================================================
extends CanvasLayer

var _hud_label: Label
var _toast_label: Label
var _toast_timer: float = 0.0

func _ready() -> void:
	EventBus.collection_updated.connect(_on_collection_updated)
	EventBus.toast_message.connect(_on_toast)
	EventBus.zone_unlocked.connect(_on_zone_unlocked)
	EventBus.sticker_earned.connect(_on_sticker_earned)
	_build_ui()
	_on_collection_updated(GameState.total_collected)

func _process(delta: float) -> void:
	if _toast_timer > 0:
		_toast_timer -= delta
		if _toast_timer <= 0:
			_toast_label.visible = false

func _build_ui() -> void:
	var root = Control.new()
	root.set_anchors_preset(Control.PRESET_FULL_RECT)
	root.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(root)
	# 左上：收集进度（大字 + emoji）
	_hud_label = Label.new()
	_hud_label.position = Vector2(20, 20)
	_hud_label.add_theme_font_size_override("font_size", 30)
	_hud_label.add_theme_color_override("font_color", Color(0.2, 0.2, 0.3))
	root.add_child(_hud_label)
	# 中心：Toast 消息（拾取/任务反馈）
	_toast_label = Label.new()
	_toast_label.set_anchors_preset(Control.PRESET_CENTER_TOP)
	_toast_label.position = Vector2(-100, 80)
	_toast_label.add_theme_font_size_override("font_size", 36)
	_toast_label.add_theme_color_override("font_color", Color(0.9, 0.4, 0.1))
	_toast_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_toast_label.custom_minimum_size = Vector2(200, 50)
	_toast_label.visible = false
	root.add_child(_toast_label)

func _on_collection_updated(total: int) -> void:
	var stickers = GameState.stickers.size()
	_hud_label.text = "⭐ %d  🏅 %d  🗺️ %d/4" % [total, stickers, GameState.unlocked_zones.size()]

func _on_toast(text: String, _emoji: String) -> void:
	_toast_label.text = text
	_toast_label.visible = true
	_toast_timer = 2.0

func _on_zone_unlocked(zone_name: String) -> void:
	_on_toast("新区域：%s" % zone_name, "🗺️")
	AudioBus.play_zone_unlock()

func _on_sticker_earned(_sticker_name: String) -> void:
	AudioBus.play_sticker()
