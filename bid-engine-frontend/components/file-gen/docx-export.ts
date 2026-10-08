// 标书导出装配：封面节 + 目录节 + 正文节，页眉页脚与页码在导出层完成，
// 正文模型只存正文（见 design.md：导出时装配，编辑器不做 Word 排版）。
import {
  AlignmentType,
  Footer,
  Header,
  LineRuleType,
  NumberFormat,
  PageNumber,
  Paragraph,
  SectionType,
  TableOfContents,
  TextRun,
} from "docx";
import { isDocumentRootOutline, type BidGenOutlineNode } from "./types";

const FONT_SONG = "宋体";
const FONT_HEI = "黑体";

export type BidDocxCover = {
  docTitle?: string;
  projectName?: string;
  projectNumber?: string;
  lotLabel?: string;
  tendererName?: string;
  bidderName?: string;
  date?: string;
};

export type BidDocxTocEntry = {
  /** 对应大纲节点 ID，用于回填两遍导出的真实页码 */
  outlineId: number;
  title: string;
  level: number;
  page?: number;
};

// ===== 样式：正文宋体小四、行距 1.5、首行缩进 2 字符；标题黑体体系 =====
export const BID_DOCX_STYLES = {
  styles: {
    default: {
      document: {
        run: {
          font: { ascii: FONT_SONG, eastAsia: FONT_SONG, hAnsi: FONT_SONG },
          size: 24,
          color: "000000",
        },
        paragraph: {
          spacing: { line: 360, lineRule: LineRuleType.AUTO },
          alignment: AlignmentType.JUSTIFIED,
          indent: { firstLine: 480 },
        },
      },
      heading1: {
        run: {
          font: { ascii: FONT_HEI, eastAsia: FONT_HEI, hAnsi: FONT_HEI },
          size: 32,
          bold: true,
          color: "000000",
        },
        paragraph: {
          alignment: AlignmentType.CENTER,
          indent: { firstLine: 0 },
          spacing: { before: 360, after: 240, line: 360, lineRule: LineRuleType.AUTO },
        },
      },
      heading2: {
        run: {
          font: { ascii: FONT_HEI, eastAsia: FONT_HEI, hAnsi: FONT_HEI },
          size: 28,
          bold: true,
          color: "000000",
        },
        paragraph: {
          alignment: AlignmentType.LEFT,
          indent: { firstLine: 0 },
          spacing: { before: 300, after: 180, line: 360, lineRule: LineRuleType.AUTO },
        },
      },
      heading3: {
        run: {
          font: { ascii: FONT_HEI, eastAsia: FONT_HEI, hAnsi: FONT_HEI },
          size: 24,
          bold: true,
          color: "000000",
        },
        paragraph: {
          alignment: AlignmentType.LEFT,
          indent: { firstLine: 0 },
          spacing: { before: 240, after: 120, line: 360, lineRule: LineRuleType.AUTO },
        },
      },
      heading4: {
        run: {
          font: { ascii: FONT_SONG, eastAsia: FONT_SONG, hAnsi: FONT_SONG },
          size: 24,
          bold: true,
          color: "000000",
        },
        paragraph: {
          alignment: AlignmentType.LEFT,
          indent: { firstLine: 0 },
          spacing: { before: 200, after: 120, line: 360, lineRule: LineRuleType.AUTO },
        },
      },
      // 正文内小标题：不进入目录（目录只取 Heading 1-4）
      heading5: {
        run: {
          font: { ascii: FONT_SONG, eastAsia: FONT_SONG, hAnsi: FONT_SONG },
          size: 24,
          bold: true,
          color: "000000",
        },
        paragraph: {
          alignment: AlignmentType.LEFT,
          indent: { firstLine: 0 },
          spacing: { before: 200, after: 120, line: 360, lineRule: LineRuleType.AUTO },
        },
      },
      heading6: {
        run: {
          font: { ascii: FONT_SONG, eastAsia: FONT_SONG, hAnsi: FONT_SONG },
          size: 24,
          bold: true,
          color: "000000",
        },
        paragraph: {
          alignment: AlignmentType.LEFT,
          indent: { firstLine: 0 },
          spacing: { before: 200, after: 120, line: 360, lineRule: LineRuleType.AUTO },
        },
      },
    },
  },
  paragraphStyles: [
    {
      id: "BidCaption",
      name: "BidCaption",
      basedOn: "Normal",
      next: "Normal",
      run: {
        font: { ascii: FONT_SONG, eastAsia: FONT_SONG, hAnsi: FONT_SONG },
        size: 21,
        color: "000000",
      },
      paragraph: {
        alignment: AlignmentType.CENTER,
        indent: { firstLine: 0 },
        spacing: { before: 60, after: 160 },
      },
    },
  ],
  features: { updateFields: true },
};

// 图注/表注识别：服务端统一按 图 X-Y / 表 X-Y 编号，导出时据此套用样式。
const CAPTION_PATTERN = /^[图表]\s*\d+\s*[-–—]\s*\d+/;

// 编辑器块对齐 → DOCX 段落对齐（未设置过对齐的块返回 null，沿用模板样式）
const DOCX_ALIGNMENTS: Record<
  string,
  (typeof AlignmentType)[keyof typeof AlignmentType]
> = {
  left: AlignmentType.LEFT,
  center: AlignmentType.CENTER,
  right: AlignmentType.RIGHT,
  justify: AlignmentType.JUSTIFIED,
};

function docxAlignment(
  value: unknown,
): (typeof AlignmentType)[keyof typeof AlignmentType] | null {
  const key = String(value ?? "").trim().toLowerCase();
  if (!key) return null;
  return DOCX_ALIGNMENTS[key] ?? null;
}

// 图片在 DOCX 中的列宽百分比 = 编辑器里的像素宽度 / 正文内容区宽度。
// 未缩放过的图片沿用 prosemirror-docx 的 70% 默认宽度。
function imageWidthPercent(width: unknown, contentWidthPx?: number): number {
  const px = Number(width);
  if (!Number.isFinite(px) || px <= 0) return 70;
  const base =
    Number.isFinite(contentWidthPx) && Number(contentWidthPx) > 0
      ? Number(contentWidthPx)
      : 600;
  return Math.min(100, Math.max(10, Math.round((px / base) * 100)));
}

// ===== 大纲 =====

/** 按父子关系与同级 sortOrder 还原大纲树序（父先于子）。 */
export function orderOutlineTree(
  nodes: BidGenOutlineNode[],
): BidGenOutlineNode[] {
  const children = new Map<number, BidGenOutlineNode[]>();
  for (const node of nodes) {
    const list = children.get(node.parentId) ?? [];
    list.push(node);
    children.set(node.parentId, list);
  }
  for (const list of Array.from(children.values())) {
    list.sort((a, b) => a.sortOrder - b.sortOrder || a.id - b.id);
  }
  const ordered: BidGenOutlineNode[] = [];
  const walk = (parentId: number) => {
    for (const child of children.get(parentId) ?? []) {
      ordered.push(child);
      walk(child.id);
    }
  };
  walk(0);
  // 兜底：树序漏掉的孤立节点（父节点缺失）按原顺序补到最后
  for (const node of nodes) {
    if (!ordered.includes(node)) ordered.push(node);
  }
  return ordered;
}

/** 目录条目：只收录大纲节点（1-4 级），正文内小标题不进目录。 */
export function buildTocEntries(
  ordered: BidGenOutlineNode[],
  pageMap?: Map<number, number>,
): BidDocxTocEntry[] {
  const entries: BidDocxTocEntry[] = [];
  for (const node of ordered) {
    if (isDocumentRootOutline(node)) continue;
    if (node.level < 1 || node.level > 4) continue;
    const title = String(node.title || "").trim();
    if (!title) continue;
    entries.push({
      outlineId: node.id,
      title,
      level: Math.min(node.level, 4),
      page: pageMap?.get(node.id),
    });
  }
  return entries;
}

/**
 * Word 层级映射：大纲标题 1-4 级 → Heading 1-4（进目录）；
 * 正文内小标题（无 outlineId）→ Heading 5/6，仅供视觉分层，不污染目录。
 */
export function remapInlineHeadings(doc: any): any {
  const walk = (node: any): any => {
    if (node.isText) return node;
    if (node.type?.name === "heading") {
      const hasOutline = Number(node.attrs?.outlineId || 0) > 0;
      const rawLevel = Number(node.attrs?.level || 1);
      const level = hasOutline
        ? Math.min(Math.max(rawLevel, 1), 4)
        : rawLevel <= 2
          ? 5
          : 6;
      return node.type.create({ ...node.attrs, level }, node.content, node.marks);
    }
    if (node.isLeaf) return node;
    const children: any[] = [];
    node.forEach((child: any) => children.push(walk(child)));
    return node.type.create(node.attrs, children, node.marks);
  };
  return walk(doc);
}

// ===== 封面 / 目录 / 页眉页脚 =====

function coverParagraph(
  text: string,
  options: {
    font: string;
    size: number;
    before: number;
    after?: number;
    bold?: boolean;
    spacing?: number;
  },
): Paragraph {
  return new Paragraph({
    alignment: AlignmentType.CENTER,
    spacing: { before: options.before, after: options.after ?? 0 },
    children: [
      new TextRun({
        text,
        bold: options.bold,
        size: options.size,
        characterSpacing: options.spacing,
        font: {
          ascii: options.font,
          eastAsia: options.font,
          hAnsi: options.font,
        },
      }),
    ],
  });
}

function buildCoverChildren(cover: BidDocxCover): Paragraph[] {
  const projectName = String(cover.projectName || "").trim();
  const children: Paragraph[] = [
    coverParagraph(projectName || " ", {
      font: FONT_HEI,
      size: 32,
      bold: true,
      before: 2400,
      after: 400,
    }),
    coverParagraph(String(cover.docTitle || "投 标 文 件"), {
      font: FONT_HEI,
      size: 72,
      bold: true,
      spacing: 60,
      before: 800,
      after: 1600,
    }),
  ];

  const fields: Array<[string, string | undefined]> = [
    ["项目编号", cover.projectNumber],
    ["标　　段", cover.lotLabel],
    ["招 标 人", cover.tendererName],
    ["投 标 人", cover.bidderName],
  ];
  let first = true;
  for (const [label, value] of fields) {
    const text = String(value || "").trim();
    if (!text) continue;
    children.push(
      coverParagraph(`${label}：${text}`, {
        font: FONT_SONG,
        size: 28,
        before: first ? 900 : 200,
        after: 0,
      }),
    );
    first = false;
  }
  children.push(
    coverParagraph(String(cover.date || "").trim() || " ", {
      font: FONT_SONG,
      size: 28,
      before: 1200,
      after: 0,
    }),
  );
  return children;
}

function buildTocChildren(
  entries: BidDocxTocEntry[],
): Array<Paragraph | TableOfContents> {
  return [
    coverParagraph("目　　录", {
      font: FONT_HEI,
      size: 32,
      bold: true,
      before: 240,
      after: 320,
    }),
    new TableOfContents("目录", {
      hyperlink: true,
      headingStyleRange: "1-4",
      cachedEntries: entries.map((entry) => ({
        title: entry.title,
        level: entry.level,
        page: entry.page,
      })),
      beginDirty: true,
    }),
  ];
}

function buildBodyHeader(cover: BidDocxCover): Header {
  const parts = [cover.projectName, cover.projectNumber]
    .map((value) => String(value || "").trim())
    .filter(Boolean);
  return new Header({
    children: [
      new Paragraph({
        alignment: AlignmentType.CENTER,
        spacing: { after: 0 },
        children: [
          new TextRun({
            text: parts.join("　"),
            size: 18,
            color: "595959",
            font: { ascii: FONT_SONG, eastAsia: FONT_SONG, hAnsi: FONT_SONG },
          }),
        ],
      }),
    ],
  });
}

function buildBodyFooter(): Footer {
  return new Footer({
    children: [
      new Paragraph({
        alignment: AlignmentType.CENTER,
        spacing: { before: 0 },
        children: [
          new TextRun({
            text: "第 ",
            size: 18,
            color: "595959",
            font: { ascii: FONT_SONG, eastAsia: FONT_SONG, hAnsi: FONT_SONG },
          }),
          new TextRun({
            children: [PageNumber.CURRENT],
            size: 18,
            color: "595959",
          }),
          new TextRun({
            text: " 页　共 ",
            size: 18,
            color: "595959",
            font: { ascii: FONT_SONG, eastAsia: FONT_SONG, hAnsi: FONT_SONG },
          }),
          new TextRun({
            children: [PageNumber.TOTAL_PAGES],
            size: 18,
            color: "595959",
          }),
          new TextRun({
            text: " 页",
            size: 18,
            color: "595959",
            font: { ascii: FONT_SONG, eastAsia: FONT_SONG, hAnsi: FONT_SONG },
          }),
        ],
      }),
    ],
  });
}

// ===== 导出主入口 =====

/**
 * ProseMirror 文档 → 装配好的 DOCX Blob（封面节 + 目录节 + 正文节）。
 * 目录页码来自两遍导出的第一遍（pageMap 为空时目录条目仍完整，仅缺页码）。
 */
export async function buildBidDocxBlob(options: {
  docNode: any;
  cover: BidDocxCover;
  toc: BidDocxTocEntry[];
  /** 编辑器正文内容区宽度（px）：用于把图片的像素宽度换算成 DOCX 列宽百分比 */
  contentWidthPx?: number;
}): Promise<Blob> {
  const {
    DocxSerializerAsync,
    defaultAsyncNodes,
    defaultMarks,
    writeDocx,
  } = await import("prosemirror-docx");

  const marks = {
    ...defaultMarks,
    textStyle(state: any, node: any, mark: any) {
      const options: Record<string, unknown> = {};
      const color = toHexColor(mark.attrs?.color);
      const backgroundColor = toHexColor(mark.attrs?.backgroundColor);
      if (color) options.color = color;
      if (backgroundColor) {
        options.shading = { fill: backgroundColor, color: "auto", type: "clear" };
      }
      return options;
    },
  };

  // 段落序列化包装：图注/表注套用 BidCaption 样式（五号居中）；对齐取自编辑器块属性
  const baseParagraph = defaultAsyncNodes.paragraph;
  const baseHeading = defaultAsyncNodes.heading;
  const paragraphs: Record<string, any> = {
    ...defaultAsyncNodes,
    paragraph(state: any, node: any, parent: any, index: number) {
      const text = String(node.textContent || "").trim();
      if (CAPTION_PATTERN.test(text)) {
        state.addParagraphOptions({ style: "BidCaption" });
      }
      const alignment = docxAlignment(node.attrs?.textAlign);
      if (alignment) state.addParagraphOptions({ alignment });
      return baseParagraph(state, node, parent, index);
    },
    heading(state: any, node: any, parent: any, index: number) {
      const alignment = docxAlignment(node.attrs?.textAlign);
      if (alignment) state.addParagraphOptions({ alignment });
      return baseHeading(state, node, parent, index);
    },
    // 图片：按编辑器中缩放后的宽度等比落版（未缩放过的沿用 70% 模板宽度）
    async image(state: any, node: any) {
      const src = String(node.attrs?.src || "");
      if (!src) return;
      const percent = imageWidthPercent(
        node.attrs?.width,
        options.contentWidthPx,
      );
      await state.image(src, percent, "center");
      state.closeBlock(node);
    },
  };

  const serializer = new DocxSerializerAsync(paragraphs, marks);
  const cover = options.cover || {};
  const document = await serializer.serializeAsync(
    remapInlineHeadings(options.docNode),
    {
      getImageBuffer: async (src: string) => {
        try {
          const res = await fetch(src);
          const buffer = await res.arrayBuffer();
          return new Uint8Array(buffer);
        } catch {
          return new Uint8Array();
        }
      },
      sections: [{}],
    },
    (state: any) => ({
      ...BID_DOCX_STYLES,
      sections: [
        {
          properties: {
            type: SectionType.NEXT_PAGE,
            page: {
              margin: { top: 1440, bottom: 1440, left: 1800, right: 1800 },
            },
          },
          children: buildCoverChildren(cover),
        },
        {
          properties: {
            type: SectionType.NEXT_PAGE,
            page: {
              margin: { top: 1440, bottom: 1440, left: 1800, right: 1800 },
              pageNumbers: { start: 1, formatType: NumberFormat.LOWER_ROMAN },
            },
          },
          children: buildTocChildren(options.toc || []),
        },
        {
          properties: {
            type: SectionType.NEXT_PAGE,
            page: {
              margin: { top: 1440, bottom: 1440, left: 1800, right: 1800 },
              pageNumbers: { start: 1, formatType: NumberFormat.DECIMAL },
            },
          },
          headers: { default: buildBodyHeader(cover) },
          footers: { default: buildBodyFooter() },
          children: state.sections[0]?.children ?? [],
        },
      ],
    }),
  );

  const buffer = await writeDocx(document);
  const bytes = new Uint8Array(buffer);
  return new Blob([bytes], {
    type: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  });
}

// 归一化 CSS 颜色为 6 位十六进制（去掉 #）。无法解析时返回 null（回退默认色）。
function toHexColor(value: unknown): string | null {
  const raw = String(value ?? "").trim();
  if (!raw) return null;
  if (raw.startsWith("#")) {
    let hex = raw.slice(1);
    if (hex.length === 3) {
      hex = hex
        .split("")
        .map((char) => char + char)
        .join("");
    }
    if (/^[0-9a-fA-F]{6}$/.test(hex)) return hex.toUpperCase();
    return null;
  }
  try {
    const ctx = document.createElement("canvas").getContext("2d");
    if (!ctx) return null;
    ctx.fillStyle = raw;
    const hex = ctx.fillStyle;
    if (/^#[0-9a-fA-F]{6}$/.test(hex)) return hex.slice(1).toUpperCase();
  } catch {
    // 忽略无法解析的颜色
  }
  return null;
}
