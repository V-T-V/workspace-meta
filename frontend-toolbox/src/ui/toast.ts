// =============================================================================
// 全局 Toast 提示 —— 复制成功 / 错误 / 信息，3 秒自动消失
// =============================================================================

type ToastKind = 'success' | 'error' | 'info';

let stack: HTMLElement | null = null;

function getStack(): HTMLElement {
  if (stack) return stack;
  stack = document.getElementById('toast-stack');
  if (!stack) {
    stack = document.createElement('div');
    stack.id = 'toast-stack';
    stack.className = 'toast-stack';
    document.body.append(stack);
  }
  return stack;
}

export function toast(message: string, kind: ToastKind = 'info', durationMs = 2400): void {
  const node = document.createElement('div');
  node.className = `toast toast--${kind}`;
  const icon = kind === 'success' ? '✓' : kind === 'error' ? '✕' : 'ℹ';
  node.append(
    Object.assign(document.createElement('span'), { className: 'toast-icon', textContent: icon }),
    Object.assign(document.createElement('span'), { className: 'toast-msg', textContent: message }),
  );
  getStack().append(node);
  // 入场动画
  requestAnimationFrame(() => node.classList.add('toast--in'));
  // 自动消失
  setTimeout(() => {
    node.classList.remove('toast--in');
    node.classList.add('toast--out');
    setTimeout(() => node.remove(), 300);
  }, durationMs);
}

export const toastSuccess = (m: string): void => toast(m, 'success');
export const toastError = (m: string): void => toast(m, 'error');
export const toastInfo = (m: string): void => toast(m, 'info');
