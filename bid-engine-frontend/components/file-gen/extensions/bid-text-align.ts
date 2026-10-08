/* Hallmark · component: toolbar-align-group · genre: modern-minimal · theme: utilitarian
 * 段落/标题对齐：把 textAlign 作为块级全局属性挂到 paragraph / heading 上。
 * 默认左对齐不写入 attrs（导出的 doc_json 对未设置过的段落保持干净）。
 * states: default · hover · focus · active · disabled
 */
/* eslint-disable no-unused-vars */
import { Extension } from "@tiptap/core";

export type BidTextAlignValue = "left" | "center" | "right" | "justify";

export const BID_TEXT_ALIGNMENTS: BidTextAlignValue[] = [
  "left",
  "center",
  "right",
  "justify",
];

/** 归一化粘贴/模板导入的 CSS 值：start→left、end→right，其余非法值回退左对齐 */
function normalizeAlignment(
  value: string | null | undefined,
): BidTextAlignValue {
  const key = String(value ?? "")
    .trim()
    .toLowerCase();
  if (key === "start") return "left";
  if (key === "end") return "right";
  return BID_TEXT_ALIGNMENTS.includes(key as BidTextAlignValue)
    ? (key as BidTextAlignValue)
    : "left";
}

export type BidTextAlignOptions = {
  /** 参与对齐的块级节点类型 */
  types: string[];
  /** 允许的对齐取值 */
  alignments: BidTextAlignValue[];
  /** 默认对齐：与该值一致时不写入 attrs，也不渲染内联样式 */
  defaultAlignment: BidTextAlignValue;
};

declare module "@tiptap/core" {
  interface Commands<ReturnType> {
    bidTextAlign: {
      /** 设置当前块的文本对齐 */
      setTextAlign: (alignment: BidTextAlignValue) => ReturnType;
      /** 恢复默认对齐（左对齐） */
      unsetTextAlign: () => ReturnType;
    };
  }
}

export const BidTextAlign = Extension.create<BidTextAlignOptions>({
  name: "bidTextAlign",

  addOptions() {
    return {
      types: ["paragraph", "heading"],
      alignments: BID_TEXT_ALIGNMENTS,
      defaultAlignment: "left",
    };
  },

  addGlobalAttributes() {
    return [
      {
        types: this.options.types,
        attributes: {
          textAlign: {
            default: this.options.defaultAlignment,
            // 模板 HTML 里带 style="text-align:center" 的段落导入后保持一致
            parseHTML: (element) => normalizeAlignment(element.style.textAlign),
            renderHTML: (attributes) => {
              const value = attributes.textAlign as BidTextAlignValue;
              if (!value || value === this.options.defaultAlignment) return {};
              return { style: `text-align: ${value}` };
            },
          },
        },
      },
    ];
  },

  addCommands() {
    return {
      setTextAlign:
        (alignment) =>
        ({ commands }) => {
          if (!this.options.alignments.includes(alignment)) return false;
          // 回到默认左对齐时清掉属性，避免 doc_json 里留冗余的 textAlign:left
          if (alignment === this.options.defaultAlignment) {
            return this.options.types
              .map((type) => commands.resetAttributes(type, "textAlign"))
              .some((applied) => applied);
          }
          // 选区可能跨段落/标题，逐类型下发，命中任意一种即视为成功
          return this.options.types
            .map((type) =>
              commands.updateAttributes(type, { textAlign: alignment }),
            )
            .some((applied) => applied);
        },
      unsetTextAlign:
        () =>
        ({ commands }) =>
          this.options.types
            .map((type) => commands.resetAttributes(type, "textAlign"))
            .some((applied) => applied),
    };
  },
});

export default BidTextAlign;
