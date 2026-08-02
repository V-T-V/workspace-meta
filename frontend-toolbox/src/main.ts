// =============================================================================
// 前端工具箱 · 入口
// 挂载外壳 → 初始化路由 → 在首页 / 工具页之间切换
// =============================================================================

import { TOOL_METAS, findMeta, loadTool } from './core/registry.ts';
import { initRouter, onRoute } from './core/router.ts';
import { renderShell, type ShellHandle } from './shell/Shell.ts';
import { renderHome } from './shell/Home.ts';
import { pushRecent } from './core/storage.ts';
import { el, errorBox } from './ui/components.ts';
import type { ToolInstance } from './types.ts';

const app = document.getElementById('app');
if (!app) throw new Error('缺少 #app 挂载点');

// 渲染外壳
const shell: ShellHandle = renderShell(app, TOOL_METAS);

let currentTool: ToolInstance | null = null;

/** 卸载当前工具（若有）。 */
function destroyCurrentTool(): void {
  if (currentTool?.destroy) {
    try {
      currentTool.destroy();
    } catch (e) {
      console.warn('[main] 工具卸载失败：', e);
    }
  }
  currentTool = null;
}

/** 显示首页。 */
function showHome(): void {
  destroyCurrentTool();
  shell.setActiveTool(null);
  renderHome(shell.main, TOOL_METAS);
}

/** 加载并显示某个工具。 */
async function showTool(id: string): Promise<void> {
  const meta = findMeta(id);
  if (!meta) {
    destroyCurrentTool();
    shell.setActiveTool(null);
    shell.main.replaceChildren();
    const box = el('div', undefined);
    box.style.padding = '60px 20px';
    box.style.textAlign = 'center';
    box.style.color = 'var(--text-muted)';
    box.append(
      el('p', undefined, `找不到工具：${id}`),
      Object.assign(el('p', undefined), {}).append(
        (() => {
          const a = document.createElement('a');
          a.href = '#/';
          a.textContent = '← 返回首页';
          a.style.color = 'var(--accent)';
          return a;
        })(),
      ) as unknown as Node,
    );
    shell.main.append(box);
    return;
  }

  // 记录最近使用
  pushRecent(id);
  shell.setActiveTool(id);

  // 先清空主区并显示加载态
  shell.main.replaceChildren();
  const loading = el('div', 'boot-loading');
  const spinner = el('div', 'boot-spinner');
  loading.append(spinner, el('p', undefined, `加载 ${meta.title}…`));
  shell.main.append(loading);

  // 懒加载工具实现
  const tool = await loadTool(id);
  destroyCurrentTool();
  shell.main.replaceChildren();

  if (!tool) {
    shell.main.append(errorBox(`工具 ${meta.title} 加载失败，请稍后重试。`));
    return;
  }

  const instance = tool.create();
  currentTool = instance;
  try {
    await instance.mount({ container: shell.main });
  } catch (e) {
    shell.main.append(errorBox(`运行出错：${(e as Error).message}`));
    console.error(`[main] 工具 ${id} 运行出错：`, e);
  }
}

// 初始化路由 + 订阅（onRoute 注册时立即触发一次，驱动首屏渲染）
initRouter();
onRoute((route) => {
  if (route.view === 'tool' && route.id) {
    void showTool(route.id);
  } else {
    showHome();
  }
});
