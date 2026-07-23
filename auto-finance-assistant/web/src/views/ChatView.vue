<script setup lang="ts">
import { ref, nextTick, watch } from 'vue'
import { useChatStore } from '../stores/chat'

const chat = useChatStore()
const scrollContainer = ref<HTMLElement>()

async function scrollToBottom() {
  await nextTick()
  if (scrollContainer.value) {
    scrollContainer.value.scrollTop = scrollContainer.value.scrollHeight
  }
}

watch(() => chat.messages.length, scrollToBottom)
watch(
  () => chat.messages[chat.messages.length - 1]?.content,
  scrollToBottom
)

async function onSend() {
  await chat.sendStream()
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    onSend()
  }
}

function copy(text: string) {
  navigator.clipboard.writeText(text)
}

const suggestions = [
  '你好，你是做什么的？',
  '申请新车贷款需要哪些资料？',
  '等额本息和等额本金有什么区别？',
]
</script>

<template>
  <div class="chat-view">
    <div class="messages" ref="scrollContainer">
      <div v-if="chat.messages.length === 0" class="empty">
        <h2>汽车金融客服助手</h2>
        <p>我可以帮你解答汽车金融产品政策、申请材料、业务流程等问题。</p>
        <div class="suggestions">
          <button
            v-for="s in suggestions"
            :key="s"
            class="suggestion"
            @click="chat.input = s; onSend()"
          >
            {{ s }}
          </button>
        </div>
        <p class="hint" v-if="chat.health?.detail">
          ⚠️ {{ chat.health.detail }}
        </p>
      </div>

      <div
        v-for="msg in chat.messages"
        :key="msg.id"
        class="message"
        :class="msg.role"
      >
        <div class="avatar">{{ msg.role === 'user' ? '我' : '助手' }}</div>
        <div class="bubble-wrap">
          <div class="bubble" :class="{ error: msg.error }">
            {{ msg.content }}
            <span v-if="msg.streaming" class="cursor">▋</span>
          </div>
          <div class="meta" v-if="msg.role === 'assistant' && !msg.streaming && msg.content">
            <button class="meta-btn" @click="copy(msg.content)">复制</button>
          </div>
        </div>
      </div>

      <div class="status-bar" v-if="chat.loading">
        <span class="spinner"></span>
        {{ chat.status || '处理中…' }}
      </div>
    </div>

    <div class="error-banner" v-if="chat.errorMessage">
      {{ chat.errorMessage }}
    </div>

    <div class="composer">
      <textarea
        v-model="chat.input"
        @keydown="onKeydown"
        placeholder="输入问题，Enter 发送，Shift+Enter 换行…"
        rows="1"
        :disabled="chat.loading"
      ></textarea>
      <button class="send" @click="onSend" :disabled="chat.loading || !chat.input.trim()">
        发送
      </button>
      <button class="stop" v-if="chat.loading" @click="chat.stop()">停止</button>
    </div>
  </div>
</template>

<style scoped>
.chat-view {
  display: flex;
  flex-direction: column;
  height: 100%;
}
.messages {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
}
.empty {
  text-align: center;
  padding-top: 80px;
  color: var(--muted);
}
.empty h2 {
  color: var(--text);
  margin-bottom: 8px;
}
.suggestions {
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-items: center;
  margin: 24px auto;
  max-width: 400px;
}
.suggestion {
  background: var(--panel);
  border: 1px solid var(--border);
  color: var(--text);
  padding: 10px 16px;
  width: 100%;
  text-align: left;
}
.suggestion:hover {
  border-color: var(--primary);
  color: var(--primary);
}
.hint {
  margin-top: 24px;
  color: var(--error);
  font-size: 13px;
}
.message {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
  max-width: 820px;
  margin-left: auto;
  margin-right: auto;
}
.message.user {
  flex-direction: row-reverse;
}
.avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: var(--muted);
  color: white;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  flex-shrink: 0;
}
.message.user .avatar {
  background: var(--primary);
}
.bubble-wrap {
  display: flex;
  flex-direction: column;
  max-width: 70%;
}
.message.user .bubble-wrap {
  align-items: flex-end;
}
.bubble {
  padding: 10px 14px;
  border-radius: var(--radius);
  white-space: pre-wrap;
  word-break: break-word;
  background: var(--assistant-bubble);
}
.message.user .bubble {
  background: var(--user-bubble);
  color: white;
}
.bubble.error {
  background: #fef2f2;
  color: var(--error);
  border: 1px solid #fecaca;
}
.cursor {
  animation: blink 1s infinite;
  color: var(--primary);
}
@keyframes blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}
.meta {
  margin-top: 4px;
  display: flex;
  gap: 8px;
}
.meta-btn {
  background: transparent;
  color: var(--muted);
  font-size: 12px;
  padding: 2px 8px;
}
.meta-btn:hover {
  color: var(--primary);
}
.status-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--muted);
  font-size: 13px;
  padding: 8px 24px;
}
.spinner {
  width: 14px;
  height: 14px;
  border: 2px solid var(--border);
  border-top-color: var(--primary);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}
@keyframes spin {
  to { transform: rotate(360deg); }
}
.error-banner {
  background: #fef2f2;
  color: var(--error);
  padding: 10px 24px;
  font-size: 13px;
}
.composer {
  border-top: 1px solid var(--border);
  background: var(--panel);
  padding: 12px 24px;
  display: flex;
  gap: 8px;
  align-items: flex-end;
}
.composer textarea {
  flex: 1;
  resize: none;
  max-height: 120px;
  padding: 10px 12px;
}
.send {
  background: var(--primary);
  color: white;
  padding: 10px 20px;
}
.send:hover:not(:disabled) {
  background: var(--primary-hover);
}
.stop {
  background: var(--error);
  color: white;
  padding: 10px 16px;
}
</style>
