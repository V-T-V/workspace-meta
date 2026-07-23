#!/usr/bin/env node
// 100+ 真实场景验证 runner
// 单轮：scenarios.jsonl | 多轮连续问答：conversations.jsonl
import { readFileSync } from 'node:fs';

const BASE = 'http://127.0.0.1:8080';
const sleep = ms => new Promise(r => setTimeout(r, ms));

function loadJsonl(path) {
  return readFileSync(path, 'utf-8').trim().split('\n')
    .filter(l => l && !l.startsWith('#')).map(JSON.parse);
}

async function chat(question, conversationId) {
  const body = { question };
  if (conversationId) body.conversationId = conversationId;
  const r = await fetch(`${BASE}/api/chat`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json; charset=utf-8' },
    body: JSON.stringify(body),
  });
  const j = await r.json();
  return j;
}

async function createConversation(title) {
  const r = await fetch(`${BASE}/api/conversations`, {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title }),
  });
  return (await r.json()).id;
}

function evalScenario(sc, answer, intent) {
  const fails = [];
  // 拒答检查
  if (sc.shouldRefuse) {
    if (intent !== 'refuse' && intent !== 'compliance_refuse' && !answer.includes('无法') && !answer.includes('不能保证') && !answer.includes('机密')) {
      fails.push(`应拒答但未拒答(intent=${intent})`);
    }
  } else {
    if (intent === 'refuse' && !sc.allowModel) {
      fails.push('不应拒答但被拒');
    }
    // 必须包含的事实
    for (const fact of sc.expectContains || []) {
      if (!answer.includes(fact)) {
        fails.push(`缺少: ${fact}`);
      }
    }
  }
  return fails;
}

// ===== 单轮场景 =====
async function runSingleScenarios() {
  console.log('\n════════ 单轮场景 ════════');
  const scenarios = loadJsonl('evaluations/scenarios.jsonl');
  const stats = { total: scenarios.length, pass: 0, fail: 0, byCat: {} };
  const fails = [];

  for (const sc of scenarios) {
    process.stdout.write(`[${sc.id}] ${sc.q.slice(0, 30)}... `);
    const resp = await chat(sc.q);
    const failsList = evalScenario(sc, resp.answer || '', resp.intent || '');
    if (failsList.length === 0) {
      console.log('✓');
      stats.pass++;
      stats.byCat[sc.cat] = stats.byCat[sc.cat] || { p: 0, f: 0 };
      stats.byCat[sc.cat].p++;
    } else {
      console.log(`✗ ${failsList[0]}`);
      stats.fail++;
      stats.byCat[sc.cat] = stats.byCat[sc.cat] || { p: 0, f: 0 };
      stats.byCat[sc.cat].f++;
      fails.push({ id: sc.id, q: sc.q, fails: failsList, answer: (resp.answer||'').slice(0,100) });
    }
    await sleep(100); // 避免 CPU 过载
  }

  console.log(`\n单轮汇总: ${stats.pass}/${stats.total} (${(stats.pass/stats.total*100).toFixed(1)}%)`);
  console.log('按分类:');
  for (const [cat, s] of Object.entries(stats.byCat)) {
    console.log(`  ${cat}: ${s.p}/${s.p+s.f}`);
  }
  return { stats, fails };
}

// ===== 多轮连续问答 =====
async function runConversations() {
  console.log('\n════════ 多轮连续问答 ════════');
  const convs = loadJsonl('evaluations/conversations.jsonl');
  const stats = { total: 0, pass: 0, fail: 0, turnPass: 0, turnFail: 0 };
  const fails = [];

  for (const cv of convs) {
    const convId = await createConversation(cv.desc);
    let convPass = true;
    process.stdout.write(`[${cv.id}] ${cv.desc}: `);

    for (let i = 0; i < cv.turns.length; i++) {
      const turn = cv.turns[i];
      const resp = await chat(turn.q, convId);
      stats.total++;

      // 评估这一轮
      let turnFails = [];
      if (turn.expectContains && turn.expectContains.length > 0) {
        for (const fact of turn.expectContains) {
          if (!(resp.answer || '').includes(fact)) {
            turnFails.push(`缺${fact}`);
          }
        }
      }
      if (resp.intent === 'refuse') {
        turnFails.push('不应拒答');
      }

      if (turnFails.length === 0) {
        stats.turnPass++;
        process.stdout.write(`T${i+1}✓ `);
      } else {
        stats.turnFail++;
        convPass = false;
        process.stdout.write(`T${i+1}✗(${turnFails[0]}) `);
        fails.push({ id: cv.id, turn: i+1, q: turn.q, fails: turnFails, answer: (resp.answer||'').slice(0,80) });
      }
      await sleep(200);
    }

    if (convPass) { stats.pass++; console.log('→ 全通过'); }
    else { stats.fail++; console.log('→ 有失败'); }
  }

  console.log(`\n多轮汇总: 会话 ${stats.pass}/${stats.pass+stats.fail}, 轮次 ${stats.turnPass}/${stats.total} (${(stats.turnPass/stats.total*100).toFixed(1)}%)`);
  return { stats, fails };
}

// ===== 主流程 =====
async function main() {
  console.log('╔══════════════════════════════════════╗');
  console.log('║  100+ 真实场景验证 + 连续问答测试    ║');
  console.log('╚══════════════════════════════════════╝');

  // 健康检查
  const h = await (await fetch(`${BASE}/api/health`)).json();
  if (h.status !== 'ok') { console.error('服务不健康:', h); process.exit(1); }
  console.log(`服务正常 · 模型: ${h.model}`);

  const t0 = Date.now();
  const single = await runSingleScenarios();
  const multi = await runConversations();
  const elapsed = ((Date.now() - t0) / 1000).toFixed(0);

  // 总结
  console.log('\n════════════ 总览 ════════════');
  console.log(`单轮: ${single.stats.pass}/${single.stats.total} | 多轮会话: ${multi.stats.pass}/${multi.stats.pass+multi.stats.fail} (${multi.stats.turnPass}/${multi.stats.total}轮)`);
  console.log(`总查询: ${single.stats.total + multi.stats.total} | 耗时: ${elapsed}s`);

  const allFails = [...single.fails, ...multi.fails];
  if (allFails.length > 0) {
    console.log(`\n失败详情 (${allFails.length}):`);
    allFails.slice(0, 15).forEach(f => {
      console.log(`  ${f.id}${f.turn ? ' T'+f.turn : ''}: ${f.fails.join(', ')}`);
      console.log(`    Q: ${f.q}`);
      console.log(`    A: ${f.answer}`);
    });
    if (allFails.length > 15) console.log(`  ... 还有 ${allFails.length - 15} 个`);
  }
  process.exit(allFails.length > 0 ? 1 : 0);
}
main().catch(e => { console.error(e); process.exit(1); });
