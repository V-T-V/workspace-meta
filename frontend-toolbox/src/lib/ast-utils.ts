// =============================================================================
// AST 工具库 —— 基于 @babel/parser 解析 JS/TS 代码为 AST
// 懒加载 Babel（仅浏览器环境），纯函数逻辑可测。
// =============================================================================

import type { File, Node } from '@babel/types';

export interface AstTreeNode {
  /** 节点类型，如 'FunctionDeclaration' */
  type: string;
  /** 显示标签（含关键属性，用于树展示） */
  label: string;
  /** 源码起始位置（0-based） */
  start?: number;
  /** 源码结束位置（0-based） */
  end?: number;
  /** 起始行号 */
  locStartLine?: number;
  /** 子节点 */
  children: AstTreeNode[];
  /** 原始节点引用（用于展开详情） */
  raw?: unknown;
}

/** 懒加载 @babel/parser（仅浏览器有 import.meta.glob 动态 import）。 */
async function loadParser(): Promise<typeof import('@babel/parser')> {
  return await import('@babel/parser');
}

/** 解析 JS/TS 代码为 Babel AST。 */
export async function parseCode(code: string, isTS = false): Promise<File> {
  const parser = await loadParser();
  return parser.parse(code, {
    sourceType: 'unambiguous',
    plugins: isTS ? ['typescript', 'jsx'] : ['jsx'],
    errorRecovery: true,
  }) as unknown as File;
}

/** 把 Babel AST 转为简化的树结构（用于 UI 展示）。 */
export function buildAstTree(node: Node, depth = 0): AstTreeNode {
  const type = node.type;
  const label = makeLabel(node);
  const start = (node as Node & { start?: number }).start;
  const end = (node as Node & { end?: number }).end;
  const locStartLine = node.loc?.start.line;

  const children: AstTreeNode[] = [];
  // 遍历节点的对象属性，找子 Node 或 Node 数组
  for (const key of Object.keys(node)) {
    if (key === 'type' || key === 'loc' || key === 'start' || key === 'end' || key === 'extra' || key === 'leadingComments' || key === 'trailingComments' || key === 'innerComments') {
      continue;
    }
    const value = (node as unknown as Record<string, unknown>)[key];
    if (isNode(value)) {
      const child = buildAstTree(value as Node, depth + 1);
      child.label = `${key}: ${child.label}`;
      children.push(child);
    } else if (Array.isArray(value)) {
      for (const item of value) {
        if (isNode(item)) {
          const child = buildAstTree(item as Node, depth + 1);
          child.label = `${key}[]: ${child.label}`;
          children.push(child);
        }
      }
    }
  }

  // 深度超过 50 防止栈溢出
  if (depth > 50) return { type, label, start, end, locStartLine, children: [], raw: node };

  return { type, label, start, end, locStartLine, children, raw: node };
}

function isNode(v: unknown): v is Node {
  return v !== null && typeof v === 'object' && 'type' in (v as Record<string, unknown>);
}

/** 生成节点的可读标签。 */
function makeLabel(node: Node): string {
  const n = node as unknown as Record<string, unknown>;
  switch (node.type) {
    case 'Identifier':
      return `Identifier: ${n.name ?? ''}`;
    case 'StringLiteral':
      return `String: "${n.value ?? ''}"`;
    case 'NumericLiteral':
      return `Number: ${n.value ?? ''}`;
    case 'BooleanLiteral':
      return `Boolean: ${n.value ?? ''}`;
    case 'FunctionDeclaration':
    case 'FunctionExpression':
    case 'ArrowFunctionExpression': {
      const name = (n.id as { name?: string } | null)?.name ?? 'anonymous';
      const params = Array.isArray(n.params) ? n.params.length : 0;
      return `${node.type}: ${name}(${params} params)`;
    }
    case 'VariableDeclaration':
      return `VariableDeclaration: ${n.kind ?? ''}`;
    case 'VariableDeclarator': {
      const name = (n.id as { name?: string })?.name ?? '';
      return `VariableDeclarator: ${name}`;
    }
    case 'CallExpression':
    case 'NewExpression': {
      const callee = (n.callee as Node)?.type ?? '';
      return `${node.type}: ${callee}`;
    }
    case 'MemberExpression': {
      const obj = (n.object as Node)?.type ?? '';
      const prop = (n.property as { name?: string })?.name ?? '';
      return `Member: ${obj}.${prop}`;
    }
    case 'BinaryExpression':
    case 'LogicalExpression':
    case 'AssignmentExpression':
      return `${node.type}: ${n.operator ?? ''}`;
    case 'IfStatement':
      return 'IfStatement';
    case 'ReturnStatement':
      return 'ReturnStatement';
    case 'ImportDeclaration':
      return `Import: ${(n.source as { value?: string })?.value ?? ''}`;
    case 'ExportNamedDeclaration':
    case 'ExportDefaultDeclaration':
      return node.type;
    default:
      return node.type;
  }
}

/** 统计 AST 信息。 */
export interface AstStats {
  nodes: number;
  functions: number;
  variables: number;
  calls: number;
  maxDepth: number;
}

export function statsAst(node: AstTreeNode): AstStats {
  let nodes = 0, functions = 0, variables = 0, calls = 0, maxDepth = 0;
  function walk(n: AstTreeNode, d: number): void {
    nodes++;
    maxDepth = Math.max(maxDepth, d);
    if (n.type.includes('Function')) functions++;
    if (n.type === 'VariableDeclarator') variables++;
    if (n.type === 'CallExpression' || n.type === 'NewExpression') calls++;
    for (const c of n.children) walk(c, d + 1);
  }
  walk(node, 0);
  return { nodes, functions, variables, calls, maxDepth };
}
