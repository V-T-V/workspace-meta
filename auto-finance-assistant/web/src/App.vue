<script setup lang="ts">
import { onMounted, onUnmounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { useChatStore } from './stores/chat'

const chat = useChatStore()
const route = useRoute()
const currentTitle = computed(() => (route.meta.title as string) || '客服助手')
let healthTimer: number | undefined

const navItems = [
  { name: 'chat', label: '客服问答', icon: '💬' },
  { name: 'knowledge', label: '知识库', icon: '📚' },
  { name: 'faq', label: 'FAQ', icon: '❓' },
  { name: 'history', label: '会话历史', icon: '📋' },
  { name: 'finance', label: '金融试算', icon: '🧮' },
  { name: 'settings', label: '系统设置', icon: '⚙️' },
]

onMounted(() => {
  chat.checkHealth()
  healthTimer = window.setInterval(() => chat.checkHealth(), 30000)
})
onUnmounted(() => {
  if (healthTimer) clearInterval(healthTimer)
})
</script>

<template>
  <div class="app-layout">
    <aside class="sidebar">
      <div class="brand">
        <h1>汽车金融客服</h1>
      </div>
      <nav class="nav">
        <RouterLink
          v-for="item in navItems"
          :key="item.name"
          :to="`/${item.name}`"
          class="nav-item"
          :class="{ active: route.name === item.name }"
        >
          <span class="icon">{{ item.icon }}</span>
          <span>{{ item.label }}</span>
        </RouterLink>
      </nav>
      <div class="health" v-if="chat.health">
        <div class="health-row">
          <span class="dot" :class="chat.health.status"></span>
          <span>系统 {{ chat.health.status }}</span>
        </div>
        <div class="health-row">
          <span class="dot" :class="chat.health.ollama"></span>
          <span>Ollama {{ chat.health.ollama }}</span>
        </div>
        <div class="model">模型：{{ chat.health.model }}</div>
        <div class="detail" v-if="chat.health.detail">{{ chat.health.detail }}</div>
      </div>
    </aside>
    <main class="main">
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.app-layout {
  display: grid;
  grid-template-columns: 220px 1fr;
  height: 100vh;
}
.sidebar {
  background: var(--panel);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  padding: 16px 12px;
  gap: 12px;
  overflow-y: auto;
}
.brand h1 {
  font-size: 15px;
  font-weight: 600;
  padding: 0 8px;
}
.nav {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
}
.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 8px;
  color: var(--muted);
  font-size: 14px;
  text-decoration: none;
  transition: all 0.15s;
}
.nav-item:hover {
  background: var(--bg);
  color: var(--text);
}
.nav-item.active {
  background: #eff6ff;
  color: var(--primary);
  font-weight: 500;
}
.icon {
  font-size: 16px;
}
.health {
  border-top: 1px solid var(--border);
  padding-top: 12px;
  font-size: 12px;
  color: var(--muted);
}
.health-row {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 4px;
}
.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--muted);
}
.dot.ok { background: var(--success); }
.dot.down, .dot.degraded, .dot.missing_model { background: var(--error); }
.model { margin-top: 4px; }
.detail { margin-top: 6px; color: var(--error); line-height: 1.4; }
.main { overflow: hidden; }
</style>
