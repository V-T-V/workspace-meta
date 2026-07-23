#============================================================
# PlayerCamera.gd — 第三人称跟随相机（收窄 pitch 防儿童眩晕）
#============================================================
extends Camera3D

const PITCH_LIMIT: float = 0.8   # 比 city-hunt 窄（1.3→0.8），防止儿童低头眩晕
const MOUSE_SENS: float = 0.003
const TOUCH_SENS: float = 0.005

var _player: CharacterBody3D
var _spring: SpringArm3D
var _cam_rig: Node3D
var touch_look_delta: Vector2 = Vector2.ZERO
var _mouse_captured: bool = false

func _ready() -> void:
	_spring = get_parent() as SpringArm3D
	_cam_rig = _spring.get_parent() if _spring else null
	_player = _cam_rig.get_parent() if _cam_rig else null
	if not DisplayServer.is_touchscreen_available():
		_capture_mouse()

func _input(event: InputEvent) -> void:
	if event is InputEventMouseMotion and _mouse_captured:
		_apply_look(event.relative.x * MOUSE_SENS, event.relative.y * MOUSE_SENS)
	if event is InputEventKey and event.pressed and event.keycode == KEY_ESCAPE:
		_mouse_captured = not _mouse_captured
		Input.mouse_mode = Input.MOUSE_MODE_CAPTURED if _mouse_captured else Input.MOUSE_MODE_VISIBLE

func _physics_process(_delta: float) -> void:
	if touch_look_delta != Vector2.ZERO:
		_apply_look(touch_look_delta.x * TOUCH_SENS, touch_look_delta.y * TOUCH_SENS)
		touch_look_delta = Vector2.ZERO
	if _cam_rig:
		_cam_rig.rotation.y = _player.yaw if _player else 0.0
	if _spring:
		_spring.rotation.x = _player.pitch if _player else 0.0

func _apply_look(dx: float, dy: float) -> void:
	if _player == null:
		return
	_player.yaw -= dx
	_player.pitch -= dy
	_player.pitch = clamp(_player.pitch, -PITCH_LIMIT, PITCH_LIMIT)

func _capture_mouse() -> void:
	_mouse_captured = true
	Input.mouse_mode = Input.MOUSE_MODE_CAPTURED
