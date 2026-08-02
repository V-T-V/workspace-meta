#============================================================
# Dandelion.gd — 蒲公英（走过触发绒毛飞散粒子）
#============================================================
# 草地/花园装饰：白色绒球 + 绿茎
# 玩家走过碰撞区 → 绒毛爆散成飞舞粒子（3 秒后重生）
# 纯视觉趣味，增强"触碰世界有反应"的沉浸感
#============================================================
extends Area3D

const RESPAWN_TIME: float = 4.0

var _puff: Node3D = null
var _triggered: bool = false
var _respawn_timer: float = 0.0
var _burst_particles: GPUParticles3D = null

func _ready() -> void:
	body_entered.connect(_on_body_entered)
	_build_dandelion()
	# 碰撞区
	var col = CollisionShape3D.new()
	var shape = SphereShape3D.new()
	shape.radius = 0.6
	col.shape = shape
	col.position = Vector3(0, 0.5, 0)
	add_child(col)
	# 预创建爆散粒子（初始不发射）
	_burst_particles = GPUParticles3D.new()
	_burst_particles.amount = 20
	_burst_particles.lifetime = 2.5
	_burst_particles.one_shot = true
	_burst_particles.emitting = false
	_burst_particles.explosiveness = 0.8
	var mat = ParticleProcessMaterial.new()
	mat.direction = Vector3(0, 1, 0)
	mat.spread = 60.0
	mat.initial_velocity_min = 1.0
	mat.initial_velocity_max = 3.0
	mat.gravity = Vector3(0, 0.3, 0)   # 轻微上浮
	mat.turbulence_enabled = true
	mat.turbulence_noise_scale = 2.0
	mat.scale_min = 0.03
	mat.scale_max = 0.06
	mat.color = Color(0.98, 0.98, 0.95, 0.9)
	_burst_particles.process_material = mat
	var puff_mesh = SphereMesh.new()
	puff_mesh.radius = 0.03
	puff_mesh.height = 0.06
	_burst_particles.draw_pass_1 = puff_mesh
	_burst_particles.position = Vector3(0, 0.6, 0)
	add_child(_burst_particles)

func _build_dandelion() -> void:
	var stem_mat = ModelFactory.get_material(Color(0.3, 0.55, 0.2), {"shaded": true})
	var puff_mat = ModelFactory.get_material(Color(0.97, 0.97, 0.93), {"emissive": Color(0.8, 0.8, 0.75), "emissive_energy": 0.2, "shaded": true})
	_puff = Node3D.new()
	add_child(_puff)
	# 茎（细圆柱）
	var stem = CSGCylinder3D.new()
	stem.radius = 0.02; stem.height = 0.5
	stem.position = Vector3(0, 0.25, 0)
	stem.material = stem_mat
	_puff.add_child(stem)
	# 绒球（小球）
	var ball = CSGSphere3D.new()
	ball.radius = 0.12; ball.position = Vector3(0, 0.55, 0)
	ball.material = puff_mat
	ball.name = "PuffBall"
	_puff.add_child(ball)
	# 几根细毛装饰（从球向外辐射）
	for i in 8:
		var a = TAU * i / 8.0
		var hair = CSGCylinder3D.new()
		hair.radius = 0.003; hair.height = 0.15
		hair.position = Vector3(cos(a) * 0.1, 0.55, sin(a) * 0.1)
		hair.rotation_degrees = Vector3(0, rad_to_deg(a), 80)
		hair.material = puff_mat
		_puff.add_child(hair)

func _process(delta: float) -> void:
	if _triggered:
		_respawn_timer -= delta
		if _respawn_timer <= 0:
			# 重生
			_triggered = false
			if _puff:
				_puff.visible = true
			monitoring = true
	else:
		# 风吹微摇
		if _puff:
			var t = Time.get_ticks_msec() * 0.002
			_puff.rotation_degrees.z = sin(t) * 3.0

func _on_body_entered(body: Node) -> void:
	if _triggered or not body.is_in_group("player"):
		return
	_triggered = true
	_respawn_timer = RESPAWN_TIME
	# 绒球消失
	if _puff:
		_puff.visible = false
	monitoring = false
	# 发射绒毛粒子
	_burst_particles.restart()
	_burst_particles.emitting = true
	AudioBus.play_note(800.0 + randf_range(-100, 100), 0.15, 0.08)
