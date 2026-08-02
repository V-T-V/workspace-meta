#============================================================
# MusicRecorder.gd — 音乐录制回放（踩音乐垫的序列可录制+回放）
#============================================================
# 按 N 键开始/停止录制：记录玩家踩音乐垫的音符序列+时间戳
# 按 B 键回放：自动重新触发音符序列
# 录制的歌曲保存到存档（最多 3 首）
#============================================================
extends CanvasLayer

const MAX_SONGS: int = 3

const Confetti = preload("res://world/Confetti.gd")
var _recording: bool = false
var _recorded_notes: Array = []   # [{freq, time}]
var _record_start: float = 0.0
var _status_label: Label
var _saved_songs: Array = []   # [[{freq,time}], ...]

func _ready() -> void:
	_build_ui()
	# 监听音乐垫触发
	EventBus.item_collected.connect(func(_t, _c): pass)   # 不用这个
	# 加载存档歌曲
	_saved_songs = GameState.get_meta("saved_songs", [])

func _build_ui() -> void:
	var root = Control.new()
	root.set_anchors_preset(Control.PRESET_FULL_RECT)
	root.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(root)
	_status_label = Label.new()
	_status_label.add_theme_font_size_override("font_size", 22)
	_status_label.add_theme_color_override("font_color", Color(0.8, 0.6, 1.0))
	_status_label.set_anchors_preset(Control.PRESET_CENTER_TOP)
	_status_label.position = Vector2(-100, 95)
	_status_label.visible = false
	root.add_child(_status_label)

func _unhandled_input(event: InputEvent) -> void:
	if event is InputEventKey and event.pressed:
		match event.keycode:
			KEY_N:
				if _recording:
					_stop_recording()
				else:
					_start_recording()
			KEY_B:
				_playback()

func _start_recording() -> void:
	_recording = true
	_recorded_notes.clear()
	_record_start = Time.get_ticks_msec() / 1000.0
	_status_label.text = "🔴 录制中... 踩音乐垫！（N 停止）"
	_status_label.visible = true
	EventBus.toast_message.emit("开始录制音乐！踩音乐垫演奏", "🔴")
	AudioBus.play_pickup()

func _stop_recording() -> void:
	_recording = false
	_status_label.visible = false
	if _recorded_notes.size() < 3:
		EventBus.toast_message.emit("音符太少，取消录制", "❌")
		_recorded_notes.clear()
		return
	# 保存歌曲
	if _saved_songs.size() >= MAX_SONGS:
		_saved_songs.pop_front()   # 删最老的
	_saved_songs.append(_recorded_notes.duplicate(true))
	GameState.set_meta("saved_songs", _saved_songs)
	EventBus.toast_message.emit("录制完成！%d 个音符（B 回放）" % _recorded_notes.size(), "💾")
	AudioBus.play_sticker()

## 供 MusicPad 调用：记录一个音符
func record_note(freq: float) -> void:
	if not _recording:
		return
	var t = Time.get_ticks_msec() / 1000.0 - _record_start
	_recorded_notes.append({"freq": freq, "time": t})

func _playback() -> void:
	if _saved_songs.is_empty():
		EventBus.toast_message.emit("还没有录制的歌曲", "🎵")
		return
	# 回放最后一首
	var song = _saved_songs[-1]
	EventBus.toast_message.emit("回放歌曲...（%d 音符）" % song.size(), "🎵")
	AudioBus.play_pickup()
	# 按时间戳播放
	for note in song:
		var delay = note["time"]
		var freq = note["freq"]
		get_tree().create_timer(delay).timeout.connect(func():
			AudioBus.play_note(freq, 0.3, 0.15)
		)
	# 回放结束彩纸
	var total_duration = song[-1]["time"] + 0.5
	get_tree().create_timer(total_duration).timeout.connect(func():
		Confetti.burst(get_tree().current_scene, get_tree().get_first_node_in_group("player").global_position, Color(0.8, 0.6, 1.0))
		EventBus.toast_message.emit("歌曲回放结束！", "🎵")
	)
