#============================================================
# Windmill.gd — 风车装饰（叶片旋转+随风变速）
#============================================================
# 荷兰风车造型：石塔身 + 4 叶片旋转
# 叶片旋转速度随天气/时间随机变化（模拟风）
# 纯视觉装饰，每区域 1-2 个
#============================================================
extends Node3D

var _blades: Node3D = null
var _rotate_speed: float = 1.0
var _speed_timer: float = 0.0

func _ready() -> void:
	_build_windmill()

func _build_windmill() -> void:
	var body_mat = ModelFactory.get_material(Color(0.75, 0.65, 0.5), {"shaded": true})
	var roof_mat = ModelFactory.get_material(Color(0.5, 0.3, 0.2), {"shaded": true})
	var blade_mat = ModelFactory.get_material(Color(0.95, 0.92, 0.85), {"shaded": true})
	var blade_trim = ModelFactory.get_material(Color(0.8, 0.2, 0.2), {"emissive": Color(0.3, 0.05, 0.05), "emissive_energy": 0.1})
	# 塔身（底大顶小=用 scale 模拟圆锥，CSGCylinder 只有单一 radius）
	var tower = CSGCylinder3D.new()
	tower.radius = 0.8; tower.height = 3.0
	tower.position = Vector3(0, 1.5, 0)
	tower.scale = Vector3(1.2, 1, 1.2)   # 底部更宽（通过缩放近似锥形）
	tower.material = body_mat
	add_child(tower)
	# 塔顶圆锥屋顶（用 cone=true 的圆柱）
	var roof = CSGCylinder3D.new()
	roof.radius = 0.7; roof.height = 0.8
	roof.cone = true   # 顶部收缩到一点
	roof.position = Vector3(0, 3.4, 0)
	roof.material = roof_mat
	add_child(roof)
	# 小窗户
	var window1 = CSGBox3D.new()
	window1.size = Vector3(0.3, 0.4, 0.05)
	window1.position = Vector3(0, 1.8, 1.0)
	var wmat = ModelFactory.get_material(Color(0.2, 0.3, 0.4), {"emissive": Color(0.3, 0.4, 0.5), "emissive_energy": 0.3})
	window1.material = wmat
	add_child(window1)
	# 门
	var door = CSGBox3D.new()
	door.size = Vector3(0.5, 0.8, 0.05)
	door.position = Vector3(0, 0.4, 1.0)
	door.material = roof_mat
	add_child(door)
	# 叶片轴心（在前方突出）
	var hub = CSGSphere3D.new()
	hub.radius = 0.15; hub.position = Vector3(0, 2.8, 0.8)
	hub.material = roof_mat
	add_child(hub)
	# 叶片旋转节点
	_blades = Node3D.new()
	_blades.position = Vector3(0, 2.8, 0.85)
	add_child(_blades)
	# 4 片大叶片（十字形）
	for i in 4:
		var angle = TAU * i / 4.0
		var blade_holder = Node3D.new()
		blade_holder.rotation_degrees.z = rad_to_deg(angle)
		_blades.add_child(blade_holder)
		# 叶片主体（白色长条）
		var blade = CSGBox3D.new()
		blade.size = Vector3(0.15, 1.5, 0.03)
		blade.position = Vector3(0, 0.8, 0)
		blade.material = blade_mat
		blade_holder.add_child(blade)
		# 叶片边缘红色装饰条
		var trim = CSGBox3D.new()
		trim.size = Vector3(0.17, 0.1, 0.04)
		trim.position = Vector3(0, 1.5, 0)
		trim.material = blade_trim
		blade_holder.add_child(trim)

func _process(delta: float) -> void:
	# 叶片旋转
	if _blades:
		_blades.rotation_degrees.z += delta * _rotate_speed * 60.0
	# 风速变化（每 5-10 秒切换）
	_speed_timer -= delta
	if _speed_timer <= 0:
		_speed_timer = randf_range(5.0, 10.0)
		_rotate_speed = randf_range(0.5, 2.0)
