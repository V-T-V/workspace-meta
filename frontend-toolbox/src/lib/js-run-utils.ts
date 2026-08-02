// =============================================================================
// JS 运行工具库 —— Babel 转译 ES6+ → ES5 + js-interpreter 沙箱执行
// 懒加载依赖（体积大，仅在 js-run 工具打开时加载）。
// =============================================================================

import type { Interpreter, Scope } from 'js-interpreter';

/** 懒加载 @babel/standalone 并转译 ES6+ 代码为 ES5。 */
export async function transpileToES5(code: string): Promise<{ code: string; error?: string }> {
  try {
    const Babel = await import('@babel/standalone');
    const result = Babel.transform(code, {
      presets: ['env'],
      filename: 'user-code.js',
    });
    return { code: result.code ?? code };
  } catch (e) {
    return { code, error: (e as Error).message };
  }
}

/** 懒加载 js-interpreter 构造器。 */
async function loadInterpreterCtor(): Promise<typeof Interpreter> {
  const mod = await import('js-interpreter');
  const Ctor = (mod as { Interpreter?: typeof Interpreter; default?: unknown }).Interpreter;
  if (Ctor) return Ctor;
  // 某些打包方式导出 default
  return mod.default as unknown as typeof Interpreter;
}

export interface RunResult {
  output: string[];
  error?: string;
  returnValue?: unknown;
}

export interface StepState {
  done: boolean;
  output: string[];
  variables: Array<{ name: string; value: string }>;
}

/** 创建带 console.log 捕获的沙箱解释器。 */
export async function createRunner(
  code: string,
  onOutput: (line: string) => void,
): Promise<{
  run: (maxSteps?: number) => RunResult;
  step: () => StepState;
  reset: () => void;
}> {
  const InterpreterCtor = await loadInterpreterCtor();
  const outputs: string[] = [];

  const initFunc = (interpreter: Interpreter, scope: unknown): void => {
    const consoleObj = interpreter.createObject();
    const logFn = interpreter.createNativeFunction((...args: unknown[]) => {
      const line = args
        .map((a) => {
          if (a === null) return 'null';
          if (a === undefined) return 'undefined';
          if (typeof a === 'object') {
            try {
              return JSON.stringify(a);
            } catch {
              return String(a);
            }
          }
          return String(a);
        })
        .join(' ');
      outputs.push(line);
      onOutput(line);
    });
    interpreter.setProperty(consoleObj, 'log', logFn);
    interpreter.setProperty(scope, 'console', consoleObj);
  };

  let interpreter: Interpreter;
  try {
    interpreter = new InterpreterCtor(code, initFunc);
  } catch (e) {
    return {
      run: () => ({ output: [], error: '初始化失败：' + (e as Error).message }),
      step: () => ({ done: true, output: [], variables: [] }),
      reset: () => {},
    };
  }

  return {
    run(maxSteps = 1_000_000): RunResult {
      try {
        let steps = 0;
        while (interpreter.step() && steps < maxSteps) {
          steps++;
        }
        if (steps >= maxSteps) {
          return { output: outputs, error: `超过最大步数 ${maxSteps}（可能死循环）` };
        }
        return { output: outputs, returnValue: interpreter.value };
      } catch (e) {
        return { output: outputs, error: (e as Error).message };
      }
    },
    step(): StepState {
      const done = !interpreter.step();
      const variables = extractVariables(interpreter);
      return { done, output: [...outputs], variables };
    },
    reset(): void {
      outputs.length = 0;
      try {
        interpreter = new InterpreterCtor(code, initFunc);
      } catch {
        /* ignore */
      }
    },
  };
}

/** 从解释器 scope 提取变量。 */
function extractVariables(interpreter: Interpreter): Array<{ name: string; value: string }> {
  try {
    const scope: Scope = interpreter.getScope();
    if (!scope.variables) return [];
    return scope.variables.map((v) => ({
      name: v.name ?? '?',
      value: formatValue(v.value),
    }));
  } catch {
    return [];
  }
}

function formatValue(v: unknown): string {
  if (v === null) return 'null';
  if (v === undefined) return 'undefined';
  if (typeof v === 'string') return `"${v}"`;
  if (typeof v === 'number' || typeof v === 'boolean') return String(v);
  try {
    return JSON.stringify(v);
  } catch {
    return String(v);
  }
}
