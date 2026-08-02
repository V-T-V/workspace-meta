#============================================================
# PhotoSpot.gd — 打卡拍照点（景点牌，拍照获贴纸）
#============================================================
# 区域特色景点的指示牌 + 装饰框
# 玩家进入拍照模式（P 键）且在此区域 → 自动判定"打卡成功"
# 每个景点打卡一次，获得专属贴纸
# 4 个景点全部打卡 → "📷摄影大师"成就
#============================================================
extends Area3D

const Confetti = preload("res://world/Confetti.gd")

@export var spot_name: String = "草地乐园"
@export var spot_emoji: String = "🌱"
@export var sticker_id: String = "📷草地打卡"

var _player_near: bool = false
var _photo_taken: bool = false
var _visual: Node3D = null
var _hint: Label3D = null

func _ready() -> void:
	add_to_group("photo_spot")
	body_entered.connect(_on_body_entered)
	body_exited.connect(_on_body_exited)
	# 已打卡则标记
	if GameState.stickers.has(sticker_id):
		_photo_taken = true
	_build_sign()
	# 碰撞区
	var col = CollisionShape3D.new()
	var shape = BoxShape3D.new()
	shape.size = Vector3(3.0, 2.0, 1.0)
	col.shape = shape
	col.position = Vector3(0, 1.0, 0)
	add_child(col)
	# 提示
	_hint = Label3D.new()
	_hint.font_size = 26
	_hint.position = Vector3(0, 3.0, 0)
	_hint.billboard = BaseMaterial3D.BILLBOARD_ENABLED
	_hint.outline_size = 6
	_hint.outline_modulate = Color(0, 0, 0, 0.6)
	_hint.visible = false
	_update_hint()
	add_child(_hint)
	# 监听拍照
	EventBus.item_collected.connect(_check_photo)   # 复用：拾取时检查
	# 更直接：监听 PhotoMode 的截图（通过 toast 间接）

func _build_sign() -> void:
	_visual = Node3D.new()
	add_child(_visual)
	var post_mat = ModelFactory.get_material(Color(0.45, 0.3, 0.15), {"shaded": true})
	var board_mat = ModelFactory.get_material(Color(0.95, 0.9, 0.75), {"emissive": Color(0.5, 0.45, 0.3), "emissive_energy": 0.15, "shaded": true})
	var frame_mat = ModelFactory.get_material(Color(0.9, 0.7, 0.2), {"metallic": 0.5, "roughness": 0.4})
	# 立柱
	var post = CSGCylinder3D.new()
	post.radius = 0.06; post.height = 2.5
	post.position = Vector3(0, 1.25, 0)
	post.material = post_mat
	_visual.add_child(post)
	# 指示牌（木板）
	var board = CSGBox3D.new()
	board.size = Vector3(1.8, 1.0, 0.08)
	board.position = Vector3(0, 2.3, 0)
	board.material = board_mat
	_visual.add_child(board)
	# 金色边框
	var frame = CSGBox3D.new()
	frame.size = Vector3(1.9, 1.1, 0.04)
	frame.position = Vector3(0, 2.3, -0.02)
	frame.material = frame_mat
	_visual.add_child(frame)
	# 景点文字（Label3D）
	var label = Label3D.new()
	label.text = "%s\n%s" % [spot_emoji, spot_name]
	label.font_size = 44
	label.position = Vector3(0, 2.3, 0.06)
	label.outline_size = 6
	label.outline_modulate = Color(0, 0, 0, 0.5)
	_visual.add_child(label)
	# 打卡标记（已拍照显示 ✅）
	var mark = Label3D.new()
	mark.text = "✅ 已打卡" if _photo_taken else "📷 打卡点"
	mark.font_size = 22
	mark.position = Vector3(0, 1.7, 0.06)
	mark.modulate = Color(0.3, 0.7, 0.3) if _photo_taken else Color(0.9, 0.7, 0.2)
	mark.name = "PhotoMark"
	_visual.add_child(mark)

func _update_hint() -> void:
	if _photo_taken:
		_hint.text = "✅ 已打卡"
		_hint.modulate = Color(0.4, 0.8, 0.4)
	else:
		_hint.text = "按 P 拍照打卡！"
		_hint.modulate = Color(0.9, 0.7, 0.2)

func _process(_delta: float) -> void:
	if _player_near:
		var t = Time.get_ticks_msec() * 0.003
		_hint.position.y = 3.0 + sin(t) * 0.1

func _on_body_entered(body: Node) -> void:
	if body.is_in_group("player"):
		_player_near = true
		_hint.visible = true
		if not _photo_taken:
			EventBus.toast_message.emit("拍照打卡点！按 P 拍照", spot_emoji)

func _on_body_exited(body: Node) -> void:
	if body.is_in_group("player"):
		_player_near = false
		_hint.visible = false

func _check_photo(_item: String, _count: int) -> void:
	# 玩家在景点附近时，任何收集动作视为"拍照"（简化判定）
	# 真正的拍照判定通过 P 键触发，这里做兜底
	if _player_near and not _photo_taken:
		# 检查是否在拍照模式（通过 PhotoMode 节点状态）
		var photo_mode = get_tree().current_scene.get_node_or_null("PhotoMode")
		if photo_mode and photo_mode.get("_active"):
			_complete_check_in()

func _on_photo_key_pressed() -> void:
	# 由 PhotoMode 调用（P 键拍照时）
	if _player_near and not _photo_taken:
		_complete_check_in()

func _complete_check_in() -> void:
	_photo_taken = true
	GameState.earn_sticker(sticker_id)
	EventBus.toast_message.emit("打卡成功！%s" % spot_name, spot_emoji)
	AudioBus.play_sticker()
	Confetti.burst(get_tree().current_scene, global_position + Vector3(0, 2, 0), Color(0.9, 0.8, 0.3))
	_update_hint()
	# 更新牌子上的标记
	var mark = _visual.get_node_or_null("PhotoMark")
	if mark:
		mark.text = "✅ 已打卡"
		mark.modulate = Color(0.3, 0.7, 0.3)
	# 检查全部打卡
	var count = 0
	for spot in get_tree().get_nodes_in_group("photo_spot"):
		if spot._photo_taken:
			count += 1
	if count >= 4:
		GameState.earn_sticker("📷摄影大师")
		EventBus.toast_message.emit("全部景点打卡！摄影大师！", "📷")
		AudioBus.play_zone_unlock()
