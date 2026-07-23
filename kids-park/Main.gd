#============================================================
# Main.gd — 主场景编排（运行时构建场景树）
#============================================================
extends Node3D

const ModelFactory = preload("res://world/ModelFactory.gd")
const Confetti = preload("res://world/Confetti.gd")

var player: CharacterBody3D
var camera: Camera3D
var _debug_timer: float = 0.0
var _screenshot_done: bool = false

func _ready() -> void:
	_build_scene()
	# 初始化彩纸对象池（避免运行时频繁拾取 new/free 节点）
	Confetti.init_pool(self)
	print("[KidsPark] 场景构建完成")

func _process(delta: float) -> void:
	# 回收彩纸池中已结束的粒子节点
	Confetti.process_pool()
	if OS.get_environment("PARK_SCREENSHOT") == "1" and not _screenshot_done:
		_debug_timer += delta
		if _debug_timer >= 5.0:
			_screenshot_done = true
			_take_screenshot()

func _take_screenshot() -> void:
	await RenderingServer.frame_post_draw
	await get_tree().process_frame
	var img := get_viewport().get_texture().get_image()
	if img:
		img.save_png("user://kidspark_debug.png")
		print("[KidsPark] 截图已保存")
	get_tree().quit(0)

func _build_scene() -> void:
	# --- 世界环境（温暖天空 + 薄雾深度感）---
	var we = WorldEnvironment.new()
	we.name = "WorldEnvironment"
	var env = Environment.new()
	env.background_mode = Environment.BG_SKY
	var sky = Sky.new()
	var sky_mat = ProceduralSkyMaterial.new()
	sky_mat.sky_top_color = Color(0.35, 0.6, 0.95)
	sky_mat.sky_horizon_color = Color(0.85, 0.92, 1.0)
	sky_mat.ground_bottom_color = Color(0.55, 0.7, 0.4)
	sky_mat.ground_horizon_color = Color(0.8, 0.88, 0.65)
	sky_mat.sun_angle_max = 35.0
	sky_mat.sun_curve = 0.15
	sky_mat.use_debanding = true
	sky.sky_material = sky_mat
	env.sky = sky
	# 薄雾（营造空间深度感，不远不近）
	env.fog_enabled = true
	env.fog_light_color = Color(0.8, 0.88, 1.0)
	env.fog_density = 0.006
	env.fog_aerial_perspective = 0.5
	env.fog_sun_scatter = 0.3   # 阳光散射（日出日落暖光穿透雾）
	# 环境光（天光 + 暖色补光）
	env.ambient_light_source = Environment.AMBIENT_SOURCE_SKY
	env.ambient_light_color = Color(1.0, 0.95, 0.85)
	env.ambient_light_energy = 0.7
	# SSAO（增强角落立体感，儿童画面更"厚实"）
	env.ssao_enabled = true
	env.ssao_radius = 1.5
	env.ssao_intensity = 2.0
	env.ssao_power = 1.5
	env.ssao_light_affect = 0.3
	# 色调映射（Filmic 让明暗过渡柔和，儿童画面更温暖）
	env.tonemap_mode = Environment.TONE_MAPPER_FILMIC
	env.tonemap_white = 1.2
	# 辉光（让发光收集物/灯泡更有"魔法感"）
	env.glow_enabled = true
	env.glow_intensity = 0.8
	env.glow_strength = 1.0
	env.glow_blend_mode = Environment.GLOW_BLEND_MODE_ADDITIVE
	env.glow_hdr_threshold = 1.0
	# 色调调整（轻微暖色偏移 + 饱和度提升，画面更鲜艳）
	env.adjustment_enabled = true
	env.adjustment_brightness = 1.05
	env.adjustment_contrast = 1.08
	env.adjustment_saturation = 1.2
	we.environment = env
	add_child(we)
	# --- 太阳/昼夜 ---
	var sun = DirectionalLight3D.new()
	sun.name = "Sun"
	sun.set_script(load("res://environment/DayNightCycle.gd"))
	sun.light_energy = 2.0
	sun.light_color = Color(1.0, 0.95, 0.8)
	sun.shadow_enabled = true
	sun.shadow_opacity = 0.5
	sun.directional_shadow_mode = DirectionalLight3D.SHADOW_PARALLEL_4_SPLITS
	sun.rotation = Vector3(deg_to_rad(-40), 0.3, deg_to_rad(-30))
	add_child(sun)
	# --- 补光（天光反向柔和填充，消除死黑阴影）---
	var fill = DirectionalLight3D.new()
	fill.name = "FillLight"
	fill.light_energy = 0.4
	fill.light_color = Color(0.6, 0.75, 1.0)   # 冷色天光（对冲暖色太阳）
	fill.shadow_enabled = false
	fill.rotation = Vector3(deg_to_rad(40), -0.5, deg_to_rad(30))
	add_child(fill)
	# --- 天气系统 ---
	var weather = Node3D.new()
	weather.name = "Weather"
	weather.set_script(load("res://environment/Weather.gd"))
	add_child(weather)
	# --- 乐园地面 + 收集物 + NPC ---
	_build_park()
	# --- 玩家 ---
	player = CharacterBody3D.new()
	player.name = "Player"
	player.set_script(load("res://player/Player.gd"))
	add_child(player)
	var pcol = CollisionShape3D.new()
	pcol.name = "CollisionShape3D"
	var cap = CapsuleShape3D.new()
	cap.height = 1.2
	cap.radius = 0.35
	pcol.shape = cap
	player.add_child(pcol)
	# 玩家角色模型（Fox.glb 替代 CSG）
	var fox_scene = load("res://assets/models/Fox.glb")
	if fox_scene:
		var fox = fox_scene.instantiate()
		# Fox 模型 ~100 单位高，缩放到 ~1.2 米
		fox.scale = Vector3(0.012, 0.012, 0.012)
		fox.rotation_degrees = Vector3(0, 180, 0)  # 朝前（-Z 方向是移动方向）
		player.add_child(fox)
		# 调整碰撞体高度匹配 Fox
		cap.height = 0.8
		cap.radius = 0.3
	else:
		# 降级：CSG 角色
		var char_model = ModelFactory.make_character(Color(0.3, 0.6, 0.95), Color(1.0, 0.85, 0.6))
		player.add_child(char_model)
	# 相机 Rig
	var cam_rig = Node3D.new()
	cam_rig.name = "CameraRig"
	player.add_child(cam_rig)
	var spring = SpringArm3D.new()
	spring.name = "SpringArm3D"
	spring.spring_length = 6.0
	spring.collision_mask = 1
	spring.shape = SphereShape3D.new()
	spring.shape.radius = 0.5
	cam_rig.add_child(spring)
	camera = Camera3D.new()
	camera.name = "Camera3D"
	camera.set_script(load("res://player/PlayerCamera.gd"))
	camera.current = true
	camera.fov = 65.0
	spring.add_child(camera)
	# --- HUD ---
	var hud = CanvasLayer.new()
	hud.name = "HUD"
	hud.set_script(load("res://ui/HUD.gd"))
	add_child(hud)
	# --- 触屏控件 ---
	var touch = CanvasLayer.new()
	touch.name = "TouchControls"
	touch.set_script(load("res://ui/TouchControls.gd"))
	add_child(touch)
	# --- 小地图 ---
	var minimap = CanvasLayer.new()
	minimap.name = "MiniMap"
	minimap.set_script(load("res://ui/MiniMap.gd"))
	add_child(minimap)
	# --- 图鉴 + 贴纸册（Tab 键切换）---
	var books = CanvasLayer.new()
	books.name = "BooksUI"
	books.set_script(load("res://ui/BooksUI.gd"))
	add_child(books)
	# --- 照相模式（P 键拍照, V 键相册）---
	var photo = CanvasLayer.new()
	photo.name = "PhotoMode"
	photo.set_script(load("res://ui/PhotoMode.gd"))
	add_child(photo)
	# --- 暂停菜单（ESC 键）---
	var pause = CanvasLayer.new()
	pause.name = "PauseMenu"
	pause.set_script(load("res://ui/PauseMenu.gd"))
	add_child(pause)
	# --- E2E 测试（环境变量触发）---
	if OS.get_environment("KIDS_PARK_E2E") == "1":
		var tester = Node.new()
		tester.name = "E2ETest"
		tester.set_script(load("res://tests/E2ETest.gd"))
		add_child(tester)
	# --- 宠物伴灵（已解锁的宠物跟随玩家）---
	_spawn_pets()
	# 后续获得贴纸时自动生成新宠物
	EventBus.sticker_earned.connect(_on_sticker_for_pet)

func _spawn_pets() -> void:
	# 根据已获得的贴纸生成对应宠物
	var pet_map := {
		"🐰小兔的朋友": "bunny",
		"🐱小猫的宝藏": "cat",
		"🐻小熊的甜点": "bear",
		"🦊小狐的冬日": "fox",
	}
	var pet_script = load("res://world/Pet.gd")
	for sticker in GameState.stickers:
		if pet_map.has(sticker):
			var pet_id: String = pet_map[sticker]
			var pet = CharacterBody3D.new()
			pet.set_script(pet_script)
			pet.pet_id = pet_id
			# 碰撞体（小胶囊）
			var col = CollisionShape3D.new()
			var shape = CapsuleShape3D.new()
			shape.height = 0.5
			shape.radius = 0.25
			col.shape = shape
			pet.add_child(col)
			add_child(pet)
			# 放在玩家身后
			var spawn = ParkGen.get_spawn()
			pet.global_position = spawn + Vector3(randf_range(-2, 2), 0.5, randf_range(2, 4))

func _on_sticker_for_pet(sticker: String) -> void:
	# 新获得贴纸时动态生成对应宠物
	var pet_map := {
		"🐰小兔的朋友": "bunny",
		"🐱小猫的宝藏": "cat",
		"🐻小熊的甜点": "bear",
		"🦊小狐的冬日": "fox",
	}
	if not pet_map.has(sticker):
		return
	# 检查是否已有该宠物（避免重复）
	var pet_id: String = pet_map[sticker]
	for existing in get_tree().get_nodes_in_group("pet"):
		if existing.pet_id == pet_id:
			return   # 已存在，不重复生成
	var pet_script = load("res://world/Pet.gd")
	var pet = CharacterBody3D.new()
	pet.set_script(pet_script)
	pet.pet_id = pet_id
	var col = CollisionShape3D.new()
	var shape = CapsuleShape3D.new()
	shape.height = 0.5
	shape.radius = 0.25
	col.shape = shape
	pet.add_child(col)
	add_child(pet)
	var p = get_tree().get_first_node_in_group("player")
	if p:
		pet.global_position = p.global_position + Vector3(randf_range(-2, 2), 0.5, 3)
	else:
		pet.global_position = ParkGen.get_spawn()
	EventBus.toast_message.emit("新伙伴加入了！", "🎉")

func _build_park() -> void:
	var park_data = ParkGen.generate_park()
	for zone_id in park_data:
		var data = park_data[zone_id]
		var center: Vector3 = data["center"]
		var zone_color: Color = data["color"]
		# 区域地面（大彩色方块）
		var ground = StaticBody3D.new()
		ground.name = "Zone_%s" % zone_id
		ground.collision_layer = 1
		var gmesh = MeshInstance3D.new()
		var gbox = BoxMesh.new()
		gbox.size = Vector3(ParkGen.ZONE_SIZE, 0.2, ParkGen.ZONE_SIZE)
		gmesh.mesh = gbox
		gmesh.position = Vector3(center.x, -0.1, center.z)
		var gmat = StandardMaterial3D.new()
		gmat.albedo_color = zone_color
		gmat.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
		gmesh.material_override = gmat
		ground.add_child(gmesh)
		var gcol = CollisionShape3D.new()
		var gshape = BoxShape3D.new()
		gshape.size = Vector3(ParkGen.ZONE_SIZE, 0.2, ParkGen.ZONE_SIZE)
		gcol.shape = gshape
		gcol.position = Vector3(center.x, -0.1, center.z)
		ground.add_child(gcol)
		add_child(ground)
		# 只在已解锁区域放收集物
		if not GameState.is_zone_unlocked(zone_id):
			continue
		# 收集物
		for item in data["items"]:
			var item_type: String = item["type"]
			var item_pos: Vector3 = item["pos"]
			var is_rare: bool = item.get("rare", false)
			var collectible = Area3D.new()
			collectible.set_script(load("res://world/Collectible.gd"))
			collectible.item_type = item_type
			# 碰撞检测区
			var col = CollisionShape3D.new()
			var shape = SphereShape3D.new()
			shape.radius = 1.5 if not is_rare else 2.5   # 稀有物拾取范围更大
			col.shape = shape
			collectible.add_child(col)
			# 可见模型——按 item_type 分配不同 glTF 或精细 CSG
			var idef = GameState.ITEM_TYPES.get(item_type, {})
			var item_color: Color = idef.get("color", Color.WHITE)
			if is_rare:
				# 稀有金色星星：超强发光 + 大体积
				var model_node = _make_gold_star()
				collectible.add_child(model_node)
			else:
				var model_node = _make_collectible_model(item_type, item_color)
				collectible.add_child(model_node)
			add_child(collectible)
			collectible.global_position = item_pos
		# NPC
		for npc_data in data["npcs"]:
			var npc_pos: Vector3 = npc_data["pos"]
			var npc_node = CharacterBody3D.new()
			npc_node.set_script(load("res://world/NPC.gd"))
			npc_node.zone_id = npc_data["zone"]
			var ncol = CollisionShape3D.new()
			var nshape = CapsuleShape3D.new()
			nshape.height = 1.0
			nshape.radius = 0.35
			ncol.shape = nshape
			npc_node.add_child(ncol)
			# NPC 模型——每区域使用专属真实 glTF 动物（PBR 质感）
			var npc_model = _make_npc_model(zone_id, zone_color)
			npc_node.add_child(npc_model)
			add_child(npc_node)
			npc_node.global_position = npc_pos
		# 装饰物（树/花/灯柱）—— 丰富乐园视觉
		_spawn_decorations(center, zone_id, zone_color)
		# 区域指示牌（入口处的彩色路标 + emoji）
		_spawn_zone_sign(center, zone_id, zone_color)

func _spawn_zone_sign(center: Vector3, zone_id: String, zone_color: Color) -> void:
	var zdef = GameState.ZONES.get(zone_id, {})
	# 立柱（朝向区域中心入口）
	var post = CSGCylinder3D.new()
	post.radius = 0.12
	post.height = 2.5
	post.position = center + Vector3(0, 1.25, ParkGen.ZONE_SIZE * 0.4)
	var pmat = StandardMaterial3D.new()
	pmat.albedo_color = zone_color.darkened(0.2)
	pmat.shading_mode = BaseMaterial3D.SHADING_MODE_PER_PIXEL
	post.material = pmat
	add_child(post)
	# 顶部指示牌（彩色方块 + emoji Label3D）
	var sign = CSGBox3D.new()
	sign.size = Vector3(1.5, 0.9, 0.1)
	sign.position = center + Vector3(0, 2.5, ParkGen.ZONE_SIZE * 0.4)
	var smat = StandardMaterial3D.new()
	smat.albedo_color = zone_color.lightened(0.3)
	smat.emissive = zone_color
	smat.emissive_energy_multiplier = 0.3
	smat.shading_mode = BaseMaterial3D.SHADING_MODE_PER_PIXEL
	sign.material = smat
	add_child(sign)
	# emoji 标签（区域图标 + 名称）
	var label = Label3D.new()
	label.text = "%s\n%s" % [zdef.get("emoji", "🗺️"), zdef.get("name", zone_id)]
	label.font_size = 48
	label.position = center + Vector3(0, 2.5, ParkGen.ZONE_SIZE * 0.4 + 0.1)
	label.billboard = BaseMaterial3D.BILLBOARD_DISABLED
	label.outline_size = 8
	label.outline_modulate = Color(0, 0, 0, 0.7)
	add_child(label)
	# 锁定状态遮罩（未解锁区域显示锁）
	if not GameState.is_zone_unlocked(zone_id):
		var lock = Label3D.new()
		lock.text = "🔒"
		lock.font_size = 64
		lock.position = center + Vector3(0, 2.5, ParkGen.ZONE_SIZE * 0.4 + 0.15)
		lock.billboard = BaseMaterial3D.BILLBOARD_DISABLED
		add_child(lock)

func _spawn_decorations(center: Vector3, zone_id: String, zone_color: Color) -> void:
	var rng := RandomNumberGenerator.new()
	rng.seed = hash(zone_id) + 999
	# --- 主装饰（树/花/灯柱/房子/glTF 模型）---
	for i in 10:
		var angle = rng.randf() * TAU
		var dist = rng.randf_range(5.0, ParkGen.ZONE_SIZE * 0.4)
		var pos = center + Vector3(cos(angle) * dist, 0, sin(angle) * dist)
		var deco_type = rng.randi() % 6  # 0=树 1=花 2=灯柱 3=房子 4=BoomBox音箱 5=Lantern灯笼
		var model: Node3D
		match deco_type:
			0:
				model = ModelFactory.make_tree(Color(0.45, 0.3, 0.15), zone_color.darkened(0.3))
			1:
				model = ModelFactory.make_flower(Color(0.3, 0.6, 0.2), zone_color.lightened(0.4))
			2:
				model = ModelFactory.make_lamp(Color(0.6, 0.6, 0.65))
				var light = OmniLight3D.new()
				light.position = Vector3(0, 2.5, 0)
				light.light_color = Color(1.0, 0.9, 0.6)
				light.light_energy = 1.5
				light.omni_range = 8.0
				light.shadow_enabled = false
				model.add_child(light)
			3:
				model = ModelFactory.make_house(zone_color.darkened(0.1), zone_color.darkened(0.4))
			4:
				# BoomBox 音箱（真实 PBR 模型）
				var boom_scene = _load_glb("BoomBox")
				if boom_scene:
					model = Node3D.new()
					var boom = boom_scene.instantiate()
					boom.scale = Vector3(5.0, 5.0, 5.0)
					model.add_child(boom)
				else:
					model = ModelFactory.make_tree(Color(0.45, 0.3, 0.15), zone_color.darkened(0.3))
			5:
				# Lantern 灯笼（真实 PBR 模型，夜晚发光）
				var lantern_scene = _load_glb("Lantern")
				if lantern_scene:
					model = Node3D.new()
					var lantern = lantern_scene.instantiate()
					lantern.scale = Vector3(0.08, 0.08, 0.08)
					model.add_child(lantern)
					# 灯笼加光源
					var ll = OmniLight3D.new()
					ll.position = Vector3(0, 0.5, 0)
					ll.light_color = Color(1.0, 0.85, 0.5)
					ll.light_energy = 2.0
					ll.omni_range = 6.0
					model.add_child(ll)
				else:
					model = ModelFactory.make_lamp(Color(0.6, 0.6, 0.65))
			_:
				model = ModelFactory.make_tree(Color(0.45, 0.3, 0.15), zone_color.darkened(0.3))
		model.position = pos
		add_child(model)
	# --- 区域专属装饰（草丛/池塘/岩石/雪堆）---
	_spawn_zone_features(center, zone_id, zone_color)
	# --- 弹跳蘑菇（每区域 2 个）---
	if GameState.is_zone_unlocked(zone_id):
		_spawn_jump_pads(center, zone_id)

func _spawn_jump_pads(center: Vector3, zone_id: String) -> void:
	var rng := RandomNumberGenerator.new()
	rng.seed = hash(zone_id) * 17 + 4242
	for i in 2:
		var a = rng.randf() * TAU
		var d = rng.randf_range(4.0, ParkGen.ZONE_SIZE * 0.35)
		var pos = center + Vector3(cos(a) * d, 0, sin(a) * d)
		var pad = Area3D.new()
		pad.set_script(load("res://world/JumpPad.gd"))
		add_child(pad)
		pad.global_position = pos

func _spawn_zone_features(center: Vector3, zone_id: String, zone_color: Color) -> void:
	var rng := RandomNumberGenerator.new()
	rng.seed = hash(zone_id) * 31 + 7777
	match zone_id:
		"grassland":
			# 草地：大量草丛 + 池塘
			for i in 25:
				var a = rng.randf() * TAU
				var d = rng.randf_range(2.0, ParkGen.ZONE_SIZE * 0.42)
				var pos = center + Vector3(cos(a) * d, 0, sin(a) * d)
				var grass = ModelFactory.make_grass_clump(Color(0.3 + rng.randf() * 0.2, 0.6, 0.25))
				grass.position = pos
				add_child(grass)
			# 一个池塘
			var pond = ModelFactory.make_pond(Color(0.3, 0.6, 0.85))
			pond.position = center + Vector3(rng.randf_range(-8, 8), 0, rng.randf_range(-8, 8))
			add_child(pond)
		"beach":
			# 沙滩：岩石 + 贝壳状装饰 + 小水洼
			for i in 12:
				var a = rng.randf() * TAU
				var d = rng.randf_range(3.0, ParkGen.ZONE_SIZE * 0.42)
				var pos = center + Vector3(cos(a) * d, 0, sin(a) * d)
				var rock = ModelFactory.make_rock(Color(0.75, 0.7, 0.6), rng.randf_range(0.3, 0.7))
				rock.position = pos
				rock.rotation_degrees = Vector3(0, rng.randf() * 360, 0)
				add_child(rock)
			# 浅水洼（反光水面）
			var puddle = ModelFactory.make_pond(Color(0.4, 0.7, 0.9))
			puddle.position = center + Vector3(rng.randf_range(-6, 6), 0, rng.randf_range(-6, 6))
			puddle.scale = Vector3(0.6, 1, 0.6)
			add_child(puddle)
		"garden":
			# 花园：密花 + 灌木
			for i in 20:
				var a = rng.randf() * TAU
				var d = rng.randf_range(2.0, ParkGen.ZONE_SIZE * 0.42)
				var pos = center + Vector3(cos(a) * d, 0, sin(a) * d)
				var flower_colors = [Color(1, 0.4, 0.5), Color(1, 0.8, 0.3), Color(0.9, 0.5, 1), Color(1, 0.6, 0.3)]
				var flower = ModelFactory.make_flower(Color(0.3, 0.5, 0.2), flower_colors[rng.randi() % flower_colors.size()])
				flower.position = pos
				add_child(flower)
		"ice":
			# 冰雪：白色岩石（雪堆）+ 冰晶
			for i in 15:
				var a = rng.randf() * TAU
				var d = rng.randf_range(3.0, ParkGen.ZONE_SIZE * 0.42)
				var pos = center + Vector3(cos(a) * d, 0, sin(a) * d)
				var snow = ModelFactory.make_rock(Color(0.9, 0.92, 0.98), rng.randf_range(0.4, 0.9))
				snow.position = pos
				add_child(snow)
			# 冰湖
			var ice_pond = ModelFactory.make_pond(Color(0.6, 0.8, 0.95))
			ice_pond.position = center + Vector3(rng.randf_range(-8, 8), 0, rng.randf_range(-8, 8))
			add_child(ice_pond)

## 生成暖色调 LUT（颜色校正查找表，让画面整体偏暖更温馨）
func _make_warm_lut() -> GradientTexture1D:
	var grad = Gradient.new()
	grad.set_color(0, Color(0.75, 0.6, 0.5))    # 暗部偏暖棕
	grad.set_color(1, Color(1.05, 0.98, 0.92))  # 亮部偏暖白
	grad.set_offset(0, 0.0)
	grad.set_offset(1, 1.0)
	var tex = GradientTexture1D.new()
	tex.gradient = grad
	return tex

## 创建稀有金色星星模型（超强发光五角星 + 光环）
func _make_gold_star() -> Node3D:
	var node = Node3D.new()
	# 主体五角星（用 CSG 球+盒子组合）
	var gold_mat = ModelFactory.get_material(Color(1.0, 0.85, 0.1), {
		"emissive": Color(1.0, 0.8, 0.1),
		"emissive_energy": 1.5,
		"metallic": 0.9,
		"roughness": 0.1,
		"shaded": true,
	})
	# 中心球
	var center = CSGSphere3D.new()
	center.radius = 0.25
	center.material = gold_mat
	node.add_child(center)
	# 5 个星尖
	for i in 5:
		var a = TAU * i / 5.0
		var spike = CSGBox3D.new()
		spike.size = Vector3(0.08, 0.08, 0.35)
		spike.position = Vector3(cos(a) * 0.3, 0, sin(a) * 0.3)
		spike.rotation_degrees = Vector3(0, rad_to_deg(a) + 90, 0)
		spike.material = gold_mat
		node.add_child(spike)
	# 发光光环（OmniLight）
	var halo = OmniLight3D.new()
	halo.light_color = Color(1.0, 0.85, 0.3)
	halo.light_energy = 3.0
	halo.omni_range = 8.0
	node.add_child(halo)
	return node

## glTF 模型缓存（避免每次 load 扫描磁盘）
var _glb_cache: Dictionary = {}

func _load_glb(glb_name: String) -> PackedScene:
	if _glb_cache.has(glb_name):
		return _glb_cache[glb_name]
	var scene = load("res://assets/models/%s.glb" % glb_name)
	if scene:
		_glb_cache[glb_name] = scene
	return scene

## 按收集物类型创建模型（glTF 真实模型优先 → 精细 CSG 降级）
func _make_collectible_model(item_type: String, color: Color) -> Node3D:
	# glTF 模型映射表：item_type → {scene, scale, rotation, tint}
	# 用真实 PBR 模型大幅提升代入感
	var glb_map := {
		# 水果/食物类 → Avocado（PBR 水果质感）
		"apple":  {"scene": "Avocado", "scale": 10.0, "rot": Vector3(0, 0, 0), "tint": color},
		"honey":  {"scene": "Avocado", "scale": 10.0, "rot": Vector3(0, 90, 0), "tint": color},
		# 水生类 → BarramundiFish（真实鱼模型）
		"shell":    {"scene": "BarramundiFish", "scale": 0.8, "rot": Vector3(0, 0, 0), "tint": color},
		"starfish": {"scene": "BarramundiFish", "scale": 0.6, "rot": Vector3(90, 0, 0), "tint": color},
		"pearl":    {"scene": "BarramundiFish", "scale": 0.5, "rot": Vector3(0, 180, 0), "tint": color},
		# 宝物类 → BoxTextured（彩色宝箱）
		"egg":        {"scene": "BoxTextured", "scale": 0.4, "rot": Vector3(0, 0, 0), "tint": color},
		"icecrystal": {"scene": "BoxTextured", "scale": 0.35, "rot": Vector3(45, 45, 0), "tint": color},
	}
	if glb_map.has(item_type):
		var cfg = glb_map[item_type]
		var scene = _load_glb(cfg["scene"])
		if scene:
			var wrapper = Node3D.new()
			var inst = scene.instantiate()
			inst.scale = Vector3(cfg["scale"], cfg["scale"], cfg["scale"])
			inst.rotation_degrees = cfg["rot"]
			# 着色：给所有 mesh 叠加色调（保持 PBR 质感的同时区分颜色）
			_apply_tint(inst, cfg["tint"])
			wrapper.add_child(inst)
			return wrapper
	# 其余类型（flower/petal/butterfly/ladybug/snowflake）用精细 CSG
	return ModelFactory.make_collectible(item_type, color)
	# 其余类型（flower/petal/butterfly/ladybug/snowflake）用精细 CSG
	return ModelFactory.make_collectible(item_type, color)

## 递归给 glTF 模型的所有 mesh 覆盖材质色调
func _apply_tint(node: Node, tint: Color) -> void:
	# 暂时禁用 tint（可能破坏 PBR 材质纹理绑定导致模型不显示）
	# 保留函数签名供未来启用
	pass

## 按区域创建 NPC 模型（真实 glTF 优先 → CSG 动物降级）
func _make_npc_model(zone_id: String, zone_color: Color) -> Node3D:
	# glTF 模型映射：每个区域用不同的真实动物模型
	var npc_glb := {
		"grassland": {"scene": "duck", "scale": 1.5, "rot": Vector3(-90, 0, 0), "tint": null},
		"beach":     {"scene": "duck", "scale": 1.5, "rot": Vector3(-90, 180, 0), "tint": Color(1.0, 0.85, 0.5)},
		"garden":    {"scene": "Fox", "scale": 0.015, "rot": Vector3(0, 0, 0), "tint": Color(0.9, 0.5, 0.3)},
		"ice":       {"scene": "Fox", "scale": 0.015, "rot": Vector3(0, 0, 0), "tint": Color(0.85, 0.9, 1.0)},
	}
	if npc_glb.has(zone_id):
		var cfg = npc_glb[zone_id]
		var scene = _load_glb(cfg["scene"])
		if scene:
			var wrapper = Node3D.new()
			var inst = scene.instantiate()
			var s: float = cfg["scale"]
			inst.scale = Vector3(s, s, s)
			inst.rotation_degrees = cfg["rot"]
			if cfg["tint"] != null:
				_apply_tint(inst, cfg["tint"])
			wrapper.add_child(inst)
			return wrapper
	# 降级：CSG 动物
	return ModelFactory.make_character_by_zone(zone_id, zone_color.darkened(0.15), zone_color.lightened(0.35))

func _unhandled_input(event: InputEvent) -> void:
	# ESC 由 PauseMenu 处理，这里不再处理
	pass
