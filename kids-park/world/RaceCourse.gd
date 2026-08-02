#============================================================
# RaceCourse.gd — 限时竞速跑酷（检查点计时赛道）
#============================================================
# 按 R 键启动：6 个环形检查点排成赛道
# 玩家依次穿过检查点 → 计时 → 完成后按时间评级
# S(<20s)/A(<30s)/B(<45s)/C(完成)
# 每次完成后刷新记录（存档保存最佳时间）
#============================================================
extends CanvasLayer

const Confetti = preload("res://world/Confetti.gd")
const CHECKPOINT_COUNT: int = 6

var _checkpoints: Array = []   # [{node, passed}]
var _active: bool = false
var _timer: float = 0.0
var _next_cp: int = 0
var _timer_label: Label
var _best_time: float = INF

func _ready() -> void:
	_build_timer_label()
	_build_checkpoints()
	_best_time = float(GameState.race_best_time) if "race_best_time" in GameState else INF

func _build_timer_label() -> void:
	var root = Control.new()
	root.set_anchors_preset(Control.PRESET_FULL_RECT)
	root.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(root)
	_timer_label = Label.new()
	_timer_label.add_theme_font_size_override("font_size", 36)
	_timer_label.add_theme_color_override("font_color", Color(0.2, 0.9, 0.5))
	_timer_label.set_anchors_preset(Control.PRESET_CENTER_TOP)
	_timer_label.position = Vector2(-80, 55)
	_timer_label.visible = false
	root.add_child(_timer_label)

func _build_checkpoints() -> void:
	# 检查点围绕中心广场环形排列
	var center = Vector3(0, 0, 0)
	var radius = 25.0
	for i in CHECKPOINT_COUNT:
		var angle = TAU * i / CHECKPOINT_COUNT
		var pos = center + Vector3(cos(angle) * radius, 2.0, sin(angle) * radius)
		var cp = Area3D.new()
		# 碰撞
		var col = CollisionShape3D.new()
		var shape = CylinderShape3D.new()
		shape.radius = 2.0; shape.height = 4.0
		col.shape = shape
		cp.add_child(col)
		# 视觉（彩色光环）
		var ring = _make_ring(Color(0.3, 0.6, 0.9) if i > 0 else Color(0.9, 0.8, 0.2))
		cp.add_child(ring)
		cp.body_entered.connect(_make_cp_handler(i))
		cp.visible = false
		cp.monitoring = false
		get_tree().current_scene.add_child(cp)
		cp.global_position = pos
		_checkpoints.append({"node": cp, "passed": false})

func _make_ring(color: Color) -> Node3D:
	var node = Node3D.new()
	var mat = ModelFactory.get_material(color, {"emissive": color, "emissive_energy": 0.5, "transparency_alpha": true, "shaded": false})
	# 外环（扁圆柱）
	var ring = CSGCylinder3D.new()
	ring.radius = 1.8; ring.height = 0.2
	ring.material = mat
	node.add_child(ring)
	# 发光
	var light = OmniLight3D.new()
	light.light_color = color
	light.light_energy = 1.0
	light.omni_range = 5.0
	node.add_child(light)
	return node

func _make_cp_handler(idx: int) -> Callable:
	return func(body):
		if body.is_in_group("player"):
			_on_checkpoint(idx)

func _unhandled_input(event: InputEvent) -> void:
	if event is InputEventKey and event.pressed and event.keycode == KEY_R:
		if not _active:
			_start_race()

func _start_race() -> void:
	_active = true
	_timer = 0.0
	_next_cp = 0
	_timer_label.visible = true
	# 显示所有检查点 + 重置
	for i in _checkpoints.size():
		_checkpoints[i].node.visible = true
		_checkpoints[i].node.monitoring = true
		_checkpoints[i].passed = false
	# 高亮第一个
	_highlight_cp(0)
	EventBus.toast_message.emit("竞速开始！穿过黄色检查点！", "🏁")
	AudioBus.play_zone_unlock()

func _highlight_cp(idx: int) -> void:
	if idx < _checkpoints.size():
		var cp = _checkpoints[idx].node
		# 让目标检查点闪烁发光（放大缩放）
		var ring = cp.get_child(1)  # visual node
		if ring:
			var tw = create_tween().set_loops(3)
			tw.tween_property(ring, "scale", Vector3(1.3, 1.3, 1.3), 0.2)
			tw.tween_property(ring, "scale", Vector3(1.0, 1.0, 1.0), 0.2)

func _on_checkpoint(idx: int) -> void:
	if not _active:
		return
	if idx != _next_cp:
		return   # 必须按顺序穿过
	_checkpoints[idx].passed = true
	_checkpoints[idx].node.visible = false
	_checkpoints[idx].node.monitoring = false
	_next_cp += 1
	AudioBus.play_pickup()
	Confetti.burst(get_tree().current_scene, _checkpoints[idx].node.global_position, Color(0.3, 0.9, 0.5))
	if _next_cp >= CHECKPOINT_COUNT:
		_finish_race()
	else:
		_highlight_cp(_next_cp)

func _process(delta: float) -> void:
	if _active:
		_timer += delta
		_timer_label.text = "🏁 %.1f 秒  (%d/%d)" % [_timer, _next_cp, CHECKPOINT_COUNT]

func _finish_race() -> void:
	_active = false
	_timer_label.visible = false
	# 隐藏剩余检查点
	for cp in _checkpoints:
		cp.node.visible = false
		cp.node.monitoring = false
	# 评级
	var grade = "C"
	if _timer < 20: grade = "S"
	elif _timer < 30: grade = "A"
	elif _timer < 45: grade = "B"
	# 最佳时间
	var is_new_record = _timer < _best_time
	if is_new_record:
		_best_time = _timer
		GameState.set_meta("race_best_time", _best_time)
	# 奖励
	var reward = ["apple", "flower", "pearl", "butterfly", "starfish"]
	for i in 3:
		GameState.collect_item(reward[i])
	EventBus.toast_message.emit("竞速完成！%s 级 (%.1fs)%s" % [grade, _timer, " 🏆新纪录!" if is_new_record else ""], "🏁")
	AudioBus.play_mission_complete()
	# S 级额外贴纸
	if grade == "S":
		GameState.earn_sticker("🏁竞速达人")
