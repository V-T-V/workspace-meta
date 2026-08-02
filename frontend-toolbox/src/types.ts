// =============================================================================
// 前端工具箱 · 核心类型契约
// 所有工具模块、注册表、路由、外壳都依赖本文件。
// =============================================================================

/** 工具的静态元数据。每个工具在 meta.ts 里声明一份，打进首包用于导航/搜索。 */
export interface ToolMeta {
  /** 全局唯一 id，也是路由段，如 'json-format'。 */
  id: string;
  /** 所属组 id（见 taxonomy.ts），如 'format'。 */
  groupId: string;
  /** 显示名，如「JSON 格式化」。 */
  title: string;
  /** 一句话描述，用于卡片/搜索结果摘要。 */
  summary: string;
  /** 搜索关键词（可选，补充标题之外的匹配词）。 */
  keywords?: string[];
  /** emoji 图标（可选）。 */
  icon?: string;
}

/** 工具挂载时收到的上下文。 */
export interface ToolContext {
  /** 工具独占的挂载容器（已清空）。 */
  container: HTMLElement;
}

/** 工具实例：负责渲染自身 UI 并在卸载时清理。 */
export interface ToolInstance {
  /** 挂载并渲染到容器。 */
  mount(ctx: ToolContext): void | Promise<void>;
  /** 卸载清理（解绑事件、停止定时器、释放 canvas 等）。 */
  destroy?(): void;
}

/** 工具模块的标准导出形状。 */
export interface Tool {
  /** 元数据（与 meta.ts 导出的一致）。 */
  meta: ToolMeta;
  /** 创建工具实例。 */
  create(): ToolInstance;
}

/** 工具分组定义（taxonomy.ts 的条目形状）。 */
export interface ToolGroup {
  /** 组 id，如 'format'。 */
  id: string;
  /** 组名，如「格式化」。 */
  name: string;
  /** emoji 图标。 */
  icon: string;
  /** 一句话描述。 */
  blurb: string;
  /** CSS 变量名，决定该组的强调色。 */
  theme: string;
}
