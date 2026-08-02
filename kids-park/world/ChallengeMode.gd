#============================================================
# ChallengeMode.gd — 限时挑战模式（连击收集）
#============================================================
# 每收集 1 个物品，启动 5 秒连击窗口：
#   - 窗口内继续拾取 → 连击数 +1，时间重置
#   - 超时未拾取 → 连击结束，结算奖励倍率
#   - 连击 3+ 时获得额外分数倍率（x1.5 / x2 / x3）
# HUD 实时显示连击数 + 剩余时间条
# 按 C 键手动触发"30 秒冲刺挑战"（限时收集尽可能多）
#============================================================
extends CanvasLayer

const COMBO_WINDOW: float = 5.0     # 连击窗口（秒）
const DASH_DURATION: float = 30.0   # 冲刺挑战时长
var _combo: int = 0
var _combo_timer: float = 0.0
var _combo_active: bool = false
var _multiplier: float = 1.0

# 冲刺模式
var _dash_active: bool = false
var _dash_timer: float = 0.0
var _dash_count: int = 0

# UI
var _combo_label: Label
var _timer_bar: ProgressBar
var _dash_label: Label

func _ready() -> void:
	EventBus.item_collected.connect(_on_item_collected)
	_build_ui()

func _build_ui() -> void:
	var root = Control.new()
	root.set_anchors_preset(Control.PRESET_FULL_RECT)
	root.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(root)
	# 连击数（右上角偏左，避开小地图）
	_combo_label = Label.new()
	_combo_label.text = ""
	_combo_label.add_theme_font_size_override("font_size", 36)
	_combo_label.add_theme_color_override("font_color", Color(1, 0.5, 0.1))
	_combo_label.set_anchors_preset(Control.PRESET_CENTER_TOP)
	_combo_label.position = Vector2(-80, 60)
	_combo_label.visible = false
	root.add_child(_combo_label)
	# 连击时间条
	_timer_bar = ProgressBar.new()
	_timer_bar.custom_minimum_size = Vector2(160, 12)
	_timer_bar.set_anchors_preset(Control.PRESET_CENTER_TOP)
	_timer_bar.position = Vector2(-80, 105)
	_timer_bar.max_value = COMBO_WINDOW
	_timer_bar.value = 0
	_timer_bar.visible = false
	# 自定义样式（橙色填充）
	var fill = StyleBoxFlat.new()
	fill.bg_color = Color(1, 0.6, 0.1)
	_timer_bar.add_theme_stylebox_override("fill", fill)
	var bg2 = StyleBoxFlat.new()
	bg2.bg_color = Color(0, 0, 0, 0.3)
	_timer_bar.add_theme_stylebox_override("background", bg2)
	root.add_child(_timer_bar)
	# 冲刺挑战计时（顶部居中）
	_dash_label = Label.new()
	_dash_label.text = ""
	_dash_label.add_theme_font_size_override("font_size", 42)
	_dash_label.add_theme_color_override("font_color", Color(0.9, 0.2, 0.5))
	_dash_label.set_anchors_preset(Control.PRESET_CENTER_TOP)
	_dash_label.position = Vector2(-100, 10)
	_dash_label.visible = false
	root.add_child(_dash_label)

func _unhandled_input(event: InputEvent) -> void:
	if event is InputEventKey and event.pressed and event.keycode == KEY_C:
		_start_dash()

func _on_item_collected(_item_type: String, _count: int) -> void:
	# 连击逻辑
	_combo += 1
	_combo_timer = COMBO_WINDOW
	_combo_active = true
	# 计算倍率
	if _combo >= 10:
		_multiplier = 3.0
	elif _combo >= 5:
		_multiplier = 2.0
	elif _combo >= 3:
		_multiplier = 1.5
	else:
		_multiplier = 1.0
	# 更新 UI
	_combo_label.text = "🔥 连击 x%d  (%.1f)" % [_combo, _multiplier]
	_combo_label.visible = true
	_timer_bar.visible = true
	_timer_bar.value = COMBO_WINDOW
	# 颜色随连击升温
	if _combo >= 10:
		_combo_label.add_theme_color_override("font_color", Color(0.9, 0.1, 0.1))
	elif _combo >= 5:
		_combo_label.add_theme_color_override("font_color", Color(0.9, 0.4, 0.1))
	else:
		_combo_label.add_theme_color_override("font_color", Color(1, 0.7, 0.2))
	# 冲刺模式计数
	if _dash_active:
		_dash_count += 1
	# 连击里程碑通知
	if _combo == 3:
		EventBus.toast_message.emit("连击开始！x1.5", "🔥")
		AudioBus.play_sticker()
	elif _combo == 5:
		EventBus.toast_message.emit("火热连击！x2", "🔥")
		AudioBus.play_mission_complete()
	elif _combo == 10:
		EventBus.toast_message.emit("超级连击！x3", "🔥")
		AudioBus.play_zone_unlock()

func _process(delta: float) -> void:
	# 连击倒计时
	if _combo_active:
		_combo_timer -= delta
		_timer_bar.value = max(0, _combo_timer)
		if _combo_timer <= 0:
			# 连击结束
			_end_combo()
	# 冲刺挑战倒计时
	if _dash_active:
		_dash_timer -= delta
		_dash_label.text = "⏰ %.0f 秒  📦 %d" % [max(0, _dash_timer), _dash_count]
		if _dash_timer <= 0:
			_end_dash()

func _end_combo() -> void:
	_combo_active = false
	_combo_label.visible = false
	_timer_bar.visible = false
	if _combo >= 3:
		# 连击奖励：发放额外物品（让连击有实际游戏效果）
		var bonus = int(_combo * _multiplier)
		var reward_items = ["apple", "flower", "pearl", "starfish", "butterfly"]
		var reward = reward_items[_combo % reward_items.size()]
		# 连击 10+ 额外给贴纸
		if _combo >= 10:
			GameState.earn_sticker("🔥连击大师")
			EventBus.toast_message.emit("超级连击！%d 连击 → 🎁连击大师贴纸" % _combo, "🔥")
			AudioBus.play_sticker()
		else:
			# 发放奖励物品
			GameState.collect_item(reward, bonus)
			EventBus.toast_message.emit("连击结束！%d 连击 → +%d %s" % [_combo, bonus, GameState.ITEM_TYPES[reward].get("emoji", "⭐")], "🎉")
	_combo = 0
	_multiplier = 1.0

func _start_dash() -> void:
	if _dash_active:
		return   # 已在冲刺中
	_dash_active = true
	_dash_timer = DASH_DURATION
	_dash_count = 0
	_dash_label.visible = true
	EventBus.toast_message.emit("30 秒冲刺开始！", "⏰")
	AudioBus.play_zone_unlock()

func _end_dash() -> void:
	_dash_active = false
	_dash_label.visible = false
	# 结算
	var grade = "D"
	if _dash_count >= 20:
		grade = "S"
	elif _dash_count >= 15:
		grade = "A"
	elif _dash_count >= 10:
		grade = "B"
	elif _dash_count >= 5:
		grade = "C"
	EventBus.toast_message.emit("冲刺结束！收集 %d 个 → %s 级" % [_dash_count, grade], "🏅")
	AudioBus.play_mission_complete()
