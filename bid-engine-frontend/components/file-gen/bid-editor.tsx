"use client";

/* eslint-disable no-unused-vars */
/* eslint-disable prettier/prettier */
/* eslint-disable no-use-before-define */
/* eslint-disable no-plusplus */

import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
  type ReactElement,
} from "react";
import {
  Box,
  Button,
  Flex,
  IconButton,
  Input,
  NumberInput,
  NumberInputField,
  NumberInputStepper,
  NumberIncrementStepper,
  NumberDecrementStepper,
  Popover,
  PopoverArrow,
  PopoverBody,
  PopoverContent,
  PopoverTrigger,
  Select,
  Spinner,
  Text,
  Tooltip,
} from "@chakra-ui/react";
import { useEditor, EditorContent } from "@tiptap/react";
import { Fragment } from "@tiptap/pm/model";
import StarterKit from "@tiptap/starter-kit";
import Heading from "@tiptap/extension-heading";
import Image from "@tiptap/extension-image";
import { Table } from "@tiptap/extension-table";
import TableRow from "@tiptap/extension-table-row";
import TableCell from "@tiptap/extension-table-cell";
import TableHeader from "@tiptap/extension-table-header";
import {
  BackgroundColor,
  Color,
  TextStyle,
} from "@tiptap/extension-text-style";
import {
  FiBold,
  FiCheck,
  FiCode,
  FiAlignCenter,
  FiAlignJustify,
  FiAlignLeft,
  FiAlignRight,
  FiImage,
  FiItalic,
  FiRotateCcw,
  FiRotateCw,
  FiStopCircle,
  FiType,
  FiUnderline,
} from "react-icons/fi";
import {
  LuCodeXml,
  LuEraser,
  LuGrid3X3,
  LuHighlighter,
  LuList,
  LuListOrdered,
  LuPalette,
  LuSparkles,
  LuStrikethrough,
} from "react-icons/lu";
import {
  textToPMJSON,
  rewriteTextToInline,
  sanitizePMDoc,
} from "./pm-json";
import { BidTextAlign, type BidTextAlignValue } from "./extensions/bid-text-align";

export type BidEditorHandle = {
  replaceChapter: (outlineId: number, contentNodes: any[]) => boolean;
  syncOutlineIds: (
    outline: { id: number; level: number; title: string }[],
  ) => void;
  findHeadingAndScroll: (outlineId: number) => void;
  getJSON: () => any;
  getHTML: () => string;
  setEditable: (v: boolean) => void;
  insertImages: (urls: string[]) => void;
  getEditor: () => any;
  insertHeading: (
    outlineId: number,
    level: number,
    title: string,
    opts?: { afterOutlineId?: number },
  ) => void;
  removeChapter: (outlineId: number) => void;
  renameHeading: (outlineId: number, title: string) => void;
  moveChapter: (
    outlineId: number,
    opts: { parentId: number; nextId?: number; newLevel: number },
  ) => void;
  applyOutlineSync: (
    mapping: Record<number, number>,
    sentHeadings?: HeadingSnap[],
  ) => void;
  resetOutlineBaseline: () => void;
  getHeadingSnapshot: () => HeadingSnap[] | null;
};

export type RewriteStreamHandlers = {
  onDelta?: (data: { text: string }) => void;
  onDone?: (data: { text: string }) => void;
  onError?: (data: { msg: string }) => void;
  onCancelled?: () => void;
};

export type RewriteStreamFn = (
  args: { text: string; context?: string; instruction?: string },
  handlers: RewriteStreamHandlers,
  signal?: AbortSignal,
) => Promise<void>;

type Props = {
  contentJSON?: any;
  contentHTML?: string;
  contentKey?: string;
  readOnly?: boolean;
  outline?: { id: number; level: number; title: string }[];
  onReady?: () => void;
  onDirty?: (dirty: boolean) => void;
  streaming?: boolean;
  streamTitle?: string;
  h?: string | number;
  onOutlineSync?: (
    headings: { outlineId: number | null; level: number; title: string }[],
  ) => void;
  // AI 重写选中片段：由页面注入（封装 service streamRewrite，自带 projectId）
  rewriteStream?: RewriteStreamFn;
};

type RewriteSession = {
  from: number;
  to: number; // 原选区结束位置（应用润色结果时替换 [from, to)）
  originalContent: any; // ProseMirror Fragment（放弃时恢复原文）
  originalText: string;
  context: string;
  inline: boolean; // 行内选区 → inline 文本替换；跨块选区 → 块解析替换
  accumulated: string;
  insertedSize: number;
  abort: AbortController;
};

/* Hallmark · component: toolbar-group · genre: modern-minimal · theme: utilitarian
 * states: default · hover · focus · active · disabled · loading · error · success
 * contrast: pass (46–50)
 */

/* Hallmark · component: toolbar-color-group · genre: modern-minimal · theme: utilitarian
 * 文字颜色 / 背景颜色：按钮底部色条实时编码光标处的当前颜色；预设色板 + 自定义取色 + 清除。
 * states: default · hover · focus · active · disabled
 */
const TEXT_COLOR_PRESETS = [
  "#000000",
  "#595959",
  "#C00000",
  "#ED7D31",
  "#FFC000",
  "#00B050",
  "#0070C0",
  "#1F4D78",
  "#7030A0",
  "#E26B9D",
  "#A0522D",
  "#FFFFFF",
];

const BG_COLOR_PRESETS = [
  "#FFFF00",
  "#FFE699",
  "#FFF2CC",
  "#C6E0B4",
  "#DDEBF7",
  "#E2EFDA",
  "#FCE4D6",
  "#F8CBAD",
  "#E6E0EC",
  "#FFC7CE",
  "#F2F2F2",
  "#D9D9D9",
];

/* Hallmark · component: toolbar-align-group · genre: modern-minimal · theme: utilitarian
 * 段落对齐：作用于光标所在块（段落 / 标题），默认左对齐。
 * states: default · hover · focus-visible · active · disabled
 */
const TEXT_ALIGN_ITEMS: {
  value: BidTextAlignValue;
  label: string;
  icon: ReactElement;
}[] = [
  { value: "left", label: "左对齐", icon: <FiAlignLeft size="15" /> },
  { value: "center", label: "居中对齐", icon: <FiAlignCenter size="15" /> },
  { value: "right", label: "右对齐", icon: <FiAlignRight size="15" /> },
  { value: "justify", label: "两端对齐", icon: <FiAlignJustify size="15" /> },
];

type ToolbarColorButtonProps = {
  type: "text" | "background";
  currentColor: string;
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  onPick: (color: string | null) => void;
  disabled?: boolean;
  buttonStyle: (active: boolean) => Record<string, unknown>;
};

function ToolbarColorButton({
  type,
  currentColor,
  isOpen,
  onOpenChange,
  onPick,
  disabled,
  buttonStyle,
}: ToolbarColorButtonProps) {
  const label = type === "text" ? "文字颜色" : "背景颜色";
  const Icon = type === "text" ? LuPalette : LuHighlighter;
  const presets = type === "text" ? TEXT_COLOR_PRESETS : BG_COLOR_PRESETS;
  const stripColor =
    currentColor || (type === "text" ? "#1A202C" : "transparent");
  const isWhite = stripColor.toUpperCase() === "#FFFFFF";

  return (
    <Popover
      isOpen={isOpen}
      onOpen={() => {
        if (!disabled) onOpenChange(true);
      }}
      onClose={() => onOpenChange(false)}
      placement="bottom"
      closeOnBlur
    >
      <PopoverTrigger>
        <span>
          <Tooltip label={label} openDelay={800}>
            <IconButton
              aria-label={label}
              size="sm"
              variant="ghost"
              isDisabled={disabled}
              icon={
                <Flex direction="column" align="center" gap="2px" pt="1px">
                  <Icon size="15" />
                  <Box
                    w="14px"
                    h="2.5px"
                    borderRadius="full"
                    bg={stripColor}
                    borderWidth={type === "background" || isWhite ? "1px" : 0}
                    borderColor="gray.300"
                  />
                </Flex>
              }
              {...buttonStyle(Boolean(currentColor))}
            />
          </Tooltip>
        </span>
      </PopoverTrigger>
      <PopoverContent w="230px">
        <PopoverArrow />
        <PopoverBody>
          <Flex direction="column" gap={3}>
            <Flex wrap="wrap" gap="6px">
              {presets.map((c) => {
                const selected = currentColor.toUpperCase() === c.toUpperCase();
                return (
                  <Box
                    key={c}
                    as="button"
                    type="button"
                    aria-label={`${label} ${c}`}
                    title={c}
                    w="24px"
                    h="24px"
                    borderRadius="full"
                    bg={c}
                    border="1px solid"
                    borderColor={
                      c.toUpperCase() === "#FFFFFF" ? "gray.300" : "transparent"
                    }
                    boxShadow={
                      selected
                        ? "0 0 0 2px var(--chakra-colors-primary-500)"
                        : "0 1px 2px rgba(0,0,0,0.15)"
                    }
                    _hover={{ transform: "scale(1.12)" }}
                    _active={{ transform: "scale(0.95)" }}
                    transition="transform 80ms ease"
                    onClick={() => {
                      onPick(c);
                      onOpenChange(false);
                    }}
                  />
                );
              })}
            </Flex>
            <Flex align="center" gap={2}>
              <Input
                type="color"
                aria-label={`${label}自定义`}
                value={currentColor || "#000000"}
                onChange={(e) => onPick(e.target.value)}
                w="56px"
                h="30px"
                p={0}
                border="1px solid"
                borderColor="gray.300"
                bg="white"
                cursor="pointer"
              />
              <Text fontSize="xs" color="gray.500" flex="1">
                自定义颜色
              </Text>
              <Button
                size="xs"
                variant="ghost"
                colorScheme="gray"
                onClick={() => {
                  onPick(null);
                  onOpenChange(false);
                }}
              >
                清除
              </Button>
            </Flex>
          </Flex>
        </PopoverBody>
      </PopoverContent>
    </Popover>
  );
}

// Toolbar divider
function ToolbarDivider() {
  return (
    <Box
      w="1px"
      h="20px"
      bg="gray.200"
      mx={1}
      flexShrink={0}
      alignSelf="center"
    />
  );
}

// Heading 扩展：支持自定义 outlineId 属性（章节区间替换/导航用）
const HeadingWithId = Heading.extend({
  addAttributes() {
    return {
      ...this.parent?.(),
      outlineId: {
        default: null,
        parseHTML: (element) => {
          const raw = element.getAttribute("data-outline-id");
          if (!raw) return null;
          const parsed = Number(raw);
          return Number.isFinite(parsed) && parsed > 0 ? parsed : null;
        },
        renderHTML: (attrs: any) =>
          attrs.outlineId ? { "data-outline-id": attrs.outlineId } : {},
      },
    };
  },
});

type OutlineHeadingPosition = {
  node: any;
  pos: number;
  outlineId: number;
  level: number;
};

function collectTopLevelOutlineHeadings(doc: any): OutlineHeadingPosition[] {
  const headings: OutlineHeadingPosition[] = [];
  doc.forEach((node: any, pos: number) => {
    const outlineId = Number(node?.attrs?.outlineId || 0);
    if (node.type.name === "heading" && outlineId > 0) {
      headings.push({
        node,
        pos,
        outlineId,
        level: Number(node.attrs.level || 1),
      });
    }
  });
  return headings;
}

function outlineSectionRange(
  doc: any,
  outlineId: number,
  includeDescendants: boolean,
) {
  const headings = collectTopLevelOutlineHeadings(doc);
  const index = headings.findIndex((item) => item.outlineId === outlineId);
  if (index < 0) return null;
  const current = headings[index];
  let end = doc.content.size;
  for (let i = index + 1; i < headings.length; i += 1) {
    if (!includeDescendants || headings[i].level <= current.level) {
      end = headings[i].pos;
      break;
    }
  }
  return { ...current, end, headings, index };
}

const BidEditor = forwardRef<BidEditorHandle, Props>(
  function BidEditor(props, ref) {
    const {
      contentJSON,
      contentHTML,
      contentKey,
      readOnly,
      outline,
      onReady,
      onDirty,
      streaming,
      streamTitle,
      h,
      onOutlineSync,
      rewriteStream,
    } = props;
    const [headingValue, setHeadingValue] = useState("0");
    const [tableOpen, setTableOpen] = useState(false);
    const [textColorOpen, setTextColorOpen] = useState(false);
    const [bgColorOpen, setBgColorOpen] = useState(false);
    const [tableRows, setTableRows] = useState(4);
    const [tableCols, setTableCols] = useState(3);
    const fileInputRef = useRef<HTMLInputElement>(null);
    // AI 润色选中片段：状态机 idle → streaming → done / error → idle
    const [rewriteStatus, setRewriteStatus] = useState<
      "idle" | "streaming" | "done" | "error"
    >("idle");
    const [rewriteError, setRewriteError] = useState("");
    const [chipState, setChipState] = useState<{
      x: number;
      y: number;
      from: number;
      to: number;
      text: string;
    } | null>(null);
    const rewriteSessionRef = useRef<RewriteSession | null>(null);
    const lastAppliedKey = useRef<string>("");
    // 大纲同步（编辑器→导航）：就绪门控 / 权威基线 / 防抖 / 在途 / 应用映射中
    const syncReadyRef = useRef(false);
    const outlineBaselineRef = useRef<HeadingSnap[] | null>(null);
    const syncTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
    const applyingRef = useRef(false);

    const editor = useEditor({
      extensions: [
        StarterKit.configure({ heading: false }),
        HeadingWithId.configure({ levels: [1, 2, 3, 4, 5, 6] }),
        TextStyle,
        // 颜色/背景色标记：让粘贴带内联颜色的内容也能保留，导出 docx/PDF 时原样输出
        Color,
        BackgroundColor,
        BidTextAlign,
        // 图片：允许 base64（本地插图即 data URL）；四角拖拽等比缩放
        Image.configure({
          allowBase64: true,
          resize: {
            enabled: true,
            directions: [
              "bottom-right",
              "bottom-left",
              "top-right",
              "top-left",
            ],
            minWidth: 40,
            minHeight: 30,
            alwaysPreserveAspectRatio: true,
          },
        }),
        Table.configure({ resizable: true }),
        TableRow,
        TableCell,
        TableHeader,
      ],
      editable: !readOnly,
      content: "",
      immediatelyRender: false,
    });

    // 初始内容加载（HTML 模板导入后按大纲顺序一次性回填 outlineId；
    // 此后不再按位置回填，避免误标用户在编辑器中新建的标题）
    useEffect(() => {
      if (!editor) return;
      const key = String(contentKey || "");
      if (!key) return;
      const hasJSON = Boolean(contentJSON && typeof contentJSON === "object");
      const hasHTML = typeof contentHTML === "string" && contentHTML.length > 0;
      const appliedKey = `${key}:${hasJSON ? "json" : "html"}`;
      if (lastAppliedKey.current === appliedKey) return;

      if (hasJSON) {
        // 历史存量文档可能含空文本节点（例如空的表格单元格）：
        // ProseMirror 解析失败会让 TipTap 静默丢弃整篇内容，表现为编辑器空白且刷新无法恢复。
        // 交给编辑器之前先做一次结构清理。
        const sanitized = sanitizePMDoc(contentJSON);
        editor.commands.setContent(sanitized ?? contentJSON);
      } else if (hasHTML) {
        editor.commands.setContent(contentHTML as string);
      }
      syncReadyRef.current = false;
      const finish = () => {
        syncReadyRef.current = true;
        outlineBaselineRef.current = collectHeadings(editor);
      };
      if (hasHTML && outline && outline.length > 0) {
        requestAnimationFrame(() => {
          syncHeadingIds(editor, outline);
          finish();
        });
      } else {
        finish();
      }
      lastAppliedKey.current = appliedKey;
      onReady?.();
    }, [contentHTML, contentJSON, contentKey, editor, onReady, outline]);

    // 可编辑状态同步（改写会话期间锁定编辑器，避免用户操作与流式替换互相干扰）
    useEffect(() => {
      if (!editor) return;
      editor.setEditable(!readOnly && rewriteStatus === "idle");
    }, [editor, readOnly, rewriteStatus]);

    // 图片宽度对齐 node.attrs.width：撤销/重做只改节点属性，
    // TipTap 内置缩放 NodeView 不会在 update 时回写 style，这里补一次同步。
    useEffect(() => {
      if (!editor) return;
      const syncImageWidth = () => {
        editor.state.doc.descendants((node: any, pos: number) => {
          if (node.type.name !== "image") return;
          const domNode = editor.view.nodeDOM(pos);
          const img =
            domNode instanceof HTMLElement
              ? domNode.querySelector("img")
              : null;
          if (!img) return;
          const width = Number(node.attrs?.width);
          if (Number.isFinite(width) && width > 0) {
            const next = `${width}px`;
            if (img.style.width !== next) img.style.width = next;
          } else if (img.style.width) {
            img.style.removeProperty("width");
          }
        });
      };
      syncImageWidth();
      editor.on("update", syncImageWidth);
      return () => {
        editor.off("update", syncImageWidth);
      };
    }, [editor]);

    // 卸载时清理防抖定时器 + 中止改写流
    useEffect(() => {
      return () => {
        if (syncTimerRef.current) {
          clearTimeout(syncTimerRef.current);
          syncTimerRef.current = null;
        }
        rewriteSessionRef.current?.abort?.abort();
        rewriteSessionRef.current = null;
      };
    }, []);

    // 编辑更新：脏标记 + 大纲结构 diff（防抖后回调 onOutlineSync）
    const scheduleOutlineSync = useCallback(() => {
      if (!editor || !syncReadyRef.current) return;
      if (readOnly || streaming) return;
      if (applyingRef.current) return;
      const snap = collectHeadings(editor);
      if (sameSnap(snap, outlineBaselineRef.current)) return;
      if (syncTimerRef.current) clearTimeout(syncTimerRef.current);
      syncTimerRef.current = setTimeout(() => {
        syncTimerRef.current = null;
        if (!editor || !editor.isEditable) return;
        const latest = collectHeadings(editor);
        if (sameSnap(latest, outlineBaselineRef.current)) return;
        outlineBaselineRef.current = latest;
        onOutlineSync?.(latest);
      }, 800);
    }, [editor, readOnly, streaming, onOutlineSync]);

    // ===== AI 重写选中片段（终态审阅场景，独立于全文生成）=====
    // 浮钮可见性：可编辑、非生成、非改写中、选中 1..3000 字符
    const evaluateChip = useCallback(() => {
      if (!editor) {
        setChipState(null);
        return;
      }
      if (readOnly || streaming || rewriteStatus !== "idle") {
        setChipState(null);
        return;
      }
      const { from, to } = editor.state.selection;
      if (from === to) {
        setChipState(null);
        return;
      }
      const text = editor.state.doc.textBetween(from, to, " ").trim();
      if (!text || text.length > 3000) {
        setChipState(null);
        return;
      }
      const coords = editor.view.coordsAtPos(from);
      setChipState({ x: coords.left, y: coords.bottom + 8, from, to, text });
    }, [editor, readOnly, streaming, rewriteStatus]);

    useEffect(() => {
      if (!editor) return;
      const updateSelection = () => {
        const { from, to } = editor.state.selection;
        let lvl = 0;
        for (let i = 1; i <= 6; i++) {
          if (editor.isActive("heading", { level: i })) {
            lvl = i;
            break;
          }
        }
        setHeadingValue(String(lvl));
        evaluateChip();
      };
      const onUpdate = () => {
        onDirty?.(true);
        scheduleOutlineSync();
      };
      const onBlur = () => setChipState(null);
      editor.on("selectionUpdate", updateSelection);
      editor.on("update", onUpdate);
      editor.on("blur", onBlur);
      return () => {
        editor.off("selectionUpdate", updateSelection);
        editor.off("update", onUpdate);
        editor.off("blur", onBlur);
      };
    }, [editor, onDirty, scheduleOutlineSync, evaluateChip]);

    // 状态/只读/流式变化时重新评估 AI 润色浮钮可见性
    useEffect(() => {
      evaluateChip();
    }, [evaluateChip]);

    // 润色气泡锚点（固定定位，编辑器滚动/窗口缩放时跟随选区）
    const [polishBubble, setPolishBubble] = useState<{
      x: number;
      y: number;
    } | null>(null);
    useEffect(() => {
      if (rewriteStatus === "idle" || !editor || !rewriteSessionRef.current) {
        setPolishBubble(null);
        return;
      }
      const updatePos = () => {
        const s = rewriteSessionRef.current;
        if (!s || !editor.view) return;
        const coords = editor.view.coordsAtPos(s.from);
        setPolishBubble({ x: coords.left, y: coords.bottom + 8 });
      };
      const scrollEl = editor.view.dom.closest(".thin-scrollbars");
      const target = scrollEl || window;
      let raf = 0;
      const onScroll = () => {
        if (raf) return;
        raf = requestAnimationFrame(() => {
          raf = 0;
          updatePos();
        });
      };
      target.addEventListener("scroll", onScroll, { passive: true });
      window.addEventListener("resize", onScroll);
      updatePos();
      return () => {
        if (raf) cancelAnimationFrame(raf);
        target.removeEventListener("scroll", onScroll);
        window.removeEventListener("resize", onScroll);
      };
    }, [editor, rewriteStatus]);

    const disabled = useMemo(
      () => !editor || readOnly || rewriteStatus !== "idle",
      [editor, readOnly, rewriteStatus],
    );

    // 累积润色文本（流式期间不改动文档，原文保持）
    const accumulateRewrite = useCallback(
      (s: RewriteSession | null, rawText: string) => {
        if (s) s.accumulated = rawText;
      },
      [],
    );

    // 应用润色结果：用累积文本替换原选区（行内/跨块两种策略）
    const applyPolish = useCallback(() => {
      if (!editor) return;
      const s = rewriteSessionRef.current;
      if (!s) return;
      const rawText = (s.accumulated || "").trim();
      if (!rawText) {
        setRewriteError("润色内容为空");
        setRewriteStatus("error");
        return;
      }
      const tr = editor.state.tr;
      let content: any;
      if (s.inline) {
        const inlineText = rewriteTextToInline(rawText);
        content = inlineText ? editor.schema.text(inlineText) : Fragment.empty;
      } else {
        content = Fragment.fromJSON(editor.schema, textToPMJSON(rawText));
      }
      try {
        editor.view.dispatch(tr.replaceWith(s.from, s.to, content));
      } catch (e) {
        console.warn("[BidEditor] polish apply failed:", e);
        setRewriteError("应用润色结果失败");
        setRewriteStatus("error");
        return;
      }
      rewriteSessionRef.current = null;
      setRewriteError("");
      setRewriteStatus("idle");
      onDirty?.(true);
    }, [editor, onDirty]);

    const startRewrite = useCallback(() => {
      if (!editor || !chipState) return;
      const { from, to, text } = chipState;
      const doc = editor.state.doc;
      const inline = doc.resolve(from).parent === doc.resolve(to).parent;
      const context = doc.resolve(from).parent.textContent;
      const originalContent = doc.slice(from, to).content;
      rewriteSessionRef.current = {
        from,
        to,
        originalContent,
        originalText: text,
        context,
        inline,
        accumulated: "",
        insertedSize: 0,
        abort: new AbortController(),
      };
      setChipState(null);
      setRewriteError("");
      setRewriteStatus("streaming");
      const s = rewriteSessionRef.current;
      rewriteStream?.(
        { text, context },
        {
          onDelta: (d) => accumulateRewrite(rewriteSessionRef.current, d.text),
          onDone: (d) => {
            accumulateRewrite(rewriteSessionRef.current, d.text);
            setRewriteStatus("done");
          },
          onError: (e) => {
            setRewriteError(e.msg || "润色失败");
            setRewriteStatus("error");
          },
          onCancelled: () => {
            // 停止后保留已生成的部分结果，气泡提供 应用并替换原文/取消
            setRewriteStatus("done");
          },
        },
        s.abort.signal,
      );
    }, [editor, chipState, rewriteStream, accumulateRewrite]);

    const regenerateRewrite = useCallback(() => {
      if (!editor) return;
      const s = rewriteSessionRef.current;
      if (!s) return;
      s.accumulated = "";
      const abort = new AbortController();
      s.abort = abort;
      setRewriteError("");
      setRewriteStatus("streaming");
      rewriteStream?.(
        { text: s.originalText, context: s.context },
        {
          onDelta: (d) => accumulateRewrite(rewriteSessionRef.current, d.text),
          onDone: (d) => {
            accumulateRewrite(rewriteSessionRef.current, d.text);
            setRewriteStatus("done");
          },
          onError: (e) => {
            setRewriteError(e.msg || "润色失败");
            setRewriteStatus("error");
          },
          onCancelled: () => setRewriteStatus("done"),
        },
        abort.signal,
      );
    }, [editor, rewriteStream, accumulateRewrite]);

    const stopRewrite = useCallback(() => {
      rewriteSessionRef.current?.abort?.abort();
      setRewriteStatus("done");
    }, []);

    // 取消：中止流 + 关闭气泡 + 保持原文
    const cancelPolish = useCallback(() => {
      rewriteSessionRef.current?.abort?.abort();
      rewriteSessionRef.current = null;
      setRewriteError("");
      setRewriteStatus("idle");
    }, []);

    useImperativeHandle(ref, () => ({
      replaceChapter: (outlineId: number, contentNodes: any[]) => {
        if (!editor) return false;
        const doc = editor.state.doc;
        const range = outlineSectionRange(doc, outlineId, false);
        if (!range) return false;
        const headingNode = doc.nodeAt(range.pos);
        if (!headingNode) return false;
        try {
          const body = Fragment.fromJSON(
            editor.schema,
            Array.isArray(contentNodes) ? contentNodes : [],
          );
          const replacement = Fragment.from(headingNode).append(body);
          editor.view.dispatch(
            editor.state.tr.replaceWith(range.pos, range.end, replacement),
          );
          return true;
        } catch {
          return false;
        }
      },
      syncOutlineIds: (items) => syncHeadingIds(editor, items),
      findHeadingAndScroll: (outlineId: number) => {
        if (!editor) return;
        const target = outlineSectionRange(editor.state.doc, outlineId, false);
        if (!target) return;
        editor
          .chain()
          .focus()
          .setTextSelection(target.pos + 1)
          .run();
        editor.view?.dispatch(editor.view.state.tr.scrollIntoView());
      },
      getJSON: () => (editor ? editor.getJSON() : null),
      getHTML: () => (editor ? editor.getHTML() : ""),
      setEditable: (v: boolean) => editor?.setEditable(v),
      insertImages: (urls: string[]) => {
        if (!editor || !urls?.length) return;
        urls.forEach((u) => {
          editor.commands.insertContent({ type: "image", attrs: { src: u } });
        });
      },
      getEditor: () => editor,
      insertHeading: (
        outlineId: number,
        level: number,
        title: string,
        opts?: { afterOutlineId?: number },
      ) => {
        if (!editor) return;
        const schema = editor.schema;
        const heading = schema.nodes.heading.create(
          { level, outlineId },
          schema.text(title),
        );
        const para = schema.nodes.paragraph.create();
        const tr = editor.state.tr;
        if (opts?.afterOutlineId) {
          const doc = editor.state.doc;
          const parentRange = outlineSectionRange(
            doc,
            opts.afterOutlineId,
            true,
          );
          if (!parentRange) {
            // 父章节未找到（异常态）：退化为追加到文档末尾
            editor.view.dispatch(
              tr.insert(editor.state.doc.content.size, [heading, para]),
            );
            return;
          }
          editor.view.dispatch(tr.insert(parentRange.end, [heading, para]));
        } else {
          // 默认追加到文档末尾
          editor.view.dispatch(
            tr.insert(editor.state.doc.content.size, [heading, para]),
          );
        }
      },
      removeChapter: (outlineId: number) => {
        if (!editor) return;
        const doc = editor.state.doc;
        const range = outlineSectionRange(doc, outlineId, true);
        if (!range) return;
        editor.view.dispatch(editor.state.tr.delete(range.pos, range.end));
      },
      moveChapter: (
        outlineId: number,
        opts: { parentId: number; nextId?: number; newLevel: number },
      ) => {
        if (!editor) return;
        const { state } = editor;
        const doc = state.doc;
        const clamp = (lv: number) => Math.min(4, Math.max(1, lv));
        // 1) 定位章节区间 [start, end)
        const sourceRange = outlineSectionRange(doc, outlineId, true);
        if (!sourceRange) return;
        const start = sourceRange.pos;
        const end = sourceRange.end;
        const oldLevel = sourceRange.level;
        const moved = doc.slice(start, end).content;
        // 2) 计算原始文档中的目标插入位置（删除前）
        const target = (() => {
          const size = doc.content.size;
          let t = size;
          const isInside = (pos: number) => pos >= start && pos < end;
          if (opts.nextId) {
            const next = outlineSectionRange(doc, opts.nextId, false);
            if (next && !isInside(next.pos)) t = next.pos;
            return t;
          }
          if (opts.parentId > 0) {
            const parent = outlineSectionRange(doc, opts.parentId, true);
            if (parent && !isInside(parent.pos)) return parent.end;
          }
          return size;
        })();
        if (target >= start && target < end) return; // 目标落在自身章节内：忽略
        // 3) 重建章节节点：标题层级按 delta 平移
        const delta = clamp(opts.newLevel) - oldLevel;
        const rebuilt: any[] = [];
        moved.forEach((node: any) => {
          if (node.type.name === "heading") {
            const lv =
              node.attrs.outlineId === outlineId
                ? clamp(opts.newLevel)
                : clamp((node.attrs.level || 1) + delta);
            rebuilt.push(
              node.type.create(
                { ...node.attrs, level: lv },
                node.content,
                node.marks,
              ),
            );
          } else {
            rebuilt.push(node);
          }
        });
        // 4) 删除 + 插入（单事务）
        let tr = state.tr.delete(start, end);
        let t = target;
        if (t > end) t -= end - start;
        t = Math.max(0, Math.min(t, tr.doc.content.size));
        tr = tr.insert(t, rebuilt);
        editor.view.dispatch(tr);
      },
      renameHeading: (outlineId: number, title: string) => {
        if (!editor) return;
        const target = outlineSectionRange(editor.state.doc, outlineId, false);
        if (!target) return;
        const from = target.pos + 1;
        const to = from + target.node.content.size;
        editor
          .chain()
          .focus()
          .setTextSelection({ from, to })
          .insertContent(title)
          .run();
      },
      applyOutlineSync: (
        mapping: Record<number, number>,
        sentHeadings?: HeadingSnap[],
      ) => {
        if (!editor) return;
        applyingRef.current = true;
        try {
          const tr = editor.state.tr;
          let modified = false;
          let idx = -1;
          editor.state.doc.descendants((node: any, pos: number) => {
            if (node.type.name !== "heading") return true;
            idx += 1;
            const newId = mapping[idx];
            if (
              newId != null &&
              (node.attrs.outlineId == null || node.attrs.outlineId !== newId)
            ) {
              tr.setNodeMarkup(pos, undefined, {
                ...node.attrs,
                outlineId: newId,
              });
              modified = true;
            }
            return true;
          });
          if (modified) {
            editor.view.dispatch(tr);
          }
        } finally {
          applyingRef.current = false;
        }
        // 权威基线 = 后端本次实际收到的状态（sent + 新 id）
        const baseline = (sentHeadings || []).map((h, i) => ({
          outlineId: mapping[i] != null ? mapping[i] : h.outlineId,
          level: h.level,
          title: h.title,
        }));
        outlineBaselineRef.current = baseline;
        // 在途期间文档继续变化 → 立即再排一次同步（逐步收敛，不产生循环）
        if (!sameSnap(collectHeadings(editor), baseline)) {
          scheduleOutlineSync();
        }
      },
      resetOutlineBaseline: () => {
        if (syncTimerRef.current) {
          clearTimeout(syncTimerRef.current);
          syncTimerRef.current = null;
        }
        if (editor) {
          outlineBaselineRef.current = collectHeadings(editor);
        }
      },
      getHeadingSnapshot: () => (editor ? collectHeadings(editor) : null),
    }));

    const onUploadImageClick = () => {
      if (disabled) return;
      fileInputRef.current?.click();
    };
    const onFileChange = async (e: any) => {
      const file = e?.target?.files?.[0];
      if (!file || !editor) return;
      const src = await new Promise<string>((resolve) => {
        const reader = new FileReader();
        reader.onload = () => resolve(String(reader.result || ""));
        reader.onerror = () => resolve("");
        reader.readAsDataURL(file);
      });
      if (src) {
        editor
          .chain()
          .focus()
          .insertContent({ type: "image", attrs: { src } })
          .run();
      }
      e.target.value = "";
    };

    // Active state helpers
    const isBold = editor?.isActive("bold") ?? false;
    const isItalic = editor?.isActive("italic") ?? false;
    const isUnderline = editor?.isActive("underline") ?? false;
    const isStrike = editor?.isActive("strike") ?? false;
    const isCode = editor?.isActive("code") ?? false;
    const isCodeBlock = editor?.isActive("codeBlock") ?? false;
    const isBulletList = editor?.isActive("bulletList") ?? false;
    const isOrderedList = editor?.isActive("orderedList") ?? false;
    const textColor =
      (editor?.getAttributes("textStyle").color as string) || "";
    const bgColor =
      (editor?.getAttributes("textStyle").backgroundColor as string) || "";
    // 当前块对齐（标题优先，其次段落；无选区时回退默认左对齐）
    const textAlign = (editor?.getAttributes("heading").textAlign ||
      editor?.getAttributes("paragraph").textAlign ||
      "left") as BidTextAlignValue;

    // Icon button style — active vs default
    const btnStyle = (active: boolean) => ({
      color: active ? "primary.700" : "gray.500",
      bg: active ? "primary.50" : "transparent",
      _hover: active
        ? { bg: "primary.100", color: "primary.800" }
        : { bg: "gray.100", color: "gray.700" },
      _active: { bg: "gray.200", transform: "scale(0.95)" },
      _focusVisible: {
        boxShadow: "0 0 0 2px var(--chakra-colors-primary-500)",
        outline: "none",
      },
      transition:
        "background-color 120ms ease, color 120ms ease, transform 80ms ease",
    });
    const applyTextColor = (color: string | null) => {
      if (!editor) return;
      if (color) editor.chain().focus().setColor(color).run();
      else editor.chain().focus().unsetColor().run();
    };
    const applyBgColor = (color: string | null) => {
      if (!editor) return;
      if (color) editor.chain().focus().setBackgroundColor(color).run();
      else editor.chain().focus().unsetBackgroundColor().run();
    };

    return (
      <Flex
        direction="column"
        h={h || "100%"}
        bg="transparent"
        overflow="hidden"
        position="relative"
      >
        {/* 工具栏 */}
        {!readOnly && (
          <Flex
            align="center"
            gap={0.5}
            px={2}
            py={1.5}
            bg="white"
            borderBottom="1px solid"
            borderColor="gray.200"
            wrap="wrap"
          >
            {/* Group 1: Undo / Redo */}
            <Tooltip label="撤销 ⌘Z" openDelay={800}>
              <IconButton
                aria-label="撤销"
                size="sm"
                variant="ghost"
                icon={<FiRotateCcw size="15" />}
                onClick={() => editor?.chain().focus().undo().run()}
                isDisabled={disabled || !editor?.can?.().undo?.()}
                {...btnStyle(false)}
              />
            </Tooltip>
            <Tooltip label="重做 ⌘⇧Z" openDelay={800}>
              <IconButton
                aria-label="重做"
                size="sm"
                variant="ghost"
                icon={<FiRotateCw size="15" />}
                onClick={() => editor?.chain().focus().redo().run()}
                isDisabled={disabled || !editor?.can?.().redo?.()}
                {...btnStyle(false)}
              />
            </Tooltip>

            <ToolbarDivider />

            {/* Group 2: Text formatting */}
            <Tooltip label="加粗 ⌘B" openDelay={800}>
              <IconButton
                aria-label="加粗"
                size="sm"
                variant="ghost"
                icon={<FiBold size="15" />}
                onClick={() => editor?.chain().focus().toggleBold().run()}
                isDisabled={disabled}
                {...btnStyle(isBold)}
              />
            </Tooltip>
            <Tooltip label="斜体 ⌘I" openDelay={800}>
              <IconButton
                aria-label="斜体"
                size="sm"
                variant="ghost"
                icon={<FiItalic size="15" />}
                onClick={() => editor?.chain().focus().toggleItalic().run()}
                isDisabled={disabled}
                {...btnStyle(isItalic)}
              />
            </Tooltip>
            <Tooltip label="下划线 ⌘U" openDelay={800}>
              <IconButton
                aria-label="下划线"
                size="sm"
                variant="ghost"
                icon={<FiUnderline size="15" />}
                onClick={() => editor?.chain().focus().toggleUnderline().run()}
                isDisabled={disabled}
                {...btnStyle(isUnderline)}
              />
            </Tooltip>
            <Tooltip label="删除线 ⌘⇧S" openDelay={800}>
              <IconButton
                aria-label="删除线"
                size="sm"
                variant="ghost"
                icon={<LuStrikethrough size="15" />}
                onClick={() => editor?.chain().focus().toggleStrike().run()}
                isDisabled={disabled}
                {...btnStyle(isStrike)}
              />
            </Tooltip>

            <ToolbarDivider />

            {/* Group 3: 段落对齐（左 / 居中 / 右 / 两端） */}
            {TEXT_ALIGN_ITEMS.map((item) => (
              <Tooltip key={item.value} label={item.label} openDelay={800}>
                <IconButton
                  aria-label={item.label}
                  size="sm"
                  variant="ghost"
                  icon={item.icon}
                  onClick={() =>
                    editor?.chain().focus().setTextAlign(item.value).run()
                  }
                  isDisabled={disabled}
                  {...btnStyle(textAlign === item.value)}
                />
              </Tooltip>
            ))}

            <ToolbarDivider />

            {/* Group 4: 文字颜色 / 背景颜色 */}
            <ToolbarColorButton
              type="text"
              currentColor={textColor}
              isOpen={textColorOpen}
              onOpenChange={setTextColorOpen}
              onPick={applyTextColor}
              disabled={disabled}
              buttonStyle={btnStyle}
            />
            <ToolbarColorButton
              type="background"
              currentColor={bgColor}
              isOpen={bgColorOpen}
              onOpenChange={setBgColorOpen}
              onPick={applyBgColor}
              disabled={disabled}
              buttonStyle={btnStyle}
            />

            <ToolbarDivider />

            {/* Group 5: Code */}
            <Tooltip label="行内代码 ⌘E" openDelay={800}>
              <IconButton
                aria-label="行内代码"
                size="sm"
                variant="ghost"
                icon={<FiCode size="15" />}
                onClick={() => editor?.chain().focus().toggleCode().run()}
                isDisabled={disabled}
                {...btnStyle(isCode)}
              />
            </Tooltip>
            <Tooltip label="代码块 ⌘⇧E" openDelay={800}>
              <IconButton
                aria-label="代码块"
                size="sm"
                variant="ghost"
                icon={<LuCodeXml size="15" />}
                onClick={() => editor?.chain().focus().toggleCodeBlock().run()}
                isDisabled={disabled}
                {...btnStyle(isCodeBlock)}
              />
            </Tooltip>

            <ToolbarDivider />

            {/* Group 6: Heading */}
            <Select
              size="sm"
              w="6.5rem"
              value={headingValue}
              onChange={(e) => {
                const lvl = Number(e.target.value);
                if (!editor || disabled) return;
                if (lvl === 0) editor.chain().focus().setParagraph().run();
                else
                  editor
                    .chain()
                    .focus()
                    .toggleHeading({ level: lvl as any })
                    .run();
              }}
              isDisabled={disabled}
              bg="white"
              borderColor="gray.200"
              fontSize="sm"
              icon={<FiType size="13" />}
              sx={{
                "&:hover": { borderColor: "gray.300" },
                "&:focus-visible": {
                  boxShadow: "0 0 0 2px var(--chakra-colors-primary-500)",
                  outline: "none",
                },
              }}
            >
              <option value="0">正文</option>
              <option value="1">标题 1</option>
              <option value="2">标题 2</option>
              <option value="3">标题 3</option>
              <option value="4">标题 4</option>
              <option value="5">标题 5</option>
              <option value="6">标题 6</option>
            </Select>

            <ToolbarDivider />

            {/* Group 7: Lists */}
            <Tooltip label="无序列表" openDelay={800}>
              <IconButton
                aria-label="无序列表"
                size="sm"
                variant="ghost"
                icon={<LuList size="15" />}
                onClick={() => editor?.chain().focus().toggleBulletList().run()}
                isDisabled={disabled}
                {...btnStyle(isBulletList)}
              />
            </Tooltip>
            <Tooltip label="有序列表" openDelay={800}>
              <IconButton
                aria-label="有序列表"
                size="sm"
                variant="ghost"
                icon={<LuListOrdered size="15" />}
                onClick={() =>
                  editor?.chain().focus().toggleOrderedList().run()
                }
                isDisabled={disabled}
                {...btnStyle(isOrderedList)}
              />
            </Tooltip>

            <ToolbarDivider />

            {/* Group 8: Insert */}
            <Tooltip label="插入图片" openDelay={800}>
              <IconButton
                aria-label="插入图片"
                size="sm"
                variant="ghost"
                icon={<FiImage size="15" />}
                onClick={onUploadImageClick}
                isDisabled={disabled}
                {...btnStyle(false)}
              />
            </Tooltip>
            <Popover
              isOpen={tableOpen}
              onOpen={() => {
                if (!disabled) setTableOpen(true);
              }}
              onClose={() => setTableOpen(false)}
              placement="bottom"
              closeOnBlur
            >
              <PopoverTrigger>
                <span>
                  <Tooltip label="插入表格" openDelay={800}>
                    <IconButton
                      aria-label="插入表格"
                      size="sm"
                      variant="ghost"
                      icon={<LuGrid3X3 size="15" />}
                      isDisabled={disabled}
                      {...btnStyle(false)}
                    />
                  </Tooltip>
                </span>
              </PopoverTrigger>
              <PopoverContent w="240px">
                <PopoverArrow />
                <PopoverBody>
                  <Flex direction="column" gap={3}>
                    <Flex align="center" justify="space-between">
                      <Text fontSize="sm" color="gray.600">
                        行数
                      </Text>
                      <NumberInput
                        size="sm"
                        value={tableRows}
                        min={1}
                        max={20}
                        onChange={(v) => setTableRows(Number(v || 1))}
                        w="5rem"
                      >
                        <NumberInputField />
                        <NumberInputStepper>
                          <NumberIncrementStepper />
                          <NumberDecrementStepper />
                        </NumberInputStepper>
                      </NumberInput>
                    </Flex>
                    <Flex align="center" justify="space-between">
                      <Text fontSize="sm" color="gray.600">
                        列数
                      </Text>
                      <NumberInput
                        size="sm"
                        value={tableCols}
                        min={1}
                        max={10}
                        onChange={(v) => setTableCols(Number(v || 1))}
                        w="5rem"
                      >
                        <NumberInputField />
                        <NumberInputStepper>
                          <NumberIncrementStepper />
                          <NumberDecrementStepper />
                        </NumberInputStepper>
                      </NumberInput>
                    </Flex>
                    <Button
                      size="sm"
                      bg="primary.600"
                      color="white"
                      _hover={{ bg: "primary.700" }}
                      _active={{ bg: "primary.800" }}
                      onClick={() => {
                        if (!editor || disabled) return;
                        editor
                          .chain()
                          .focus()
                          .insertTable({
                            rows: tableRows,
                            cols: tableCols,
                            withHeaderRow: false,
                          })
                          .run();
                        setTableOpen(false);
                      }}
                    >
                      插入
                    </Button>
                  </Flex>
                </PopoverBody>
              </PopoverContent>
            </Popover>

            <ToolbarDivider />

            {/* Group 9: Clear format */}
            <Tooltip label="清除格式" openDelay={800}>
              <IconButton
                aria-label="清除格式"
                size="sm"
                variant="ghost"
                icon={<LuEraser size="15" />}
                onClick={() =>
                  editor?.chain().focus().unsetAllMarks().clearNodes().run()
                }
                isDisabled={disabled}
                {...btnStyle(false)}
              />
            </Tooltip>

            <input
              ref={fileInputRef}
              type="file"
              accept="image/*"
              style={{ display: "none" }}
              onChange={onFileChange}
            />
          </Flex>
        )}

        {/* 生成中横幅 */}
        {streaming && (
          <Flex
            align="center"
            gap={2}
            px={4}
            py={2}
            bg="workbench.control"
            color="workbench.paper"
            fontSize="sm"
          >
            <Box w="2" h="2" borderRadius="full" bg="gold.400" />
            <Text fontWeight="medium">AI 正在撰写：{streamTitle || "…"}</Text>
            <Box flex="1" />
            <Text opacity={0.7} fontSize="xs">
              编辑器已锁定 · 自动保存挂起
            </Text>
          </Flex>
        )}

        {/* 编辑区 */}
        <Box
          flex="1"
          overflowY="auto"
          className="thin-scrollbars"
          px={{ base: 4, md: 10 }}
          py={8}
        >
          <Box w="full" minW={0}>
            <style>{`
            .bid-prose .tiptap { outline: none; min-height: 60vh; max-width: 75ch; margin-inline: auto; }
            .bid-prose .tiptap h1 { font-size: 1.9rem; font-weight: 700; color: #1E3A5F; margin: 1.6em 0 0.6em; line-height: 1.35; }
            .bid-prose .tiptap h2 { font-size: 1.5rem; font-weight: 700; color: #1E3A5F; margin: 1.4em 0 0.5em; line-height: 1.4; }
            .bid-prose .tiptap h3 { font-size: 1.25rem; font-weight: 600; color: #172E4C; margin: 1.2em 0 0.45em; line-height: 1.45; }
            .bid-prose .tiptap h4 { font-size: 1.1rem; font-weight: 600; color: #172E4C; margin: 1em 0 0.4em; }
            .bid-prose .tiptap p { font-size: 1rem; line-height: 1.9; color: #2D3748; margin: 0.7em 0; }
            .bid-prose .tiptap ul, .bid-prose .tiptap ol { padding-left: 1.6em; margin: 0.6em 0; line-height: 1.8; color: #2D3748; }
            .bid-prose .tiptap code { background: #F0F4FA; color: #1E3A5F; padding: 0.15em 0.35em; border-radius: 4px; font-size: 0.9em; font-family: 'SF Mono', 'Fira Code', 'Consolas', monospace; }
            .bid-prose .tiptap pre { background: #1E293B; color: #E2E8F0; padding: 1em 1.2em; border-radius: 8px; overflow-x: auto; margin: 1em 0; }
            .bid-prose .tiptap pre code { background: none; color: inherit; padding: 0; border-radius: 0; font-size: 0.9em; }
            .bid-prose .tiptap u { text-decoration: underline; text-underline-offset: 2px; }
            .bid-prose .tiptap s { text-decoration: line-through; }
            .bid-prose .tiptap table { border-collapse: collapse; width: 100%; margin: 1em 0; }
            .bid-prose .tiptap th, .bid-prose .tiptap td { border: 1px solid #CBD5E0; padding: 8px 12px; font-size: 0.95rem; }
            .bid-prose .tiptap th { background: #F0F4FA; font-weight: 600; color: #1E3A5F; }
            /* 图片：height:auto 恒定保持原始比例（宽度缩小/放大都不变形） */
            .bid-prose .tiptap img { max-width: 100%; height: auto !important; border-radius: 8px; display: block; }
            /* 可缩放图片容器：整行居中，四角手柄悬停/拖拽时显形 */
            .bid-prose .tiptap [data-resize-container] { width: 100%; justify-content: center; margin: 0.8em 0; }
            .bid-prose .tiptap [data-resize-wrapper] { max-width: 100%; }
            .bid-prose .tiptap [data-resize-container] img { margin: 0; }
            .bid-prose .tiptap [data-resize-handle] { width: 12px; height: 12px; margin: -6px; border-radius: 3px; background: #FFFFFF; border: 1.5px solid #1E3A5F; box-shadow: 0 1px 3px rgba(30,58,95,0.25); opacity: 0; transition: opacity 120ms ease, transform 120ms ease; z-index: 2; }
            .bid-prose .tiptap [data-resize-container]:hover [data-resize-handle],
            .bid-prose .tiptap [data-resize-container][data-resize-state="true"] [data-resize-handle],
            .bid-prose .tiptap .ProseMirror-selectednode [data-resize-handle] { opacity: 1; }
            .bid-prose .tiptap [data-resize-handle]:hover { transform: scale(1.15); }
            /* 只读（生成中/蓝图预览）时手柄一律不出现 */
            .bid-prose .tiptap[contenteditable="false"] [data-resize-handle] { display: none; }
            .bid-prose .tiptap [data-resize-handle="bottom-right"],
            .bid-prose .tiptap [data-resize-handle="top-left"] { cursor: nwse-resize; }
            .bid-prose .tiptap [data-resize-handle="bottom-left"],
            .bid-prose .tiptap [data-resize-handle="top-right"] { cursor: nesw-resize; }
            .bid-prose .tiptap [data-resize-container][data-resize-state="true"] img { box-shadow: 0 0 0 2px #D4A853; }
            .bid-prose .tiptap .ProseMirror-selectednode { outline: 2px solid #D4A853; }
            .bid-prose .tiptap p.is-editor-empty:first-child::before { content: attr(data-placeholder); color: #A0AEC0; float: left; height: 0; pointer-events: none; }
          `}</style>
            <Box
              className="bid-prose"
              bg="workbench.paper"
              border="1px solid"
              borderColor="workbench.line"
              borderRadius="14px"
              boxShadow="md"
              px={{ base: 5, md: 10 }}
              py={8}
              minH="70vh"
            >
              <EditorContent editor={editor} />
            </Box>
          </Box>
        </Box>

        {/* AI 润色浮钮：选中文本后出现（onMouseDown preventDefault 防止失焦丢失选区） */}
        {chipState && rewriteStatus === "idle" && (
          <Box
            position="fixed"
            zIndex={30}
            style={{ left: chipState.x, top: chipState.y }}
            onMouseDown={(e: any) => e.preventDefault()}
          >
            <Button
              size="sm"
              variant="solid"
              bg="workbench.control"
              color="workbench.paper"
              leftIcon={<LuSparkles />}
              boxShadow="lg"
              _hover={{
                bg: "workbench.controlRaised",
                transform: "translateY(-1px)",
              }}
              _active={{ transform: "translateY(0)" }}
              _focusVisible={{
                boxShadow: "none",
                outline: "2px solid",
                outlineColor: "gold.400",
                outlineOffset: "2px",
              }}
              onClick={startRewrite}
            >
              AI 润色
            </Button>
          </Box>
        )}

        {/* AI 润色气泡：流式加载 / 完成（可滚动预览 + 应用/取消）/ 失败（重试/取消） */}
        {rewriteStatus !== "idle" && polishBubble && (
          <Box
            position="fixed"
            zIndex={30}
            style={{
              left: Math.max(
                8,
                Math.min(
                  polishBubble.x,
                  (typeof window !== "undefined" ? window.innerWidth : 1200) -
                    400,
                ),
              ),
              top: polishBubble.y,
            }}
          >
            <Flex
              direction="column"
              w="380px"
              maxW="calc(100dvw - 24px)"
              bg="workbench.paper"
              border="1px solid"
              borderColor="workbench.line"
              borderRadius="14px"
              boxShadow="xl"
              overflow="hidden"
            >
              {/* 头部 */}
              <Flex
                align="center"
                gap={2}
                px={3}
                py={2}
                borderBottom="1px solid"
                borderColor="gray.100"
              >
                <Flex
                  w="6"
                  h="6"
                  borderRadius="md"
                  align="center"
                  justify="center"
                  bg="primary.50"
                  color="primary.600"
                  flexShrink={0}
                >
                  <LuSparkles size="13" />
                </Flex>
                <Text fontSize="sm" fontWeight="600" color="gray.700">
                  AI 润色
                </Text>
                {rewriteStatus === "error" && (
                  <Text
                    fontSize="xs"
                    color="error.500"
                    noOfLines={1}
                    minW="0"
                    flex="1"
                  >
                    {rewriteError || "润色失败"}
                  </Text>
                )}
                <Box flex="1" />
              </Flex>

              {rewriteStatus === "streaming" && (
                <Flex align="center" gap={2} px={4} py={4}>
                  <Spinner size="sm" color="primary.500" />
                  <Text fontSize="sm" color="gray.700" fontWeight="medium">
                    AI 润色中…
                  </Text>
                  <Box flex="1" />
                  <Button
                    size="xs"
                    variant="ghost"
                    colorScheme="error"
                    leftIcon={<FiStopCircle />}
                    onClick={stopRewrite}
                  >
                    停止
                  </Button>
                </Flex>
              )}

              {(rewriteStatus === "done" || rewriteStatus === "error") && (
                <>
                  <Box
                    maxH="40vh"
                    overflowY="auto"
                    className="thin-scrollbars"
                    px={3}
                    py={2.5}
                  >
                    <Text
                      fontSize="sm"
                      color={
                        rewriteStatus === "error" ? "error.600" : "gray.700"
                      }
                      whiteSpace="pre-wrap"
                      lineHeight="1.8"
                    >
                      {rewriteStatus === "error"
                        ? rewriteError || "润色失败，请重试"
                        : rewriteSessionRef.current?.accumulated ||
                          "（暂无内容）"}
                    </Text>
                  </Box>
                  <Flex
                    gap={2}
                    px={3}
                    py={2.5}
                    borderTop="1px solid"
                    borderColor="gray.100"
                  >
                    {rewriteStatus === "error" ? (
                      <>
                        <Button
                          size="sm"
                          variant="ghost"
                          flex="1"
                          leftIcon={<FiRotateCcw />}
                          onClick={regenerateRewrite}
                        >
                          重试
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          flex="1"
                          onClick={cancelPolish}
                        >
                          取消
                        </Button>
                      </>
                    ) : (
                      <>
                        <Button
                          size="sm"
                          variant="ghost"
                          flex="1"
                          onClick={cancelPolish}
                        >
                          取消
                        </Button>
                        <Button
                          size="sm"
                          flex="1"
                          bg="primary.600"
                          color="white"
                          leftIcon={<FiCheck />}
                          _hover={{ bg: "primary.700" }}
                          _active={{ bg: "primary.800" }}
                          _focusVisible={{
                            boxShadow: "0 0 0 3px rgba(212,168,83,0.45)",
                            outline: "none",
                          }}
                          onClick={applyPolish}
                        >
                          应用并替换原文
                        </Button>
                      </>
                    )}
                  </Flex>
                </>
              )}
            </Flex>
          </Box>
        )}
      </Flex>
    );
  },
);

// ===== 大纲结构快照（编辑器 → 导航同步）=====
type HeadingSnap = { outlineId: number | null; level: number; title: string };

function collectHeadings(editor: any): HeadingSnap[] {
  const out: HeadingSnap[] = [];
  editor.state.doc.descendants((node: any) => {
    if (node.type.name === "heading") {
      out.push({
        outlineId: node.attrs.outlineId ?? null,
        level: node.attrs.level,
        title: node.textContent,
      });
    }
    return true;
  });
  return out;
}

function sameSnap(a: HeadingSnap[] | null, b: HeadingSnap[] | null): boolean {
  if (!a || !b) return false;
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i++) {
    if (a[i].outlineId !== b[i].outlineId) return false;
    if (a[i].level !== b[i].level) return false;
    if (a[i].title !== b[i].title) return false;
  }
  return true;
}

// 按大纲顺序为标题节点回填 outlineId
function syncHeadingIds(
  editor: any,
  outline: { id: number; level: number; title: string }[],
) {
  if (!editor || !outline || outline.length === 0) return;
  const tr = editor.state.tr;
  let modified = false;
  const headings: { pos: number; node: any }[] = [];
  editor.state.doc.descendants((node: any, pos: number) => {
    if (node.type.name === "heading") {
      headings.push({ pos, node });
    }
    return true;
  });
  const existing = new Set(
    headings
      .map((item) => Number(item.node.attrs.outlineId || 0))
      .filter((id) => id > 0),
  );
  if (outline.every((item) => existing.has(item.id))) return;
  headings.forEach((h, idx) => {
    const item = outline[idx];
    if (!item) return;
    if (h.node.attrs.outlineId === item.id) return;
    tr.setNodeMarkup(h.pos, undefined, { ...h.node.attrs, outlineId: item.id });
    modified = true;
  });
  if (modified) {
    editor.view.dispatch(tr);
  }
}

export default BidEditor;
