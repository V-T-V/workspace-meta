#============================================================
# Weather.gd — 天气系统（雨/雪/晴/花瓣 粒子）
#============================================================
# 跟随玩家头顶生成天气粒子，根据所在区域自动切换：
#   草地：晴天蝴蝶飘飞 / 花园：花瓣飞舞
#   沙滩：细雨 / 冰雪：雪花
# 每隔一段时间随机变化，增加沉浸感
#============================================================
extends Node3D

const WEATHER_CHANGE_INTERVAL: float = 60.0  # 每 60 秒可能切换天气

var _particles: GPUParticles3D
var _player: CharacterBody3D = null
var _current_zone: String = ""
var _timer: float = 0.0
var _weather_active: bool = false

func _ready() -> void:
	# 创建跟随粒子系统
	_particles = GPUParticles3D.new()
	_particles.amount = 60
	_particles.lifetime = 3.0
	_particles.visibility_aabb = AABB(Vector3(-30, -10, -30), Vector3(60, 40, 60))
	_particles.explosiveness = 0.0
	_particles.randomness = 1.0
	# 初始无粒子
	_particles.draw_pass_1 = null
	add_child(_particles)
	_particles.emitting = false

func _process(delta: float) -> void:
	if _player == null or not is_instance_valid(_player):
		_player = get_tree().get_first_node_in_group("player")
		if _player == null:
			return
	# 跟随玩家头顶
	_particles.global_position = _player.global_position + Vector3(0, 15, 0)
	# 定时切换天气
	_timer += delta
	if _timer >= WEATHER_CHANGE_INTERVAL:
		_timer = 0.0
		_maybe_change_weather()

func set_zone_weather(zone: String) -> void:
	if zone == _current_zone:
		return
	_current_zone = zone
	_apply_zone_weather()

func _apply_zone_weather() -> void:
	match _current_zone:
		"grassland":
			_set_weather("sunny", Color(1, 0.95, 0.5), 30)   # 阳光金粉
		"beach":
			_set_weather("rain", Color(0.5, 0.7, 1.0), 80)   # 细雨
		"garden":
			_set_weather("petals", Color(1, 0.5, 0.7), 40)   # 花瓣
		"ice":
			_set_weather("snow", Color(1, 1, 1), 100)        # 雪花
		_:
			_clear_weather()

func _set_weather(type: String, color: Color, amount: int) -> void:
	_weather_active = true
	_particles.amount = amount
	_particles.emitting = true
	var mat := ParticleProcessMaterial.new()
	mat.color = color
	match type:
		"rain":
			mat.direction = Vector3(0.1, -1, 0)
			mat.spread = 10.0
			mat.initial_velocity_min = 15.0
			mat.initial_velocity_max = 20.0
			mat.gravity = Vector3(0, -5, 0)
			mat.scale_min = 0.02
			mat.scale_max = 0.04
			var rain_mesh = PlaneMesh.new()
			rain_mesh.size = Vector2(0.03, 0.3)
			_particles.draw_pass_1 = rain_mesh
		"snow":
			mat.direction = Vector3(0, -1, 0)
			mat.spread = 30.0
			mat.initial_velocity_min = 1.0
			mat.initial_velocity_max = 3.0
			mat.gravity = Vector3(0, -1, 0)
			mat.turbulence_enabled = true
			mat.turbulence_noise_scale = 1.0
			mat.scale_min = 0.05
			mat.scale_max = 0.12
			var snow_mesh = SphereMesh.new()
			snow_mesh.radius = 0.05
			snow_mesh.height = 0.1
			_particles.draw_pass_1 = snow_mesh
		"petals":
			mat.direction = Vector3(0.5, -0.5, 0.3)
			mat.spread = 60.0
			mat.initial_velocity_min = 1.0
			mat.initial_velocity_max = 3.0
			mat.gravity = Vector3(0, -0.5, 0)
			mat.turbulence_enabled = true
			mat.scale_min = 0.08
			mat.scale_max = 0.15
			var petal_mesh = PlaneMesh.new()
			petal_mesh.size = Vector2(0.12, 0.12)
			_particles.draw_pass_1 = petal_mesh
		"sunny":
			mat.direction = Vector3(0, -1, 0)
			mat.spread = 180.0
			mat.initial_velocity_min = 0.3
			mat.initial_velocity_max = 1.0
			mat.gravity = Vector3(0, 0, 0)
			mat.scale_min = 0.03
			mat.scale_max = 0.06
			var sun_mesh = SphereMesh.new()
			sun_mesh.radius = 0.03
			sun_mesh.height = 0.06
			_particles.draw_pass_1 = sun_mesh
	_particles.process_material = mat

func _clear_weather() -> void:
	_weather_active = false
	_particles.emitting = false

func _maybe_change_weather() -> void:
	# 30% 概率切换晴天↔区域天气（增加变化感）
	if randf() < 0.3:
		if _weather_active:
			_clear_weather()
		else:
			_apply_zone_weather()
