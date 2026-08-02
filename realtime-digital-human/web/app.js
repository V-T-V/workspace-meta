// 实时数字人前端（深度优化版）
// 修复要点（本轮）：
//   P0-2 帧渲染：createImageBitmap + rAF 节流 + 丢过期帧（消除 GC 抖动）
//   P0-1 音频播放：Web Audio API + AudioBufferSourceNode 无缝拼接（消除 MP3 帧边界爆音）
//   P0-2 音频采集：buffer 4096→2048，AGC 关闭（降低延迟）
//   P1-3 重连状态：记录 wasRecording，重连后提示
//   P1-4 指数退避：最多 10 次，封顶 30s
//   P2-9 说话指示：数字人回复时显示 speaking-indicator

const WS_MSG_FRAME = 0x01;
const WS_MSG_AUDIO = 0x02;
const WS_MSG_TEXT = 0x03;
const WS_MSG_END = 0x04;
const WS_MSG_LATENCY = 0x05;
const WS_MSG_UTTERANCE_END = 0x10;
const WS_MSG_INTERRUPT = 0x11;
const WS_MSG_ENGINE_STATUS = 0x14;

// P3-12：字幕区自动滚到底（长对话不溢出）
function scrollSubtitleToBottom() {
  const sub = document.getElementById("subtitle");
  sub.scrollTop = sub.scrollHeight;
}

// DOM
const statusEl = document.getElementById("status");
const micBtn = document.getElementById("mic-btn");
const stopBtn = document.getElementById("stop-btn");
const indicator = document.getElementById("indicator");
const canvas = document.getElementById("avatar-canvas");
const ctx = canvas.getContext("2d");
const subUser = document.getElementById("sub-user");
const subAssistant = document.getElementById("sub-assistant");
const latFrame = document.getElementById("lat-frame");
const latToken = document.getElementById("lat-token");
// P2-7：音量控制 + #2 角色选择器（null-safe，防 DOM 未就绪）
const volumeSlider = document.getElementById("volume");
const muteBtn = document.getElementById("mute-btn");
const personaSelect = document.getElementById("persona-select");
let currentVolume = 1.0;  // 0.0 - 1.0
let isMuted = false;

// ---------- 状态 ----------
let ws = null;
let mediaStream = null;
let audioCtx = null;          // 录音用的 AudioContext
let sourceNode = null;
let processor = null;
let recording = false;
let wsConnected = false;
let wasRecordingBeforeDisconnect = false;  // P1-3：记录断线前意图
let retryCount = 0;            // P1-4：重连计数
const MAX_RETRY = 10;

// ---------- 默认画像 ----------
function drawPlaceholder(text = "等待连接…") {
  ctx.fillStyle = "#000";
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  ctx.fillStyle = "#4fd1c5";
  ctx.font = "24px sans-serif";
  ctx.textAlign = "center";
  ctx.fillText(text, canvas.width / 2, canvas.height / 2);
}
drawPlaceholder();

// ---------- 错误/状态 UI ----------
function setStatus(text, cls) {
  statusEl.textContent = text;
  statusEl.className = "status " + (cls || "");
}
function showError(msg) {
  setStatus("⚠ " + msg, "disconnected");
  setTimeout(() => {
    if (wsConnected) setStatus("已连接", "connected");
    else setStatus("未连接", "");
  }, 3000);
}

// ---------- #2 角色选择 ----------
async function loadPersonas() {
  if (!personaSelect) return;  // DOM 未就绪
  try {
    const r = await fetch("/personas");
    const data = await r.json();
    if (data.personas && data.personas.length > 0) {
      personaSelect.innerHTML = "";
      data.personas.forEach((p) => {
        const opt = document.createElement("option");
        opt.value = p.id;
        opt.textContent = p.name;
        if (p.id === data.default) opt.selected = true;
        personaSelect.appendChild(opt);
      });
      personaSelect.style.display = "inline-block";
    }
  } catch (e) {
    console.warn("加载角色列表失败:", e);
  }
}
if (personaSelect) {
  personaSelect.addEventListener("change", (e) => {
    const personaId = e.target.value;
    sendBytes(0x12, new TextEncoder().encode(personaId));  // WS_MSG_SET_PERSONA
    subAssistant.textContent = "";  // 切角色清空旧字幕
  });
}

// ---------- #3 加载对话历史 ----------
async function loadHistory() {
  try {
    const sid = getSessionId();
    const r = await fetch(`/sessions/${sid}/history`);
    const data = await r.json();
    if (data.history && data.history.length > 0) {
      // 显示最近几轮的历史（只展示最后 6 条避免太长）
      const recent = data.history.slice(-6);
      subAssistant.textContent = recent
        .map((m) => m.role === "user" ? `我：${m.content}` : `数字人：${m.content}`)
        .join("\n");
      scrollSubtitleToBottom();
    }
  } catch (e) {
    console.debug("加载历史失败（可能未启用持久化）:", e);
  }
}

// ---------- WS 连接 ----------
function getSessionId() {
  const saved = localStorage.getItem("dh_session");
  if (saved) return saved;
  const id = "web-" + Math.random().toString(36).slice(2, 10);
  localStorage.setItem("dh_session", id);
  return id;
}

function connect() {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  const sid = getSessionId();
  // P1：支持 token 鉴权（URL ?token=xxx 自动透传给 WS）
  const queryString = location.search || "";  // 含 ?token=xxx
  ws = new WebSocket(`${proto}://${location.host}/ws/${sid}${queryString}`);
  ws.binaryType = "arraybuffer";

  ws.onopen = () => {
    wsConnected = true;
    retryCount = 0;  // P1-4：重置重连计数
    setStatus("已连接", "connected");
    drawPlaceholder("点麦克风开始对话");
    // P1-3：若断线前正在录音，提示用户重新点（不自动恢复，避免静默录音）
    if (wasRecordingBeforeDisconnect) {
      wasRecordingBeforeDisconnect = false;
      showError("连接已恢复，请重新点麦克风开始对话");
    }
    // #2：加载角色列表
    loadPersonas();
    // #3：加载对话历史（持久化恢复）
    loadHistory();
  };

  ws.onclose = () => {
    wsConnected = false;
    setStatus("已断开，重连中…", "disconnected");
    if (recording) {
      wasRecordingBeforeDisconnect = true;  // P1-3：记录意图
      stopRecording(true);
    }
    // P1-4：指数退避 + 上限
    if (retryCount < MAX_RETRY) {
      const delay = Math.min(2000 * Math.pow(1.5, retryCount), 30000);
      retryCount++;
      setTimeout(connect, delay);
    } else {
      setStatus("连接失败，请刷新页面", "disconnected");
    }
  };

  ws.onerror = () => { console.error("WS error"); };

  ws.onmessage = (ev) => {
    if (typeof ev.data === "string") return;
    const buf = new Uint8Array(ev.data);
    if (buf.length < 1) return;
    handleMessage(buf[0], buf.subarray(1));  // P1-3：subarray 零拷贝 view
  };
}

function sendBytes(type, payload) {
  if (!ws || ws.readyState !== WebSocket.OPEN) return false;
  const out = new Uint8Array(1 + payload.length);
  out[0] = type;
  out.set(payload, 1);
  ws.send(out.buffer);
  return true;
}

// ---------- 消息处理 ----------
function handleMessage(type, payload) {
  switch (type) {
    case WS_MSG_FRAME:
      drawFrame(payload);
      break;
    case WS_MSG_AUDIO:
      showSpeaking(true);  // P2-9：数字人开始说话
      enqueueAudio(payload);
      break;
    case WS_MSG_TEXT: {
      const text = new TextDecoder().decode(payload);
      if (text.startsWith("[user]")) {
        subUser.textContent = text.slice(6).trim();
      } else if (text.startsWith("[assistant]")) {
        subAssistant.textContent += text.slice(11).trim();
        scrollSubtitleToBottom();  // P3-12
      } else if (text.startsWith("[error:fatal]")) {
        showError("❌ " + text.slice(13).trim());  // m7：致命错误醒目提示
      } else if (text.startsWith("[error:warn]")) {
        showError("⚠ " + text.slice(12).trim());   // m7：警告级
      } else if (text.startsWith("[error]")) {
        showError(text.slice(7).trim());            // 兼容旧格式
      }
      break;
    }
    case WS_MSG_END:
      indicator.style.display = "none";
      showSpeaking(false);  // P2-9：说完
      break;
    case WS_MSG_INTERRUPT:
      // #1 Barge-in：用户开口打断，立即停止音频播放 + 清队列
      interruptPlayback();
      showSpeaking(false);
      break;
    case WS_MSG_LATENCY: {
      try {
        const s = JSON.parse(new TextDecoder().decode(payload));
        if (s.first_frame_ms != null) latFrame.textContent = (s.first_frame_ms / 1000).toFixed(2) + "s";
        if (s.first_token_ms != null) latToken.textContent = (s.first_token_ms / 1000).toFixed(2) + "s";
      } catch (e) {}
      break;
    case WS_MSG_ENGINE_STATUS:
      // 引擎状态：显示降级警告（如 Ollama 未装、ASR 降级）
      try {
        const st = JSON.parse(new TextDecoder().decode(payload));
        if (st.warnings && st.warnings.length > 0) {
          showError(st.warnings.join("；"));
        }
      } catch (e) {}
      break;
    }
  }
}

// ---------- P0-2 帧渲染：createImageBitmap + rAF 节流 + 丢过期帧 ----------
let pendingBitmap = null;
let rafScheduled = false;

async function drawFrame(jpegBytes) {
  try {
    // createImageBitmap：异步解码、不创建 DOM 节点、性能优于 Image
    const bmp = await createImageBitmap(new Blob([jpegBytes], { type: "image/jpeg" }));
    // 只保留最新帧（丢过期帧，避免堆积）
    if (pendingBitmap) pendingBitmap.close();
    pendingBitmap = bmp;
    if (!rafScheduled) {
      rafScheduled = true;
      requestAnimationFrame(() => {
        rafScheduled = false;
        if (pendingBitmap) {
          ctx.drawImage(pendingBitmap, 0, 0, canvas.width, canvas.height);
          pendingBitmap.close();
          pendingBitmap = null;
        }
      });
    }
  } catch (e) {
    console.warn("drawFrame 失败:", e);
  }
}

// ---------- P2-9 说话指示 ----------
let speakingIndicator = null;
function showSpeaking(on) {
  if (on && !speakingIndicator) {
    speakingIndicator = document.createElement("div");
    speakingIndicator.className = "speaking-indicator";
    speakingIndicator.textContent = "正在回答…";
    canvas.parentElement.appendChild(speakingIndicator);
  } else if (!on && speakingIndicator) {
    speakingIndicator.remove();
    speakingIndicator = null;
  }
}

// ---------- P0-1 音频播放：Web Audio 无缝拼接 ----------
// 服务端 TTS 输出 PCM16（16k mono），前端转 Float32 填 AudioBuffer 播放。
// P0 修复：之前服务端输出 MP3，下游 MuseTalk 拿到乱码；现统一 PCM16，前端也用 PCM。
// P2-7：通过 GainNode 控制音量/静音。
let playbackCtx = null;
let gainNode = null;
let nextStartTime = 0;
const TTS_SAMPLE_RATE = 16000;  // 与服务端 PCM16 一致

async function enqueueAudio(pcm16Bytes) {
  try {
    if (!playbackCtx) {
      playbackCtx = new (window.AudioContext || window.webkitAudioContext)();
      gainNode = playbackCtx.createGain();
      gainNode.gain.value = currentVolume;
      gainNode.connect(playbackCtx.destination);
      nextStartTime = playbackCtx.currentTime;
    }
    // PCM16 → Float32（-1.0 ~ 1.0）
    const sampleCount = pcm16Bytes.byteLength / 2;
    const float32 = new Float32Array(sampleCount);
    const view = new DataView(pcm16Bytes.buffer, pcm16Bytes.byteOffset, pcm16Bytes.byteLength);
    for (let i = 0; i < sampleCount; i++) {
      float32[i] = view.getInt16(i * 2, true) / 32768;  // little-endian PCM16
    }
    // 填 AudioBuffer（用源采样率 16k，AudioContext 自动重采样到设备率）
    const audioBuf = playbackCtx.createBuffer(1, sampleCount, TTS_SAMPLE_RATE);
    audioBuf.copyToChannel(float32, 0);
    const src = playbackCtx.createBufferSource();
    src.buffer = audioBuf;
    src.connect(gainNode);
    const now = playbackCtx.currentTime;
    const startTime = Math.max(now, nextStartTime);
    src.start(startTime);
    src.onended = () => { if (currentSource === src) currentSource = null; };
    currentSource = src;  // #1：记录用于打断
    nextStartTime = startTime + audioBuf.duration;
  } catch (e) {
    console.warn("音频播放失败:", e);
  }
}

// #1 Barge-in：打断当前音频播放（用户开口时调用）
let currentSource = null;  // 当前正在播的 AudioBufferSourceNode
function interruptPlayback() {
  if (currentSource) {
    try { currentSource.stop(); } catch (e) {}  // 立即停
    currentSource = null;
  }
  audioQueue.length = 0;  // 清空待播队列
  if (playbackCtx) {
    nextStartTime = playbackCtx.currentTime;  // 重置衔接时间
  }
}

// P2-7：音量控制 + 语速控制
if (volumeSlider) {
  volumeSlider.addEventListener("input", (e) => {
    currentVolume = e.target.value / 100;
    isMuted = false;
    if (muteBtn) muteBtn.textContent = "🔊";
    if (gainNode) gainNode.gain.value = currentVolume;
  });
}
if (muteBtn) {
  muteBtn.addEventListener("click", () => {
    isMuted = !isMuted;
    if (gainNode) gainNode.gain.value = isMuted ? 0 : currentVolume;
    muteBtn.textContent = isMuted ? "🔇" : "🔊";
  });
}
// 语速控制（运行时调整 TTS rate）
const ttsRateSlider = document.getElementById("tts-rate");
const ttsRateLabel = document.getElementById("tts-rate-label");
if (ttsRateSlider) {
  ttsRateSlider.addEventListener("input", (e) => {
    const val = parseInt(e.target.value);
    const rateStr = (val >= 0 ? "+" : "") + val + "%";
    if (ttsRateLabel) ttsRateLabel.textContent = rateStr;
    sendBytes(0x13, new TextEncoder().encode(rateStr));  // WS_MSG_SET_TTS_RATE
  });
}

// ---------- 麦克风采集 ----------
async function startRecording() {
  if (recording) return;
  if (!wsConnected) {
    showError("未连接服务器，无法录音");
    return;
  }
  try {
    mediaStream = await navigator.mediaDevices.getUserMedia({
      audio: {
        channelCount: 1,
        sampleRate: 16000,
        echoCancellation: true,
        noiseSuppression: true,
        autoGainControl: false,  // P2-6：关闭 AGC，降低延迟
      },
    });
  } catch (e) {
    showError("无法访问麦克风：" + e.message);
    return;
  }

  audioCtx = new (window.AudioContext || window.webkitAudioContext)({ sampleRate: 16000 });
  sourceNode = audioCtx.createMediaStreamSource(mediaStream);
  // P0-2：buffer 2048（128ms，比 4096 减 128ms 延迟）；ScriptProcessor 仍用（兼容性好）
  processor = audioCtx.createScriptProcessor(2048, 1, 1);
  processor.onaudioprocess = (e) => {
    if (!recording) return;
    const input = e.inputBuffer.getChannelData(0);
    const pcm = new Int16Array(input.length);
    for (let i = 0; i < input.length; i++) {
      const s = Math.max(-1, Math.min(1, input[i]));
      pcm[i] = s < 0 ? s * 0x8000 : s * 0x7fff;
    }
    sendBytes(WS_MSG_AUDIO, new Uint8Array(pcm.buffer));
  };
  sourceNode.connect(processor);

  recording = true;
  micBtn.classList.add("recording");
  micBtn.textContent = "🔴 录音中…";
  micBtn.disabled = true;
  stopBtn.disabled = false;
  indicator.style.display = "block";
  subAssistant.textContent = "";
}

function stopRecording(silent = false) {
  if (!recording && !mediaStream) return;
  recording = false;
  if (processor) {
    try { processor.disconnect(); } catch (e) {}
    processor.onaudioprocess = null;
    processor = null;
  }
  if (sourceNode) {
    try { sourceNode.disconnect(); } catch (e) {}
    sourceNode = null;
  }
  if (mediaStream) {
    mediaStream.getTracks().forEach((t) => t.stop());
    mediaStream = null;
  }
  if (audioCtx) {
    audioCtx.close().catch(() => {});
    audioCtx = null;
  }
  micBtn.classList.remove("recording");
  micBtn.textContent = "🎤 开始对话";
  micBtn.disabled = false;
  stopBtn.disabled = true;
  indicator.style.display = "none";
  showSpeaking(false);
  if (!silent) {
    sendBytes(WS_MSG_UTTERANCE_END, new Uint8Array(0));
  }
}

// P3-10：用 pointerdown 统一鼠标/触摸，消除移动端 300ms 点击延迟
micBtn.addEventListener("pointerdown", (e) => { e.preventDefault(); if (!recording) startRecording(); });
stopBtn.addEventListener("pointerdown", (e) => { e.preventDefault(); stopRecording(false); });

// ---------- 仪表盘数据拉取 ----------
async function refreshDashboard() {
  try {
    const r = await fetch("/api/dashboard");
    const d = await r.json();
    const fmt = (ms) => ms > 0 ? (ms / 1000).toFixed(2) + "s" : "-";
    const el = (id) => document.getElementById(id);
    if (el("d-sessions")) el("d-sessions").textContent = d.sessions;
    if (el("d-max")) el("d-max").textContent = d.max_sessions;
    if (el("d-frame-p50")) el("d-frame-p50").textContent = fmt(d.latency.first_frame_p50);
    if (el("d-frame-p95")) el("d-frame-p95").textContent = fmt(d.latency.first_frame_p95);
    if (el("d-total")) el("d-total").textContent = d.pipeline_total;
    if (el("d-errors")) el("d-errors").textContent = d.tts_fail_total;
    if (el("d-uptime")) {
      const u = d.uptime_seconds;
      el("d-uptime").textContent = u > 3600 ? Math.floor(u/3600)+"h" : u > 60 ? Math.floor(u/60)+"m" : u+"s";
    }
  } catch (e) { /* 静默 */ }
}
setInterval(refreshDashboard, 3000);
refreshDashboard();

// ---------- 文字输入 + 清空对话 ----------
const textInput = document.getElementById("text-input");
const sendTextBtn = document.getElementById("send-text-btn");
const clearBtn = document.getElementById("clear-btn");

function sendTextInput() {
  if (!textInput || !wsConnected) return;
  const text = textInput.value.trim();
  if (!text) return;
  sendBytes(0x15, new TextEncoder().encode(text));  // WS_MSG_TEXT_INPUT
  textInput.value = "";
}

if (sendTextBtn) {
  sendTextBtn.addEventListener("pointerdown", (e) => { e.preventDefault(); sendTextInput(); });
}
if (textInput) {
  textInput.addEventListener("keydown", (e) => {
    if (e.key === "Enter") { e.preventDefault(); sendTextInput(); }
  });
}
if (clearBtn) {
  clearBtn.addEventListener("pointerdown", (e) => {
    e.preventDefault();
    sendBytes(0x16, new Uint8Array(0));  // WS_MSG_CLEAR_HISTORY
    subUser.textContent = "";
    subAssistant.textContent = "";
  });
}

// ---------- 启动 ----------
connect();
