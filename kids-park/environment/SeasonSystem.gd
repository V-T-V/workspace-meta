#============================================================
# SeasonSystem.gd — 季节系统（春夏秋冬切换场景色调）
#============================================================
# 每游戏内 2 天切换一个季节（春→夏→秋→冬循环）
# 季节影响：
#   - 全局色调偏移（春粉/夏绿/秋橙/冬蓝）
#   - 天气粒子强度（冬雪更密）
#   - 落叶/樱花/雪花季节专属粒子
# 通过 WorldEnvironment.tonemap_adjustment 动态调色
#============================================================
extends Node3D

const SEASON_DURATION: float = 240.0   # 2 个昼夜周期 = 1 季（120s × 2）

enum Season { SPRING, SUMMER, AUTUMN, WINTER }

var _current_season: int = Season.SPRING
var _season_timer: float = 0.0
var _world_env: WorldEnvironment = null
var _seasonal_particles: GPUParticles3D = null

const SEASON_DATA := {
	Season.SPRING: {"name": "春", "emoji": "🌸", "tint": Color(1.0, 0.95, 0.95), "sat": 1.15, "particle_color": Color(1.0, 0.7, 0.85)},
	Season.SUMMER: {"name": "夏", "emoji": "☀️", "tint": Color(1.0, 1.0, 0.95), "sat": 1.25, "particle_color": Color(1.0, 0.95, 0.4)},
	Season.AUTUMN: {"name": "秋", "emoji": "🍂", "tint": Color(1.05, 0.95, 0.85), "sat": 1.05, "particle_color": Color(0.9, 0.5, 0.2)},
	Season.WINTER: {"name": "冬", "emoji": "❄️", "tint": Color(0.92, 0.95, 1.0), "sat": 0.95, "particle_color": Color(0.9, 0.95, 1.0)},
}

func _ready() -> void:
	_world_env = get_tree().current_scene.get_node_or_null("WorldEnvironment")
	_apply_season()

func _process(delta: float) -> void:
	_season_timer += delta
	if _season_timer >= SEASON_DURATION:
		_season_timer = 0.0
		_current_season = (_current_season + 1) % 4
		_apply_season()
		var sdef = SEASON_DATA[_current_season]
		EventBus.toast_message.emit("季节变换：%s %s" % [sdef["emoji"], sdef["name"]], sdef["emoji"])
		AudioBus.play_zone_unlock()

func _apply_season() -> void:
	var sdef = SEASON_DATA[_current_season]
	# 调整环境色调（饱和度 + 色彩偏移）
	if _world_env and _world_env.environment:
		_world_env.environment.adjustment_saturation = sdef["sat"]
	# 季节专属粒子（樱花/阳光/落叶/雪花）
	_update_seasonal_particles(sdef)

func _update_seasonal_particles(sdef: Dictionary) -> void:
	if _seasonal_particles == null:
		_seasonal_particles = GPUParticles3D.new()
		_seasonal_particles.amount = 30
		_seasonal_particles.lifetime = 5.0
		_seasonal_particles.explosiveness = 0.0
		_seasonal_particles.position = Vector3(0, 20, 0)
		var mat = ParticleProcessMaterial.new()
		mat.direction = Vector3(0.5, -1, 0.3)
		mat.spread = 60.0
		mat.initial_velocity_min = 0.5
		mat.initial_velocity_max = 2.0
		mat.gravity = Vector3(0, -0.5, 0)
		mat.turbulence_enabled = true
		mat.scale_min = 0.05
		mat.scale_max = 0.12
		_seasonal_particles.process_material = mat
		var mesh = SphereMesh.new()
		mesh.radius = 0.06
		mesh.height = 0.12
		_seasonal_particles.draw_pass_1 = mesh
		add_child(_seasonal_particles)
	# 更新粒子颜色
	var mat = _seasonal_particles.process_material as ParticleProcessMaterial
	if mat:
		mat.color = sdef["particle_color"]
	# 冬天雪更多
	_seasonal_particles.amount = 60 if _current_season == Season.WINTER else 30

func get_current_season() -> int:
	return _current_season

func get_season_name() -> String:
	return SEASON_DATA[_current_season]["name"]
