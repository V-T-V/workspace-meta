#============================================================
# AudioBus.gd — 音效总线（autoload 单例）
#============================================================
# 用纯代码合成简单音效（无需外部音频文件）：
#   - 拾取：明亮的"叮"声（短促正弦波上扬）
#   - 重生：柔和"咕嘟"声
#   - 任务完成：和弦琶音
#   - 解锁区域：号角声
#   - 跳跃：轻快"啵"
#   - 走路：轻微踏地（低频）
#   - 背景音乐：区域专属旋律循环（C 大调五声音阶）
# 全部用 AudioStreamGenerator 实时生成，零外部依赖。
#============================================================
extends Node

var _player: AudioStreamPlayer
var _generator: AudioStreamGenerator
var _mix_rate: float = 44100.0
var _queue: Array = []   # 待播放音效队列 [{samples, pos}]
var _playing: bool = false

# --- 背景音乐系统 ---
var _music_player: AudioStreamPlayer
var _music_generator: AudioStreamGenerator
var _music_playback: AudioStreamGeneratorPlayback
var _current_zone: String = ""
var _music_note_idx: int = 0
var _music_timer: float = 0.0
var _music_note_duration: float = 0.3   # 每个音符持续时间
var _music_volume: float = 0.12         # BGM 音量（比音效低）
var _music_sample_count: int = 0        # 总采样计数（用于波形相位计算）

# 区域专属旋律（C 大调五声音阶，儿童友好不会刺耳）
# 频率：C4=261.6, D4=293.7, E4=329.6, G4=392, A4=440, C5=523.3
const ZONE_MELODIES := {
	"grassland": [261.6, 329.6, 392.0, 329.6, 440.0, 392.0, 329.6, 261.6],   # 明快上行
	"beach":     [392.0, 440.0, 523.3, 440.0, 392.0, 329.6, 392.0, 440.0],   # 轻柔波浪
	"garden":    [329.6, 392.0, 440.0, 523.3, 440.0, 392.0, 329.6, 261.6],   # 花般绽放
	"ice":       [523.3, 440.0, 392.0, 329.6, 392.0, 440.0, 523.3, 440.0],   # 清脆冰晶
}

func _ready() -> void:
	_player = AudioStreamPlayer.new()
	_player.bus = "Master"
	_generator = AudioStreamGenerator.new()
	_generator.mix_rate = _mix_rate
	_generator.buffer_length = 0.5   # 500ms 缓冲
	_player.stream = _generator
	add_child(_player)
	_player.play()
	# 背景音乐播放器（独立 AudioStreamPlayer）
	_music_player = AudioStreamPlayer.new()
	_music_player.bus = "Master"
	_music_player.volume_db = -6.0   # 比 SFX 低
	_music_generator = AudioStreamGenerator.new()
	_music_generator.mix_rate = _mix_rate
	_music_generator.buffer_length = 1.0   # 1 秒缓冲（音乐需要更长）
	_music_player.stream = _music_generator
	add_child(_music_player)
	_music_player.play()
	_music_playback = _music_player.get_stream_playback()

func _exit_tree() -> void:
	# 停止播放并清空队列，避免 AudioStreamGeneratorPlayback 泄漏
	_queue.clear()
	if _player:
		_player.stop()
	if _music_player:
		_music_player.stop()

func _process(delta: float) -> void:
	_fill_buffer()
	_fill_music_buffer(delta)

## 切换区域背景音乐
func set_zone_music(zone: String) -> void:
	if zone == _current_zone:
		return
	_current_zone = zone
	_music_note_idx = 0
	_music_timer = 0.0

## 停止背景音乐
func stop_music() -> void:
	_current_zone = ""

func _fill_buffer() -> void:
	var playback = _player.get_stream_playback() as AudioStreamGeneratorPlayback
	if playback == null:
		return
	var frames = playback.get_frames_available()
	if frames <= 0:
		return
	for i in frames:
		var sample = 0.0
		# 混合所有正在播放的音效
		var still_playing: Array = []
		for sfx in _queue:
			if sfx.pos < sfx.samples.size():
				sample += sfx.samples[sfx.pos] * sfx.volume
				sfx.pos += 1
				still_playing.append(sfx)
		_queue = still_playing
		# 削峰
		sample = clamp(sample, -1.0, 1.0)
		playback.push_frame(Vector2(sample, sample))

## 填充背景音乐缓冲区（区域专属旋律循环）
func _fill_music_buffer(delta: float) -> void:
	if _music_playback == null or _current_zone == "":
		# 无音乐时填充静音
		if _music_playback:
			var frames = _music_playback.get_frames_available()
			for i in frames:
				_music_playback.push_frame(Vector2.ZERO)
		return
	var melody = ZONE_MELODIES.get(_current_zone, ZONE_MELODIES["grassland"])
	_music_timer += delta
	var frames = _music_playback.get_frames_available()
	for i in frames:
		# 当前音符
		var freq = melody[_music_note_idx % melody.size()]
		var note_pos = _music_timer / _music_note_duration   # 0~1 在当前音符内的位置
		# ADSR 包络（柔和的音符起伏）
		var env = 0.0
		if note_pos < 0.1:
			env = note_pos / 0.1          # Attack
		elif note_pos < 0.7:
			env = 1.0                      # Sustain
		else:
			env = (1.0 - note_pos) / 0.3   # Release
		env = clamp(env, 0.0, 1.0) * _music_volume
		# 正弦波 + 轻微泛音（更柔和）
		var t = _music_sample_count / _mix_rate
		var sample = sin(t * freq * TAU) * env
		sample += sin(t * freq * 2 * TAU) * env * 0.15   # 八度泛音
		_music_sample_count += 1
		# 推进音符
		_music_timer += 1.0 / _mix_rate
		if _music_timer >= _music_note_duration:
			_music_timer = 0.0
			_music_note_idx += 1
		_music_playback.push_frame(Vector2(sample, sample))

## 生成一个正弦波音效样本数组
func _make_tone(freq: float, duration: float, volume: float = 0.3, decay: float = 6.0) -> PackedFloat32Array:
	var n = int(duration * _mix_rate)
	var samples := PackedFloat32Array()
	samples.resize(n)
	for i in n:
		var t = float(i) / _mix_rate
		# 指数衰减包络
		var env = exp(-t * decay)
		samples[i] = sin(t * freq * TAU) * env * volume
	return samples

## 生成琶音（多频率序列）
func _make_arpeggio(freqs: Array, note_dur: float, volume: float = 0.25) -> PackedFloat32Array:
	var samples := PackedFloat32Array()
	for freq in freqs:
		var tone = _make_tone(freq, note_dur, volume, 4.0)
		samples.append_array(tone)
	return samples

func play_pickup() -> void:
	# 上扬"叮"：880Hz → 1320Hz 快速滑音
	_queue.append({"samples": _make_tone(880.0, 0.08, 0.25, 8.0), "pos": 0, "volume": 1.0})
	_queue.append({"samples": _make_tone(1320.0, 0.15, 0.2, 6.0), "pos": 0, "volume": 0.8})

func play_respawn() -> void:
	# 柔和"咕嘟"：440Hz 下滑
	_queue.append({"samples": _make_tone(440.0, 0.2, 0.2, 4.0), "pos": 0, "volume": 1.0})

func play_mission_complete() -> void:
	# C 大调琶音：C5 E5 G5 C6
	_queue.append({"samples": _make_arpeggio([523.0, 659.0, 784.0, 1047.0], 0.12, 0.25), "pos": 0, "volume": 1.0})

func play_zone_unlock() -> void:
	# 号角：低频长音
	_queue.append({"samples": _make_tone(330.0, 0.4, 0.3, 2.0), "pos": 0, "volume": 1.0})
	_queue.append({"samples": _make_tone(440.0, 0.4, 0.25, 2.0), "pos": 0, "volume": 0.8})

func play_jump() -> void:
	# 轻快"啵"：高频短促
	_queue.append({"samples": _make_tone(660.0, 0.1, 0.2, 10.0), "pos": 0, "volume": 1.0})

func play_sticker() -> void:
	# 闪亮音效：高音琶音
	_queue.append({"samples": _make_arpeggio([1047.0, 1319.0, 1568.0], 0.1, 0.22), "pos": 0, "volume": 1.0})

func play_step() -> void:
	# 脚步声：低频短促"嗒"（白噪声+低通近似）
	_queue.append({"samples": _make_tone(120.0, 0.05, 0.12, 25.0), "pos": 0, "volume": 1.0})
