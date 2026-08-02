#============================================================
# MusicPad.gd — 音乐垫（踩上去发不同音符）
#============================================================
# 每个区域放一组 8 个彩色垫子（排成圆形或直线）
# 踩上去发 C 大调音阶（Do Re Mi Fa Sol La Ti Do）
# 儿童可以自由"演奏"音乐
#============================================================
extends Area3D

const Confetti = preload("res://world/Confetti.gd")

@export var note_freq: float = 261.6   # C4 默认
@export var pad_color: Color = Color(0.9, 0.3, 0.3)
var _cooldown: float = 0.0
var _visual: Node3D = null

func _ready() -> void:
	body_entered.connect(_on_body_entered)
	_visual = _build_visual()
	add_child(_visual)
	# 碰撞
	var col = CollisionShape3D.new()
	var shape = BoxShape3D.new()
	shape.size = Vector3(1.2, 0.2, 1.2)
	col.shape = shape
	col.position = Vector3(0, 0.1, 0)
	add_child(col)

func _build_visual() -> Node3D:
	var node = Node3D.new()
	# 垫子主体（扁平方块）
	var pad = CSGBox3D.new()
	pad.size = Vector3(1.1, 0.15, 1.1)
	pad.position = Vector3(0, 0.075, 0)
	var mat = ModelFactory.get_material(pad_color, {"emissive": pad_color, "emissive_energy": 0.3, "shaded": true})
	pad.material = mat
	pad.name = "Pad"
	node.add_child(pad)
	# 音符标记（顶部小白点 = 音符位置）
	var dot = CSGSphere3D.new()
	dot.radius = 0.1
	dot.position = Vector3(0, 0.2, 0)
	var dmat = ModelFactory.get_material(Color(0.95, 0.95, 0.9), {"emissive": Color.WHITE, "emissive_energy": 0.3})
	dot.material = dmat
	node.add_child(dot)
	return node

func _process(delta: float) -> void:
	if _cooldown > 0:
		_cooldown -= delta
		# 恢复垫子高度
		if _visual:
			var pad = _visual.get_node_or_null("Pad")
			if pad:
				pad.position.y = lerp(pad.position.y, 0.075, delta * 8.0)

func _on_body_entered(body: Node) -> void:
	if _cooldown > 0:
		return
	if not body.is_in_group("player"):
		return
	_cooldown = 0.3
	# 播放音符
	AudioBus.play_note(note_freq, 0.4, 0.2)
	# 通知音乐录制器
	var recorder = get_tree().current_scene.get_node_or_null("MusicRecorder")
	if recorder and recorder.has_method("record_note"):
		recorder.record_note(note_freq)
	# 垫子按下动画
	if _visual:
		var pad = _visual.get_node_or_null("Pad")
		if pad:
			pad.position.y = 0.03   # 按下
	# 彩纸
	Confetti.burst(get_tree().current_scene, global_position + Vector3(0, 0.5, 0), pad_color)
