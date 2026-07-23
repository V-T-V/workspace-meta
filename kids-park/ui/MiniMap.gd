#============================================================
# MiniMap.gd — 小地图（右上角俯视，显示区域/玩家/NPC/收集物）
#============================================================
# 用 SubViewport + Camera3D 俯视渲染（无需画布坐标转换，真实 3D 视角）
# 儿童友好：大色块区域 + 醒目玩家箭头 + NPC 圆点
#============================================================
extends CanvasLayer

var _viewport: SubViewport
var _mini_cam: Camera3D
var _player: CharacterBody3D
var _root: Node3D   # 主场景根（用于把 minimap 相机挂到世界）

func _ready() -> void:
	call_deferred("_init_minimap")

func _init_minimap() -> void:
	_player = get_tree().get_first_node_in_group("player")
	_root = get_tree().current_scene
	if _player == null or _root == null:
		return
	# SubViewport（独立渲染，不占主屏）
	_viewport = SubViewport.new()
	_viewport.size = Vector2i(180, 180)
	_viewport.render_target_update_mode = SubViewport.UPDATE_ALWAYS
	_viewport.transparent_bg = true
	# 小地图相机（俯视）
	_mini_cam = Camera3D.new()
	_mini_cam.size = 90.0            # 正交范围（覆盖约 90 米见方）
	_mini_cam.projection = Camera3D.PROJECTION_ORTHOGONAL
	_mini_cam.position = Vector3(_player.global_position.x, 60, _player.global_position.z)
	_mini_cam.rotation_degrees = Vector3(-90, 0, 0)   # 朝下
	_mini_cam.cull_mask = 0x7FFFFFFF  # 看到所有层
	_viewport.add_child(_mini_cam)
	_root.add_child(_viewport)
	# UI：右上角显示 viewport 纹理
	var screen := Control.new()
	screen.set_anchors_preset(Control.PRESET_FULL_RECT)
	screen.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(screen)
	var rect := TextureRect.new()
	rect.texture = _viewport.get_texture()
	rect.custom_minimum_size = Vector2(180, 180)
	rect.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
	# 右上角定位
	rect.set_anchors_preset(Control.PRESET_TOP_RIGHT)
	rect.position = Vector2(-190, 20)
	# 圆角边框（儿童友好暖色）
	var border := StyleBoxFlat.new()
	border.bg_color = Color(1, 1, 1, 0.15)
	border.border_width_left = 3
	border.border_width_right = 3
	border.border_width_top = 3
	border.border_width_bottom = 3
	border.border_color = Color(0.95, 0.8, 0.3, 0.9)
	border.corner_radius_top_left = 12
	border.corner_radius_top_right = 12
	border.corner_radius_bottom_left = 12
	border.corner_radius_bottom_right = 12
	border.expand_margin_left = 4
	border.expand_margin_right = 4
	border.expand_margin_top = 4
	border.expand_margin_bottom = 4
	rect.add_theme_stylebox_override("texture", border)
	screen.add_child(rect)

func _process(_delta: float) -> void:
	if _mini_cam and _player and is_instance_valid(_player):
		# 相机跟随玩家 XZ
		_mini_cam.position.x = _player.global_position.x
		_mini_cam.position.z = _player.global_position.z
