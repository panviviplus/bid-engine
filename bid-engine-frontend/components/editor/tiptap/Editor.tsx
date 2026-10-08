"use client";

/* eslint-disable no-unused-vars */
/* eslint-disable prettier/prettier */

import {
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
  forwardRef,
  useCallback,
} from "react";
import {
  Box,
  Flex,
  Button,
  Select,
  Input,
  Spinner,
  Text,
  IconButton,
  Popover,
  PopoverTrigger,
  PopoverContent,
  PopoverBody,
  NumberInput,
  NumberInputField,
  NumberInputStepper,
  NumberIncrementStepper,
  NumberDecrementStepper,
} from "@chakra-ui/react";
import { Drawer, DrawerOverlay, DrawerContent, DrawerHeader, DrawerBody } from "@chakra-ui/modal";
import { PopoverArrow } from "@chakra-ui/popover";
import { useEditor, EditorContent } from "@tiptap/react";
import { Extension } from "@tiptap/core";
import StarterKit from "@tiptap/starter-kit";
import Heading from "@tiptap/extension-heading";
import Image from "@tiptap/extension-image";
import { Table } from "@tiptap/extension-table";
import TableRow from "@tiptap/extension-table-row";
import TableCell from "@tiptap/extension-table-cell";
import TableHeader from "@tiptap/extension-table-header";
import { TextStyle } from "@tiptap/extension-text-style";
import { FiList, FiRotateCcw, FiRotateCw, FiGrid } from "react-icons/fi";

export type EditorContextMenuItem = {
  key: string;
  label: string;
  disabled?: boolean;
  onClick: (ctx: {
    selectionText: string;
    replaceSelection: (text: string) => void;
    close: () => void;
  }) => void | Promise<void>;
};

type EditorProps = {
  objectKey: string;
  readOnly?: boolean;
  onReadyChange?: (ready: boolean) => void;
  onSave?: () => void;
  h?: string | number;
  contentJSON?: any;
  contentHTML?: string;
  contentKey?: string;
  contextMenuItems?: EditorContextMenuItem[];
  connected?: boolean;
  streaming?: boolean;
  streamProgress?: number;
  streamTotal?: number;
  streamPercent?: number;
};

export type EditorRef = {
  insertImages: (urls: string[]) => void;
  insertTable: (rows: any[], headers?: string[]) => void;
  findTextAndScroll: (text: string) => void;
  getHTML: () => string;
};

function toTableHTML(rows: any[], headers?: string[]) {
  if (!Array.isArray(rows) || rows.length === 0) return "";
  const cols = headers && headers.length ? headers : Object.keys(rows[0] || {});
  let html = "<table style=\"border-collapse:collapse;border:1px solid #000\"><thead><tr>";
  cols.forEach((h) => {
    html += `<th style="border:1px solid #000">${h}</th>`;
  });
  html += "</tr></thead><tbody>";
  rows.forEach((r) => {
    html += "<tr>";
    cols.forEach((h) => {
      html += `<td style="border:1px solid #000">${r?.[h] ?? ""}</td>`;
    });
    html += "</tr>";
  });
  html += "</tbody></table>";
  return html;
}

const Color = Extension.create({
  name: "color",
  addGlobalAttributes() {
    return [
      {
        types: ["textStyle"],
        attributes: {
          color: {
            default: null,
            parseHTML: (element) => (element as HTMLElement).style?.color || null,
            renderHTML: (attrs) => (attrs.color ? { style: `color: ${attrs.color}` } : {}),
          },
        },
      },
    ];
  },
  addCommands() {
    return {
      setColor:
        (color: string) =>
        ({ chain }: any) =>
          chain().setMark("textStyle", { color }).run(),
      unsetColor:
        () =>
        ({ chain }: any) =>
          chain().setMark("textStyle", { color: null }).run(),
    } as any;
  },
});

const Editor = forwardRef<EditorRef, EditorProps>(function Editor(props, ref) {
  const { objectKey, readOnly, onReadyChange, onSave, h, contentJSON, contentHTML, contentKey, contextMenuItems } = props;
  const [ready, setReady] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const [menuBusy, setMenuBusy] = useState(false);
  const [menuPos, setMenuPos] = useState<{ x: number; y: number }>({ x: 0, y: 0 });
  const [selectionText, setSelectionText] = useState<string>("");
  const selectionRef = useRef<{ from: number; to: number }>({ from: 0, to: 0 });
  const lastAppliedKey = useRef<string>("");
  const fileInputRef = useRef<HTMLInputElement>(null);
  const editor = useEditor({
    extensions: [
      StarterKit.configure({ heading: false }),
      Heading.configure({ levels: [1, 2, 3, 4, 5, 6] }),
      TextStyle,
      Color,
      Image,
      Table.configure({ resizable: true }),
      TableRow,
      TableHeader,
      TableCell,
    ],
    editable: !readOnly,
    content: "",
    immediatelyRender: false,
  });

  useEffect(() => {
    if (!editor) return;
    const key = String(contentKey || objectKey || "");
    if (!key) return;
    const hasJSON = Boolean(contentJSON && typeof contentJSON === "object");
    const hasHTML = typeof contentHTML === "string";
    if (!hasJSON && !hasHTML) return;

    const appliedKey = `${key}:${hasJSON ? "json" : "html"}`;
    if (lastAppliedKey.current === appliedKey) return;

    if (hasJSON) {
      editor.commands.setContent(contentJSON);
    } else {
      editor.commands.setContent(contentHTML ?? "");
    }

    lastAppliedKey.current = appliedKey;
    setReady(true);
    if (onReadyChange) onReadyChange(true);
  }, [contentHTML, contentJSON, contentKey, objectKey, editor, onReadyChange]);

  useEffect(() => {
    if (!editor) return;
    if (!menuOpen) return;
    const onDoc = (e: MouseEvent) => {
      const t = e.target as HTMLElement | null;
      if (!t) return;
      if (t.closest("[data-editor-context-menu]") || t.closest(".tiptap")) return;
      setMenuOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setMenuOpen(false);
    };
    document.addEventListener("mousedown", onDoc);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDoc);
      document.removeEventListener("keydown", onKey);
    };
  }, [menuOpen, editor]);

  useImperativeHandle(ref, () => ({
    insertImages: (urls: string[]) => {
      if (!editor || !urls?.length) return;
      urls.forEach((u) => {
        editor.commands.insertContent({ type: "image", attrs: { src: u } });
      });
    },
    insertTable: (rows: any[], headers?: string[]) => {
      if (!editor || !Array.isArray(rows) || rows.length === 0) return;
      const html = toTableHTML(rows, headers);
      editor.commands.insertContent(html);
    },
    findTextAndScroll: (text: string) => {
      if (!editor || !text) return;
      const view = editor.view;
      const docText = view.state.doc.textBetween(0, view.state.doc.content.size, "\n");
      const idx = docText.indexOf(text);
      if (idx >= 0) {
        view.dispatch(
          view.state.tr.setSelection(
            // @ts-ignore
            view.state.selection.constructor.create(view.state.doc, idx, idx)
          )
        );
        const el = document.querySelector(".tiptap");
        if (el) el.scrollIntoView({ behavior: "smooth", block: "center" });
      }
    },
    getHTML: () => {
      if (!editor) return "";
      return editor.getHTML();
    },
  }));

  const disabled = useMemo(() => !editor || readOnly, [editor, readOnly]);

  const [hasSelection, setHasSelection] = useState(false);
  const [headingValue, setHeadingValue] = useState("0");
  const [tocItems, setTocItems] = useState<{ level: number; text: string; pos: number }[]>([]);
  const [tocOpen, setTocOpen] = useState(false);

  const [colorValue, setColorValue] = useState<string>("#000000");
  const [colorOpen, setColorOpen] = useState(false);
  const [pendingColor, setPendingColor] = useState<string>("#000000");

  const [tableOpen, setTableOpen] = useState(false);
  const [tableRows, setTableRows] = useState(3);
  const [tableCols, setTableCols] = useState(3);

  useEffect(() => {
    if (!editor) return;
    const updateSelection = () => {
      const { from, to } = editor.state.selection;
      setHasSelection(from !== to);
      let lvl = 0;
      for (let i = 1; i <= 6; i++) {
        if (editor.isActive("heading", { level: i })) {
          lvl = i;
          break;
        }
      }
      setHeadingValue(String(lvl));
    };
    const updateToc = () => {
      const items: { level: number; text: string; pos: number }[] = [];
      editor.state.doc.descendants((node: any, pos: number) => {
        if (node?.type?.name !== "heading") return;
        const text = String(node.textContent || "").trim();
        if (!text) return;
        items.push({ level: Number(node.attrs?.level || 1), text, pos });
      });
      setTocItems(items);
    };
    updateSelection();
    updateToc();
    editor.on("selectionUpdate", updateSelection);
    editor.on("update", updateToc);
    return () => {
      editor.off("selectionUpdate", updateSelection);
      editor.off("update", updateToc);
    };
  }, [editor]);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && (e.key === "s" || e.key === "S")) {
        e.preventDefault();
        if (!disabled && onSave) onSave();
      }
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [disabled, onSave]);

  const applyColor = (color: string) => {
    if (!editor || disabled) return;
    editor.chain().focus().setColor(color).run();
    setColorValue(color);
  };

  const replaceSelection = useCallback(
    (text: string) => {
      if (!editor || disabled) return;
      const { from, to } = selectionRef.current;
      if (!from && !to) return;
      editor.commands.insertContentAt({ from, to }, text);
    },
    [editor, disabled],
  );

  const onContextMenu = (e: any) => {
    if (!editor) return;
    if (!contextMenuItems || contextMenuItems.length === 0) return;
    if (disabled) return;
    const { from, to } = editor.state.selection;
    if (from === to) return;
    const text = editor.state.doc.textBetween(from, to, " ");
    if (!text.trim()) return;
    e.preventDefault();
    selectionRef.current = { from, to };
    setSelectionText(text);
    setMenuPos({ x: e.clientX, y: e.clientY });
    setMenuOpen(true);
  };

  const handleMenuItem = async (item: EditorContextMenuItem) => {
    if (menuBusy) return;
    if (item.disabled) return;
    setMenuBusy(true);
    try {
      await item.onClick({
        selectionText,
        replaceSelection,
        close: () => setMenuOpen(false),
      });
    } finally {
      setMenuBusy(false);
    }
  };

  const onBold = () => {
    if (disabled || !hasSelection) return;
    editor?.chain().focus().toggleBold().run();
  };
  const onUndo = () => {
    if (!editor || disabled) return;
    editor.chain().focus().undo().run();
  };
  const onRedo = () => {
    if (!editor || disabled) return;
    editor.chain().focus().redo().run();
  };
  const onClearFormat = () => {
    if (!editor || disabled || !hasSelection) return;
    editor.chain().focus().unsetAllMarks().unsetColor().clearNodes().run();
  };
  const onInsertTableConfirm = () => {
    if (!editor || disabled) return;
    const rows = Math.min(20, Math.max(1, Number(tableRows || 1)));
    const cols = Math.min(20, Math.max(1, Number(tableCols || 1)));
    editor.chain().focus().insertTable({ rows, cols, withHeaderRow: false }).run();
    setTableOpen(false);
  };
  const onUploadImageClick = () => {
    if (disabled) return;
    fileInputRef.current?.click();
  };
  const onFileChange = async (e: any) => {
    const file = e?.target?.files?.[0];
    if (!file) return;
    if (!editor) return;
    const src = await new Promise<string>((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(String(reader.result || ""));
      reader.onerror = () => reject(new Error("read failed"));
      reader.readAsDataURL(file);
    }).catch(() => "");
    if (src) {
      editor?.commands.insertContent({ type: "image", attrs: { src } });
    }
    e.target.value = "";
  };

  const connected = props.connected ?? false;
  const streaming = props.streaming ?? false;
  const streamProgress = props.streamProgress ?? 0;
  const streamTotal = props.streamTotal ?? 0;
  const streamPercent = useMemo(() => {
    if (!streamTotal || streamTotal <= 0) return 0;
    const p = Math.floor((streamProgress * 100) / streamTotal);
    return p > 100 ? 100 : p;
  }, [streamProgress, streamTotal]);

  return (
    <Flex
      direction="column"
      h={h || "100%"}
      bg="white"
      border="1px solid"
      borderColor="gray.200"
      borderRadius="2xl"
      overflow="hidden"
    >
      <Drawer isOpen={tocOpen} placement="left" onClose={() => setTocOpen(false)} size="xs">
        <DrawerOverlay />
        <DrawerContent>
          <DrawerHeader>目录预览</DrawerHeader>
          <DrawerBody>
            <Flex direction="column" gap={1}>
              {tocItems.map((it, idx) => (
                <Button
                  key={`${it.pos}-${idx}`}
                  variant="ghost"
                  justifyContent="flex-start"
                  size="sm"
                  onClick={() => {
                    if (!editor) return;
                    editor.chain().focus().setTextSelection(it.pos + 1).run();
                    editor.view?.dispatch(editor.view.state.tr.scrollIntoView());
                  }}
                >
                  <Text fontSize="sm" pl={`${Math.max(0, (it.level || 1) - 1) * 12}px`} noOfLines={1}>
                    {it.text}
                  </Text>
                </Button>
              ))}
            </Flex>
          </DrawerBody>
        </DrawerContent>
      </Drawer>
      {!readOnly && (
        <Flex direction="column" gap={2} px={3} pt={3} pb={2} bg="white" borderBottom="1px solid" borderColor="gray.200">
          <Flex gap={2} align="center" wrap="wrap">
            <IconButton
              aria-label="目录预览"
              size="sm"
              variant="ghost"
              icon={<FiList />}
              color={tocOpen ? "blue.500" : "gray.500"}
              onClick={() => setTocOpen((v: boolean) => !v)}
              isDisabled={disabled || tocItems.length === 0}
            />
            <IconButton
              aria-label="撤销"
              size="sm"
              variant="ghost"
              icon={<FiRotateCcw />}
              onClick={onUndo}
              isDisabled={disabled || !editor?.can?.().undo?.()}
            />
            <IconButton
              aria-label="重做"
              size="sm"
              variant="ghost"
              icon={<FiRotateCw />}
              onClick={onRedo}
              isDisabled={disabled || !editor?.can?.().redo?.()}
            />
            <Button
              size="sm"
              bg="gray.700"
              color="white"
              _hover={{ bg: "gray.800" }}
              _active={{ bg: "gray.900" }}
              onClick={onBold}
              isDisabled={disabled || !hasSelection}
            >
              加粗
            </Button>
            <Select
              size="sm"
              w="7rem"
              value={headingValue}
              onChange={(e) => {
                const lvl = Number(e.target.value);
                if (!editor || disabled) return;
                if (lvl === 0) {
                  editor.chain().focus().setParagraph().run();
                } else if (lvl >= 1 && lvl <= 6) {
                  editor.chain().focus().toggleHeading({ level: lvl as 1 | 2 | 3 | 4 | 5 | 6 }).run();
                }
              }}
              isDisabled={disabled}
            >
              <option value="0">普通文本</option>
              <option value="1">H1</option>
              <option value="2">H2</option>
              <option value="3">H3</option>
              <option value="4">H4</option>
              <option value="5">H5</option>
              <option value="6">H6</option>
            </Select>
            <Popover
              isOpen={colorOpen}
              onOpen={() => {
                if (disabled) return;
                setPendingColor(colorValue);
                setColorOpen(true);
              }}
              onClose={() => setColorOpen(false)}
              placement="bottom"
            >
              <PopoverTrigger>
                <Button
                  size="sm"
                  bg="primary.700"
                  color="white"
                  _hover={{ bg: "primary.800" }}
                  _active={{ bg: "primary.900" }}
                  isDisabled={disabled}
                >
                  选择颜色
                </Button>
              </PopoverTrigger>
              <PopoverContent w="280px">
                <PopoverArrow />
                <PopoverBody>
                  <Flex wrap="wrap" gap={2}>
                    {[
                      "#000000",
                      "#4A5568",
                      "#718096",
                      "#A0AEC0",
                      "#E53E3E",
                      "#DD6B20",
                      "#D69E2E",
                      "#38A169",
                      "#319795",
                      "#3182CE",
                      "#2B6CB0",
                      "#6B46C1",
                      "#B83280",
                    ].map((c) => (
                      <Box
                        key={c}
                        w="22px"
                        h="22px"
                        borderRadius="4px"
                        border="1px solid"
                        borderColor={pendingColor.toLowerCase() === c.toLowerCase() ? "blue.400" : "gray.200"}
                        bg={c}
                        cursor="pointer"
                        onClick={() => setPendingColor(c)}
                      />
                    ))}
                  </Flex>
                  <Flex align="center" gap={2} mt={3}>
                    <Box w="22px" h="22px" borderRadius="4px" border="1px solid" borderColor="gray.200" bg={pendingColor} />
                    <Input
                      size="sm"
                      value={pendingColor}
                      onChange={(e) => setPendingColor(String(e.target.value || ""))}
                    />
                    <Button
                      size="sm"
                      bg="primary.600"
                      color="white"
                      _hover={{ bg: "primary.700" }}
                      _active={{ bg: "primary.800" }}
                      onClick={() => {
                        const raw = String(pendingColor || "").trim();
                        const v = raw.startsWith("#") ? raw : `#${raw}`;
                        if (!/^#[0-9a-fA-F]{6}$/.test(v)) return;
                        applyColor(v);
                        setColorOpen(false);
                      }}
                    >
                      确定
                    </Button>
                    <Button size="sm" variant="ghost" onClick={() => setColorOpen(false)}>
                      取消
                    </Button>
                  </Flex>
                </PopoverBody>
              </PopoverContent>
            </Popover>
            <Button
              size="sm"
              bg="primary.600"
              color="white"
              _hover={{ bg: "primary.700" }}
              _active={{ bg: "primary.800" }}
              onClick={onUploadImageClick}
              isDisabled={disabled}
            >
              插入图片
            </Button>
            <Popover
              isOpen={tableOpen}
              onOpen={() => {
                if (disabled) return;
                setTableOpen(true);
              }}
              onClose={() => setTableOpen(false)}
              placement="bottom"
            >
              <PopoverTrigger>
                <Button
                  size="sm"
                  bg="primary.600"
                  color="white"
                  _hover={{ bg: "primary.700" }}
                  _active={{ bg: "primary.800" }}
                  isDisabled={disabled}
                  leftIcon={<FiGrid />}
                >
                  插入表格
                </Button>
              </PopoverTrigger>
              <PopoverContent w="260px">
                <PopoverArrow />
                <PopoverBody>
                  <Flex direction="column" gap={3}>
                    <Flex align="center" justify="space-between" gap={3}>
                      <Text fontSize="sm" color="gray.600">
                        行数
                      </Text>
                      <NumberInput size="sm" value={tableRows} min={1} max={20} onChange={(v: any) => setTableRows(Number(v || 1))}>
                        <NumberInputField />
                        <NumberInputStepper>
                          <NumberIncrementStepper />
                          <NumberDecrementStepper />
                        </NumberInputStepper>
                      </NumberInput>
                    </Flex>
                    <Flex align="center" justify="space-between" gap={3}>
                      <Text fontSize="sm" color="gray.600">
                        列数
                      </Text>
                      <NumberInput size="sm" value={tableCols} min={1} max={20} onChange={(v: any) => setTableCols(Number(v || 1))}>
                        <NumberInputField />
                        <NumberInputStepper>
                          <NumberIncrementStepper />
                          <NumberDecrementStepper />
                        </NumberInputStepper>
                      </NumberInput>
                    </Flex>
                    <Flex justify="flex-end" gap={2}>
                      <Button size="sm" variant="ghost" onClick={() => setTableOpen(false)}>
                        取消
                      </Button>
                      <Button
                        size="sm"
                        bg="primary.600"
                        color="white"
                        _hover={{ bg: "primary.700" }}
                        _active={{ bg: "primary.800" }}
                        onClick={onInsertTableConfirm}
                      >
                        确定
                      </Button>
                    </Flex>
                  </Flex>
                </PopoverBody>
              </PopoverContent>
            </Popover>
            <Button
              size="sm"
              bg="gray.600"
              color="white"
              _hover={{ bg: "gray.700" }}
              _active={{ bg: "gray.800" }}
              onClick={() => editor && editor.commands.setHardBreak()}
              isDisabled={disabled}
            >
              换行
            </Button>
            <Button
              size="sm"
              bg="neutral.700"
              color="white"
              _hover={{ bg: "neutral.r85" }}
              _active={{ bg: "gray.900" }}
              onClick={onClearFormat}
              isDisabled={disabled || !hasSelection}
            >
              清除格式
            </Button>
            <Button
              size="sm"
              bg="blue.500"
              color="white"
              _hover={{ bg: "primary.700" }}
              _active={{ bg: "primary.800" }}
              onClick={() => onSave && onSave()}
              isDisabled={disabled}
            >
              保存
            </Button>
            <Input type="file" ref={fileInputRef} onChange={onFileChange} display="none" />
          </Flex>
          {(connected || streamTotal > 0) && (
            <Flex align="center" gap={2} w="50%" pt={1}>
              <Text fontSize="sm" color="gray.600">文档解析：</Text>
              {streaming ? (
                <Box flex="1" h="6px" bg="gray.200" borderRadius="4px" overflow="hidden">
                  <Box h="100%" w={`${streamPercent}%`} bg="G.200" transition="width 0.2s ease" />
                </Box>
              ) : (
                <Text fontSize="sm" color="G.500">已连接</Text>
              )}
              {streaming ? (
                <Text fontSize="sm" color="gray.600">{streamProgress}/{streamTotal}（{streamPercent}%）</Text>
              ) : null}
            </Flex>
          )}
        </Flex>
      )}
      <Box
        flex="1"
        overflow="auto"
        bg="#FFFFFF"
        p={3}
        position="relative"
        onContextMenu={onContextMenu}
        sx={{
          ".tiptap .ProseMirror": {
            outline: "none",
            whiteSpace: "pre-wrap",
          },
          ".tiptap .ProseMirror h1": { fontSize: "1.875rem", fontWeight: 700, lineHeight: "2.25rem" },
          ".tiptap .ProseMirror h2": { fontSize: "1.5rem", fontWeight: 700, lineHeight: "2rem" },
          ".tiptap .ProseMirror h3": { fontSize: "1.25rem", fontWeight: 700, lineHeight: "1.75rem" },
          ".tiptap .ProseMirror h4": { fontSize: "1.125rem", fontWeight: 700, lineHeight: "1.5rem" },
          ".tiptap .ProseMirror h5": { fontSize: "1rem", fontWeight: 700, lineHeight: "1.5rem" },
          ".tiptap .ProseMirror h6": { fontSize: "0.875rem", fontWeight: 700, lineHeight: "1.25rem" },
          ".tiptap .ProseMirror table": { borderCollapse: "collapse", width: "100%" },
          ".tiptap .ProseMirror td, .tiptap .ProseMirror th": {
            border: "1px solid #000",
            padding: "4px",
          },
          ".tiptap .ProseMirror img": { maxWidth: "100%", height: "auto" },
        }}
      >
        <EditorContent editor={editor} className="tiptap" />
        {!ready && (
          <Flex
            position="absolute"
            inset={0}
            align="center"
            justify="center"
            bg="rgba(255,255,255,0.75)"
          >
            <Spinner mr={2} />
            <Text fontSize="sm">加载中...</Text>
          </Flex>
        )}
        {menuOpen && (
          <Box
            data-editor-context-menu
            position="fixed"
            left={`${menuPos.x}px`}
            top={`${menuPos.y}px`}
            bg="#FFFFFF"
            border="1px solid #E0E0E0"
            borderRadius="6px"
            boxShadow="0px 8px 16px 0px rgba(0,0,0,0.08)"
            zIndex={1000}
            minW="140px"
            overflow="hidden"
          >
            {contextMenuItems?.map((it) => (
              <Button
                key={it.key}
                variant="ghost"
                justifyContent="flex-start"
                w="100%"
                borderRadius={0}
                size="sm"
                isDisabled={Boolean(it.disabled) || menuBusy}
                onClick={() => handleMenuItem(it)}
              >
                {menuBusy ? <Spinner size="xs" mr={2} /> : null}
                {it.label}
              </Button>
            ))}
          </Box>
        )}
      </Box>
    </Flex>
  );
});

export default Editor;
