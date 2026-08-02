#============================================================
# WaterRipple.gd — 水波纹特效（玩家踩水面产生扩散涟漪）
#============================================================
# 监听玩家位置，当 y < 0.5（水面高度）时周期性生成扩散圆环
# 每个涟漪：从中心向外扩散+淡出的扁平环（CSG/CylinderMesh）
# 挂载在主场景，自动跟随玩家
#============================================================
extends Node3D

const RIPPLE_INTERVAL: float = 0.4   # 每 0.4 秒一个涟漪（蹚水时）
const RIPPLE_LIFETIME: float = 1.2
const WATER_LEVEL: float = 0.5       # 水面高度阈值

var _player: CharacterBody3D = null
var _timer: float = 0.0
var _active_ripples: Array = []   # [{mesh, timer, max_scale}]

func _ready() -> void:
	# 预创建 6 个涟漪 mesh 池
	for i in 6:
		var mesh_inst = MeshInstance3D.new()
		var ring = CylinderMesh.new()
		ring.top_radius = 0.0
		ring.bottom_radius = 0.0
		ring.height = 0.02
		mesh_inst.mesh = ring
		var mat = StandardMaterial3D.new()
		mat.albedo_color = Color(1, 1, 1, 0)
		mat.transparency = BaseMaterial3D.TRANSPARENCY_ALPHA
		mat.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
		mat.emissive = Color(0.7, 0.9, 1.0)
		mesh_inst.material_override = mat
		mesh_inst.visible = false
		add_child(mesh_inst)
		_active_ripples.append({"mesh": mesh_inst, "timer": 0.0, "max_scale": 1.0, "active": false})

func _process(delta: float) -> void:
	if _player == null or not is_instance_valid(_player):
		_player = get_tree().get_first_node_in_group("player")
		if _player == null:
			return
	# 检测是否在水面（y 低 + 处于移动中）
	var in_water = _player.global_position.y < WATER_LEVEL
	# 更新现有涟漪
	for r in _active_ripples:
		if not r.active:
			continue
		r.timer += delta
		var progress = r.timer / RIPPLE_LIFETIME
		if progress >= 1.0:
			r.active = false
			r.mesh.visible = false
		else:
			# 扩散 + 淡出
			var scale_factor = lerp(0.5, r.max_scale, progress)
			var mesh_inst = r.mesh
			var ring = mesh_inst.mesh as CylinderMesh
			if ring:
				ring.top_radius = scale_factor
				ring.bottom_radius = scale_factor
			var mat = mesh_inst.material_override as StandardMaterial3D
			if mat:
				mat.albedo_color.a = (1.0 - progress) * 0.6
	# 蹚水时生成新涟漪
	if in_water:
		_timer -= delta
		if _timer <= 0:
			_timer = RIPPLE_INTERVAL
			_spawn_ripple(_player.global_position)

func _spawn_ripple(pos: Vector3) -> void:
	# 找一个空闲涟漪
	for r in _active_ripples:
		if not r.active:
			r.active = true
			r.timer = 0.0
			r.max_scale = randf_range(1.5, 2.5)
			r.mesh.global_position = Vector3(pos.x, WATER_LEVEL, pos.z)
			r.mesh.visible = true
			return
