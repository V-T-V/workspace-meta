#============================================================
# DayNightCycle.gd — 温暖昼夜循环（纯氛围，不触发敌人）
#============================================================
# 太阳旋转 + 天空色/环境光/雾色插值
# 一天 = 180 秒（3 分钟，适合儿童短局体验）
#============================================================
extends DirectionalLight3D

const DAY_LENGTH: float = 180.0  # 3 分钟一天
var time_of_day: float = 0.25    # 0.25=早上开始
var _world_env: WorldEnvironment = null

# 儿童友好的温暖色调（不走暗夜风，始终保持明亮）
const SKY_KEYS := [
	[0.0,  Color(0.4, 0.35, 0.55), Color(0.3, 0.25, 0.4), 0.3],   # 黎明（暖紫）
	[0.25, Color(0.5, 0.75, 1.0),  Color(0.7, 0.85, 1.0),  2.0],   # 早上（蓝天）
	[0.5,  Color(0.3, 0.65, 0.95), Color(0.6, 0.8, 1.0),   2.2],   # 正午（最亮）
	[0.75, Color(0.9, 0.6, 0.4),   Color(0.8, 0.5, 0.35),  1.5],   # 黄昏（暖橙）
	[1.0,  Color(0.4, 0.35, 0.55), Color(0.3, 0.25, 0.4),  0.3],   # 回到黎明
]

func _ready() -> void:
	_world_env = get_node_or_null("/root/Main/WorldEnvironment")

func _process(delta: float) -> void:
	time_of_day += delta / DAY_LENGTH
	if time_of_day >= 1.0:
		time_of_day -= 1.0
	_update_sky()

func _update_sky() -> void:
	var i := 0
	while i < SKY_KEYS.size() - 1 and time_of_day > SKY_KEYS[i + 1][0]:
		i += 1
	var k0 = SKY_KEYS[i]
	var k1 = SKY_KEYS[min(i + 1, SKY_KEYS.size() - 1)]
	var alpha = (time_of_day - k0[0]) / (k1[0] - k0[0]) if k1[0] > k0[0] else 0.0
	alpha = clamp(alpha, 0.0, 1.0)
	light_energy = lerp(float(k0[3]), float(k1[3]), alpha)
	if _world_env and _world_env.environment:
		var sky_mat = _world_env.environment.sky.sky_material
		if sky_mat and sky_mat is ProceduralSkyMaterial:
			sky_mat.sky_top_color = k0[1].lerp(k1[1], alpha)
			sky_mat.sky_horizon_color = k0[1].lerp(k1[1], alpha).lightened(0.2)
		_world_env.environment.ambient_light_color = k0[2].lerp(k1[2], alpha)
		_world_env.environment.ambient_light_energy = max(light_energy * 0.4, 0.3)
		# 雾色跟随天空（薄雾营造深度）
		if _world_env.environment.fog_enabled:
			_world_env.environment.fog_light_color = k0[2].lerp(k1[2], alpha)
	# 太阳旋转
	var angle = (time_of_day - 0.25) * TAU
	rotation = Vector3(angle * 0.5, 0.3, deg_to_rad(-30))

func is_night() -> bool:
	return light_energy < 0.8
