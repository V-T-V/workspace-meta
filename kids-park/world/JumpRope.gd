#============================================================
# JumpRope.gd — 跳绳迷你游戏（按 J 启动）
#============================================================
# 玩家靠近任意 NPC 时按 J 开始：
#   - 一根跳绳每 SWING_PERIOD 秒摆动一圈
#   - 绳子扫过最低点的 ±JUMP_WINDOW 秒为"起跳窗口"
#   - 在窗口内按 Space = 成功（combo+1，奖励递增）
#   - 错过/过早 = combo 清零（无惩罚，仅鼓励）
#   - 60 秒结束，combo>=10 给 "🤸跳绳达人" 贴纸 + 大量物品
# 儿童友好：没有失败惩罚，只有正反馈；节奏提示用大 emoji
#============================================================
extends CanvasLayer

const Confetti = preload("res://world/Confetti.gd")
const SESSION_TIME: float = 60.0
const SWING_PERIOD: float = 1.4      # 绳子摆一圈的时长（秒）
const JUMP_WINDOW: float = 0.32      # 起跳判定窗口（±秒）
const NEAR_NPC_DIST: float = 5.0

var _active: bool = false
var _timer: float = 0.0              # 本局剩余时间
var _swing_t: float = 0.0            # 绳子相位 [0, SWING_PERIOD)
var _combo: int = 0
var _best_combo: int = 0
var _successes: int = 0
var _last_judged_phase: float = -1.0 # 上次判定过的相位峰值，避免同一峰重复判定
var _npc: CharacterBody3D = null
# 视觉
var _timer_label: Label
var _combo_label: Label
var _prompt_label: Label
var _rope_root: Node3D = null        # 3D 跳绳组（2 把手 + 绳）
var _rope_a: MeshInstance3D = null   # 绳子（用旋转的细圆柱表示）
var _handle_a: Node3D = null
var _handle_b: Node3D = null

func _ready() -> void:
	_build_ui()

func _build_ui() -> void:
	var root = Control.new()
	root.set_anchors_preset(Control.PRESET_FULL_RECT)
	root.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(root)
	# 顶部计时器
	_timer_label = Label.new()
	_timer_label.text = ""
	_timer_label.add_theme_font_size_override("font_size", 40)
	_timer_label.add_theme_color_override("font_color", Color(0.9, 0.4, 0.6))
	_timer_label.set_anchors_preset(Control.PRESET_CENTER_TOP)
	_timer_label.position = Vector2(-80, 60)
	_timer_label.visible = false
	root.add_child(_timer_label)
	# 中央 combo
	_combo_label = Label.new()
	_combo_label.text = ""
	_combo_label.add_theme_font_size_override("font_size", 56)
	_combo_label.add_theme_color_override("font_color", Color(0.95, 0.6, 0.2))
	_combo_label.set_anchors_preset(Control.PRESET_CENTER)
	_combo_label.position = Vector2(-60, -80)
	_combo_label.visible = false
	root.add_child(_combo_label)
	# 起跳提示（大 emoji，节奏脉动）
	_prompt_label = Label.new()
	_prompt_label.text = ""
	_prompt_label.add_theme_font_size_override("font_size", 90)
	_prompt_label.set_anchors_preset(Control.PRESET_CENTER)
	_prompt_label.position = Vector2(-60, -160)
	_prompt_label.visible = false
	root.add_child(_prompt_label)

func _unhandled_input(event: InputEvent) -> void:
	if event is InputEventKey and event.pressed and event.keycode == KEY_J:
		if not _active:
			_try_start()
	# 跳绳中按 Space 判定
	if _active and event is InputEventKey and event.pressed and event.keycode == KEY_SPACE:
		_judge()

## 启动条件：附近有 NPC
func _try_start() -> void:
	var player = get_tree().get_first_node_in_group("player")
	if player == null:
		return
	var nearest = null
	var nd = NEAR_NPC_DIST
	for npc in get_tree().get_nodes_in_group("npc"):
		var d = player.global_position.distance_to(npc.global_position)
		if d < nd:
			nd = d
			nearest = npc
	if nearest == null:
		EventBus.toast_message.emit("靠近小动物才能跳绳哦", "🤸")
		return
	_npc = nearest
	_start()

func _start() -> void:
	_active = true
	_timer = SESSION_TIME
	_swing_t = 0.0
	_combo = 0
	_best_combo = 0
	_successes = 0
	_last_judged_phase = -1.0
	_timer_label.visible = true
	_combo_label.visible = true
	_prompt_label.visible = true
	_spawn_rope()
	EventBus.toast_message.emit("跳绳开始！跟着节奏按 Space！", "🤸")
	AudioBus.play_zone_unlock()

## 在玩家面前生成跳绳模型（2 把手 + 1 根绳）
func _spawn_rope() -> void:
	var player = get_tree().get_first_node_in_group("player")
	if player == null or _npc == null:
		return
	# 绳子摆在玩家与 NPC 中间
	var center = (player.global_position + _npc.global_position) * 0.5
	_rope_root = Node3D.new()
	get_tree().current_scene.add_child(_rope_root)
	_rope_root.global_position = center
	# 把手 A / B（两侧矮柱）
	_handle_a = _make_handle(Vector3(-1.2, 0, 0))
	_handle_b = _make_handle(Vector3(1.2, 0, 0))
	_rope_root.add_child(_handle_a)
	_rope_root.add_child(_handle_b)
	# 绳子：一根细长圆柱，围绕 X 轴旋转模拟摆动
	_rope_a = MeshInstance3D.new()
	var rope_mesh = CylinderMesh.new()
	rope_mesh.top_radius = 0.03
	rope_mesh.bottom_radius = 0.03
	rope_mesh.height = 2.4
	var rmat = StandardMaterial3D.new()
	rmat.albedo_color = Color(0.95, 0.7, 0.3)
	rmat.emissive = Color(0.6, 0.4, 0.1)
	rmat.emissive_energy_multiplier = 0.3
	_rope_a.mesh = rope_mesh
	_rope_a.material_override = rmat
	_rope_a.position = Vector3(0, 0.6, 0)   # 旋转中心在地面以上
	_rope_root.add_child(_rope_a)

func _make_handle(offset: Vector3) -> Node3D:
	var node = Node3D.new()
	var post = CSGCylinder3D.new()
	post.radius = 0.06
	post.height = 1.2
	post.position = offset + Vector3(0, 0.6, 0)
	var mat = StandardMaterial3D.new()
	mat.albedo_color = Color(0.4, 0.4, 0.45)
	post.material = mat
	node.add_child(post)
	return node

func _process(delta: float) -> void:
	if not _active:
		return
	_timer -= delta
	if _timer <= 0:
		_end()
		return
	# 推进绳子相位
	_swing_t = fmod(_swing_t + delta, SWING_PERIOD)
	# 绳子旋转：相位 0 = 顶部，相位 SWING_PERIOD/2 = 底部（玩家需在此起跳）
	# 用 sin 映射到旋转角，底部对应绳子水平贴地
	var phase_norm = _swing_t / SWING_PERIOD   # [0,1)
	var angle = phase_norm * TAU   # 一圈
	if _rope_a:
		_rope_a.rotation_degrees.x = rad_to_deg(angle)
	# 自动判定"漏跳"：过底部峰值后若未判定，则 combo 清零（在 _judge 内处理窗口）
	# 提示显示：接近底部时放大 emoji
	var dist_to_bottom = _phase_distance_to_bottom(phase_norm)
	_prompt_label.text = "🦘"
	_prompt_label.modulate.a = 1.0 - clamp(dist_to_bottom * 3.0, 0.0, 0.8)
	# 更新计时 + combo 显示
	_timer_label.text = "🤸 %.0f 秒" % max(0, _timer)
	_combo_label.text = "×%d 连击" % _combo if _combo > 0 else ""

## 当前相位到"底部峰值"(0.5)的距离（归一化 0~0.5）
func _phase_distance_to_bottom(phase_norm: float) -> float:
	var target = 0.5
	var d = abs(phase_norm - target)
	return min(d, 1.0 - d)

## 玩家按 Space 的判定
func _judge() -> void:
	var phase_norm = _swing_t / SWING_PERIOD
	# 距底部峰值的时间（秒）
	var dist_phase = _phase_distance_to_bottom(phase_norm)
	var dist_time = dist_phase * SWING_PERIOD
	if dist_time <= JUMP_WINDOW:
		# 成功！combo+1
		_combo += 1
		_successes += 1
		_best_combo = max(_best_combo, _combo)
		AudioBus.play_jump()
		# 每连击奖励
		var reward = _reward_for_combo(_combo)
		GameState.collect_item(reward["item"], reward["amount"])
		EventBus.toast_message.emit("完美！+1 连击 (%d)" % _combo, "✨")
		if _combo % 5 == 0:
			Confetti.burst(get_tree().current_scene, _rope_root.global_position + Vector3(0, 1, 0), Color(1, 0.8, 0.2))
			AudioBus.play_sticker()
	else:
		# 太早/太晚：combo 清零（无其他惩罚）
		if _combo >= 3:
			EventBus.toast_message.emit("差一点点～继续！", "😅")
		_combo = 0

## 连击奖励：每 3 连击 +1 物品，10 连击额外给珍珠
func _reward_for_combo(combo: int) -> Dictionary:
	if combo > 0 and combo % 10 == 0:
		return {"item": "pearl", "amount": 1}
	return {"item": "apple", "amount": 1}

## 结束局：清理 + 发奖
func _end() -> void:
	_active = false
	_timer_label.visible = false
	_combo_label.visible = false
	_prompt_label.visible = false
	_clear_rope()
	# 总结奖励
	if _best_combo >= 10:
		GameState.earn_sticker("🤸跳绳达人")
		GameState.collect_item("pearl", 3)
		EventBus.toast_message.emit("跳绳大师！最高 %d 连击 + 贴纸！" % _best_combo, "🏆")
		AudioBus.play_mission_complete()
	elif _best_combo >= 3:
		GameState.collect_item("apple", 2)
		EventBus.toast_message.emit("跳了 %d 次！加油～" % _successes, "🤸")
		AudioBus.play_pickup()
	else:
		EventBus.toast_message.emit("下次再来挑战吧～", "🤸")

## 清理 3D 跳绳
func _clear_rope() -> void:
	if _rope_root and is_instance_valid(_rope_root):
		_rope_root.queue_free()
	_rope_root = null
	_rope_a = null
	_handle_a = null
	_handle_b = null
