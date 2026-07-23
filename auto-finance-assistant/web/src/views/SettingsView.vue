<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const health = ref<any>(null)
const systemInfo = ref<any>(null)
const metrics = ref<any>(null)
const backups = ref<any[]>([])
const backupMsg = ref('')
const pwInput = ref(auth.password)

async function loadHealth() {
  const resp = await fetch('/api/health')
  health.value = await resp.json()
}
async function loadSystemInfo() {
  try {
    const resp = await auth.authedFetch('/api/system/info')
    if (resp.ok) systemInfo.value = await resp.json()
  } catch {}
}
async function loadMetrics() {
  try {
    const resp = await auth.authedFetch('/api/metrics')
    if (resp.ok) metrics.value = await resp.json()
  } catch { metrics.value = null }
}
async function loadBackups() {
  try {
    const resp = await auth.authedFetch('/api/system/backups')
    if (resp.ok) backups.value = (await resp.json()).items || []
  } catch { backups.value = [] }
}
async function doBackup() {
  backupMsg.value = '备份中...'
  const resp = await auth.authedFetch('/api/system/backup', { method: 'POST' })
  const data = await resp.json()
  backupMsg.value = data.name ? `备份成功：${data.name}` : (data?.error?.message || '失败')
  await loadBackups()
}
function savePw() {
  auth.setPassword(pwInput.value)
  refresh()
}
function clearPw() {
  auth.clear()
  pwInput.value = ''
  refresh()
}
function refresh() { loadHealth(); loadSystemInfo(); loadMetrics(); loadBackups() }

const statusLabel: Record<string, string> = { ok: '正常', degraded: '降级', down: '不可用', missing_model: '缺模型' }
const metricLabels: Record<string, string> = {
  conversations: '会话数', messages: '消息数', faqs: 'FAQ 数', documents: '文档数',
  feedback: '反馈数', refusedAnswers: '拒答数', vectorIndex: '向量索引', queueActive: '执行中', queueWaiting: '等待中',
}

onMounted(refresh)
</script>

<template>
  <div class="page">
    <header class="page-header">
      <h2>系统设置</h2>
      <button @click="refresh" class="sm">刷新</button>
    </header>

    <section class="card">
      <h3>管理员认证</h3>
      <div class="pw-row">
        <input v-model="pwInput" type="password" placeholder="管理员密码（与服务端 admin_password 一致）" />
        <button @click="savePw" class="primary-btn">保存</button>
        <button @click="clearPw" class="sm">清除</button>
      </div>
      <div class="muted" style="margin-top:8px">密码为空时，管理接口在服务端配置为开放模式（仅本地监听安全）。</div>
    </section>

    <section class="card">
      <h3>健康状态</h3>
      <div class="status-grid" v-if="health">
        <div class="status-item"><span>系统</span><span class="badge" :class="health.status">{{ statusLabel[health.status] }}</span></div>
        <div class="status-item"><span>数据库</span><span class="badge" :class="health.database">{{ statusLabel[health.database] || health.database }}</span></div>
        <div class="status-item"><span>Ollama</span><span class="badge" :class="health.ollama">{{ statusLabel[health.ollama] || health.ollama }}</span></div>
        <div class="status-item"><span>模型</span><span>{{ health.model }}</span></div>
        <div class="status-item"><span>版本</span><span>{{ health.version }}</span></div>
      </div>
      <div class="warn" v-if="health?.detail">{{ health.detail }}</div>
    </section>

    <section class="card">
      <h3>运行指标</h3>
      <div class="metrics-grid" v-if="metrics">
        <div class="metric" v-for="(v, k) in metrics" :key="k">
          <div class="metric-value">{{ v }}</div>
          <div class="metric-label">{{ metricLabels[k] || k }}</div>
        </div>
      </div>
      <div v-else class="muted">加载中...</div>
    </section>

    <section class="card">
      <h3>系统信息</h3>
      <div class="info-grid" v-if="systemInfo">
        <div class="info-row"><span>Ollama 地址</span><span>{{ systemInfo.ollamaBaseUrl }}</span></div>
        <div class="info-row"><span>Ollama 可达</span><span>{{ systemInfo.ollamaReachable ? '是' : '否' }}</span></div>
        <div class="info-row"><span>本地模型</span><span>{{ (systemInfo.models || []).join(', ') || '无' }}</span></div>
        <div class="info-row"><span>生成并发</span><span>{{ systemInfo.concurrency }}</span></div>
      </div>
    </section>

    <section class="card">
      <h3>备份管理</h3>
      <button class="primary-btn" @click="doBackup">立即备份</button>
      <div class="backup-msg" v-if="backupMsg">{{ backupMsg }}</div>
      <table class="backup-table" v-if="backups.length > 0">
        <thead><tr><th>备份文件</th><th>大小</th><th>时间</th></tr></thead>
        <tbody>
          <tr v-for="b in backups" :key="b.name"><td>{{ b.name }}</td><td>{{ (b.size / 1024).toFixed(0) }} KB</td><td class="muted">{{ b.modTime?.slice(0, 16).replace('T', ' ') }}</td></tr>
        </tbody>
      </table>
    </section>
  </div>
</template>

<style scoped>
.page { padding: 24px; height: 100%; overflow-y: auto; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
.page-header h2 { font-size: 20px; }
.card { background: white; border-radius: 12px; padding: 20px; margin-bottom: 16px; }
.card h3 { font-size: 16px; margin-bottom: 14px; }
.pw-row { display: flex; gap: 8px; }
.pw-row input { flex: 1; }
.sm { font-size: 13px; padding: 6px 14px; background: var(--bg); border: 1px solid var(--border); border-radius: 8px; }
.status-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 12px; }
.status-item { display: flex; justify-content: space-between; padding: 10px 14px; background: var(--bg); border-radius: 8px; font-size: 14px; }
.badge { padding: 2px 10px; border-radius: 12px; font-size: 12px; background: #f1f5f9; }
.badge.ok { background: #dcfce7; color: #16a34a; }
.badge.down, .badge.degraded, .badge.missing_model { background: #fee2e2; color: var(--error); }
.warn { margin-top: 12px; padding: 10px 14px; background: #fef3c7; color: #d97706; border-radius: 8px; font-size: 13px; }
.metrics-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(120px, 1fr)); gap: 12px; }
.metric { text-align: center; padding: 16px; background: var(--bg); border-radius: 8px; }
.metric-value { font-size: 24px; font-weight: 600; color: var(--primary); }
.metric-label { font-size: 12px; color: var(--muted); margin-top: 4px; }
.info-grid { display: flex; flex-direction: column; gap: 8px; }
.info-row { display: flex; justify-content: space-between; padding: 8px 0; border-bottom: 1px solid var(--border); font-size: 14px; }
.info-row span:last-child { color: var(--muted); }
.primary-btn { background: var(--primary); color: white; padding: 8px 20px; border-radius: 8px; }
.backup-msg { margin-top: 8px; font-size: 13px; color: var(--success); }
.backup-table { width: 100%; margin-top: 12px; font-size: 13px; }
.backup-table th { text-align: left; padding: 8px; border-bottom: 2px solid var(--border); color: var(--muted); }
.backup-table td { padding: 8px; border-bottom: 1px solid var(--border); }
.muted { color: var(--muted); }
</style>
