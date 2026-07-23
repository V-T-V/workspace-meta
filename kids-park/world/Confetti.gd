#============================================================
# Confetti.gd — 彩纸庆祝效果（拾取/完成任务时爆发）
#============================================================
# 对象池模式：预创建 N 个 GPUParticles3D，循环复用
# 避免：频繁拾取时反复 new/free 粒子节点和材质（移动端 GC 压力）
# 多色彩纸：每颗粒子独立色相（color_ramp + randomness）
#============================================================
extends Node3D

const POOL_SIZE: int = 16          # 池大小（同时最多 16 个并发爆发）
const PARTICLE_AMOUNT: int = 24
const PARTICLE_LIFETIME: float = 1.2

static var _pool: Array = []       # [{node, busy: bool, free_at: float}]
static var _world: Node = null
static var _shared_mesh: BoxMesh = null
static var _color_ramps: Dictionary = {}   # color.to_html() -> GradientTexture1D 缓存

## 初始化对象池（由 Main.gd 在 _ready 时调用一次）
static func init_pool(world_root: Node) -> void:
	_world = world_root
	_shared_mesh = BoxMesh.new()
	_shared_mesh.size = Vector3(0.15, 0.15, 0.15)
	for i in POOL_SIZE:
		var p := GPUParticles3D.new()
		p.amount = PARTICLE_AMOUNT
		p.lifetime = PARTICLE_LIFETIME
		p.one_shot = true
		p.emitting = false   # 初始不发射
		p.explosiveness = 0.85
		p.randomness = 1.0
		p.draw_pass_1 = _shared_mesh
		# 每个 pool 节点复用一个 material 模板（burst 时只改 color_ramp）
		var mat := ParticleProcessMaterial.new()
		mat.direction = Vector3(0, 1, 0)
		mat.spread = 70.0
		mat.initial_velocity_min = 3.0
		mat.initial_velocity_max = 8.0
		mat.gravity = Vector3(0, -10, 0)
		mat.scale_min = 0.12
		mat.scale_max = 0.28
		p.process_material = mat
		p.visible = false
		world_root.add_child(p)
		_pool.append({"node": p, "busy": false, "free_at": 0.0})

## 在 world_root 下生成彩纸爆发（从池中取空闲节点）
static func burst(world_root: Node, pos: Vector3, color: Color = Color(1, 0.8, 0.2)) -> void:
	# 首次调用时懒初始化
	if _world == null or not is_instance_valid(_world):
		init_pool(world_root)
	# 找一个空闲槽（找不到就复用最早结束的）
	var slot = null
	var oldest = null
	for s in _pool:
		if not s.busy:
			slot = s
			break
		if oldest == null or s.free_at < oldest.free_at:
			oldest = s
	if slot == null:
		slot = oldest
	var p: GPUParticles3D = slot.node
	# 更新位置和颜色（复用材质，只替换 color_ramp）
	p.global_position = pos + Vector3(0, 1.0, 0)
	var mat = p.process_material as ParticleProcessMaterial
	mat.color_ramp = _get_color_ramp(color)
	# 重置并触发发射
	p.restart()
	p.emitting = true
	p.visible = true
	slot.busy = true
	slot.free_at = Time.get_ticks_msec() / 1000.0 + PARTICLE_LIFETIME + 0.3

## 每帧检查回收（由 Main.gd 的 _process 调用）
static func process_pool() -> void:
	if _pool.is_empty():
		return
	var now = Time.get_ticks_msec() / 1000.0
	for s in _pool:
		if s.busy and now >= s.free_at:
			s.busy = false
			s.node.emitting = false
			s.node.visible = false

## 获取（缓存）多色 GradientTexture1D
static func _get_color_ramp(base: Color) -> GradientTexture1D:
	var key = base.to_html()
	if _color_ramps.has(key):
		return _color_ramps[key]
	var tex := GradientTexture1D.new()
	tex.gradient = _make_color_ramp(base)
	_color_ramps[key] = tex
	return tex

## 生成多色渐变 ramp（基础色 → 亮黄 → 粉 → 蓝 → 绿 → 紫 → 橙 → 基础色）
## 配合 randomness=1.0 让每颗粒子在不同时刻取不同色
static func _make_color_ramp(base: Color) -> Gradient:
	var ramp := Gradient.new()
	var colors := [
		base,                          # 基础色
		Color(1.0, 0.95, 0.4),        # 亮黄
		Color(1.0, 0.5, 0.7),         # 粉
		Color(0.6, 0.9, 1.0),         # 天蓝
		Color(0.5, 1.0, 0.6),         # 嫩绿
		Color(0.85, 0.55, 1.0),       # 淡紫
		Color(1.0, 0.75, 0.3),        # 橙
		base,                          # 回到基础色
	]
	for i in colors.size():
		ramp.add_point(float(i) / (colors.size() - 1), colors[i])
	return ramp
