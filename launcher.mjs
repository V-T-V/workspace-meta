#!/usr/bin/env node
/**
 * M_X_M 工作区本地启动器。
 *
 * 用途：让 workspace-overview 网页能"一键启动项目"——
 *   网页点"启动" → 调本 launcher → 后台跑 npm run dev → 轮询端口 → 就绪后网页开新 tab。
 *
 * 用法：
 *   node launcher.mjs            # 启动 launcher，监听 :17888
 *
 * 然后用浏览器打开 workspace-overview-v2.html，点项目卡片的"启动"按钮。
 *
 * 安全：仅监听 127.0.0.1，不对外。只允许启动预定义的项目白名单。
 */

import { createServer } from 'node:http';
import { spawn } from 'node:child_process';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { dirname } from 'node:path';

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)));
const LAUNCHER_PORT = 17888;

/**
 * 项目清单：可启动的项目配置。
 * port 是分配的独立端口（避免多项目都抢 5173）。
 * cmd 是启动命令（数组形式，第一项是命令名）。
 * readyPath 是轮询就绪的路径（返回 200 视为启动成功）。
 */
const PROJECTS = {
  agenttrain:         { dir: 'agenttrain',         port: 5180, cmd: ['npm','run','dev','--','--port','5180'], readyPath: '/' },
  'kids-games':       { dir: 'kids-games',         port: 5181, cmd: ['npm','run','dev','--','--port','5181'], readyPath: '/' },
  'algorithms-atlas': { dir: 'algorithms-atlas',   port: 5182, cmd: ['npm','run','dev','--','--port','5182'], readyPath: '/' },
  wildera:            { dir: 'wildera',            port: 5183, cmd: ['npm','run','dev','--','--port','5183'], readyPath: '/' },
  'future-world-3055':{ dir: 'future-world-3055',  port: 5184, cmd: ['npm','run','dev','--','--port','5184'], readyPath: '/' },
  'taixu-dao-world':  { dir: 'taixu-dao-world',    port: 5185, cmd: ['npm','run','dev','--','--port','5185','--host','127.0.0.1'], readyPath: '/' },
  dashan:             { dir: 'dashan',             port: 5186, cmd: ['npm','run','dev'], readyPath: '/', note: 'concurrently 起 web+server' },
  tripplan:           { dir: 'tripplan',           port: 5187, cmd: ['npm','run','dev'], readyPath: '/', note: 'concurrently 起 web+server' },
  stockai:            { dir: 'stock-ai',           port: 0,    cmd: ['npm','run','agent'], readyPath: null, note: 'CLI REPL，无 web 端口' },
  'ai-expansion-analysis': { dir: 'ai-expansion-analysis', port: 0, cmd: [], readyPath: null, note: '静态 HTML，直接打开文件，无需启动' },
};

// 运行中的进程表：key -> { proc, status, port, startedAt }
const running = new Map();

import http from 'node:http';

// 清除代理环境变量，避免本机 HTTP 代理拦截 127.0.0.1 的本地连接
delete process.env.HTTP_PROXY; delete process.env.http_proxy;
delete process.env.HTTPS_PROXY; delete process.env.https_proxy;
delete process.env.ALL_PROXY; delete process.env.all_proxy;
process.env.NO_PROXY = '127.0.0.1,localhost';
process.env.no_proxy = '127.0.0.1,localhost';

/** 端口就绪轮询：直连 localhost（vite 默认监听 localhost/IPv6，不用 127.0.0.1），连上且返回 < 500 视为就绪 */
function pollReady(port, path, timeoutMs = 60000) {
  const start = Date.now();
  return new Promise((resolvePromise) => {
    const tryOnce = () => {
      if (Date.now() - start > timeoutMs) return resolvePromise(false);
      const req = http.get({ hostname: 'localhost', port, path, agent: false }, (res) => {
        resolvePromise(res.statusCode < 500);
        res.resume();
      });
      req.on('error', () => setTimeout(tryOnce, 800));
      req.setTimeout(2000, () => { req.destroy(); setTimeout(tryOnce, 800); });
    };
    tryOnce();
  });
}

/** 启动一个项目（立即返回，后台轮询就绪；前端用 /status 查结果） */
async function startProject(key) {
  const conf = PROJECTS[key];
  if (!conf) return { ok: false, error: '未知项目' };
  if (running.has(key) && running.get(key).status === 'ready') {
    return { ok: true, status: 'ready', port: conf.port, url: conf.port ? `http://localhost:${conf.port}` : null, note: '已在运行' };
  }
  if (running.has(key) && running.get(key).status === 'starting') {
    return { ok: true, status: 'starting', note: '正在启动中…' };
  }

  const cwd = resolve(ROOT, conf.dir);
  const proc = spawn(conf.cmd[0], conf.cmd.slice(1), { cwd, shell: true, detached: false, stdio: ['ignore','pipe','pipe'] });

  const rec = { proc, status: 'starting', port: conf.port, startedAt: new Date().toISOString(), logs: [] };
  running.set(key, rec);

  const pushLog = (d) => { const s = d.toString().trim(); if (s) rec.logs.push(s.slice(-500)); if (rec.logs.length > 50) rec.logs.shift(); };
  proc.stdout.on('data', pushLog);
  proc.stderr.on('data', pushLog);
  proc.on('exit', (code) => { rec.status = 'stopped'; rec.exitCode = code; });

  // 无 web 端口的项目（CLI/静态）直接标记就绪
  if (!conf.port || !conf.readyPath) {
    rec.status = 'ready';
    return { ok: true, status: 'ready', note: conf.note, port: conf.port };
  }

  // 后台异步轮询端口就绪（不阻塞 HTTP 响应）
  pollReady(conf.port, conf.readyPath).then((ready) => {
    rec.status = ready ? 'ready' : 'started_unverified';
  });

  return { ok: true, status: 'starting', port: conf.port, url: conf.port ? `http://localhost:${conf.port}` : null, note: '已发起启动，正在编译…（首次需几十秒）' };
}

/** 停止项目 */
function stopProject(key) {
  const rec = running.get(key);
  if (!rec) return { ok: false, error: '未在运行' };
  try { rec.proc.kill('SIGTERM'); } catch {}
  rec.status = 'stopped';
  running.delete(key);
  return { ok: true };
}

/** HTTP 服务 */
const server = createServer(async (req, res) => {
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Content-Type', 'application/json; charset=utf-8');
  const url = new URL(req.url, `http://127.0.0.1:${LAUNCHER_PORT}`);

  // GET /list —— 所有可启动项目
  if (req.method === 'GET' && url.pathname === '/list') {
    const list = Object.entries(PROJECTS).map(([k, c]) => ({
      key: k, dir: c.dir, port: c.port, note: c.note || null,
      running: running.has(k) ? running.get(k).status : 'stopped',
    }));
    return res.end(JSON.stringify({ ok: true, projects: list }));
  }

  // GET /status?key=xx —— 单个状态
  if (req.method === 'GET' && url.pathname === '/status') {
    const key = url.searchParams.get('key');
    const rec = running.get(key);
    return res.end(JSON.stringify({ ok: true, status: rec ? rec.status : 'stopped', port: rec?.port, logs: rec?.logs?.slice(-10) }));
  }

  // POST /start?key=xx —— 启动（阻塞到就绪或超时）
  if (req.method === 'POST' && url.pathname === '/start') {
    const key = url.searchParams.get('key');
    const result = await startProject(key);
    return res.end(JSON.stringify(result));
  }

  // POST /stop?key=xx —— 停止
  if (req.method === 'POST' && url.pathname === '/stop') {
    const key = url.searchParams.get('key');
    return res.end(JSON.stringify(stopProject(key)));
  }

  res.statusCode = 404;
  res.end(JSON.stringify({ ok: false, error: '未知路由' }));
});

server.listen(LAUNCHER_PORT, '127.0.0.1', () => {
  console.log(`\n  🚀 M_X_M 工作区启动器已运行：http://127.0.0.1:${LAUNCHER_PORT}`);
  console.log(`  打开 workspace-overview-v2.html，点项目卡片"启动"按钮即可。\n`);
  console.log(`  可启动项目：`);
  Object.entries(PROJECTS).forEach(([k, c]) => {
    console.log(`    ${k.padEnd(22)} :${c.port || 'CLI'}  (${c.cmd.join(' ') || '静态文件'})`);
  });
  console.log(`\n  Ctrl+C 退出（会停止所有已启动的项目）。\n`);
});

// 退出时清理所有子进程
process.on('SIGINT', () => {
  console.log('\n正在停止所有项目...');
  for (const [k, rec] of running) {
    try { rec.proc.kill('SIGTERM'); console.log(`  ✓ 停止 ${k}`); } catch {}
  }
  process.exit(0);
});
