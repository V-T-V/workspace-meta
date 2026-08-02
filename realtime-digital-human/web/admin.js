// 数字人管理控制台逻辑（原生 JS，无框架）
// 所有请求带 token（如有，从 URL ?token= 透传）

const QS = location.search || "";  // 透传 ?token=xxx

function toast(msg, type = "success") {
  const el = document.getElementById("toast");
  el.textContent = msg;
  el.className = `toast show ${type}`;
  setTimeout(() => el.classList.remove("show"), 2500);
}

// ---------- 仪表盘轮询 ----------
async function refreshDashboard() {
  try {
    const r = await fetch(`/api/dashboard${QS}`);
    const d = await r.json();
    const engines = d.engine_status || {};
    const wrap = document.getElementById("engine-status");
    wrap.innerHTML = "";
    const labels = { asr: "ASR", llm: "LLM", tts: "TTS", lipsync: "唇形" };
    for (const key of ["asr", "llm", "tts", "lipsync"]) {
      const name = engines[key] || "?";
      const isMock = String(name).startsWith("Mock");
      const chip = document.createElement("span");
      chip.className = "engine-chip " + (isMock ? "mock" : "ok");
      chip.textContent = `${labels[key]}: ${name}` + (isMock ? " ⚠" : "");
      wrap.appendChild(chip);
    }
    const u = d.uptime_seconds;
    const upStr = u > 3600 ? Math.floor(u/3600)+"h"+Math.floor(u%3600/60)+"m"
                : u > 60 ? Math.floor(u/60)+"m" : u+"s";
    document.getElementById("dashboard-meta").textContent =
      `会话: ${d.sessions}/${d.max_sessions} | 运行: ${upStr} | 首帧P50: ${(d.latency.first_frame_p50/1000).toFixed(2)}s`;
  } catch (e) { /* 静默 */ }
}

// ---------- 配置加载与保存 ----------
async function loadConfig() {
  try {
    const r = await fetch(`/api/admin/config${QS}`);
    const data = await r.json();
    const c = data.config;
    document.getElementById("cfg-path").textContent = "配置文件: " + data.config_path;

    // 填充表单
    const setVal = (id, val) => { const el = document.getElementById(id); if (el && val !== undefined && val !== null) el.value = val; };
    setVal("asr-backend", c.asr?.backend);
    setVal("asr-model", c.asr?.model);
    setVal("asr-base-url", c.asr?.base_url);
    setVal("asr-api-key", c.asr?.api_key);  // 脱敏值（sk-***abcd 或 未设置）
    setVal("llm-backend", c.llm?.backend);
    setVal("llm-model", c.llm?.model);
    setVal("llm-base-url", c.llm?.base_url);
    setVal("llm-api-key", c.llm?.api_key);
    setVal("tts-backend", c.tts?.backend);
    setVal("tts-voice", c.tts?.voice);
    setVal("lipsync-backend", c.lipsync?.backend);

    if (data.warnings && data.warnings.length > 0) {
      toast(`配置有 ${data.warnings.length} 条告警`, "warn");
    }
  } catch (e) {
    toast("加载配置失败: " + e.message, "error");
  }
}

async function saveConfig() {
  const body = {
    asr: {
      backend: document.getElementById("asr-backend").value,
      model: document.getElementById("asr-model").value,
      base_url: document.getElementById("asr-base-url").value,
      api_key: document.getElementById("asr-api-key").value,  // 脱敏占位符后端会跳过
    },
    llm: {
      backend: document.getElementById("llm-backend").value,
      model: document.getElementById("llm-model").value,
      base_url: document.getElementById("llm-base-url").value,
      api_key: document.getElementById("llm-api-key").value,
    },
    tts: {
      backend: document.getElementById("tts-backend").value,
      voice: document.getElementById("tts-voice").value,
    },
    lipsync: {
      backend: document.getElementById("lipsync-backend").value,
    },
  };
  try {
    const r = await fetch(`/api/admin/config${QS}`, {
      method: "POST", headers: {"Content-Type": "application/json"},
      body: JSON.stringify(body),
    });
    const d = await r.json();
    if (r.ok) {
      toast(`已保存 ${d.updated.length} 个字段，需重启生效`, "success");
      if (d.warnings && d.warnings.length > 0) {
        toast(`告警: ${d.warnings[0]}`, "warn");
      }
    } else {
      toast("保存失败: " + (d.error || r.status), "error");
    }
  } catch (e) {
    toast("保存异常: " + e.message, "error");
  }
}

// ---------- 测试连接 ----------
async function testConn(type) {
  const result = document.getElementById("test-" + type);
  result.className = "test-result pending";
  result.textContent = "测试中...";
  try {
    const r = await fetch(`/api/admin/test/${type}${QS}`, {method: "POST"});
    const d = await r.json();
    const ok = d.ok;
    let detail = d.detail || "";
    if (d.models) detail += ` | 模型: ${d.models.slice(0, 5).join(", ")}`;
    result.className = "test-result " + (ok ? "ok" : "fail");
    result.textContent = (ok ? "✅ " : "❌ ") + detail;
  } catch (e) {
    result.className = "test-result fail";
    result.textContent = "❌ 测试异常: " + e.message;
  }
}

async function runSelftest() {
  const result = document.getElementById("test-selftest");
  result.className = "test-result pending";
  result.textContent = "自检中...";
  try {
    const r = await fetch(`/api/admin/selftest${QS}`);
    const d = await r.json();
    const lines = d.checks.map(c => `${c.ok ? "✅" : "❌"} ${c.name}: ${c.detail || (c.ok ? "OK" : "失败")}`);
    result.className = "test-result " + (d.passed === d.total ? "ok" : "fail");
    result.textContent = `通过 ${d.passed}/${d.total}\n` + lines.join("\n");
  } catch (e) {
    result.className = "test-result fail";
    result.textContent = "❌ 自检异常: " + e.message;
  }
}

// ---------- 日志 ----------
async function refreshLogs() {
  try {
    const level = document.getElementById("log-level").value;
    const url = `/api/admin/logs${QS}${level ? `&level=${level}` : ""}`.replace("?&", "?");
    const r = await fetch(url);
    const d = await r.json();
    const view = document.getElementById("log-view");
    view.textContent = (d.lines || []).join("");
    if (document.getElementById("log-autoscroll").checked) {
      view.scrollTop = view.scrollHeight;
    }
  } catch (e) { /* 静默 */ }
}

// ---------- 重启 ----------
async function restartService() {
  if (!confirm("确定重启服务？将有 5-10 秒中断，浏览器会自动重连。")) return;
  toast("正在重启...", "warn");
  try {
    await fetch(`/api/admin/restart${QS}`, {method: "POST"});
  } catch (e) { /* 重启会断连，预期内 */ }

  // 轮询等待服务恢复
  let ok = false;
  for (let i = 0; i < 30; i++) {
    await new Promise(r => setTimeout(r, 1000));
    try {
      const r = await fetch(`/health${QS}`, {signal: AbortSignal.timeout(2000)});
      if (r.ok) { ok = true; break; }
    } catch (e) { /* 还没起来 */ }
  }
  if (ok) {
    toast("重启成功", "success");
    setTimeout(() => location.reload(), 1000);
  } else {
    toast("重启超时，请手动刷新", "error");
  }
}

// ---------- 启动 ----------
refreshDashboard();
loadConfig();
refreshLogs();
setInterval(refreshDashboard, 3000);
setInterval(() => {
  if (document.getElementById("log-autorefresh").checked) refreshLogs();
}, 3000);

// ---------- 会话历史与记忆 ----------
async function loadSessions() {
  try {
    const r = await fetch(`/api/admin/sessions${QS}`);
    const d = await r.json();
    const wrap = document.getElementById("sessions-list");
    const meta = document.getElementById("sessions-meta");
    if (!d.sessions || d.sessions.length === 0) {
      wrap.innerHTML = '<p class="meta">暂无会话（' + (d.msg || "") + '）</p>';
      meta.textContent = "暂无会话";
      return;
    }
    meta.textContent = `共 ${d.sessions.length} 个会话`;
    wrap.innerHTML = "";
    d.sessions.forEach(s => {
      const item = document.createElement("div");
      item.style.cssText = "padding:8px;border:1px solid #334155;border-radius:6px;margin-bottom:6px;cursor:pointer;font-size:0.85rem";
      item.onmouseenter = () => item.style.background = "#1e293b";
      item.onmouseleave = () => item.style.background = "";
      item.onclick = () => loadMemory(s.id);
      const t = new Date(s.last_active * 1000).toLocaleString("zh-CN", {hour12: false});
      item.innerHTML = `<div style="color:#38bdf8">${s.id}</div>
        <div class="meta">${s.msg_count} 条消息 · ${t}</div>`;
      wrap.appendChild(item);
    });
  } catch (e) {
    toast("加载会话失败: " + e.message, "error");
  }
}

async function loadMemory(sessionId) {
  try {
    const r = await fetch(`/api/admin/sessions/${sessionId}/memory${QS}`);
    const d = await r.json();
    const view = document.getElementById("memory-view");
    if (d.msg) { view.textContent = d.msg; return; }
    let html = "";
    // 摘要（长期记忆）置顶高亮
    if (d.summaries && d.summaries.length > 0) {
      html += '<div style="color:#fbbf24;margin-bottom:8px;padding:6px;background:rgba(251,191,36,0.1);border-radius:4px">';
      html += '<b>🧠 长期记忆（摘要）:</b><br>';
      d.summaries.forEach(s => { html += `<div style="margin-top:4px">${s.content}</div>`; });
      html += "</div>";
    }
    // 完整历史
    html += '<div style="color:#64748b;margin:8px 0 4px">完整历史 (' + d.total + ' 条):</div>';
    (d.history || []).forEach(m => {
      const role = m.role === "user" ? "用户" : m.role === "assistant" ? "助手" : "系统";
      const color = m.role === "user" ? "#4fd1c5" : m.role === "assistant" ? "#f6ad55" : "#94a3b8";
      html += `<div style="margin:3px 0"><span style="color:${color}">${role}:</span> ${m.content}</div>`;
    });
    view.innerHTML = html || "无历史记录";
  } catch (e) {
    toast("加载记忆失败: " + e.message, "error");
  }
}
