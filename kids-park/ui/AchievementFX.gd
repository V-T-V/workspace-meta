#============================================================
# AchievementFX.gd — 成就达成全屏庆祝特效
#============================================================
# 监听 sticker_earned + 成就解锁
# 触发：全屏光芒爆发（从中心向外扩散的金色光环）
#       + 多次彩纸 + 上扬琶音 + 文字弹出动画
#============================================================
extends CanvasLayer

const Confetti = preload("res://world/Confetti.gd")

var _ray: ColorRect
var _text_label: Label
var _active: bool = false

func _ready() -> void:
	_build_ui()
	EventBus.sticker_earned.connect(_on_celebrate)
	# 也监听成就（通过 toast 间接，但 sticker 更明确）

func _build_ui() -> void:
	var root = Control.new()
	root.set_anchors_preset(Control.PRESET_FULL_RECT)
	root.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(root)
	# 光芒遮罩（全屏金色渐变）
	_ray = ColorRect.new()
	_ray.color = Color(1, 0.85, 0.2, 0)
	_ray.set_anchors_preset(Control.PRESET_FULL_RECT)
	_ray.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_ray.visible = false
	root.add_child(_ray)
	# 文字标签（居中弹出）
	_text_label = Label.new()
	_text_label.text = ""
	_text_label.add_theme_font_size_override("font_size", 48)
	_text_label.add_theme_color_override("font_color", Color(1, 0.9, 0.3))
	_text_label.set_anchors_preset(Control.PRESET_CENTER)
	_text_label.position = Vector2(-200, -30)
	_text_label.custom_minimum_size = Vector2(400, 60)
	_text_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_text_label.visible = false
	root.add_child(_text_label)

func _on_celebrate(sticker_name: String) -> void:
	if _active:
		return   # 已在播放中
	_active = true
	_ray.visible = true
	_text_label.visible = true
	_text_label.text = "🎉 %s" % sticker_name
	# 光芒淡入淡出动画
	var tw = create_tween()
	tw.tween_property(_ray, "color:a", 0.5, 0.2)
	tw.tween_property(_ray, "color:a", 0.3, 0.3)
	tw.tween_property(_ray, "color:a", 0.0, 0.5)
	# 文字缩放弹出
	_text_label.scale = Vector2(0.5, 0.5)
	var tw2 = create_tween()
	tw2.tween_property(_text_label, "scale", Vector2(1.2, 1.2), 0.2).set_ease(Tween.EASE_OUT)
	tw2.tween_property(_text_label, "scale", Vector2(1.0, 1.0), 0.15)
	# 多次彩纸爆发（围绕玩家）
	var player = get_tree().get_first_node_in_group("player")
	var center = player.global_position if player else Vector3.ZERO
	for i in 4:
		var delay = i * 0.15
		get_tree().create_timer(delay).timeout.connect(func():
			var angle = randf() * TAU
			var offset = Vector3(cos(angle) * 3, 2, sin(angle) * 3)
			Confetti.burst(get_tree().current_scene, center + offset, Color(1, 0.85, 0.2))
		)
	# 结束后清理
	get_tree().create_timer(1.5).timeout.connect(func():
		_ray.visible = false
		_text_label.visible = false
		_active = false
	)
