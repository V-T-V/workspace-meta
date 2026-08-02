#============================================================
# CollectibleChain.gd — 收集物连锁提示（拾取后附近物品发光）
#============================================================
# 监听 item_collected 信号
# 拾取后，找到最近的未收集物品，给它一个临时高亮光环
# 引导儿童"下一个去哪里"（减少迷路挫败感）
#============================================================
extends Node3D

const HIGHLIGHT_DURATION: float = 3.0   # 高亮持续 3 秒
const SEARCH_RADIUS: float = 15.0       # 搜索半径
const HIGHLIGHT_COLOR: Color = Color(1.0, 0.9, 0.3, 0.4)

var _highlighted: Node3D = null
var _highlight_timer: float = 0.0
var _halo: OmniLight3D = null
var _player: CharacterBody3D = null

func _ready() -> void:
	# 高亮光环（OmniLight，跟随被高亮的物品）
	_halo = OmniLight3D.new()
	_halo.light_color = HIGHLIGHT_COLOR
	_halo.light_energy = 0.0
	_halo.omni_range = 3.0
	_halo.visible = false
	add_child(_halo)
	EventBus.item_collected.connect(_on_item_collected)

func _process(delta: float) -> void:
	if _player == null or not is_instance_valid(_player):
		_player = get_tree().get_first_node_in_group("player")
	# 高亮计时
	if _highlight_timer > 0:
		_highlight_timer -= delta
		# 光环跟随被高亮物品 + 脉冲
		if _highlighted and is_instance_valid(_highlighted):
			_halo.global_position = _highlighted.global_position + Vector3(0, 0.5, 0)
			var pulse = 1.0 + sin(Time.get_ticks_msec() * 0.008) * 0.5
			_halo.light_energy = 2.0 * pulse
		if _highlight_timer <= 0:
			_halo.visible = false
			_highlighted = null

func _on_item_collected(_item_type: String, _count: int) -> void:
	if _player == null:
		return
	# 找最近的未收集物品
	var nearest: Node3D = null
	var nearest_dist = SEARCH_RADIUS
	for c in get_tree().get_nodes_in_group("collectible"):
		if not c is Area3D:
			continue
		if c.get("_collected"):
			continue   # 已收集（重生中）
		var d = _player.global_position.distance_to(c.global_position)
		if d < nearest_dist and d > 2.0:   # 排除太近的（已经在眼前）
			nearest_dist = d
			nearest = c
	if nearest:
		_highlighted = nearest
		_highlight_timer = HIGHLIGHT_DURATION
		_halo.visible = true
