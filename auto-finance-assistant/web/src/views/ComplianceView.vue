<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const pwInput = ref(auth.password)
const stats = ref<any>(null)
const logs = ref<any[]>([])
const auditLogs = ref<any[]>([])
const loading = ref(false)
const filterType = ref('')

const typeLabels: Record<string, string> = {
  guard_block: 'Guard 拦截',
  compliance_block: '合规拒答',
  model_invoke: '模型调用',
  rag_refuse: 'RAG 拒答',
}

const typeColors: Record<string, string> = {
  guard_block: '#f59e0b',
  compliance_block: '#ef4444',
  model_invoke: '#10b981',
  rag_refuse: '#8b5cf6',
}

async function loadStats() {
  try {
    const resp = await auth.authedFetch('/api/compliance/stats')
    if (resp.ok) stats.value = await resp.json()
  } catch {}
}

async function loadLogs() {
  loading.value = true
  try {
    const url = filterType.value
      ? `/api/compliance/logs?type=${filterType.value}&limit=50`
      : '/api/compliance/logs?limit=50'
    const resp = await auth.authedFetch(url)
    if (resp.ok) logs.value = (await resp.json()).items || []
  } catch { logs.value = [] }
  loading.value = false
}

async function loadAuditLogs() {
  try {
    const resp = await auth.authedFetch('/api/audit/logs?limit=50')
    if (resp.ok) auditLogs.value = (await resp.json()).items || []
  } catch { auditLogs.value = [] }
}

function refresh() {
  loadStats(); loadLogs(); loadAuditLogs()
}

function fmtTime(t: string) {
  return t ? t.replace('T', ' ').slice(0, 19) : ''
}

onMounted(() => {
  if (auth.password) refresh()
})
</script>

<template>
  <div class="page">
    <header class="page-header">
      <h2>合规审计</h2>
      <button @click="refresh" class="sm" :disabled="!auth.password">刷新</button>
    </header>

    <!-- 认证 -->
    <section v-if="!auth.password" class="card">
      <h3>需要管理员认证</h3>
      <div class="pw-row">
        <input v-model="pwInput" type="password" placeholder="管理员密码" @keyup.enter="auth.setPassword(pwInput); refresh()" />
        <button @click="auth.setPassword(pwInput); refresh()" class="primary-btn">认证</button>
      </div>
    </section>

    <template v-else>
      <!-- 统计概览 -->
      <section class="card" v-if="stats">
        <h3>拦截统计</h3>
        <div class="stats-grid">
          <div class="stat-item">
            <div class="stat-val">{{ stats.totalBlocks }}</div>
            <div class="stat-lbl">总拦截次数</div>
          </div>
          <div class="stat-item">
            <div class="stat-val">{{ stats.totalRequests }}</div>
            <div class="stat-lbl">总请求数</div>
          </div>
          <div class="stat-item highlight">
            <div class="stat-val">{{ stats.blockRate?.toFixed(1) }}%</div>
            <div class="stat-lbl">拦截率</div>
          </div>
        </div>
        <div class="stats-breakdown" v-if="stats.stats?.length">
          <div v-for="s in stats.stats" :key="s.eventType + s.actionTaken" class="breakdown-item">
            <span class="bd-type" :style="{ color: typeColors[s.eventType] || '#64748b' }">
              {{ typeLabels[s.eventType] || s.eventType }}
            </span>
            <span class="bd-action">{{ s.actionTaken }}</span>
            <span class="bd-count">{{ s.count }}</span>
          </div>
        </div>
      </section>

      <!-- 合规日志 -->
      <section class="card">
        <h3>合规日志 <span class="muted">（输入/输出/拦截全链路证据）</span></h3>
        <div class="filter-row">
          <button :class="['filter-btn', filterType === '' ? 'active' : '']" @click="filterType = ''; loadLogs()">全部</button>
          <button :class="['filter-btn', filterType === 'guard_block' ? 'active' : '']" @click="filterType = 'guard_block'; loadLogs()">Guard 拦截</button>
          <button :class="['filter-btn', filterType === 'compliance_block' ? 'active' : '']" @click="filterType = 'compliance_block'; loadLogs()">合规拒答</button>
          <button :class="['filter-btn', filterType === 'model_invoke' ? 'active' : '']" @click="filterType = 'model_invoke'; loadLogs()">模型调用</button>
        </div>
        <div v-if="loading" class="muted">加载中...</div>
        <table v-else-if="logs.length" class="log-table">
          <thead>
            <tr><th>时间</th><th>事件</th><th>动作</th><th>意图</th><th>输入</th><th>耗时</th></tr>
          </thead>
          <tbody>
            <tr v-for="log in logs" :key="log.id">
              <td class="muted">{{ fmtTime(log.createdAt) }}</td>
              <td><span class="event-badge" :style="{ background: (typeColors[log.eventType] || '#64748b') + '22', color: typeColors[log.eventType] || '#64748b' }">{{ typeLabels[log.eventType] || log.eventType }}</span></td>
              <td>{{ log.actionTaken }}</td>
              <td class="mono">{{ log.intent || '-' }}</td>
              <td class="truncate">{{ log.rawInput || '-' }}</td>
              <td>{{ log.durationMs }}ms</td>
            </tr>
          </tbody>
        </table>
        <div v-else class="muted">暂无记录</div>
      </section>

      <!-- 管理审计日志 -->
      <section class="card">
        <h3>管理操作审计</h3>
        <table v-if="auditLogs.length" class="log-table">
          <thead>
            <tr><th>时间</th><th>操作</th><th>路径</th><th>IP</th></tr>
          </thead>
          <tbody>
            <tr v-for="log in auditLogs" :key="log.id">
              <td class="muted">{{ fmtTime(log.createdAt) }}</td>
              <td><span class="action-badge">{{ log.action }}</span></td>
              <td class="mono">{{ log.detail || '-' }}</td>
              <td class="mono">{{ log.ipAddress || '-' }}</td>
            </tr>
          </tbody>
        </table>
        <div v-else class="muted">暂无审计记录</div>
      </section>
    </template>
  </div>
</template>

<style scoped>
.page { padding: 24px; height: 100%; overflow-y: auto; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
.page-header h2 { font-size: 20px; }
.card { background: white; border-radius: 12px; padding: 20px; margin-bottom: 16px; }
.card h3 { font-size: 16px; margin-bottom: 14px; }
.muted { color: var(--muted); font-size: 13px; }
.pw-row { display: flex; gap: 8px; }
.pw-row input { flex: 1; }
.sm { font-size: 13px; padding: 6px 14px; background: var(--bg); border: 1px solid var(--border); border-radius: 8px; }
.primary-btn { background: var(--primary); color: white; padding: 8px 20px; border-radius: 8px; border: none; cursor: pointer; }

.stats-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; margin-bottom: 16px; }
.stat-item { text-align: center; padding: 16px; background: var(--bg); border-radius: 8px; }
.stat-item.highlight { background: #fef2f2; }
.stat-val { font-size: 28px; font-weight: 700; color: var(--primary); }
.stat-item.highlight .stat-val { color: #ef4444; }
.stat-lbl { font-size: 12px; color: var(--muted); margin-top: 4px; }

.stats-breakdown { display: flex; flex-direction: column; gap: 6px; }
.breakdown-item { display: flex; align-items: center; gap: 12px; padding: 6px 12px; background: var(--bg); border-radius: 6px; font-size: 13px; }
.bd-type { font-weight: 600; min-width: 100px; }
.bd-action { color: var(--muted); flex: 1; }
.bd-count { font-weight: 600; }

.filter-row { display: flex; gap: 8px; margin-bottom: 12px; }
.filter-btn { font-size: 12px; padding: 4px 12px; background: var(--bg); border: 1px solid var(--border); border-radius: 6px; cursor: pointer; }
.filter-btn.active { background: var(--primary); color: white; border-color: var(--primary); }

.log-table { width: 100%; font-size: 12px; }
.log-table th { text-align: left; padding: 8px; border-bottom: 2px solid var(--border); color: var(--muted); }
.log-table td { padding: 8px; border-bottom: 1px solid var(--border); }
.mono { font-family: monospace; font-size: 11px; }
.truncate { max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.event-badge { padding: 2px 8px; border-radius: 4px; font-size: 11px; font-weight: 500; }
.action-badge { padding: 2px 8px; border-radius: 4px; font-size: 11px; background: #f1f5f9; color: #475569; }
</style>
