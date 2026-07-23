<script setup lang="ts">
import { ref, onMounted } from 'vue'

interface Conv { id: string; title: string; createdAt: string; updatedAt: string }
interface Msg { id: string; role: string; content: string; createdAt: string }

const conversations = ref<Conv[]>([])
const selectedConv = ref<Conv | null>(null)
const messages = ref<Msg[]>([])
const loading = ref(false)

async function loadList() {
  loading.value = true
  const resp = await fetch('/api/conversations')
  const data = await resp.json()
  conversations.value = data.items || []
  loading.value = false
}

async function select(c: Conv) {
  selectedConv.value = c
  const resp = await fetch(`/api/conversations/${c.id}`)
  const data = await resp.json()
  messages.value = data.messages || []
}

async function feedback(msgId: string, rating: number) {
  await fetch('/api/feedback', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ messageId: msgId, rating }),
  })
}

onMounted(loadList)
</script>

<template>
  <div class="page">
    <h2>会话历史</h2>
    <div class="split">
      <div class="conv-list">
        <div class="loading" v-if="loading">加载中...</div>
        <div v-if="conversations.length === 0" class="empty">暂无会话</div>
        <div v-for="c in conversations" :key="c.id" class="conv-item" :class="{ active: selectedConv?.id === c.id }" @click="select(c)">
          <div class="conv-title">{{ c.title }}</div>
          <div class="conv-time">{{ c.updatedAt?.slice(0, 16).replace('T', ' ') }}</div>
        </div>
      </div>
      <div class="conv-detail">
        <div v-if="!selectedConv" class="empty">选择左侧会话查看详情</div>
        <template v-else>
          <div class="msg" v-for="m in messages" :key="m.id" :class="m.role">
            <div class="msg-role">{{ m.role === 'user' ? '用户' : m.role === 'assistant' ? '助手' : m.role }}</div>
            <div class="msg-content">{{ m.content }}</div>
            <div class="msg-actions" v-if="m.role === 'assistant'">
              <button class="fb" @click="feedback(m.id, 1)">👍</button>
              <button class="fb" @click="feedback(m.id, -1)">👎</button>
              <span class="msg-time">{{ m.createdAt?.slice(11, 16) }}</span>
            </div>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<style scoped>
.page { padding: 24px; height: 100%; overflow: hidden; display: flex; flex-direction: column; }
.page h2 { font-size: 20px; margin-bottom: 20px; }
.split { display: grid; grid-template-columns: 280px 1fr; gap: 16px; flex: 1; overflow: hidden; }
.conv-list { overflow-y: auto; background: white; border-radius: 12px; padding: 8px; }
.conv-item { padding: 12px; border-radius: 8px; cursor: pointer; }
.conv-item:hover { background: var(--bg); }
.conv-item.active { background: #eff6ff; }
.conv-title { font-size: 14px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.conv-time { font-size: 12px; color: var(--muted); margin-top: 4px; }
.conv-detail { overflow-y: auto; background: white; border-radius: 12px; padding: 20px; }
.empty { text-align: center; color: var(--muted); padding: 40px; }
.loading { color: var(--muted); padding: 20px; text-align: center; }
.msg { margin-bottom: 20px; }
.msg.user .msg-content { background: #eff6ff; }
.msg.assistant .msg-content { background: var(--bg); }
.msg-role { font-size: 12px; color: var(--muted); margin-bottom: 4px; }
.msg-content { padding: 12px 14px; border-radius: 8px; font-size: 14px; line-height: 1.6; white-space: pre-wrap; }
.msg-actions { display: flex; align-items: center; gap: 8px; margin-top: 6px; }
.fb { font-size: 14px; padding: 2px 6px; background: transparent; border: 1px solid var(--border); border-radius: 6px; }
.msg-time { font-size: 11px; color: var(--muted); margin-left: auto; }
</style>
