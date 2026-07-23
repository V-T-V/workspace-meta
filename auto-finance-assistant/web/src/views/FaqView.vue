<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()

interface Faq {
  id: string; category: string; question: string; answer: string
  keywords: string; enabled: boolean; priority: number
}
interface TestResult { strategy: string; score: number; hit: boolean; faq?: Faq }

const faqs = ref<Faq[]>([])
const loading = ref(false)
const showForm = ref(false)
const editing = ref<Faq | null>(null)
const form = ref({ category: '', question: '', answer: '', keywords: '', enabled: true, priority: 0 })
const testQuestion = ref('')
const testResult = ref<TestResult | null>(null)
const errorMsg = ref('')

async function load() {
  loading.value = true
  const resp = await fetch('/api/faqs')
  const data = await resp.json()
  faqs.value = data.items || []
  loading.value = false
}

function openCreate() {
  editing.value = null
  form.value = { category: '', question: '', answer: '', keywords: '', enabled: true, priority: 0 }
  showForm.value = true
}
function openEdit(f: Faq) {
  editing.value = f
  form.value = { category: f.category, question: f.question, answer: f.answer, keywords: f.keywords, enabled: f.enabled, priority: f.priority }
  showForm.value = true
}

async function save() {
  errorMsg.value = ''
  if (!form.value.question || !form.value.answer) { errorMsg.value = '问题和答案不能为空'; return }
  const body = { ...form.value }
  const url = editing.value ? `/api/faqs/${editing.value.id}` : '/api/faqs'
  const method = editing.value ? 'PUT' : 'POST'
  const resp = await auth.authedFetch(url, { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
  if (!resp.ok) { const d = await resp.json(); errorMsg.value = d?.error?.message || '保存失败'; return }
  showForm.value = false
  await load()
}

async function remove(id: string) {
  if (!confirm('确认删除？')) return
  await auth.authedFetch(`/api/faqs/${id}`, { method: 'DELETE' })
  await load()
}

async function toggle(f: Faq) {
  // 发送完整对象，避免后端把 priority 清零
  await auth.authedFetch(`/api/faqs/${f.id}`, {
    method: 'PUT', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ...f, enabled: !f.enabled }),
  })
  await load()
}

async function test() {
  if (!testQuestion.value.trim()) return
  const resp = await fetch('/api/faqs/test', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ question: testQuestion.value }) })
  testResult.value = await resp.json()
}

onMounted(load)
</script>

<template>
  <div class="page">
    <header class="page-header">
      <h2>FAQ 管理</h2>
      <button class="primary-btn" @click="openCreate">+ 新建 FAQ</button>
    </header>

    <div class="test-box">
      <h4>测试匹配</h4>
      <div class="test-row">
        <input v-model="testQuestion" placeholder="输入测试问题..." @keydown.enter="test" />
        <button @click="test">测试</button>
      </div>
      <div class="test-result" v-if="testResult">
        策略：{{ testResult.strategy }} · 分数：{{ testResult.score?.toFixed(2) }} ·
        <span :class="testResult.hit ? 'ok-text' : 'warn-text'">{{ testResult.hit ? '命中' : '未命中' }}</span>
        <span v-if="testResult.faq"> → {{ testResult.faq.question }}</span>
      </div>
    </div>

    <table class="table">
      <thead>
        <tr><th>问题</th><th>分类</th><th>关键词</th><th>优先级</th><th>启用</th><th>操作</th></tr>
      </thead>
      <tbody>
        <tr v-if="faqs.length === 0"><td colspan="6" class="empty">暂无 FAQ</td></tr>
        <tr v-for="f in faqs" :key="f.id">
          <td>{{ f.question }}</td>
          <td>{{ f.category || '-' }}</td>
          <td class="muted">{{ f.keywords || '-' }}</td>
          <td>{{ f.priority }}</td>
          <td><span class="badge" :class="f.enabled ? 'on' : 'off'" @click="toggle(f)">{{ f.enabled ? '是' : '否' }}</span></td>
          <td class="actions">
            <button @click="openEdit(f)" class="sm">编辑</button>
            <button @click="remove(f.id)" class="sm danger">删除</button>
          </td>
        </tr>
      </tbody>
    </table>

    <div class="modal" v-if="showForm" @click.self="showForm = false">
      <div class="modal-content">
        <div class="modal-header"><h3>{{ editing ? '编辑 FAQ' : '新建 FAQ' }}</h3><button @click="showForm = false" class="sm">关闭</button></div>
        <div class="form-body">
          <div class="error-banner" v-if="errorMsg">{{ errorMsg }}</div>
          <label>问题<textarea v-model="form.question" rows="2"></textarea></label>
          <label>答案<textarea v-model="form.answer" rows="3"></textarea></label>
          <div class="form-row">
            <label>分类<input v-model="form.category" /></label>
            <label>优先级<input type="number" v-model.number="form.priority" /></label>
          </div>
          <label>关键词（空格分隔）<input v-model="form.keywords" placeholder="申请 贷款 材料" /></label>
          <label class="checkbox"><input type="checkbox" v-model="form.enabled" /> 启用</label>
          <div class="form-actions"><button class="primary-btn" @click="save">保存</button></div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.page { padding: 24px; height: 100%; overflow-y: auto; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
.page-header h2 { font-size: 20px; }
.primary-btn { background: var(--primary); color: white; padding: 8px 16px; border-radius: 8px; }
.primary-btn:hover { background: var(--primary-hover); }
.test-box { background: var(--bg); border-radius: 8px; padding: 14px 16px; margin-bottom: 20px; }
.test-box h4 { font-size: 14px; margin-bottom: 8px; }
.test-row { display: flex; gap: 8px; }
.test-row input { flex: 1; }
.test-row button { padding: 8px 16px; background: var(--primary); color: white; border-radius: 8px; }
.test-result { margin-top: 8px; font-size: 13px; color: var(--muted); }
.ok-text { color: var(--success); font-weight: 500; }
.warn-text { color: var(--error); }
.table { width: 100%; border-collapse: collapse; font-size: 14px; background: white; }
.table th { text-align: left; padding: 10px 12px; border-bottom: 2px solid var(--border); color: var(--muted); font-weight: 500; }
.table td { padding: 10px 12px; border-bottom: 1px solid var(--border); }
.empty { text-align: center; color: var(--muted); padding: 40px; }
.muted { color: var(--muted); }
.badge { padding: 2px 10px; border-radius: 12px; font-size: 12px; cursor: pointer; }
.badge.on { background: #dcfce7; color: #16a34a; }
.badge.off { background: #f1f5f9; color: var(--muted); }
.sm { font-size: 12px; padding: 4px 10px; background: var(--bg); border: 1px solid var(--border); margin-right: 4px; }
.sm.danger:hover { border-color: var(--error); color: var(--error); }
.modal { position: fixed; inset: 0; background: rgba(0,0,0,0.4); display: flex; align-items: center; justify-content: center; z-index: 100; }
.modal-content { background: white; border-radius: 12px; width: 560px; max-height: 80vh; overflow-y: auto; }
.modal-header { display: flex; justify-content: space-between; align-items: center; padding: 16px 20px; border-bottom: 1px solid var(--border); }
.form-body { padding: 20px; display: flex; flex-direction: column; gap: 12px; }
.form-body label { display: flex; flex-direction: column; gap: 4px; font-size: 13px; color: var(--muted); }
.form-body textarea, .form-body input { border: 1px solid var(--border); border-radius: 6px; padding: 8px; font-size: 14px; }
.form-row { display: flex; gap: 12px; }
.form-row label { flex: 1; }
.checkbox { flex-direction: row !important; align-items: center; gap: 8px; }
.form-actions { display: flex; justify-content: flex-end; }
.error-banner { background: #fef2f2; color: var(--error); padding: 8px 12px; border-radius: 6px; font-size: 13px; }
</style>
