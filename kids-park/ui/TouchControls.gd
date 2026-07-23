#============================================================
# TouchControls.gd — 触屏双摇杆 + 大按钮（跳跃/互动）
#============================================================
extends CanvasLayer

const JOYSTICK_MAX: float = 60.0
var _player: CharacterBody3D
var _camera: Camera3D
var _move_touch_id: int = -1
var _look_touch_id: int = -1
var _move_joystick_origin: Vector2
var _stick_bg: Panel
var _stick_knob: Panel

func _ready() -> void:
	if not DisplayServer.is_touchscreen_available():
		queue_free()
		return
	call_deferred("_init_refs")

func _init_refs() -> void:
	_player = get_tree().get_first_node_in_group("player")
	if _player:
		_camera = _player.get_node_or_null("CameraRig/SpringArm3D/Camera3D")
	_build_ui()

func _build_ui() -> void:
	var root = Control.new()
	root.set_anchors_preset(Control.PRESET_FULL_RECT)
	root.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(root)
	var vs = get_viewport().get_visible_rect().size
	# 摇杆
	_stick_bg = Panel.new()
	_stick_bg.position = Vector2(40, vs.y - 160)
	_stick_bg.custom_minimum_size = Vector2(120, 120)
	var bg_style = StyleBoxFlat.new()
	bg_style.bg_color = Color(1, 1, 1, 0.2)
	bg_style.corner_radius_top_left = 60
	bg_style.corner_radius_top_right = 60
	bg_style.corner_radius_bottom_left = 60
	bg_style.corner_radius_bottom_right = 60
	_stick_bg.add_theme_stylebox_override("panel", bg_style)
	root.add_child(_stick_bg)
	_move_joystick_origin = _stick_bg.position + Vector2(60, 60)
	_stick_knob = Panel.new()
	_stick_knob.custom_minimum_size = Vector2(50, 50)
	var knob_style = StyleBoxFlat.new()
	knob_style.bg_color = Color(1, 1, 1, 0.5)
	knob_style.corner_radius_top_left = 25
	knob_style.corner_radius_top_right = 25
	knob_style.corner_radius_bottom_left = 25
	knob_style.corner_radius_bottom_right = 25
	_stick_knob.add_theme_stylebox_override("panel", knob_style)
	_stick_knob.position = _move_joystick_origin - Vector2(25, 25)
	root.add_child(_stick_knob)
	# 跳跃按钮（绿色大圆）
	_add_btn(root, "🦘", Vector2(vs.x - 140, vs.y - 160),
		func(): if _player: _player.touch_jump = true,
		func(): if _player: _player.touch_jump = false)
	# 互动按钮（橙色大圆）
	_add_btn(root, "✨", Vector2(vs.x - 140, vs.y - 270),
		func(): _try_interact())

func _add_btn(parent: Control, text: String, pos: Vector2, on_down: Callable, on_up: Callable = Callable()) -> void:
	var btn = Button.new()
	btn.text = text
	btn.position = pos
	btn.custom_minimum_size = Vector2(90, 90)
	btn.add_theme_font_size_override("font_size", 32)
	btn.button_down.connect(on_down)
	if on_up.is_valid():
		btn.button_up.connect(on_up)
	parent.add_child(btn)

func _try_interact() -> void:
	# 找最近 NPC 并互动
	if _player == null:
		return
	for npc in get_tree().get_nodes_in_group("npc"):
		if npc.has_method("interact") and _player.global_position.distance_to(npc.global_position) < 4.0:
			npc.interact()
			return

func _unhandled_input(event: InputEvent) -> void:
	if event is InputEventScreenTouch:
		if event.pressed:
			if event.position.x < get_viewport().get_visible_rect().size.x * 0.5 and _move_touch_id == -1:
				_move_touch_id = event.index
			elif event.position.x >= get_viewport().get_visible_rect().size.x * 0.5 and _look_touch_id == -1:
				_look_touch_id = event.index
		else:
			if event.index == _move_touch_id:
				_move_touch_id = -1
				if _player:
					_player.touch_move_x = 0
					_player.touch_move_y = 0
				if _stick_knob:
					_stick_knob.position = _move_joystick_origin - Vector2(25, 25)
			elif event.index == _look_touch_id:
				_look_touch_id = -1
	elif event is InputEventScreenDrag:
		if event.index == _move_touch_id:
			var delta = event.position - _move_joystick_origin
			var clamped = delta.limit_length(JOYSTICK_MAX)
			if _stick_knob:
				_stick_knob.position = _move_joystick_origin + clamped - Vector2(25, 25)
			if _player:
				_player.touch_move_x = clamped.x / JOYSTICK_MAX
				_player.touch_move_y = clamped.y / JOYSTICK_MAX
		elif event.index == _look_touch_id:
			if _camera:
				_camera.touch_look_delta += event.relative
