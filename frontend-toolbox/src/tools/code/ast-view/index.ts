import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, button, checkbox, el } from '../../../ui/components.ts';
import { parseCode, buildAstTree, statsAst, type AstTreeNode } from '../../../lib/ast-utils.ts';
import { createTreeView, type TreeNodeData } from '../../../ui/tree-view.ts';

/** AstTreeNode → TreeNodeData 递归转换。 */
function toTreeData(node: AstTreeNode): TreeNodeData {
  return {
    label: node.label,
    typeTag: node.type,
    onClick: () => {},
    data: node,
    children: node.children.map(toTreeData),
    expanded: false,
  };
}

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let inp: HTMLTextAreaElement;
    let treeHost: HTMLElement;
    let detailBox: HTMLElement;
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        inp = textarea('输入 JS/TS 代码 …', 8);
        inp.value = 'function add(a, b) {\n  return a + b;\n}\nconst x = add(1, 2);\nconsole.log(x);';

        const { wrapper: tsWrap, input: tsCb } = checkbox('TypeScript', false);
        treeHost = el('div', 'ftb-ast-tree');
        detailBox = el('div');
        const statsBox = el('div', 'ftb-stat-grid');

        const parse = async (): Promise<void> => {
          const code = inp.value;
          if (!code.trim()) {
            treeHost.replaceChildren(el('div', 'ftb-desc', '（输入代码后解析）'));
            statsBox.replaceChildren();
            return;
          }
          try {
            const ast = await parseCode(code, tsCb.checked);
            const tree = buildAstTree(ast as unknown as Parameters<typeof buildAstTree>[0]);
            const data = toTreeData(tree);
            data.expanded = true;
            // 默认展开第二层
            for (const c of data.children ?? []) c.expanded = true;
            const tv = createTreeView(data);
            treeHost.replaceChildren(tv.container);

            const s = statsAst(tree);
            statsBox.replaceChildren();
            for (const [k, v] of Object.entries(s) as Array<[string, number]>) {
              const labels: Record<string, string> = {
                nodes: '节点数', functions: '函数', variables: '变量', calls: '调用', maxDepth: '最大深度',
              };
              const card = el('div', 'ftb-stat');
              card.append(el('div', 'ftb-stat-value', String(v)), el('div', 'ftb-stat-label', labels[k] ?? k));
              statsBox.append(card);
            }

            detailBox.replaceChildren();
            detailBox.append(el('div', 'ftb-desc', '点击树节点查看详情'));
          } catch (e) {
            treeHost.replaceChildren(el('div', 'ftb-error', '⚠ 解析失败：' + (e as Error).message));
          }
        };

        const btn = button('🔍 解析', () => void parse());
        inp.addEventListener('input', () => { /* 手动解析 */ });

        const row = el('div', 'ftb-io-row');
        const left = el('div');
        left.append(el('div', 'ftb-desc', '代码：'), inp);
        const right = el('div');
        right.append(el('div', 'ftb-desc', 'AST 树：'), treeHost);
        row.append(left, right);

        layout.inputArea.append(btn, tsWrap, row);
        layout.outputArea.append(statsBox, detailBox);
        ctx.container.append(layout.container);
        void parse();
      },
    } satisfies ToolInstance;
  },
};
export default tool;
