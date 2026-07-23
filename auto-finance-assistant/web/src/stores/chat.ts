import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface Message {
  id: string
  role: 'user' | 'assistant' | 'system'
  content: string
  createdAt?: string
  streaming?: boolean
  error?: boolean
  sources?: Source[]
}

export interface Source {
  documentId?: string
  documentName?: string
  version?: string
  section?: string
  pageNumber?: number
  effectiveDate?: string
}

export interface Conversation {
  id: string
  title: string
  createdAt: string
  updatedAt: string
}

interface HealthStatus {
  status: string
  database: string
  ollama: string
  model: string
  version: string
  detail?: string
}

export const useChatStore = defineStore('chat', () => {
  const conversations = ref<Conversation[]>([])
  const currentConversationId = ref<string>('')
  const messages = ref<Message[]>([])
  const input = ref('')
  const loading = ref(false)
  const status = ref('')
  const traceId = ref('')
  const health = ref<HealthStatus | null>(null)
  const errorMessage = ref('')
  const abortController = ref<AbortController | null>(null)

  async function checkHealth() {
    try {
      const resp = await fetch('/api/health')
      health.value = await resp.json()
    } catch (e) {
      health.value = {
        status: 'down',
        database: 'unknown',
        ollama: 'unknown',
        model: '',
        version: '',
        detail: '无法连接服务',
      }
    }
  }

  async function loadConversations() {
    try {
      const resp = await fetch('/api/conversations')
      const data = await resp.json()
      conversations.value = data.items || []
    } catch (e) {
      // 静默失败，首次无会话属正常
    }
  }

  async function createConversation(title = '新会话'): Promise<string> {
    const resp = await fetch('/api/conversations', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title }),
    })
    const data = await resp.json()
    if (!resp.ok) throw new Error(data?.error?.message || '创建会话失败')
    const conv: Conversation = data
    conversations.value.unshift(conv)
    return conv.id
  }

  async function selectConversation(id: string) {
    currentConversationId.value = id
    messages.value = []
    try {
      const resp = await fetch(`/api/conversations/${id}`)
      if (!resp.ok) return
      const data = await resp.json()
      messages.value = (data.messages || []).map((m: any) => ({
        id: m.id,
        role: m.role,
        content: m.content,
        createdAt: m.createdAt,
      }))
    } catch (e) {
      // ignore
    }
  }

  // sendStream 用 fetch + ReadableStream 解析 SSE，逐 token 渲染
  async function sendStream() {
    const question = input.value.trim()
    if (!question || loading.value) return

    errorMessage.value = ''
    input.value = ''

    // 若无当前会话，创建一个（标题取问题前 20 字）
    if (!currentConversationId.value) {
      try {
        const id = await createConversation(question.slice(0, 20))
        currentConversationId.value = id
      } catch (e: any) {
        errorMessage.value = e.message
        return
      }
    }

    // 追加 user 消息
    messages.value.push({
      id: crypto.randomUUID(),
      role: 'user',
      content: question,
    })

    // 追加占位 assistant 消息（流式填充）
    const assistantMsg: Message = {
      id: crypto.randomUUID(),
      role: 'assistant',
      content: '',
      streaming: true,
    }
    messages.value.push(assistantMsg)

    loading.value = true
    status.value = '排队中'

    abortController.value = new AbortController()
    try {
      const resp = await fetch('/api/chat/stream', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Accept: 'text/event-stream',
        },
        body: JSON.stringify({
          conversationId: currentConversationId.value,
          question,
        }),
        signal: abortController.value.signal,
      })

      if (!resp.ok || !resp.body) {
        const errData = await resp.json().catch(() => ({}))
        throw new Error(errData?.error?.message || `HTTP ${resp.status}`)
      }

      const reader = resp.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      try {
        while (true) {
          const { done, value } = await reader.read()
          if (done) break
          buffer += decoder.decode(value, { stream: true })

          // 按空行分割 SSE 事件块（兼容 CRLF）
          const blocks = buffer.split(/\n\n+/)
          buffer = blocks.pop() || ''

          for (const block of blocks) {
            const ev = parseSSE(block)
            if (!ev) continue
            handleEvent(ev, assistantMsg)
          }
        }
      } finally {
        // 释放 reader，防资源泄漏
        try { reader.releaseLock() } catch {}
        try { await reader.cancel() } catch {}
      }
    } catch (e: any) {
      if (e.name === 'AbortError') {
        assistantMsg.content += '\n\n_（已停止）_'
      } else {
        errorMessage.value = e.message
        assistantMsg.error = true
        assistantMsg.content = assistantMsg.content || `生成失败：${e.message}`
      }
    } finally {
      assistantMsg.streaming = false
      loading.value = false
      status.value = ''
      abortController.value = null
    }
  }

  function stop() {
    abortController.value?.abort()
  }

  interface SSEEvent {
    event: string
    data: any
  }

  function parseSSE(block: string): SSEEvent | null {
    const lines = block.split('\n')
    let event = 'message'
    let dataStr = ''
    for (const line of lines) {
      if (line.startsWith('event: ')) event = line.slice(7).trim()
      else if (line.startsWith('data: ')) dataStr += line.slice(6)
    }
    if (!dataStr) return null
    try {
      return { event, data: JSON.parse(dataStr) }
    } catch {
      return { event, data: dataStr }
    }
  }

  function handleEvent(ev: SSEEvent, target: Message) {
    switch (ev.event) {
      case 'status':
        status.value = ev.data.status || ''
        if (ev.data.traceId) traceId.value = ev.data.traceId
        break
      case 'token':
        target.content += ev.data.token || ''
        break
      case 'source':
        // 收集 RAG 来源（流式推送）
        if (ev.data.documentName) {
          if (!target.sources) target.sources = []
          target.sources.push({
            documentName: ev.data.documentName,
            section: ev.data.section,
            version: ev.data.version,
            effectiveDate: ev.data.effectiveDate,
          })
        }
        break
      case 'complete':
        // 完成时可选更新来源（M4）
        if (ev.data.sources) target.sources = ev.data.sources
        break
      case 'error':
        target.error = true
        target.content = target.content || ev.data.message || '生成失败'
        errorMessage.value = ev.data.message || ''
        break
    }
  }

  function clearMessages() {
    messages.value = []
    currentConversationId.value = ''
  }

  return {
    conversations,
    currentConversationId,
    messages,
    input,
    loading,
    status,
    traceId,
    health,
    errorMessage,
    checkHealth,
    loadConversations,
    createConversation,
    selectConversation,
    sendStream,
    stop,
    clearMessages,
  }
})
