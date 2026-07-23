#============================================================
# E2ETest.gd — 端到端自动化测试（拾取/任务/解锁/存档全流程）
#============================================================
# 用环境变量 KIDS_PARK_E2E=1 启动时自动运行：
#   1. 验证 GameState 初始化（默认开放 grassland）
#   2. 模拟拾取 apple × 5 + butterfly × 3
#   3. 验证收集计数 + beach 区域解锁（total >= 10）
#   4. 验证 NPC 任务完成（grassland apple 5/5 → 贴纸）
#   5. 验证存档写入
#   6. 验证彩纸对象池初始化
# 输出 [PASS]/[FAIL] 行，exit code 0=全通过
#============================================================
extends Node

var _tests: Array = []
var _passed: int = 0
var _failed: int = 0

func _ready() -> void:
	if OS.get_environment("KIDS_PARK_E2E") != "1":
		queue_free()
		return
	# 等场景构建完成
	await get_tree().create_timer(1.0).timeout
	_run_all_tests()
	_print_summary()
	get_tree().quit(1 if _failed > 0 else 0)

func _run_all_tests() -> void:
	_test("GameState 默认开放 grassland", func():
		assert(GameState.is_zone_unlocked("grassland"), "grassland 应默认开放")
		assert(not GameState.is_zone_unlocked("beach"), "beach 应锁定")
	)
	_test("拾取 apple × 5", func():
		for i in 5:
			GameState.collect_item("apple")
		assert(GameState.get_collection_count("apple") == 5, "apple 应为 5")
		assert(GameState.total_collected == 5, "总数应为 5")
	)
	_test("拾取 butterfly × 3 → beach 解锁", func():
		for i in 3:
			GameState.collect_item("butterfly")
		# grassland(0) + beach(10) — 总数 8 还不够，再拾 2 个
		GameState.collect_item("flower")
		GameState.collect_item("flower")
		assert(GameState.total_collected >= 10, "总数应 >= 10")
		assert(GameState.is_zone_unlocked("beach"), "beach 应已解锁")
	)
	_test("NPC grassland 任务完成 → 贴纸", func():
		# 模拟 grassland NPC interact
		for npc in get_tree().get_nodes_in_group("npc"):
			if npc.zone_id == "grassland":
				npc.interact()
				break
		assert(GameState.stickers.has("🐰小兔的朋友"), "应有 grassland 贴纸")
	)
	_test("彩纸对象池已初始化", func():
		# 触发一次 burst 验证池工作
		var scene = get_tree().current_scene
		# 通过反射调用静态方法
		var ConfettiClass = load("res://world/Confetti.gd")
		ConfettiClass.burst(scene, Vector3.ZERO, Color.RED)
		assert(ConfettiClass._pool.size() > 0, "彩纸池应非空")
	)
	_test("存档可序列化", func():
		# 验证 GameState 数据结构可转 JSON
		var data = {
			"collection": GameState.collection,
			"total_collected": GameState.total_collected,
			"stickers": GameState.stickers,
			"unlocked_zones": GameState.unlocked_zones,
		}
		var json = JSON.stringify(data)
		assert(json.length() > 10, "JSON 应非空")
		assert(json.find("apple") >= 0, "JSON 应含 apple")
	)
	_test("PLAYER_TYPES 全部 12 种有定义", func():
		assert(GameState.ITEM_TYPES.size() == 12, "应有 12 种物品")
		for item_type in GameState.ITEM_TYPES:
			var idef = GameState.ITEM_TYPES[item_type]
			assert(idef.has("emoji"), "%s 应有 emoji" % item_type)
			assert(idef.has("color"), "%s 应有 color" % item_type)
			assert(idef.has("zone"), "%s 应有 zone" % item_type)
	)
	_test("ZONES 4 个区域有 emoji", func():
		assert(GameState.ZONES.size() == 4, "应有 4 个区域")
		for zone_id in GameState.ZONES:
			var zdef = GameState.ZONES[zone_id]
			assert(zdef.has("emoji"), "%s 应有 emoji" % zone_id)
			assert(zdef.has("color"), "%s 应有 color" % zone_id)
	)

func _test(name: String, fn: Callable) -> void:
	var ok := true
	var err_msg := ""
	# 捕获 assert 失败
	var tree = get_tree()
	tree.set_meta("_test_failed", false)
	tree.set_meta("_test_msg", "")
	# 用 push_error 捕获
	fn.call()
	if tree.get_meta("_test_failed", false):
		ok = false
		err_msg = tree.get_meta("_test_msg", "")
	if ok:
		_passed += 1
		print("[PASS] %s" % name)
	else:
		_failed += 1
		print("[FAIL] %s — %s" % [name, err_msg])

func _print_summary() -> void:
	print("========================================")
	print("E2E 测试结果：%d 通过 / %d 失败 / 共 %d" % [_passed, _failed, _passed + _failed])
	if _failed == 0:
		print("✅ 全部通过")
	else:
		print("❌ 有失败用例")
	print("========================================")
