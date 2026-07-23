<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()

interface Doc {
  id: string; name: string; originalName: string; fileType: string
  fileSize: number; version: string; institution: string; productCode: string
  status: string; chunkCount: number; effectiveDate: string; createdAt: string
}
interface Chunk { id: string; sequence: number; title: string; section: string; content: string; pageNumber: number; tokenCount: number }

const docs = ref<Doc[]>([])
const loading = ref(false)
const uploading = ref(false)
const selectedDoc = ref<Doc | null>(null)
const chunks = ref<Chunk[]>([])
const showChunks = ref(false)
const errorMsg = ref('')

async function loadDocs() {
  loading.value = true
  try {
    const resp = await fetch('/api/documents')
    const data = await resp.json()
    docs.value = data.items || []
  } catch (e) { errorMsg.value = '加载失败' }
  loading.value = false
}

async function onUpload(e: Event) {
  const input = e.target as HTMLInputElement
  if (!input.files?.length) return
  uploading.value = true
  errorMsg.value = ''
  for (const file of Array.from(input.files)) {
    const fd = new FormData()
    fd.append('file', file)
    try {
      const resp = await auth.authedFetch('/api/documents', { method: 'POST', body: fd })
      const data = await resp.json()
      if (!resp.ok) throw new Error(data?.error?.message || '上传失败')
    } catch (err: any) { errorMsg.value = `${file.name}: ${err.message}` }
  }
  input.value = ''
  uploading.value = false
  await loadDocs()
}

async function publish(id: string) {
  await auth.authedFetch(`/api/documents/${id}/publish`, { method: 'POST' })
  await loadDocs()
}
async function disable(id: string) {
  await auth.authedFetch(`/api/documents/${id}/disable`, { method: 'POST' })
  await loadDocs()
}
async function embed(id: string) {
  errorMsg.value = '正在向量化（CPU 较慢）...'
  const resp = await auth.authedFetch(`/api/documents/${id}/embed`, { method: 'POST' })
  const data = await resp.json()
  errorMsg.value = data.embedded != null ? `向量化完成：${data.embedded} 片段，总索引 ${data.vectorCount}` : (data?.error?.message || '失败')
}
async function remove(id: string) {
  if (!confirm('确认删除该文档？')) return
  await auth.authedFetch(`/api/documents/${id}`, { method: 'DELETE' })
  await loadDocs()
}
async function viewChunks(doc: Doc) {
  selectedDoc.value = doc
  showChunks.value = true
  const resp = await fetch(`/api/documents/${doc.id}/chunks`)
  const data = await resp.json()
  chunks.value = data.items || []
}

const statusLabel: Record<string, string> = { draft: '草稿', processing: '处理中', active: '已发布', inactive: '已停用', failed: '失败' }

onMounted(loadDocs)
</script>

<template>
  <div class="page">
    <header class="page-header">
      <h2>知识库管理</h2>
      <label class="upload-btn">
        {{ uploading ? '上传中...' : '+ 上传文档' }}
        <input type="file" multiple accept=".txt,.md,.html,.docx,.xlsx,.pdf" @change="onUpload" :disabled="uploading" hidden />
      </label>
    </header>
    <div class="error-banner" v-if="errorMsg">{{ errorMsg }}</div>
    <div class="loading" v-if="loading">加载中...</div>
    <table class="table" v-else>
      <thead>
        <tr><th>文档名</th><th>类型</th><th>状态</th><th>片段</th><th>版本</th><th>创建时间</th><th>操作</th></tr>
      </thead>
      <tbody>
        <tr v-if="docs.length === 0"><td colspan="7" class="empty">暂无文档，点击右上角上传</td></tr>
        <tr v-for="d in docs" :key="d.id">
          <td>{{ d.name }}</td>
          <td>{{ d.fileType }}</td>
          <td><span class="badge" :class="d.status">{{ statusLabel[d.status] || d.status }}</span></td>
          <td>{{ d.chunkCount }}</td>
          <td>{{ d.version || '-' }}</td>
          <td class="muted">{{ d.createdAt?.slice(0, 10) }}</td>
          <td class="actions">
            <button v-if="d.status === 'draft'" @click="publish(d.id)" class="sm">发布</button>
            <button v-if="d.status === 'active'" @click="disable(d.id)" class="sm">停用</button>
            <button v-if="d.status === 'active'" @click="embed(d.id)" class="sm">向量化</button>
            <button @click="viewChunks(d)" class="sm">分块</button>
            <button @click="remove(d.id)" class="sm danger">删除</button>
          </td>
        </tr>
      </tbody>
    </table>

    <div class="modal" v-if="showChunks" @click.self="showChunks = false">
      <div class="modal-content">
        <div class="modal-header">
          <h3>{{ selectedDoc?.name }} - 分块预览（{{ chunks.length }}）</h3>
          <button @click="showChunks = false" class="sm">关闭</button>
        </div>
        <div class="chunk-list">
          <div v-for="c in chunks" :key="c.id" class="chunk">
            <div class="chunk-meta">#{{ c.sequence }} · {{ c.section || '无章节' }} · {{ c.tokenCount }} tokens</div>
            <div class="chunk-content">{{ c.content }}</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.page { padding: 24px; height: 100%; overflow-y: auto; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
.page-header h2 { font-size: 20px; }
.upload-btn { background: var(--primary); color: white; padding: 8px 16px; border-radius: 8px; cursor: pointer; font-size: 14px; }
.upload-btn:hover { background: var(--primary-hover); }
.error-banner { background: #fef2f2; color: var(--error); padding: 10px 14px; border-radius: 8px; margin-bottom: 16px; font-size: 13px; }
.loading { color: var(--muted); padding: 40px; text-align: center; }
.table { width: 100%; border-collapse: collapse; font-size: 14px; }
.table th { text-align: left; padding: 10px 12px; border-bottom: 2px solid var(--border); color: var(--muted); font-weight: 500; }
.table td { padding: 10px 12px; border-bottom: 1px solid var(--border); }
.empty { text-align: center; color: var(--muted); padding: 40px; }
.badge { padding: 2px 8px; border-radius: 12px; font-size: 12px; }
.badge.active { background: #dcfce7; color: #16a34a; }
.badge.draft { background: #fef3c7; color: #d97706; }
.badge.inactive { background: #f1f5f9; color: var(--muted); }
.badge.failed { background: #fee2e2; color: var(--error); }
.muted { color: var(--muted); font-size: 13px; }
.actions { white-space: nowrap; }
.sm { font-size: 12px; padding: 4px 10px; background: var(--bg); border: 1px solid var(--border); margin-right: 4px; }
.sm:hover { border-color: var(--primary); color: var(--primary); }
.sm.danger:hover { border-color: var(--error); color: var(--error); }
.modal { position: fixed; inset: 0; background: rgba(0,0,0,0.4); display: flex; align-items: center; justify-content: center; z-index: 100; }
.modal-content { background: white; border-radius: 12px; width: 80%; max-width: 800px; max-height: 80vh; display: flex; flex-direction: column; }
.modal-header { display: flex; justify-content: space-between; align-items: center; padding: 16px 20px; border-bottom: 1px solid var(--border); }
.chunk-list { padding: 16px 20px; overflow-y: auto; }
.chunk { margin-bottom: 16px; padding: 12px; background: var(--bg); border-radius: 8px; }
.chunk-meta { font-size: 12px; color: var(--muted); margin-bottom: 6px; }
.chunk-content { font-size: 14px; line-height: 1.6; }
</style>
