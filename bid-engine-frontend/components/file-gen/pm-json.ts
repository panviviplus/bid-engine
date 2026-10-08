// markdown-lite → ProseMirror JSON 节点数组。
// 与后端 pkg/handler/bidgen/pmjson.go 的 textToPMJSON 保持逻辑一致：
// 支持空行分段、#/##/###/#### 标题（标题行与正文同块时拆分）、- 无序列表、
// 1. 有序列表、| 管道表格。
// 供重写选中片段等前端本地渲染使用；生成流仍由后端解析后下发 JSON。

export type PmNode = {
  type: string;
  attrs?: Record<string, unknown>;
  content?: PmNode[];
  text?: string;
};

const regexpNumberedList = /^\d+\.\s+/;

export function textNode(text: string): PmNode {
  return { type: "text", text };
}

/**
 * 递归清理会让 ProseMirror 解析失败的节点。
 *
 * ProseMirror 的 TextNode.fromJSON 要求 text 必须是非空字符串，否则抛
 * RangeError("Invalid text node in JSON")；TipTap 在不开启 errorOnInvalidContent 时
 * 会静默丢弃整篇文档，表现为编辑器空白。空表格单元格是这类空文本节点最常见的来源
 * （生成侧与历史存量文档都可能存在），因此加载与本地转换都要过一遍清理。
 */
function sanitizeNodesInternal(
  nodes: unknown,
  report: { repaired: boolean },
): PmNode[] {
  if (!Array.isArray(nodes)) return [];
  const result: PmNode[] = [];
  for (const raw of nodes) {
    if (!raw || typeof raw !== "object") {
      report.repaired = true;
      continue;
    }
    const node = raw as PmNode;
    if (node.type === "text") {
      if (typeof node.text !== "string" || node.text.trim() === "") {
        report.repaired = true;
        continue;
      }
      result.push(node);
      continue;
    }
    const next: PmNode = { ...node };
    if (Array.isArray(node.content)) {
      next.content = sanitizeNodesInternal(node.content, report);
    }
    switch (node.type) {
      case "tableCell":
      case "tableHeader":
        // 单元格必须至少包含一个块级节点
        if (!next.content || next.content.length === 0) {
          next.content = [{ type: "paragraph" }];
          report.repaired = true;
        }
        break;
      case "tableRow":
      case "table":
      case "listItem":
      case "bulletList":
      case "orderedList":
      case "blockquote":
        // 被掏空的容器没有意义，直接丢弃
        if (!next.content || next.content.length === 0) {
          report.repaired = true;
          continue;
        }
        break;
      default:
        break;
    }
    result.push(next);
  }
  return result;
}

export function sanitizePMNodes(nodes: unknown): PmNode[] {
  return sanitizeNodesInternal(nodes, { repaired: false });
}

/** 清理整篇文档 JSON，并报告是否发生了修复（用于把历史脏数据写回服务端）。 */
export function sanitizePMDocWithReport(doc: unknown): {
  doc: PmNode | null;
  repaired: boolean;
} {
  if (!doc || typeof doc !== "object") {
    return { doc: null, repaired: false };
  }
  const report = { repaired: false };
  const source = doc as PmNode;
  const content = sanitizeNodesInternal(source.content, report);
  return {
    doc: {
      ...source,
      type: source.type || "doc",
      content: content.length > 0 ? content : [{ type: "paragraph" }],
    },
    repaired: report.repaired,
  };
}

/** 清理整篇文档 JSON（在交给编辑器 setContent 之前调用）。 */
export function sanitizePMDoc(doc: unknown): PmNode | null {
  return sanitizePMDocWithReport(doc).doc;
}

export function paragraphNode(children: PmNode[] = []): PmNode {
  return { type: "paragraph", content: children };
}

export function headingNode(level: number, text: string): PmNode {
  return {
    type: "heading",
    attrs: { level },
    content: [textNode(text)],
  };
}

function cleanInline(s: string): string {
  return s.replace(/\*\*/g, "").replace(/__/g, "").replace(/`/g, "").trim();
}

function isBulletList(block: string): boolean {
  const lines = block.split("\n");
  for (const line of lines) {
    const t = line.trim();
    if (t === "") continue;
    if (!t.startsWith("- ") && !t.startsWith("* ")) return false;
  }
  return true;
}

function isOrderedList(block: string): boolean {
  const lines = block.split("\n");
  let count = 0;
  for (const line of lines) {
    const t = line.trim();
    if (t === "") continue;
    if (!regexpNumberedList.test(t)) return false;
    count += 1;
  }
  return count > 0;
}

function bulletListNode(block: string): PmNode {
  const items: PmNode[] = [];
  for (const line of block.split("\n")) {
    let t = line.trim();
    if (t === "") continue;
    t = t.replace(/^- /, "").replace(/^\* /, "");
    items.push({
      type: "listItem",
      content: [paragraphNode([textNode(cleanInline(t))])],
    });
  }
  return { type: "bulletList", content: items };
}

function orderedListNode(block: string): PmNode {
  const items: PmNode[] = [];
  for (const line of block.split("\n")) {
    const t = line.trim();
    if (t === "") continue;
    const idx = t.indexOf(". ");
    if (idx < 0) continue;
    items.push({
      type: "listItem",
      content: [paragraphNode([textNode(cleanInline(t.slice(idx + 2).trim()))])],
    });
  }
  return { type: "orderedList", content: items };
}

function parsePipeTable(block: string): string[][] {
  const rows: string[][] = [];
  for (const line of block.split("\n")) {
    let t = line.trim();
    if (t === "" || t.includes("---")) continue;
    t = t.replace(/^\|/, "").replace(/\|$/, "");
    rows.push(t.split("|").map((c) => c.trim()));
  }
  return rows;
}

function tableNode(rows: string[][]): PmNode {
  const tableContent: PmNode[] = [];
  rows.forEach((row, i) => {
    const cells: PmNode[] = [];
    row.forEach((cell) => {
      const cType = i === 0 ? "tableHeader" : "tableCell";
      cells.push({ type: cType, content: [paragraphNode([textNode(cell)])] });
    });
    tableContent.push({ type: "tableRow", content: cells });
  });
  return { type: "table", attrs: { resizable: true }, content: tableContent };
}

function headingMarker(block: string): { marker: string; level: number } {
  if (block.startsWith("#### ")) return { marker: "#### ", level: 4 };
  if (block.startsWith("### ")) return { marker: "### ", level: 3 };
  if (block.startsWith("## ")) return { marker: "## ", level: 2 };
  return { marker: "# ", level: 2 };
}

// 将 LLM 输出的 markdown-lite 文本转为 ProseMirror 节点数组
export function textToPMJSON(text: string): PmNode[] {
  const trimmed = text.trim();
  if (!trimmed) return [];
  const nodes: PmNode[] = [];
  for (const rawBlock of trimmed.split(/\n\n/)) {
    const block = rawBlock.trim();
    if (!block) continue;
    if (block.startsWith("|")) {
      const table = parsePipeTable(block);
      if (table.length > 0) nodes.push(tableNode(table));
      continue;
    }
    if (isBulletList(block)) {
      nodes.push(bulletListNode(block));
      continue;
    }
    if (isOrderedList(block)) {
      nodes.push(orderedListNode(block));
      continue;
    }
    if (
      block.startsWith("#### ") ||
      block.startsWith("### ") ||
      block.startsWith("## ") ||
      block.startsWith("# ")
    ) {
      // 标题行与正文同块（LLM 常在标题后不空行）：只取首行作标题，
      // 剩余行作为正文递归解析，避免整块正文被吞进标题文本（与 Go 侧一致）
      const { marker, level } = headingMarker(block);
      const nl = block.indexOf("\n");
      const title = nl >= 0 ? block.slice(0, nl) : block;
      nodes.push(headingNode(level, title.slice(marker.length).trim()));
      if (nl >= 0) {
        const rest = block.slice(nl + 1).trim();
        if (rest) nodes.push(...textToPMJSON(rest));
      }
      continue;
    }
    // 单行内可能有换行，合并为一段
    const flat = block.replace(/\n/g, " ");
    nodes.push(paragraphNode([textNode(cleanInline(flat))]));
  }
  if (nodes.length === 0) return [paragraphNode()];
  return sanitizePMNodes(nodes);
}

// 行内改写结果：去除 markdown 标记并按空格合并为单行纯文本（用于行内选区替换）
export function rewriteTextToInline(text: string): string {
  return text
    .split("\n")
    .map((line) => line.trim().replace(/^#{1,6}\s+/, ""))
    .filter(Boolean)
    .join(" ");
}
