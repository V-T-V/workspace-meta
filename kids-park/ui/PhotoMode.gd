#============================================================
# PhotoMode.gd — 照相模式（拍照 + 相册浏览）
#============================================================
# 按 P 键进入拍照模式：
#   - 隐藏 HUD/触屏控件
#   - 显示取景框 + 拍照按钮
#   - 按 P 或点击快门拍照
#   - 照片保存到 user://photos/ 目录
# 按 V 键打开相册浏览已拍照片
# 儿童友好：大快门按钮 + 闪光动画 + 拍照音效
#============================================================
extends CanvasLayer

var _frame: Control         # 取景框 UI
var _shutter_btn: Button     # 快门按钮
var _active: bool = false    # 是否在拍照模式
var _flash: ColorRect        # 闪光白屏
var _photo_count: int = 0

func _ready() -> void:
	_build_ui()
	# 确保照片目录存在
	var dir = DirAccess.open("user://")
	if dir and not dir.dir_exists("photos"):
		dir.make_dir("photos")
	# 统计已有照片
	var photos = _list_photos()
	_photo_count = photos.size()

func _build_ui() -> void:
	var root = Control.new()
	root.set_anchors_preset(Control.PRESET_FULL_RECT)
	root.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(root)
	# 取景框（四角 L 形装饰）
	_frame = Control.new()
	_frame.set_anchors_preset(Control.PRESET_FULL_RECT)
	_frame.visible = false
	_frame.mouse_filter = Control.MOUSE_FILTER_IGNORE
	root.add_child(_frame)
	# 四角装饰线
	for corner in [["tl", 0, 0], ["tr", 1, 0], ["bl", 0, 1], ["br", 1, 1]]:
		var line_h = ColorRect.new()
		line_h.color = Color(1, 0.9, 0.3, 0.8)
		line_h.size = Vector2(60, 4)
		var ax: int = corner[1]
		var ay: int = corner[2]
		var px = 20 if ax == 0 else (1280 - 60 - 20)
		var py = 40 if ay == 0 else (720 - 80)
		line_h.position = Vector2(px, py)
		_frame.add_child(line_h)
		var line_v = ColorRect.new()
		line_v.color = Color(1, 0.9, 0.3, 0.8)
		line_v.size = Vector2(4, 60)
		var vpx = 20 if ax == 0 else (1280 - 24)
		var vpy = 40 if ay == 0 else (720 - 60 - 40)
		line_v.position = Vector2(vpx, vpy)
		_frame.add_child(line_v)
	# 快门按钮（底部中央大圆）
	_shutter_btn = Button.new()
	_shutter_btn.text = "📸"
	_shutter_btn.add_theme_font_size_override("font_size", 40)
	_shutter_btn.custom_minimum_size = Vector2(100, 100)
	_shutter_btn.set_anchors_preset(Control.PRESET_CENTER_BOTTOM)
	_shutter_btn.position = Vector2(-50, -140)
	_shutter_btn.visible = false
	_shutter_btn.pressed.connect(_take_photo)
	_frame.add_child(_shutter_btn)
	# 提示文字
	var hint = Label.new()
	hint.text = "按 P 拍照 · 再按 P 退出"
	hint.add_theme_font_size_override("font_size", 18)
	hint.add_theme_color_override("font_color", Color(1, 0.9, 0.3))
	hint.set_anchors_preset(Control.PRESET_CENTER_TOP)
	hint.position = Vector2(-100, 20)
	_frame.add_child(hint)
	# 闪光白屏
	_flash = ColorRect.new()
	_flash.color = Color(1, 1, 1, 0)
	_flash.set_anchors_preset(Control.PRESET_FULL_RECT)
	_flash.mouse_filter = Control.MOUSE_FILTER_IGNORE
	root.add_child(_flash)

func _unhandled_input(event: InputEvent) -> void:
	if event is InputEventKey and event.pressed:
		# P 键切换拍照模式
		if event.keycode == KEY_P:
			_toggle_photo_mode()
		# V 键打开相册
		elif event.keycode == KEY_V:
			_show_album()

func _toggle_photo_mode() -> void:
	_active = not _active
	_frame.visible = _active
	# 隐藏/显示其他 UI 层
	for layer_name in ["HUD", "TouchControls", "MiniMap", "BooksUI"]:
		var layer = get_tree().current_scene.get_node_or_null(layer_name)
		if layer:
			layer.visible = not _active

func _take_photo() -> void:
	# 闪光动画
	var tw = create_tween()
	tw.tween_property(_flash, "color:a", 0.9, 0.05)
	tw.tween_property(_flash, "color:a", 0.0, 0.3)
	# 拍照音效
	AudioBus.play_pickup()
	# 截图保存
	await RenderingServer.frame_post_draw
	var img = get_viewport().get_texture().get_image()
	if img:
		_photo_count += 1
		var filename = "user://photos/photo_%03d.png" % _photo_count
		img.save_png(filename)
		EventBus.toast_message.emit("照片已保存！(%d)" % _photo_count, "📸")

func _list_photos() -> Array:
	var photos: Array = []
	var dir = DirAccess.open("user://photos")
	if dir:
		dir.list_dir_begin()
		var fname = dir.get_next()
		while fname != "":
			if fname.ends_with(".png"):
				photos.append("user://photos/" + fname)
			fname = dir.get_next()
	photos.sort()
	return photos

func _show_album() -> void:
	var photos = _list_photos()
	if photos.is_empty():
		EventBus.toast_message.emit("还没有照片，按 P 拍第一张！", "📸")
		return
	# 简易相册：显示最后一张照片 + 左右切换
	var album = Control.new()
	album.set_anchors_preset(Control.PRESET_FULL_RECT)
	var bg = ColorRect.new()
	bg.color = Color(0, 0, 0, 0.85)
	bg.set_anchors_preset(Control.PRESET_FULL_RECT)
	album.add_child(bg)
	get_tree().current_scene.add_child(album)
	# 加载最后一张照片预览
	var tex = load(photos[-1])
	if tex:
		var preview = TextureRect.new()
		preview.texture = tex
		preview.set_anchors_preset(Control.PRESET_CENTER)
		preview.custom_minimum_size = Vector2(640, 360)
		preview.position = Vector2(-320, -180)
		album.add_child(preview)
	# 关闭提示
	var close_hint = Label.new()
	close_hint.text = "共 %d 张照片 · 按任意键关闭" % photos.size()
	close_hint.add_theme_font_size_override("font_size", 22)
	close_hint.add_theme_color_override("font_color", Color(1, 0.9, 0.3))
	close_hint.set_anchors_preset(Control.PRESET_CENTER_BOTTOM)
	close_hint.position = Vector2(-150, -60)
	album.add_child(close_hint)
	# 任意键关闭
	album.set_process_unhandled_input(true)
	album.unhandled_input.connect(func(ev):
		if ev is InputEventKey and ev.pressed:
			album.queue_free()
	)
