#============================================================
# ModelFactory.gd — 用 CSG 组合体替代纯方块（无需外部素材）
#============================================================
# 用 CSGPrimitive 组合出更有形状感的模型：
#   - 树：圆柱干 + 圆锥冠
#   - 花：圆柱茎 + 球形花头
#   - 房子：立方体 + 三棱柱屋顶
#   - 角色：圆柱体身 + 球形头 + 小耳朵
#   - 收集物：用不同形状区分（苹果=球, 星=多面体, 贝壳=扁球）
#============================================================
class_name ModelFactory
extends RefCounted

# --- 材质缓存（按颜色哈希复用，避免重复创建 StandardMaterial3D）---
# 移动端尤其重要：48 收集物 × 每个含 1-8 个 CSG = 数百 material 实例
static var _mat_cache: Dictionary = {}

## 获取（或创建并缓存）一个 StandardMaterial3D
## opts 可包含：emissive(Color), emissive_energy(float), metallic(float), roughness(float), shaded(bool)
static func get_material(color: Color, opts: Dictionary = {}) -> StandardMaterial3D:
	# 用颜色 + 关键参数生成缓存键
	var key := "%s|%s|%s|%s|%s|%s" % [
		color.to_html(),
		opts.get("emissive", Color.BLACK).to_html(),
		opts.get("emissive_energy", 0.0),
		opts.get("metallic", 0.0),
		opts.get("roughness", 1.0),
		opts.get("shaded", true),
	]
	if _mat_cache.has(key):
		return _mat_cache[key]
	var mat := StandardMaterial3D.new()
	mat.albedo_color = color
	if opts.get("emissive", Color.BLACK) != Color.BLACK:
		mat.emissive = opts["emissive"]
		mat.emissive_energy_multiplier = opts.get("emissive_energy", 0.5)
	mat.metallic = opts.get("metallic", 0.0)
	mat.roughness = opts.get("roughness", 1.0)
	if opts.get("shaded", true):
		mat.shading_mode = BaseMaterial3D.SHADING_MODE_PER_PIXEL
	_mat_cache[key] = mat
	return mat

## 创建一棵树（圆柱干 + 圆锥冠）
static func make_tree(trunk_color: Color, crown_color: Color) -> Node3D:
	var node = Node3D.new()
	# 树干
	var trunk = CSGCylinder3D.new()
	trunk.radius = 0.2
	trunk.height = 1.5
	trunk.position = Vector3(0, 0.75, 0)
	var tmat = StandardMaterial3D.new()
	tmat.albedo_color = trunk_color
	tmat.shading_mode = BaseMaterial3D.SHADING_MODE_PER_PIXEL
	trunk.material = tmat
	node.add_child(trunk)
	# 树冠（圆锥 = 压扁的圆柱）
	var crown = CSGCylinder3D.new()
	crown.radius = 0.8
	crown.height = 1.2
	crown.position = Vector3(0, 2.0, 0)
	var cmat = StandardMaterial3D.new()
	cmat.albedo_color = crown_color
	cmat.shading_mode = BaseMaterial3D.SHADING_MODE_PER_PIXEL
	crown.material = cmat
	node.add_child(crown)
	return node
## 创建一朵花（圆柱茎 + 球形花头 + 发光）
static func make_flower(stem_color: Color, head_color: Color) -> Node3D:
	var node = Node3D.new()
	# 茎
	var stem = CSGCylinder3D.new()
	stem.radius = 0.05
	stem.height = 0.8
	stem.position = Vector3(0, 0.4, 0)
	var smat = StandardMaterial3D.new()
	smat.albedo_color = stem_color
	stem.material = smat
	node.add_child(stem)
	# 花头
	var head = CSGSphere3D.new()
	head.radius = 0.2
	head.position = Vector3(0, 0.9, 0)
	var hmat = StandardMaterial3D.new()
	hmat.albedo_color = head_color
	hmat.emissive = head_color
	hmat.emissive_energy_multiplier = 0.4
	head.material = hmat
	node.add_child(head)
	return node
## 创建一个房子（立方体 + 三棱柱屋顶）
static func make_house(wall_color: Color, roof_color: Color) -> Node3D:
	var node = Node3D.new()
	# 墙体
	var wall = CSGBox3D.new()
	wall.size = Vector3(1.5, 1.2, 1.5)
	wall.position = Vector3(0, 0.6, 0)
	var wmat = StandardMaterial3D.new()
	wmat.albedo_color = wall_color
	wall.material = wmat
	node.add_child(wall)
	# 屋顶（旋转的 CSGBox 模拟斜屋顶）
	var roof = CSGBox3D.new()
	roof.size = Vector3(1.8, 0.1, 1.8)
	roof.position = Vector3(0, 1.25, 0)
	roof.rotation_degrees = Vector3(0, 45, 0)
	roof.scale = Vector3(1.0, 1.0, 0.5)
	var rmat = StandardMaterial3D.new()
	rmat.albedo_color = roof_color
	roof.material = rmat
	node.add_child(roof)
	return node
## 创建角色（圆柱体身 + 球形头 + 两个耳朵）
static func make_character(body_color: Color, head_color: Color) -> Node3D:
	var node = Node3D.new()
	# 身体（圆柱）
	var body = CSGCylinder3D.new()
	body.radius = 0.25
	body.height = 0.6
	body.position = Vector3(0, 0.3, 0)
	var bmat = StandardMaterial3D.new()
	bmat.albedo_color = body_color
	bmat.shading_mode = BaseMaterial3D.SHADING_MODE_PER_PIXEL
	body.material = bmat
	node.add_child(body)
	# 头（球）
	var head = CSGSphere3D.new()
	head.radius = 0.25
	head.position = Vector3(0, 0.8, 0)
	var hmat = StandardMaterial3D.new()
	hmat.albedo_color = head_color
	hmat.shading_mode = BaseMaterial3D.SHADING_MODE_PER_PIXEL
	head.material = hmat
	node.add_child(head)
	# 左耳
	var ear_l = CSGSphere3D.new()
	ear_l.radius = 0.08
	ear_l.position = Vector3(-0.15, 1.0, 0)
	ear_l.material = hmat
	node.add_child(ear_l)
	# 右耳
	var ear_r = CSGSphere3D.new()
	ear_r.radius = 0.08
	ear_r.position = Vector3(0.15, 1.0, 0)
	ear_r.material = hmat
	node.add_child(ear_r)
	return node
## 按区域创建不同动物 NPC（各自独有的耳朵/尾巴/颜色）
## zone: grassland→小兔, beach→小猫, garden→小熊, ice→小狐
static func make_character_by_zone(zone: String, body_color: Color, head_color: Color) -> Node3D:
	var node = Node3D.new()
	var bmat := StandardMaterial3D.new()
	bmat.albedo_color = body_color
	bmat.shading_mode = BaseMaterial3D.SHADING_MODE_PER_PIXEL
	var hmat := StandardMaterial3D.new()
	hmat.albedo_color = head_color
	hmat.shading_mode = BaseMaterial3D.SHADING_MODE_PER_PIXEL
	# 通用身体
	var body := CSGCylinder3D.new()
	body.radius = 0.28; body.height = 0.6; body.position = Vector3(0, 0.3, 0)
	body.material = bmat
	node.add_child(body)
	# 通用头
	var head := CSGSphere3D.new()
	head.radius = 0.26; head.position = Vector3(0, 0.82, 0)
	head.material = hmat
	node.add_child(head)
	# 鼻子（通用小点）
	var nose := CSGSphere3D.new()
	nose.radius = 0.04; nose.position = Vector3(0, 0.78, 0.24)
	var nmat := StandardMaterial3D.new(); nmat.albedo_color = Color(0.9, 0.4, 0.4)
	nose.material = nmat
	node.add_child(nose)
	# 眼睛（通用两个小球）
	for sx in [-1, 1]:
		var eye := CSGSphere3D.new()
		eye.radius = 0.035; eye.position = Vector3(sx * 0.1, 0.86, 0.23)
		var emat := StandardMaterial3D.new(); emat.albedo_color = Color(0.05, 0.05, 0.1)
		eye.material = emat
		node.add_child(eye)
	match zone:
		"grassland":
			# 小兔：两只长耳朵（细圆柱朝上）
			for sx in [-1, 1]:
				var ear := CSGCylinder3D.new()
				ear.radius = 0.05; ear.height = 0.45
				ear.position = Vector3(sx * 0.1, 1.15, 0)
				ear.material = hmat
				node.add_child(ear)
				# 耳内粉色
				var inner := CSGCylinder3D.new()
				inner.radius = 0.025; inner.height = 0.45
				inner.position = Vector3(sx * 0.1, 1.15, 0.02)
				var inmat := StandardMaterial3D.new(); inmat.albedo_color = Color(1.0, 0.6, 0.65)
				inner.material = inmat
				node.add_child(inner)
			# 圆球尾巴
			var tail := CSGSphere3D.new()
			tail.radius = 0.1; tail.position = Vector3(0, 0.35, -0.3)
			tail.material = hmat
			node.add_child(tail)
		"beach":
			# 小猫：三角耳（细锥） + 长尾
			for sx in [-1, 1]:
				var ear := CSGCylinder3D.new()
				ear.radius = 0.08; ear.height = 0.22
				ear.position = Vector3(sx * 0.14, 1.04, 0)
				ear.scale = Vector3(0.5, 1.0, 0.5)
				ear.material = hmat
				node.add_child(ear)
			# 尾巴：斜放细圆柱
			var tail := CSGCylinder3D.new()
			tail.radius = 0.05; tail.height = 0.55
			tail.position = Vector3(0, 0.45, -0.32)
			tail.rotation_degrees = Vector3(50, 0, 0)
			tail.material = hmat
			node.add_child(tail)
		"garden":
			# 小熊：圆耳朵（短粗圆柱） + 大圆鼻
			for sx in [-1, 1]:
				var ear := CSGCylinder3D.new()
				ear.radius = 0.11; ear.height = 0.08
				ear.position = Vector3(sx * 0.2, 1.02, 0)
				ear.material = hmat
				node.add_child(ear)
			# 大鼻头（覆盖原小鼻子）
			var snout := CSGSphere3D.new()
			snout.radius = 0.09; snout.position = Vector3(0, 0.76, 0.24)
			var sm := StandardMaterial3D.new(); sm.albedo_color = head_color.lightened(0.3)
			snout.material = sm
			node.add_child(snout)
		"ice":
			# 小狐：尖耳（前倾锥） + 大蓬尾
			for sx in [-1, 1]:
				var ear := CSGCylinder3D.new()
				ear.radius = 0.07; ear.height = 0.3
				ear.position = Vector3(sx * 0.14, 1.05, -0.02)
				ear.rotation_degrees = Vector3(-15, 0, 0)
				ear.scale = Vector3(0.55, 1.0, 0.55)
				ear.material = hmat
				node.add_child(ear)
			# 蓬松大尾巴（椭球）
			var tail := CSGSphere3D.new()
			tail.radius = 0.18
			tail.scale = Vector3(0.7, 1.6, 0.7)
			tail.position = Vector3(0, 0.6, -0.4)
			tail.rotation_degrees = Vector3(35, 0, 0)
			var tmat := StandardMaterial3D.new(); tmat.albedo_color = Color(0.95, 0.9, 0.85)
			tail.material = tmat
			node.add_child(tail)
		_:
			# 默认：圆耳朵
			for sx in [-1, 1]:
				var ear := CSGSphere3D.new()
				ear.radius = 0.09; ear.position = Vector3(sx * 0.16, 1.0, 0)
				ear.material = hmat
				node.add_child(ear)
	return node
## 创建收集物模型（按类型精细定制 CSG 组合体，材质全部走缓存）
static func make_collectible(item_type: String, color: Color) -> Node3D:
	var node = Node3D.new()
	# 主材质（带 emissive 发光，走缓存）
	var mat = get_material(color, {
		"emissive": color, "emissive_energy": 0.5,
		"metallic": 0.3, "roughness": 0.3, "shaded": true,
	})
	# 常用辅色（缓存）
	var brown_mat = get_material(Color(0.4, 0.25, 0.1))
	var darkbrown_mat = get_material(Color(0.5, 0.35, 0.1))
	var leaf_mat = get_material(Color(0.3, 0.6, 0.15))
	var stem_mat = get_material(Color(0.25, 0.5, 0.15))
	var yellow_mat = get_material(Color(1, 0.8, 0.1), {"emissive": Color(1, 0.8, 0.1), "emissive_energy": 0.3})
	var black_mat = get_material(Color(0.1, 0.05, 0))

	match item_type:
		"apple":
			# 苹果：球 + 小柄 + 叶
			var body = CSGSphere3D.new()
			body.radius = 0.22
			body.material = mat
			node.add_child(body)
			var stem = CSGCylinder3D.new()
			stem.radius = 0.02; stem.height = 0.12
			stem.position = Vector3(0, 0.28, 0)
			stem.material = brown_mat
			node.add_child(stem)
			var leaf = CSGBox3D.new()
			leaf.size = Vector3(0.08, 0.02, 0.04)
			leaf.position = Vector3(0.06, 0.3, 0)
			leaf.rotation_degrees = Vector3(0, 0, 30)
			leaf.material = leaf_mat
			node.add_child(leaf)
		"flower", "petal":
			# 花：茎 + 5 片花瓣球
			var stem = CSGCylinder3D.new()
			stem.radius = 0.03; stem.height = 0.3
			stem.position = Vector3(0, -0.15, 0)
			stem.material = stem_mat
			node.add_child(stem)
			for i in 5:
				var a = TAU * i / 5.0
				var petal = CSGSphere3D.new()
				petal.radius = 0.08
				petal.position = Vector3(cos(a) * 0.1, 0.05, sin(a) * 0.1)
				petal.scale = Vector3(1, 0.5, 1)
				petal.material = mat
				node.add_child(petal)
			var center = CSGSphere3D.new()
			center.radius = 0.06; center.position = Vector3(0, 0.06, 0)
			center.material = yellow_mat
			node.add_child(center)
		"butterfly":
			# 蝴蝶：身体圆柱 + 两对翅膀（扁平 box）
			var body = CSGCylinder3D.new()
			body.radius = 0.03; body.height = 0.2
			body.rotation_degrees = Vector3(90, 0, 0)
			body.material = mat
			node.add_child(body)
			var wing_mat = get_material(color.lightened(0.2), {
				"emissive": color, "emissive_energy": 0.3, "shaded": true,
			})
			for side in [-1, 1]:
				var wing = CSGBox3D.new()
				wing.size = Vector3(0.2, 0.02, 0.15)
				wing.position = Vector3(side * 0.12, 0, 0)
				wing.material = wing_mat
				node.add_child(wing)
		"shell":
			# 贝壳：压扁半球
			var shell = CSGSphere3D.new()
			shell.radius = 0.22; shell.scale = Vector3(1, 0.6, 1)
			shell.material = mat
			node.add_child(shell)
		"starfish":
			# 海星：中心球 + 5 个手臂 box
			var center = CSGSphere3D.new()
			center.radius = 0.1; center.material = mat
			node.add_child(center)
			for i in 5:
				var a = TAU * i / 5.0
				var arm = CSGBox3D.new()
				arm.size = Vector3(0.06, 0.04, 0.18)
				arm.position = Vector3(cos(a) * 0.14, 0, sin(a) * 0.14)
				arm.rotation_degrees = Vector3(0, rad_to_deg(a) + 90, 0)
				arm.material = mat
				node.add_child(arm)
		"pearl":
			# 珍珠：完美球 + 高反光（独立材质，走缓存）
			var pearl_mat = get_material(Color(0.95, 0.93, 0.98), {
				"metallic": 0.9, "roughness": 0.05,
				"emissive": Color(0.5, 0.5, 0.6), "emissive_energy": 0.2,
			})
			var pearl = CSGSphere3D.new()
			pearl.radius = 0.18
			pearl.material = pearl_mat
			node.add_child(pearl)
		"honey":
			# 蜂蜜罐：圆柱 + 小盖
			var pot = CSGCylinder3D.new()
			pot.radius = 0.15; pot.height = 0.25
			pot.material = mat
			node.add_child(pot)
			var lid = CSGCylinder3D.new()
			lid.radius = 0.17; lid.height = 0.05
			lid.position = Vector3(0, 0.15, 0)
			lid.material = darkbrown_mat
			node.add_child(lid)
		"ladybug":
			# 瓢虫：半球 + 小头
			var body = CSGSphere3D.new()
			body.radius = 0.18; body.scale = Vector3(1, 0.6, 1.1)
			body.material = mat
			node.add_child(body)
			var head = CSGSphere3D.new()
			head.radius = 0.06; head.position = Vector3(0, 0.02, 0.2)
			head.material = black_mat
			node.add_child(head)
		"snowflake":
			# 雪花：6 条交叉细柱
			for i in 6:
				var a = TAU * i / 6.0
				var arm = CSGCylinder3D.new()
				arm.radius = 0.02; arm.height = 0.3
				arm.rotation_degrees = Vector3(90, rad_to_deg(a), 0)
				arm.material = mat
				node.add_child(arm)
		"icecrystal":
			# 冰晶：八面体（旋转 box）
			var gem = CSGBox3D.new()
			gem.size = Vector3(0.22, 0.22, 0.22)
			gem.rotation_degrees = Vector3(45, 45, 0)
			gem.material = mat
			node.add_child(gem)
		"egg":
			# 蛋：椭圆球
			var egg = CSGSphere3D.new()
			egg.radius = 0.18; egg.scale = Vector3(0.85, 1.15, 0.85)
			egg.material = mat
			node.add_child(egg)
		_:
			var s = CSGSphere3D.new()
			s.radius = 0.22; s.material = mat
			node.add_child(s)
	return node
## 创建灯柱
static func make_lamp(pole_color: Color) -> Node3D:
	var node = Node3D.new()
	# 杆
	var pole = CSGCylinder3D.new()
	pole.radius = 0.08
	pole.height = 2.5
	pole.position = Vector3(0, 1.25, 0)
	var pmat = StandardMaterial3D.new()
	pmat.albedo_color = pole_color
	pole.material = pmat
	node.add_child(pole)
	# 灯头（球 + 发光）
	var bulb = CSGSphere3D.new()
	bulb.radius = 0.2
	bulb.position = Vector3(0, 2.5, 0)
	var bmat = StandardMaterial3D.new()
	bmat.albedo_color = Color(1.0, 0.95, 0.7)
	bmat.emissive = Color(1.0, 0.85, 0.4)
	bmat.emissive_energy_multiplier = 2.0
	bulb.material = bmat
	node.add_child(bulb)
	return node
## 创建篱笆（一排小圆柱）
static func make_fence(color: Color, length: float = 3.0) -> Node3D:
	var node = Node3D.new()
	var posts = int(length / 0.5)
	for i in posts:
		var post = CSGCylinder3D.new()
		post.radius = 0.06
		post.height = 0.6
		post.position = Vector3(i * 0.5 - length * 0.5, 0.3, 0)
		var mat = StandardMaterial3D.new()
		mat.albedo_color = color
		post.material = mat
		node.add_child(post)
	return node
## 创建小池塘（蓝色扁平圆柱 + 反光材质）
static func make_pond(color: Color) -> Node3D:
	var node = Node3D.new()
	# 水面（高金属度+低粗糙度=镜面反射感）
	var water = CSGCylinder3D.new()
	water.radius = 2.5
	water.height = 0.08
	water.position = Vector3(0, 0.04, 0)
	var wmat = StandardMaterial3D.new()
	wmat.albedo_color = color
	wmat.metallic = 0.9
	wmat.roughness = 0.05
	wmat.transparency = BaseMaterial3D.TRANSPARENCY_ALPHA
	wmat.albedo_color.a = 0.75
	wmat.emissive = color.lightened(0.3)
	wmat.emissive_energy_multiplier = 0.15
	water.material = wmat
	node.add_child(water)
	# 池底（深色，增加深度感）
	var bottom = CSGCylinder3D.new()
	bottom.radius = 2.4
	bottom.height = 0.05
	bottom.position = Vector3(0, -0.05, 0)
	var bmat = StandardMaterial3D.new()
	bmat.albedo_color = Color(0.15, 0.2, 0.3)
	bottom.material = bmat
	node.add_child(bottom)
	# 池边石头圈（8 个小球点缀）
	for i in 8:
		var a = TAU * i / 8.0
		var stone = CSGSphere3D.new()
		stone.radius = 0.25
		stone.position = Vector3(cos(a) * 2.6, 0.1, sin(a) * 2.6)
		var smat = StandardMaterial3D.new()
		smat.albedo_color = Color(0.5, 0.48, 0.45)
		stone.material = smat
		node.add_child(stone)
	return node

## 创建草丛（3-5 根草叶，CSG 细锥体）
static func make_grass_clump(color: Color) -> Node3D:
	var node = Node3D.new()
	var rng = RandomNumberGenerator.new()
	rng.seed = randi()
	var blades = rng.randi_range(3, 5)
	for i in blades:
		var blade = CSGCylinder3D.new()
		blade.radius = 0.02
		blade.height = rng.randf_range(0.3, 0.5)
		var a = rng.randf() * TAU
		var r = rng.randf_range(0, 0.1)
		blade.position = Vector3(cos(a) * r, blade.height * 0.5, sin(a) * r)
		blade.rotation_degrees = Vector3(rng.randf_range(-10, 10), 0, rng.randf_range(-10, 10))
		var mat = get_material(color, {"shaded": true})
		blade.material = mat
		node.add_child(blade)
	return node

## 创建岩石（不规则球，用于装饰）
static func make_rock(color: Color, size: float = 0.6) -> Node3D:
	var node = Node3D.new()
	var rock = CSGSphere3D.new()
	rock.radius = size
	rock.material = get_material(color)
	# 压扁+不规则缩放模拟自然岩石
	rock.scale = Vector3(1.2, 0.7, 1.0)
	node.add_child(rock)
	return node
