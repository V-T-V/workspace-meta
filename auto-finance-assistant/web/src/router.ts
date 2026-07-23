import { createRouter, createWebHistory } from 'vue-router'
import ChatView from './views/ChatView.vue'
import KnowledgeView from './views/KnowledgeView.vue'
import FaqView from './views/FaqView.vue'
import HistoryView from './views/HistoryView.vue'
import FinanceView from './views/FinanceView.vue'
import SettingsView from './views/SettingsView.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/chat' },
    { path: '/chat', name: 'chat', component: ChatView, meta: { title: '客服问答' } },
    { path: '/knowledge', name: 'knowledge', component: KnowledgeView, meta: { title: '知识库' } },
    { path: '/faq', name: 'faq', component: FaqView, meta: { title: 'FAQ 管理' } },
    { path: '/history', name: 'history', component: HistoryView, meta: { title: '会话历史' } },
    { path: '/finance', name: 'finance', component: FinanceView, meta: { title: '金融试算' } },
    { path: '/settings', name: 'settings', component: SettingsView, meta: { title: '系统设置' } },
  ],
})
