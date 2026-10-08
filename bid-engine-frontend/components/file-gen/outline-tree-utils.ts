// 大纲树结构工具：拖拽排序/跨级移动的纯函数实现，供蓝图确认页与编辑器大纲导航共用。
import type { BidGenOutlineNode } from "./types";

// targetId 是否位于 dragId 的子树内（含 dragId 自身）
function isInSubtree(
  nodes: BidGenOutlineNode[],
  dragId: number,
  targetId: number,
): boolean {
  const childrenOf = (pid: number) => nodes.filter((n) => n.parentId === pid);
  const stack = [dragId];
  while (stack.length) {
    const cur = stack.pop()!;
    if (cur === targetId) return true;
    for (const c of childrenOf(cur)) stack.push(c.id);
  }
  return false;
}

/**
 * 移动 dragId 到 targetParentId 下、nextId 之前（nextId=0 表示追加到末尾）。
 * 返回重算层级（1-4 夹取）与同级 sortOrder（ordinal * 100）后的全量扁平大纲。
 * 非法移动（移入自身子树、nextId 位于自身子树内）返回 null。
 */
export function reorderOutline(
  nodes: BidGenOutlineNode[],
  dragId: number,
  targetParentId: number,
  nextId: number,
): { nodes: BidGenOutlineNode[]; dragLevel: number } | null {
  const drag = nodes.find((n) => n.id === dragId);
  if (!drag) return null;
  if (targetParentId !== 0 && isInSubtree(nodes, dragId, targetParentId))
    return null;
  if (nextId === dragId || (nextId !== 0 && isInSubtree(nodes, dragId, nextId)))
    return null;

  // 1) 子节点映射（同级按 sortOrder 排序）
  const childrenMap = new Map<number, BidGenOutlineNode[]>();
  for (const n of nodes) {
    const arr = childrenMap.get(n.parentId) || [];
    arr.push(n);
    childrenMap.set(n.parentId, arr);
  }
  for (const arr of Array.from(childrenMap.values()))
    arr.sort((a, b) => a.sortOrder - b.sortOrder);

  // drag 子树 id 集合
  const dragSubtree = new Set<number>();
  const collect = (id: number) => {
    dragSubtree.add(id);
    for (const c of childrenMap.get(id) || []) collect(c.id);
  };
  collect(dragId);

  // 2) 从原父级移除 drag 子树
  const oldSiblings = (childrenMap.get(drag.parentId) || []).filter(
    (n) => !dragSubtree.has(n.id),
  );
  childrenMap.set(drag.parentId, oldSiblings);

  // 3) 插入目标父级 nextId 之前（或末尾）
  const targetSiblings = childrenMap.get(targetParentId) || [];
  const idx =
    nextId === 0 ? -1 : targetSiblings.findIndex((n) => n.id === nextId);
  const insertAt = idx < 0 ? targetSiblings.length : idx;
  targetSiblings.splice(insertAt, 0, drag);
  childrenMap.set(targetParentId, targetSiblings);

  // 4) 重算 level（parent+1，夹取 1-4）与同级 sortOrder（ordinal*100）
  const levelOf = new Map<number, number>();
  const result: BidGenOutlineNode[] = [];
  const emit = (pid: number, parentLevel: number) => {
    const siblings = childrenMap.get(pid) || [];
    siblings.forEach((n, i) => {
      const level = Math.min(4, Math.max(1, parentLevel + 1));
      levelOf.set(n.id, level);
      result.push({ ...n, level, sortOrder: (i + 1) * 100 });
    });
    for (const n of siblings) emit(n.id, levelOf.get(n.id)!);
  };
  emit(0, 0);
  return { nodes: result, dragLevel: levelOf.get(dragId) ?? drag.level };
}

// 目录搜索：按标题模糊筛选，结果保留命中节点及其全部祖先（用于渲染层级关系）。
// 返回 visibleIds（应展示的节点）与 matchedIds（命中关键词的节点，供高亮）。
export function filterOutlineTree(
  nodes: BidGenOutlineNode[],
  query: string,
): { visibleIds: Set<number>; matchedIds: Set<number> } {
  const visibleIds = new Set<number>();
  const matchedIds = new Set<number>();
  const q = query.trim().toLowerCase();
  if (!q) return { visibleIds, matchedIds };
  const byId = new Map<number, BidGenOutlineNode>();
  nodes.forEach((n) => byId.set(n.id, n));
  for (const n of nodes) {
    if (!n.title.toLowerCase().includes(q)) continue;
    matchedIds.add(n.id);
    let cur: BidGenOutlineNode | undefined = n;
    while (cur) {
      visibleIds.add(cur.id);
      cur = cur.parentId ? byId.get(cur.parentId) : undefined;
    }
  }
  return { visibleIds, matchedIds };
}
