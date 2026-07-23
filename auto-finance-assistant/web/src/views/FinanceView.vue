<script setup lang="ts">
import { ref } from 'vue'

const principal = ref(200000)
const annualRate = ref(4.5)
const months = ref(36)
const calcType = ref<'equal-payment' | 'equal-principal'>('equal-payment')
const result = ref<any>(null)
const errorMsg = ref('')

// 首付计算
const vehiclePrice = ref(200000)
const downPaymentPct = ref(0.2)
const downResult = ref<any>(null)

async function calcLoan() {
  errorMsg.value = ''
  result.value = null
  const endpoint = calcType.value === 'equal-payment' ? 'equal-payment' : 'equal-principal'
  const resp = await fetch(`/api/finance/${endpoint}`, {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ principal: principal.value, annualRate: annualRate.value, months: months.value }),
  })
  const data = await resp.json()
  if (!resp.ok) { errorMsg.value = data?.error?.message || '计算失败'; return }
  result.value = data
}

async function calcDown() {
  const resp = await fetch('/api/finance/down-payment', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ vehiclePrice: vehiclePrice.value, downPaymentPct: downPaymentPct.value }),
  })
  downResult.value = await resp.json()
}
</script>

<template>
  <div class="page">
    <h2>金融试算</h2>
    <div class="grid">
      <section class="card">
        <h3>贷款试算</h3>
        <div class="type-tabs">
          <button :class="{ active: calcType === 'equal-payment' }" @click="calcType = 'equal-payment'">等额本息</button>
          <button :class="{ active: calcType === 'equal-principal' }" @click="calcType = 'equal-principal'">等额本金</button>
        </div>
        <label>贷款本金（元）<input type="number" v-model.number="principal" /></label>
        <label>年利率（%）<input type="number" step="0.01" v-model.number="annualRate" /></label>
        <label>期数（月）<input type="number" v-model.number="months" /></label>
        <button class="primary-btn" @click="calcLoan">计算</button>
        <div class="error" v-if="errorMsg">{{ errorMsg }}</div>
        <div class="result" v-if="result">
          <div class="result-row"><span>每月还款</span><strong>{{ result.monthlyPayment || (result.firstPayment + ' ~ ' + result.lastPayment) }}</strong></div>
          <div class="result-row" v-if="result.monthlyPrincipal"><span>每月本金</span><strong>{{ result.monthlyPrincipal }}</strong></div>
          <div class="result-row"><span>总还款</span><strong>{{ result.totalPayment }}</strong></div>
          <div class="result-row"><span>总利息</span><strong>{{ result.totalInterest }}</strong></div>
          <div class="disclaimer">{{ result.disclaimer }}</div>
        </div>
      </section>

      <section class="card">
        <h3>首付计算</h3>
        <label>车价（元）<input type="number" v-model.number="vehiclePrice" /></label>
        <label>首付比例<input type="range" min="0" max="1" step="0.05" v-model.number="downPaymentPct" /> <span class="pct">{{ (downPaymentPct * 100).toFixed(0) }}%</span></label>
        <button class="primary-btn" @click="calcDown">计算</button>
        <div class="result" v-if="downResult">
          <div class="result-row"><span>首付金额</span><strong>{{ downResult.downPayment }}</strong></div>
          <div class="result-row"><span>贷款本金</span><strong>{{ downResult.loanPrincipal }}</strong></div>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.page { padding: 24px; height: 100%; overflow-y: auto; }
.page h2 { font-size: 20px; margin-bottom: 20px; }
.grid { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; }
.card { background: white; border-radius: 12px; padding: 24px; }
.card h3 { font-size: 16px; margin-bottom: 16px; }
.type-tabs { display: flex; gap: 8px; margin-bottom: 16px; }
.type-tabs button { flex: 1; padding: 8px; border: 1px solid var(--border); background: var(--bg); border-radius: 8px; }
.type-tabs button.active { background: var(--primary); color: white; border-color: var(--primary); }
label { display: flex; flex-direction: column; gap: 6px; font-size: 13px; color: var(--muted); margin-bottom: 12px; }
label input { border: 1px solid var(--border); border-radius: 6px; padding: 8px; font-size: 14px; }
.pct { color: var(--primary); font-weight: 500; }
.primary-btn { background: var(--primary); color: white; padding: 10px 20px; border-radius: 8px; width: 100%; }
.primary-btn:hover { background: var(--primary-hover); }
.error { color: var(--error); font-size: 13px; margin-top: 8px; }
.result { margin-top: 16px; background: var(--bg); border-radius: 8px; padding: 16px; }
.result-row { display: flex; justify-content: space-between; padding: 6px 0; font-size: 14px; }
.result-row strong { font-size: 16px; }
.disclaimer { margin-top: 12px; font-size: 12px; color: var(--muted); line-height: 1.5; padding-top: 12px; border-top: 1px solid var(--border); }
</style>
