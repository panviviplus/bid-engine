/**
 * 左侧导航的选中态匹配规则。
 *
 * 约定：子路径归属最近的父级菜单，例如
 *   - `/intel/42`                → 选中“招标情报站 → 情报大厅”(`/intel`)
 *   - `/system/intel/notices/42` → 选中“系统管理 → 招标情报管理”(`/system/intel`)
 * 因此**详情页放在哪个路由下，就决定了左导航高亮到哪个菜单**：跨模块的详情页
 * 必须挂在自己模块的路径下，否则会把导航带到别的模块（历史缺陷：情报管理的
 * 公告详情挂在 `/intel/[id]`，左导航被带到情报大厅）。
 *
 * 抽取为独立 .mjs 是为了可被 node --test 直接断言，不引入 React/JSX。
 */
export function findBestMatchHref(nodes, asPath) {
  let best = "";
  const walk = (node) => {
    if (node?.href) {
      if (asPath === node.href || asPath?.startsWith(`${node.href}/`)) {
        if (node.href.length > best.length) best = node.href;
      }
    }
    if (node?.children?.length) {
      node.children.forEach(walk);
    }
  };
  (nodes || []).forEach(walk);
  return best;
}

export function nodeIsActive(node, asPath, bestHref) {
  if (!node) return false;
  const selfActive = node.href ? bestHref === node.href : false;
  if (!node.children?.length) return selfActive;
  return (
    selfActive || node.children.some((n) => nodeIsActive(n, asPath, bestHref))
  );
}
