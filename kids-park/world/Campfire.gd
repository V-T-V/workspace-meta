#============================================================
# Campfire.gd — 篝火（休息点 + 区域快速传送）
#============================================================
# 每个区域中心放一个篝火
# 玩家靠近按 F 键打开传送菜单：
#   - 显示所有已解锁区域的篝火
#   - 点击传送（瞬时跳转 + 彩纸）
# 篝火火焰持续跳动 + 火星粒子
#============================================================
extends Area3D

const Confetti = preload("res://world/Confetti.gd")

@export var zone_id: String = "grassland"
var _player_near: bool = false
var _hint: Label3D = null

func _ready() -> void:
	body_entered.connect(_on_body_entered)
	body_exited.connect(_on_body_exited)
	# 火焰跳动（给 Flame 子节点做动画）
	_flame_animation()
	# 交互提示
	_hint = Label3D.new()
	_hint.text = "🔥 按 F 传送"
	_hint.font_size = 28
	_hint.position = Vector3(0, 2.0, 0)
	_hint.billboard = BaseMaterial3D.BILLBOARD_ENABLED
	_hint.outline_size = 6
	_hint.outline_modulate = Color(0, 0, 0, 0.6)
	_hint.visible = false
	add_child(_hint)

func _flame_animation() -> void:
	# 火焰缩放跳动循环
	var flame = get_node_or_null("Flame")
	if flame == null:
		# 查找子节点的 Flame
		for c in get_children():
			if c is CollisionShape3D:
				continue
			flame = c.get_node_or_null("Flame")
			if flame:
				break
	if flame:
		var tw = create_tween().set_loops()
		tw.tween_property(flame, "scale", Vector3(0.9, 1.3, 0.9), 0.15)
		tw.tween_property(flame, "scale", Vector3(1.1, 1.7, 1.1), 0.2)
		tw.tween_property(flame, "scale", Vector3(1.0, 1.5, 1.0), 0.15)

func _process(_delta: float) -> void:
	if _player_near and _hint:
		var t = Time.get_ticks_msec() * 0.003
		_hint.position.y = 2.0 + sin(t) * 0.1

func _on_body_entered(body: Node) -> void:
	if body.is_in_group("player"):
		_player_near = true
		if _hint:
			_hint.visible = true

func _on_body_exited(body: Node) -> void:
	if body.is_in_group("player"):
		_player_near = false
		if _hint:
			_hint.visible = false

func _unhandled_input(event: InputEvent) -> void:
	if event is InputEventKey and event.pressed and event.keycode == KEY_F:
		if _player_near:
			_open_teleport_menu()

func _open_teleport_menu() -> void:
	# 创建传送菜单
	var menu = Control.new()
	menu.set_anchors_preset(Control.PRESET_FULL_RECT)
	var dim = ColorRect.new()
	dim.color = Color(0, 0, 0, 0.5)
	dim.set_anchors_preset(Control.PRESET_FULL_RECT)
	menu.add_child(dim)
	var panel = Panel.new()
	panel.set_anchors_preset(Control.PRESET_CENTER)
	panel.custom_minimum_size = Vector2(350, 320)
	panel.position = Vector2(-175, -160)
	var bg = StyleBoxFlat.new()
	bg.bg_color = Color(0.98, 0.92, 0.8, 0.97)
	bg.corner_radius_top_left = 16
	bg.corner_radius_top_right = 16
	bg.corner_radius_bottom_left = 16
	bg.corner_radius_bottom_right = 16
	bg.border_width_top = 4
	bg.border_color = Color(0.9, 0.5, 0.2)
	panel.add_theme_stylebox_override("panel", bg)
	menu.add_child(panel)
	# 标题
	var title = Label.new()
	title.text = "🔥 篝火传送"
	title.add_theme_font_size_override("font_size", 28)
	title.position = Vector2(20, 15)
	panel.add_child(title)
	# 已解锁区域列表
	var vbox = VBoxContainer.new()
	vbox.position = Vector2(20, 60)
	vbox.custom_minimum_size = Vector2(310, 240)
	vbox.add_theme_constant_override("separation", 10)
	panel.add_child(vbox)
	for zone_id in GameState.ZONES:
		var zdef = GameState.ZONES[zone_id]
		if not GameState.is_zone_unlocked(zone_id):
			continue
		var target_zone = zone_id
		var btn = Button.new()
		btn.text = "%s %s" % [zdef.get("emoji", "🗺️"), zdef.get("name", zone_id)]
		btn.add_theme_font_size_override("font_size", 22)
		btn.custom_minimum_size = Vector2(310, 55)
		btn.pressed.connect(func():
			_teleport(target_zone)
			menu.queue_free()
		)
		vbox.add_child(btn)
	# 关闭按钮
	var close_btn = Button.new()
	close_btn.text = "❌ 关闭"
	close_btn.add_theme_font_size_override("font_size", 20)
	close_btn.custom_minimum_size = Vector2(310, 45)
	close_btn.pressed.connect(func(): menu.queue_free())
	vbox.add_child(close_btn)
	menu.set_process_unhandled_input(true)
	menu.unhandled_input.connect(func(ev):
		if ev is InputEventKey and ev.pressed:
			menu.queue_free()
	)
	get_tree().current_scene.add_child(menu)

func _teleport(target_zone: String) -> void:
	var player = get_tree().get_first_node_in_group("player")
	if player == null:
		return
	var center = ParkGen.ZONE_CENTERS[target_zone]
	player.global_position = center + Vector3(0, 1, 0)
	# 传送彩纸 + 音效
	Confetti.burst(get_tree().current_scene, player.global_position, Color(0.5, 0.9, 1.0))
	AudioBus.play_zone_unlock()
	EventBus.toast_message.emit("传送至 %s！" % GameState.ZONES[target_zone].get("name", target_zone), "🔥")
