#============================================================
# ParkGen.gd — 乐园布局生成（autoload 单例）
#============================================================
# 生成 4 个彩色区域地面 + 散布收集物 + NPC 摆放位置。
# 区域在 XZ 平面上排成 2×2 方阵。
#============================================================
extends Node

const ZONE_SIZE: float = 40.0      # 每个区域边长（世界单位）
const ITEMS_PER_ZONE: int = 12     # 每个区域初始收集物数量
const NPC_PER_ZONE: int = 1        # 每个区域 NPC 数量

# 4 个区域的中心坐标
const ZONE_CENTERS := {
	"grassland": Vector3(-25, 0, -25),
	"beach":     Vector3(25, 0, -25),
	"garden":    Vector3(-25, 0, 25),
	"ice":       Vector3(25, 0, 25),
}

## 生成区域布局数据：返回 {zone_id -> {position, color, items: [{type,pos,rare}], npcs: [{pos}]}}
func generate_park() -> Dictionary:
	var result := {}
	for zone_id in GameState.ZONES:
		var center = ZONE_CENTERS[zone_id]
		var zone_color: Color = GameState.ZONES[zone_id]["color"]
		var zone_items: Array = GameState.get_zone_items(zone_id)
		# 收集物（含 1 个稀有金色星星）
		var items: Array = []
		var rng := RandomNumberGenerator.new()
		rng.seed = hash(zone_id)
		for i in ITEMS_PER_ZONE:
			var angle = rng.randf() * TAU
			var dist = rng.randf_range(3.0, ZONE_SIZE * 0.45)
			var item_type = zone_items[rng.randi() % zone_items.size()]
			items.append({
				"type": item_type,
				"pos": center + Vector3(cos(angle) * dist, 0.5, sin(angle) * dist),
				"rare": false,
			})
		# 稀有金色星星（每区域 1 个，位置较隐蔽）
		var rare_angle = rng.randf() * TAU
		var rare_dist = rng.randf_range(ZONE_SIZE * 0.35, ZONE_SIZE * 0.45)
		items.append({
			"type": "goldstar",
			"pos": center + Vector3(cos(rare_angle) * rare_dist, 1.0, sin(rare_angle) * rare_dist),
			"rare": true,
		})
		# NPC
		var npcs: Array = []
		var npc_angle = rng.randf() * TAU
		npcs.append({
			"pos": center + Vector3(cos(npc_angle) * 6.0, 0, sin(npc_angle) * 6.0),
			"zone": zone_id,
		})
		result[zone_id] = {
			"center": center,
			"color": zone_color,
			"items": items,
			"npcs": npcs,
		}
	return result

## 获取玩家出生点（草地中心）
func get_spawn() -> Vector3:
	return ZONE_CENTERS["grassland"] + Vector3(0, 1, 0)
