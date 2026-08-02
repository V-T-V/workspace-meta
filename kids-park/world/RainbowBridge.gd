#============================================================
# RainbowBridge.gd — 彩虹桥（区域间彩色拱桥）
#============================================================
# 连接两个区域的半圆拱桥，7 色彩虹条纹
# 玩家走上桥时桥身微微发光（增强仪式感）
# 站在桥顶可以看到两侧区域全貌（观景台效果）
#============================================================
extends Area3D

const BRIDGE_RADIUS: float = 4.0   # 拱桥半径

var _bridge: Node3D = null
var _player_on: bool = false

func _ready() -> void:
	body_entered.connect(_on_body_entered)
	body_exited.connect(_on_body_exited)
	_bridge = _build_bridge()
	add_child(_bridge)
	# 碰撞区
	var col = CollisionShape3D.new()
	var shape = BoxShape3D.new()
	shape.size = Vector3(3.0, 1.0, BRIDGE_RADIUS * 2.5)
	col.shape = shape
	col.position = Vector3(0, BRIDGE_RADIUS * 0.5, 0)
	add_child(col)

func _build_bridge() -> Node3D:
	var node = Node3D.new()
	# 7 色彩虹（每色一段拱形 = 压扁的圆柱段）
	var rainbow_colors = [
		Color(0.9, 0.2, 0.2),   # 红
		Color(0.95, 0.5, 0.15), # 橙
		Color(0.95, 0.85, 0.2), # 黄
		Color(0.3, 0.8, 0.35),  # 绿
		Color(0.3, 0.6, 0.95),  # 蓝
		Color(0.4, 0.3, 0.9),   # 靛
		Color(0.7, 0.3, 0.85),  # 紫
	]
	for i in rainbow_colors.size():
		var arc = CSGCylinder3D.new()
		arc.radius = BRIDGE_RADIUS + i * 0.15
		arc.height = 0.6
		arc.position = Vector3(0, 0.3, 0)
		# 只保留上半部分（压扁+旋转成拱）
		arc.rotation_degrees = Vector3(90, 0, 0)
		arc.scale = Vector3(1, 0.5, 1)
		var mat = ModelFactory.get_material(rainbow_colors[i], {
			"emissive": rainbow_colors[i],
			"emissive_energy": 0.3,
			"shaded": true,
		})
		arc.material = mat
		arc.name = "Arc%d" % i
		node.add_child(arc)
	# 桥面（白色地砖）
	var path = CSGBox3D.new()
	path.size = Vector3(2.0, 0.1, BRIDGE_RADIUS * 2)
	path.position = Vector3(0, 0.05, 0)
	var pmat = ModelFactory.get_material(Color(0.95, 0.95, 0.9), {"emissive": Color(0.8, 0.8, 0.75), "emissive_energy": 0.1})
	path.material = pmat
	node.add_child(path)
	# 桥顶装饰灯
	var light = OmniLight3D.new()
	light.position = Vector3(0, 1.5, 0)
	light.light_color = Color(0.9, 0.9, 1.0)
	light.light_energy = 0.5
	light.omni_range = 5.0
	node.add_child(light)
	return node

func _process(_delta: float) -> void:
	if _player_on:
		# 玩家在桥上时，彩虹发光增强
		var t = Time.get_ticks_msec() * 0.002
		for i in 7:
			var arc = _bridge.get_node_or_null("Arc%d" % i)
			if arc and arc.material_override is StandardMaterial3D:
				arc.material_override.emissive_energy_multiplier = 0.3 + sin(t + i * 0.5) * 0.3

func _on_body_entered(body: Node) -> void:
	if body.is_in_group("player"):
		_player_on = true
		EventBus.toast_message.emit("彩虹桥！", "🌈")

func _on_body_exited(body: Node) -> void:
	if body.is_in_group("player"):
		_player_on = false
