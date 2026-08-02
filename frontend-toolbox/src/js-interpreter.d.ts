declare module '@babel/standalone' {
  export function transform(code: string, options: Record<string, unknown>): { code: string | null };
}

declare module 'js-interpreter' {
  export interface Scope {
    variables?: Array<{ name: string; value?: unknown }>;
  }
  export class Interpreter {
    constructor(code: string, initFunc?: (i: Interpreter, scope: unknown) => void);
    step(): boolean;
    run(): boolean;
    value: unknown;
    paused_: boolean;
    getScope(): Scope;
    createNativeFunction(fn: (...args: unknown[]) => unknown): unknown;
    createObject(): unknown;
    setProperty(obj: unknown, name: string, value: unknown): void;
    nativeToPseudo(v: unknown): unknown;
  }
  const _default: typeof Interpreter | { Interpreter: typeof Interpreter };
  export default _default;
}
