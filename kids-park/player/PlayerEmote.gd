#============================================================
# PlayerEmote.gd — 玩家表情系统（按数字键做表情动作）
#============================================================
# 1-6 数字键触发不同表情：
#   1=😊挥手 2=❤️爱心 3=⭐星星 4=🎉庆祝 5=💤睡觉 6=🎶唱歌
# 表情以 Label3D 显示在玩家头顶 + 角色动画（缩放/旋转）
# 持续 3 秒后消失
# 拍照打卡时做表情 = 更有趣的照片
#============================================================
extends CanvasLayer

const Confetti = preload("res://world/Confetti.gd")
const EMOTE_DURATION: float = 3.0

const EMOTES := {
	KEY_1: {"emoji": "👋", "name": "挥手", "anim": "wave"},
	KEY_2: {"emoji": "❤️", "name": "爱心", "anim": "heart"},
	KEY_3: {"emoji": "⭐", "name": "星星", "anim": "star"},
	KEY_4: {"emoji": "🎉", "name": "庆祝", "anim": "celebrate"},
	KEY_5: {"emoji": "💤", "name": "睡觉", "anim": "sleep"},
	KEY_6: {"emoji": "🎶", "name": "唱歌", "anim": "sing"},
}

var _emote_label: Label3D = null
var _emote_timer: float = 0.0
var _player: CharacterBody3D = null
var _emote_particles: GPUParticles3D = null

func _ready() -> void:
	# 表情 Label3D（复用，不每次新建）
	_emote_label = Label3D.new()
	_emote_label.font_size = 72
	_emote_label.position = Vector3(0, 2.2, 0)
	_emote_label.billboard = BaseMaterial3D.BILLBOARD_ENABLED
	_emote_label.outline_size = 10
	_emote_label.outline_modulate = Color(0, 0, 0, 0.6)
	_emote_label.visible = false
	# 延迟添加到玩家
	call_deferred("_attach_to_player")

func _attach_to_player() -> void:
	_player = get_tree().get_first_node_in_group("player")
	if _player:
		_player.add_child(_emote_label)

func _unhandled_input(event: InputEvent) -> void:
	if event is InputEventKey and event.pressed:
		if EMOTES.has(event.keycode):
			_play_emote(EMOTES[event.keycode])

func _play_emote(emote: Dictionary) -> void:
	if _player == null:
		_player = get_tree().get_first_node_in_group("player")
		if _player == null:
			return
	if _emote_label.get_parent() == null:
		_player.add_child(_emote_label)
	_emote_label.text = emote["emoji"]
	_emote_label.visible = true
	_emote_label.scale = Vector3(0.5, 0.5, 0.5)
	_emote_timer = EMOTE_DURATION
	# 弹出动画
	var tw = create_tween()
	tw.tween_property(_emote_label, "scale", Vector3(1.3, 1.3, 1.3), 0.15).set_ease(Tween.EASE_OUT)
	tw.tween_property(_emote_label, "scale", Vector3(1.0, 1.0, 1.0), 0.1)
	# 音效
	match emote["anim"]:
		"heart", "celebrate":
			AudioBus.play_sticker()
		"sing":
			AudioBus.play_note(523.0, 0.3, 0.15)
			AudioBus.play_note(659.0, 0.3, 0.15)
		"sleep":
			AudioBus.play_note(200.0, 0.5, 0.1)
		_:
			AudioBus.play_pickup()
	# 庆祝表情发射彩纸
	if emote["anim"] == "celebrate":
		Confetti.burst(get_tree().current_scene, _player.global_position + Vector3(0, 1, 0), Color(1, 0.8, 0.2))

func _process(delta: float) -> void:
	if _emote_timer > 0:
		_emote_timer -= delta
		# 浮动动画
		var t = Time.get_ticks_msec() * 0.005
		_emote_label.position.y = 2.2 + sin(t) * 0.1
		if _emote_timer <= 0:
			_emote_label.visible = false
