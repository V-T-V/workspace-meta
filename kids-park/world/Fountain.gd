#============================================================
# Fountain.gd — 喷泉装饰（粒子水柱+水花飞溅）
#============================================================
# 中心广场装饰：圆形水池 + 中央水柱粒子向上喷射 + 落水涟漪
# 纯视觉装饰，增强场景生动感
#============================================================
extends Node3D

var _water_particles: GPUParticles3D
var _splash_particles: GPUParticles3D

func _ready() -> void:
	_build_structure()
	_build_water()

func _build_structure() -> void:
	var stone_mat = ModelFactory.get_material(Color(0.6, 0.58, 0.55), {"shaded": true})
	var water_mat = ModelFactory.get_material(Color(0.2, 0.5, 0.8), {"metallic": 0.8, "roughness": 0.1, "emissive": Color(0.1, 0.3, 0.5), "emissive_energy": 0.3})
	# 外圈石壁（扁圆柱）
	var basin = CSGCylinder3D.new()
	basin.radius = 2.5; basin.height = 0.6
	basin.position = Vector3(0, 0.3, 0)
	basin.material = stone_mat
	add_child(basin)
	# 内圈水面（蓝色扁圆柱）
	var water = CSGCylinder3D.new()
	water.radius = 2.3; water.height = 0.4
	water.position = Vector3(0, 0.35, 0)
	water.material = water_mat
	add_child(water)
	# 中央柱子
	var pillar = CSGCylinder3D.new()
	pillar.radius = 0.3; pillar.height = 1.5
	pillar.position = Vector3(0, 1.0, 0)
	pillar.material = stone_mat
	add_child(pillar)
	# 柱顶装饰球
	var top = CSGSphere3D.new()
	top.radius = 0.4; top.position = Vector3(0, 1.9, 0)
	top.material = stone_mat
	add_child(top)
	# 池边装饰小球（一圈）
	for i in 8:
		var a = TAU * i / 8.0
		var dot = CSGSphere3D.new()
		dot.radius = 0.15
		dot.position = Vector3(cos(a) * 2.5, 0.7, sin(a) * 2.5)
		dot.material = stone_mat
		add_child(dot)

func _build_water() -> void:
	# 主水柱粒子（从柱顶向上喷射）
	_water_particles = GPUParticles3D.new()
	_water_particles.amount = 40
	_water_particles.lifetime = 1.5
	_water_particles.explosiveness = 0.0
	_water_particles.position = Vector3(0, 2.0, 0)
	var wmat = ParticleProcessMaterial.new()
	wmat.direction = Vector3(0, 1, 0)
	wmat.spread = 15.0
	wmat.initial_velocity_min = 4.0
	wmat.initial_velocity_max = 7.0
	wmat.gravity = Vector3(0, -12, 0)
	wmat.scale_min = 0.05
	wmat.scale_max = 0.12
	wmat.color = Color(0.4, 0.7, 1.0, 0.8)
	_water_particles.process_material = wmat
	var drop_mesh = SphereMesh.new()
	drop_mesh.radius = 0.06
	drop_mesh.height = 0.12
	_water_particles.draw_pass_1 = drop_mesh
	add_child(_water_particles)
	# 水花飞溅粒子（落水点小水花）
	_splash_particles = GPUParticles3D.new()
	_splash_particles.amount = 20
	_splash_particles.lifetime = 0.5
	_splash_particles.explosiveness = 0.5
	_splash_particles.position = Vector3(0, 0.4, 0)
	var smat = ParticleProcessMaterial.new()
	smat.direction = Vector3(0, 1, 0)
	smat.spread = 60.0
	smat.initial_velocity_min = 1.0
	smat.initial_velocity_max = 3.0
	smat.gravity = Vector3(0, -8, 0)
	smat.scale_min = 0.03
	smat.scale_max = 0.08
	smat.color = Color(0.6, 0.85, 1.0, 0.6)
	_splash_particles.process_material = smat
	var splash_mesh = SphereMesh.new()
	splash_mesh.radius = 0.04
	splash_mesh.height = 0.08
	_splash_particles.draw_pass_1 = splash_mesh
	add_child(_splash_particles)

func _process(delta: float) -> void:
	# 水柱随机抖动（模拟水流不规则）
	var t = Time.get_ticks_msec() * 0.001
	_water_particles.position.x = sin(t * 3) * 0.1
	_water_particles.position.z = cos(t * 2.5) * 0.1
