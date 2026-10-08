"use client";

/* eslint-disable no-unused-vars */
/* Hallmark · component: outline-tree · genre: modern-minimal · theme: chakra-smart-bid (preserved tokens)
 * states: default · hover · focus-visible · active · disabled · drag · scroll · truncation-tooltip
 * contrast: pass (neutral.400 on white, primary.600 on neutral.50)
 */
import { useCallback, useLayoutEffect, useMemo, useRef, useState } from "react";
import type { DragEvent, ReactNode } from "react";
import {
  Box,
  Flex,
  IconButton,
  Input,
  InputGroup,
  InputLeftElement,
  InputRightElement,
  Menu,
  MenuButton,
  MenuDivider,
  MenuItem,
  MenuList,
  Portal,
  Text,
  Tooltip,
} from "@chakra-ui/react";
import {
  FiChevronRight,
  FiChevronDown,
  FiMoreVertical,
  FiEdit3,
  FiTrash2,
  FiPlus,
  FiArrowUp,
  FiArrowDown,
  FiSearch,
  FiX,
  FiZap,
  FiRotateCcw,
  FiZoomIn,
  FiZoomOut,
  FiCheckCircle,
  FiCornerUpLeft,
} from "react-icons/fi";
import { LuPanelLeftClose, LuGripVertical, LuSparkles } from "react-icons/lu";
import { filterOutlineTree } from "./outline-tree-utils";
import { isDocumentRootOutline, type BidGenOutlineNode } from "./types";
import type { GenMode } from "@/service/bid-gen";

type Props = {
  nodes: BidGenOutlineNode[];
  activeId?: number;
  readOnly?: boolean;
  variant?: "nav" | "blueprint";
  onJump: (outlineId: number) => void;
  onAdd?: (parentId: number, title: string, level: number) => void;
  onRename?: (id: number, title: string) => void;
  onDelete?: (id: number) => void;
  onCollapse?: () => void;
  /** 拖拽排序/跨级移动：nextId=0 表示追加到 targetParentId 末尾 */
  onMove?: (dragId: number, targetParentId: number, nextId: number) => void;
  /** 是否开启搜索目录功能（仅详情页大纲导航开启，蓝图确认页不开启） */
  searchable?: boolean;
  /** 章节级 AI 操作：write=生成本章/重新生成（直接触发），rewrite/expand/condense=打开弹层 */
  onChapterAiAction?: (outlineId: number, mode: GenMode) => void;
  /** 人工标记章节写作完成/取消完成（completed=true 置为已完成，false 回退待生成） */
  onToggleComplete?: (outlineId: number, completed: boolean) => void;
};

// 树形缩进常量：8px/级；chevron 槽 12px + gap 4px = 标题文字相对行容器左偏移 16px。
// 行渲染、重命名输入框、新增输入框共用 titleLeft(depth)，保证输入框与最终渲染位置像素级对齐。
const LEVEL_INDENT = 8;
const TITLE_LEFT_OFFSET = 16;

const isDocumentRoot = isDocumentRootOutline;

const GEN_STATUS: Record<string, { label: string; color: string; bg: string }> =
  {
    pending: { label: "待生成", color: "gray.400", bg: "gray.50" },
    generating: { label: "生成中", color: "gold.600", bg: "gold.50" },
    succeeded: { label: "已生成", color: "success.600", bg: "success.50" },
    failed: { label: "失败", color: "error.600", bg: "error.50" },
  };

// 蓝图类型标记：必须项 / AI 建议项 / 可选项（两者皆非）
function typeMarker(n: BidGenOutlineNode): { color: string; label: string } {
  if (n.isRequiredFile) return { color: "gold.500", label: "必须项" };
  if (n.isAiSuggested) return { color: "primary.500", label: "AI 建议项" };
  return { color: "gray.400", label: "可选项" };
}

function titleWeight(genStatus: string, isTop: boolean): number {
  if (genStatus === "succeeded") return isTop ? 600 : 500;
  return isTop ? 500 : 400;
}

function Legend({ color, label }: { color: string; label: string }) {
  return (
    <Flex align="center" gap={1}>
      <Box w="1.5" h="1.5" borderRadius="full" bg={color} />
      {label}
    </Flex>
  );
}

// 新增/重命名输入框统一交互态
const inputSx = {
  bg: "white",
  borderColor: "gray.200",
  _hover: { borderColor: "gray.300" },
  _focusVisible: {
    boxShadow: "0 0 0 2px var(--chakra-colors-primary-400)",
    borderColor: "primary.400",
    outline: "none",
  },
  _disabled: { bg: "gray.50", color: "gray.400", cursor: "not-allowed" },
};

// 构建单行拖拽预览幽灵图：默认浏览器快照会带上整棵子树，
// 这里只显示被拖的单个标题行（手柄 + 标题 + 子章节数提示）
function buildDragGhost(title: string, kidCount: number): HTMLElement {
  const ghost = document.createElement("div");
  ghost.style.cssText =
    "position:fixed;left:-9999px;top:0;display:flex;align-items:center;gap:6px;" +
    "padding:6px 12px;background:#fff;border:1px solid var(--chakra-colors-primary-200);" +
    "border-radius:8px;box-shadow:0 8px 24px rgba(30,58,95,0.18);" +
    "font-size:13px;color:#1A202C;white-space:nowrap;max-width:260px;overflow:hidden;";
  const grip = document.createElement("span");
  grip.textContent = "⋮⋮";
  grip.style.cssText = "color:#A0AEC0;font-size:12px;flex-shrink:0;";
  const label = document.createElement("span");
  label.textContent = title;
  label.style.cssText = "overflow:hidden;text-overflow:ellipsis;";
  ghost.appendChild(grip);
  ghost.appendChild(label);
  if (kidCount > 0) {
    const badge = document.createElement("span");
    badge.textContent = `\u00b7 ${kidCount} 个子章节`;
    badge.style.cssText = "color:#718096;font-size:12px;flex-shrink:0;";
    ghost.appendChild(badge);
  }
  return ghost;
}

// 目录搜索命中标题高亮：把关键词片段用 mark 高亮（gold 底 primary 字，与章节状态点同系）
function renderHighlightedTitle(title: string, query: string): ReactNode[] {
  const q = query.toLowerCase();
  const lower = title.toLowerCase();
  const parts: ReactNode[] = [];
  let idx = 0;
  let pos = lower.indexOf(q);
  while (pos >= 0) {
    if (pos > idx) parts.push(title.slice(idx, pos));
    parts.push(
      <Box
        key={pos}
        as="mark"
        bg="gold.200"
        color="primary.800"
        px="0.5"
        borderRadius="sm"
      >
        {title.slice(pos, pos + query.length)}
      </Box>,
    );
    idx = pos + query.length;
    pos = lower.indexOf(q, idx);
  }
  if (idx < title.length) parts.push(title.slice(idx));
  return parts;
}

// 章节标题：单行省略 + 仅在被截断时用气泡展示全称（侧栏调宽后自动重算）
function OutlineTitle({
  title,
  children,
  fontSize,
  color,
  fontWeight,
}: {
  title: string;
  children: ReactNode;
  fontSize?: string;
  color?: string;
  fontWeight?: number;
}) {
  const textRef = useRef<HTMLParagraphElement>(null);
  const [truncated, setTruncated] = useState(false);

  const update = useCallback(() => {
    const el = textRef.current;
    if (!el) return;
    setTruncated(el.scrollWidth > el.clientWidth + 1);
  }, []);

  useLayoutEffect(() => {
    update();
    const el = textRef.current;
    if (!el) return undefined;
    if (typeof ResizeObserver !== "undefined") {
      const ro = new ResizeObserver(update);
      ro.observe(el);
      return () => ro.disconnect();
    }
    window.addEventListener("resize", update);
    return () => window.removeEventListener("resize", update);
  }, [update]);

  const text = (
    <Text
      ref={textRef}
      fontSize={fontSize}
      color={color}
      fontWeight={fontWeight}
      isTruncated
    >
      {children}
    </Text>
  );

  if (!truncated) return text;
  return (
    <Tooltip label={title} openDelay={800} hasArrow placement="right">
      {text}
    </Tooltip>
  );
}

export default function OutlineNav({
  nodes,
  activeId,
  readOnly,
  variant = "nav",
  onJump,
  onAdd,
  onRename,
  onDelete,
  onCollapse,
  onMove,
  searchable,
  onChapterAiAction,
  onToggleComplete,
}: Props) {
  const [collapsed, setCollapsed] = useState<Set<number>>(new Set());
  const [editingId, setEditingId] = useState<number | null>(null);
  const [editText, setEditText] = useState("");
  const [addParentId, setAddParentId] = useState<number | null>(null);
  const [addText, setAddText] = useState("");
  // 拖拽状态：dragId 当前拖拽节点；dropTarget 落点（before/after/child）
  const [dragId, setDragId] = useState<number | null>(null);
  const [dropTarget, setDropTarget] = useState<{
    id: number;
    zone: "before" | "after" | "child";
  } | null>(null);

  const isBlueprint = variant === "blueprint";

  // ===== 目录搜索：按标题模糊筛选，保留祖先层级 + 命中高亮 + 搜索态只读 =====
  const [searchQuery, setSearchQuery] = useState("");
  const searchMode = searchable && searchQuery.trim().length > 0;
  const searchFilter = useMemo(
    () => (searchable ? filterOutlineTree(nodes, searchQuery) : null),
    [nodes, searchQuery, searchable],
  );
  const searchEmpty =
    searchMode && searchFilter ? searchFilter.visibleIds.size === 0 : false;

  // 有子章节的节点：标记完成 / AI 写作都会连带整棵子树，菜单文案同步说明
  const hasChildrenIds = useMemo(
    () => new Set(nodes.filter((n) => n.parentId > 0).map((n) => n.parentId)),
    [nodes],
  );

  const toggleCollapse = (id: number) => {
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  // 同级内上移/下移（键盘可访问的拖拽兜底）
  const moveSibling = useCallback(
    (n: BidGenOutlineNode, dir: -1 | 1) => {
      if (!onMove) return;
      const siblings = nodes
        .filter((x) => x.parentId === n.parentId)
        .sort((a, b) => a.sortOrder - b.sortOrder);
      const idx = siblings.findIndex((x) => x.id === n.id);
      if (idx < 0) return;
      const to = idx + dir;
      if (to < 0 || to >= siblings.length) return;
      if (dir === -1) {
        onMove(n.id, n.parentId, siblings[idx - 1].id);
      } else {
        onMove(
          n.id,
          n.parentId,
          idx + 2 < siblings.length ? siblings[idx + 2].id : 0,
        );
      }
    },
    [nodes, onMove],
  );

  // 拖拽落点解析：before/after → 同级内插入；child → 作为 target 子节点追加
  const handleDrop = useCallback(
    (
      e: DragEvent<HTMLDivElement>,
      targetId: number,
      zone: "before" | "after" | "child",
    ) => {
      e.preventDefault();
      e.stopPropagation();
      const dragIdNum: number =
        Number(e.dataTransfer.getData("text/plain")) || dragId || 0;
      const target = nodes.find((n) => n.id === targetId);
      setDragId(null);
      setDropTarget(null);
      if (!onMove || !target || dragIdNum <= 0 || dragIdNum === targetId)
        return;
      if (zone === "child") {
        onMove(dragIdNum, targetId, 0);
        return;
      }
      const siblings = nodes
        .filter((n) => n.parentId === target.parentId)
        .sort((a, b) => a.sortOrder - b.sortOrder);
      const idx = siblings.findIndex((n) => n.id === targetId);
      let nextId = targetId;
      if (zone === "after") {
        nextId =
          idx >= 0 && idx + 1 < siblings.length ? siblings[idx + 1].id : 0;
      }
      onMove(dragIdNum, target.parentId, nextId);
    },
    [nodes, onMove, dragId],
  );

  const tree = useMemo(() => {
    const childrenOf = (pid: number) =>
      nodes
        .filter((n) => n.parentId === pid)
        .sort((a, b) => a.sortOrder - b.sortOrder);
    const render = (pid: number, depth: number): any[] => {
      const rows = childrenOf(pid);
      // 仅搜索态（query 非空）才按 visibleIds 过滤；
      // searchFilter 在 query 为空时是空集合对象，不能当无过滤用，否则整棵树被清空。
      const visible =
        searchMode && searchFilter
          ? rows.filter((n) => searchFilter.visibleIds.has(n.id))
          : rows;
      return visible.map((n) => {
        const kids = render(n.id, depth + 1);
        const isCollapsed = !searchMode && collapsed.has(n.id);
        const hasKids = kids.length > 0;
        const isTop = depth === 0;
        const titleColor = isTop ? "gray.700" : "gray.600";
        const weight = titleWeight(n.genStatus, isTop);
        const st = GEN_STATUS[n.genStatus] || GEN_STATUS.pending;
        const tm = typeMarker(n);
        const marker = isBlueprint
          ? { color: tm.color, label: tm.label }
          : isDocumentRoot(n)
            ? { color: "primary.500", label: "目录根节点" }
            : { color: st.color, label: st.label };
        const isDrop = dropTarget?.id === n.id;
        const showBefore = isDrop && dropTarget.zone === "before";
        const showAfter = isDrop && dropTarget.zone === "after";
        const showChild = isDrop && dropTarget.zone === "child";
        const draggable =
          !searchMode && !!onMove && !readOnly && editingId !== n.id;
        let chevronLabel: string | undefined;
        if (hasKids) {
          chevronLabel = isCollapsed ? "展开子章节" : "收起子章节";
        }
        // 章节写作单元 = 整棵子树：含子章节时标记完成 / AI 写作都会连带子章节，文案同步说明
        const subtreeSuffix = hasChildrenIds.has(n.id) ? "（含子章节）" : "";
        const isDone = n.genStatus === "succeeded";
        const completeLabel = `${isDone ? "取消已完成" : "设为已完成"}${subtreeSuffix}`;
        const generateLabel = `${isDone ? "重新生成" : "生成本章"}${subtreeSuffix}`;
        return (
          <Box key={n.id} w="100%">
            <Box position="relative">
              <Flex
                align="center"
                gap={1}
                py={1}
                px={2}
                borderRadius="md"
                cursor="pointer"
                ml={depth * LEVEL_INDENT}
                role="group"
                bg={
                  activeId === n.id || showChild ? "primary.50" : "transparent"
                }
                boxShadow={
                  showChild
                    ? "inset 0 0 0 1.5px var(--chakra-colors-primary-400)"
                    : undefined
                }
                opacity={dragId === n.id ? 0.35 : 1}
                style={{
                  outline:
                    dragId === n.id
                      ? "1.5px dashed var(--chakra-colors-primary-400)"
                      : undefined,
                  outlineOffset: -1,
                  transition: "opacity 0.15s ease",
                }}
                _hover={{ bg: activeId === n.id ? "primary.50" : "gray.100" }}
                onClick={() => onJump(n.id)}
                onDragOver={(e) => {
                  if (!onMove || dragId === null) return;
                  e.preventDefault();
                  e.dataTransfer.dropEffect = "move";
                  const rect = e.currentTarget.getBoundingClientRect();
                  const y = e.clientY - rect.top;
                  let zone: "before" | "after" | "child" = "child";
                  if (y < rect.height / 3) zone = "before";
                  else if (y > (rect.height * 2) / 3) zone = "after";
                  if (dropTarget?.id !== n.id || dropTarget?.zone !== zone) {
                    setDropTarget({ id: n.id, zone });
                  }
                }}
                onDragLeave={() => {
                  if (dropTarget?.id === n.id) setDropTarget(null);
                }}
                onDrop={(e) => handleDrop(e, n.id, dropTarget?.zone || "child")}
              >
                <Box
                  w="5"
                  h="5"
                  flexShrink={0}
                  display="flex"
                  alignItems="center"
                  justifyContent="center"
                  borderRadius="md"
                  cursor={hasKids ? "pointer" : "default"}
                  color="gray.400"
                  role={hasKids ? "button" : undefined}
                  tabIndex={hasKids ? 0 : -1}
                  aria-label={chevronLabel}
                  _hover={
                    hasKids
                      ? { color: "gray.600", bg: "neutral.100" }
                      : undefined
                  }
                  _active={hasKids ? { transform: "scale(0.92)" } : undefined}
                  _focusVisible={{
                    boxShadow: "0 0 0 2px var(--chakra-colors-primary-400)",
                    outline: "none",
                  }}
                  onClick={(e) => {
                    if (hasKids) {
                      e.stopPropagation();
                      toggleCollapse(n.id);
                    }
                  }}
                  onKeyDown={(e) => {
                    if (hasKids && (e.key === "Enter" || e.key === " ")) {
                      e.preventDefault();
                      e.stopPropagation();
                      toggleCollapse(n.id);
                    }
                  }}
                >
                  {hasKids &&
                    (isCollapsed ? (
                      <FiChevronRight size="13" color="#A0AEC0" />
                    ) : (
                      <FiChevronDown size="13" color="#A0AEC0" />
                    ))}
                </Box>
                {draggable && (
                  <Box
                    flexShrink={0}
                    w="5"
                    h="5"
                    display="flex"
                    alignItems="center"
                    justifyContent="center"
                    borderRadius="md"
                    cursor="grab"
                    color="neutral.400"
                    fontSize="xs"
                    draggable
                    title="拖拽调整章节顺序"
                    aria-label="拖拽调整章节顺序"
                    _hover={{ color: "primary.600", bg: "neutral.100" }}
                    _active={{
                      cursor: "grabbing",
                      color: "primary.700",
                      bg: "primary.100",
                    }}
                    _focusVisible={{
                      boxShadow: "0 0 0 2px var(--chakra-colors-primary-400)",
                      outline: "none",
                    }}
                    onDragStart={(e) => {
                      e.dataTransfer.setData("text/plain", String(n.id));
                      e.dataTransfer.effectAllowed = "move";
                      setDragId(n.id);
                      // 自定义单行拖拽预览（只显示被拖的标题行，不显示整棵子树）
                      const ghost = buildDragGhost(n.title, kids.length);
                      document.body.appendChild(ghost);
                      e.dataTransfer.setDragImage(ghost, 12, 12);
                      window.setTimeout(() => {
                        if (ghost.parentNode)
                          ghost.parentNode.removeChild(ghost);
                      }, 0);
                    }}
                    onDragEnd={() => {
                      setDragId(null);
                      setDropTarget(null);
                    }}
                  >
                    <LuGripVertical />
                  </Box>
                )}
                <Box flex="1" minW="0" maxW="560px">
                  {editingId === n.id ? (
                    <Input
                      size="xs"
                      value={editText}
                      autoFocus
                      fontSize={isTop ? "sm" : "xs"}
                      sx={inputSx}
                      onChange={(e) => setEditText(e.target.value)}
                      onBlur={() => {
                        if (editText.trim() && onRename)
                          onRename(n.id, editText.trim());
                        setEditingId(null);
                      }}
                      onKeyDown={(e) => {
                        if (e.key === "Enter")
                          (e.target as HTMLInputElement).blur();
                        if (e.key === "Escape") setEditingId(null);
                      }}
                      onClick={(e) => e.stopPropagation()}
                    />
                  ) : (
                    <OutlineTitle
                      title={n.title}
                      fontSize={isTop ? "sm" : "xs"}
                      color={titleColor}
                      fontWeight={weight}
                    >
                      {searchMode && searchFilter?.matchedIds.has(n.id)
                        ? renderHighlightedTitle(n.title, searchQuery.trim())
                        : n.title}
                    </OutlineTitle>
                  )}
                </Box>
                {/* 固定右侧操作列：状态点 + 节点菜单不随标题横向滑动 */}
                <Flex
                  position="sticky"
                  right={0}
                  zIndex={2}
                  align="center"
                  gap={1}
                  pl={2}
                  flexShrink={0}
                  bg={activeId === n.id || showChild ? "primary.50" : "white"}
                  _groupHover={{
                    bg:
                      activeId === n.id || showChild ? "primary.50" : "gray.100",
                  }}
                  borderLeft="1px solid"
                  borderColor="gray.100"
                >
                  <Tooltip label={marker.label}>
                    <Box
                      w="2"
                      h="2"
                      borderRadius="full"
                      bg={marker.color}
                      flexShrink={0}
                    />
                  </Tooltip>
                  {!readOnly && !searchMode && (
                    <Menu placement="right-start">
                      <MenuButton
                        as={IconButton}
                        aria-label="节点操作"
                        size="xs"
                        variant="ghost"
                        icon={<FiMoreVertical size="13" />}
                        onClick={(e: any) => e.stopPropagation()}
                      />
                      <Portal>
                      <MenuList minW="150px">
                        {/* 章节级 AI 操作 */}
                        {onChapterAiAction &&
                          !isDocumentRoot(n) &&
                          (n.genStatus === "pending" || n.genStatus === "failed") && (
                            <MenuItem
                              icon={<FiZap />}
                              fontSize="sm"
                              color="primary.600"
                              onClick={(e) => {
                                e.stopPropagation();
                                onChapterAiAction(n.id, "write");
                              }}
                            >
                              {generateLabel}
                            </MenuItem>
                          )}
                        {onChapterAiAction &&
                          !isDocumentRoot(n) &&
                          n.genStatus === "succeeded" && (
                          <MenuItem
                            icon={<FiRotateCcw />}
                            fontSize="sm"
                            color="primary.600"
                            onClick={(e) => {
                              e.stopPropagation();
                              onChapterAiAction(n.id, "write");
                            }}
                          >
                            {generateLabel}
                          </MenuItem>
                        )}
                        {onChapterAiAction &&
                          !isDocumentRoot(n) &&
                          n.genStatus === "succeeded" && (
                          <Menu placement="right-start">
                            <MenuButton
                              as={MenuItem}
                              icon={<LuSparkles />}
                              fontSize="sm"
                              onClick={(e: any) => e.stopPropagation()}
                            >
                              AI 改写
                            </MenuButton>
                            <Portal>
                            <MenuList minW="130px">
                              <MenuItem
                                icon={<FiRotateCcw />}
                                fontSize="sm"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  onChapterAiAction(n.id, "rewrite");
                                }}
                              >
                                重写本章
                              </MenuItem>
                              <MenuItem
                                icon={<FiZoomIn />}
                                fontSize="sm"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  onChapterAiAction(n.id, "expand");
                                }}
                              >
                                扩写
                              </MenuItem>
                              <MenuItem
                                icon={<FiZoomOut />}
                                fontSize="sm"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  onChapterAiAction(n.id, "condense");
                                }}
                              >
                                缩写
                              </MenuItem>
                            </MenuList>
                            </Portal>
                          </Menu>
                        )}
                        {onChapterAiAction && !isDocumentRoot(n) && (
                          <>
                            <MenuDivider />
                          </>
                        )}
                        {/* 人工写作完成标记：不经过 AI 生成的章节由此置位，方可计入整体完成度 */}
                        {onToggleComplete && !isDocumentRoot(n) && (
                          <>
                            <MenuItem
                              icon={
                                n.genStatus === "succeeded" ? (
                                  <FiCornerUpLeft />
                                ) : (
                                  <FiCheckCircle />
                                )
                              }
                              fontSize="sm"
                              color={
                                n.genStatus === "succeeded"
                                  ? "neutral.500"
                                  : "success.600"
                              }
                              onClick={(e) => {
                                e.stopPropagation();
                                onToggleComplete(
                                  n.id,
                                  n.genStatus !== "succeeded",
                                );
                              }}
                            >
                              {completeLabel}
                            </MenuItem>
                            <MenuDivider />
                          </>
                        )}
                        <MenuItem
                          icon={<FiEdit3 />}
                          fontSize="sm"
                          onClick={(e) => {
                            e.stopPropagation();
                            setEditingId(n.id);
                            setEditText(n.title);
                          }}
                        >
                          重命名
                        </MenuItem>
                        <MenuItem
                          icon={<FiPlus />}
                          fontSize="sm"
                          onClick={(e) => {
                            e.stopPropagation();
                            setAddParentId(n.id);
                            setAddText("");
                          }}
                        >
                          添加子章节
                        </MenuItem>
                        {onMove && (
                          <>
                            <MenuItem
                              icon={<FiArrowUp />}
                              fontSize="sm"
                              onClick={(e) => {
                                e.stopPropagation();
                                moveSibling(n, -1);
                              }}
                            >
                              上移
                            </MenuItem>
                            <MenuItem
                              icon={<FiArrowDown />}
                              fontSize="sm"
                              onClick={(e) => {
                                e.stopPropagation();
                                moveSibling(n, 1);
                              }}
                            >
                              下移
                            </MenuItem>
                          </>
                        )}
                        <MenuItem
                          icon={<FiTrash2 />}
                          fontSize="sm"
                          color="error.500"
                          onClick={(e) => {
                            e.stopPropagation();
                            onDelete?.(n.id);
                          }}
                        >
                          删除章节
                        </MenuItem>
                      </MenuList>
                      </Portal>
                    </Menu>
                  )}
                </Flex>
              </Flex>
              {/* 拖拽落点指示线 */}
              {showBefore && (
                <Box
                  position="absolute"
                  top="-1px"
                  left={depth * LEVEL_INDENT + 6}
                  right={2}
                  h="2px"
                  borderRadius="full"
                  bg="primary.400"
                  pointerEvents="none"
                />
              )}
              {showAfter && (
                <Box
                  position="absolute"
                  bottom="-1px"
                  left={depth * LEVEL_INDENT + 6}
                  right={2}
                  h="2px"
                  borderRadius="full"
                  bg="primary.400"
                  pointerEvents="none"
                />
              )}
            </Box>
            {addParentId === n.id && (
              <Flex
                align="center"
                gap={1}
                ml={(depth + 1) * LEVEL_INDENT + TITLE_LEFT_OFFSET}
                py={1}
              >
                <Input
                  size="xs"
                  placeholder="子章节标题"
                  value={addText}
                  autoFocus
                  sx={inputSx}
                  onChange={(e) => setAddText(e.target.value)}
                  onBlur={() => {
                    if (addText.trim() && onAdd)
                      onAdd(n.id, addText.trim(), Math.min(4, n.level + 1));
                    setAddParentId(null);
                  }}
                  onKeyDown={(e) => {
                    if (e.key === "Enter")
                      (e.target as HTMLInputElement).blur();
                    if (e.key === "Escape") setAddParentId(null);
                  }}
                  onClick={(e) => e.stopPropagation()}
                />
              </Flex>
            )}
            {hasKids && !isCollapsed && kids}
          </Box>
        );
      });
    };
    return render(0, 0);
  }, [
    nodes,
    collapsed,
    activeId,
    readOnly,
    isBlueprint,
    editingId,
    editText,
    addParentId,
    addText,
    onJump,
    onAdd,
    onRename,
    onDelete,
    onMove,
    dragId,
    dropTarget,
    handleDrop,
    moveSibling,
    onChapterAiAction,
    onToggleComplete,
    hasChildrenIds,
    searchMode,
    searchFilter,
    searchQuery,
  ]);

  return (
    <Flex direction="column" h="100%">
      <Flex
        align="center"
        justify="space-between"
        px={3}
        py={2}
        borderBottom="1px solid"
        borderColor="gray.100"
      >
        <Flex align="center" gap={0.5}>
          <Text
            fontSize="xs"
            fontWeight="700"
            color="gray.500"
            letterSpacing="0.05em"
          >
            {isBlueprint ? "大纲蓝图" : "大纲导航"}
          </Text>
          {!readOnly && !searchMode && (
            <Tooltip label="添加顶层章节">
              <IconButton
                aria-label="添加顶层章节"
                size="xs"
                variant="ghost"
                color="gray.400"
                icon={<FiPlus size="14" />}
                _hover={{ color: "primary.600", bg: "primary.50" }}
                _active={{ bg: "primary.100", transform: "scale(0.95)" }}
                _focusVisible={{
                  boxShadow: "0 0 0 2px var(--chakra-colors-primary-400)",
                  outline: "none",
                }}
                onClick={() => {
                  setAddParentId(0);
                  setAddText("");
                }}
              />
            </Tooltip>
          )}
        </Flex>
        {onCollapse && (
          <Tooltip label="收起大纲">
            <IconButton
              aria-label="收起大纲"
              size="xs"
              variant="ghost"
              color="gray.400"
              icon={<LuPanelLeftClose size="15" />}
              _hover={{ color: "gray.600" }}
              onClick={onCollapse}
            />
          </Tooltip>
        )}
      </Flex>

      {/* 目录搜索框 */}
      {searchable && (
        <Box px={2} py={2} borderBottom="1px solid" borderColor="gray.100">
          <InputGroup size="sm">
            <InputLeftElement pointerEvents="none" color="gray.400">
              <FiSearch size="14" />
            </InputLeftElement>
            <Input
              placeholder="搜索目录标题…"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Escape") setSearchQuery("");
              }}
              bg="gray.50"
              borderRadius="md"
              borderColor="gray.200"
              _hover={{ borderColor: "gray.300", bg: "white" }}
              _focusVisible={{
                boxShadow: "0 0 0 2px var(--chakra-colors-primary-400)",
                borderColor: "primary.400",
                outline: "none",
                bg: "white",
              }}
              _disabled={{
                bg: "gray.100",
                color: "gray.400",
                cursor: "not-allowed",
              }}
            />
            {searchQuery && (
              <InputRightElement>
                <IconButton
                  aria-label="清除搜索"
                  size="xs"
                  variant="ghost"
                  color="gray.400"
                  icon={<FiX size="14" />}
                  _hover={{ color: "gray.600" }}
                  _focusVisible={{
                    boxShadow: "0 0 0 2px var(--chakra-colors-primary-400)",
                    outline: "none",
                  }}
                  onClick={() => setSearchQuery("")}
                />
              </InputRightElement>
            )}
          </InputGroup>
        </Box>
      )}

      <Box flex="1" overflow="auto" className="thin-scrollbars" px={2} py={2}>
        {/* 内容容器：width:max-content 让横向滚动范围精确等于最长目录名宽度；min-width:100% 保证短目录时铺满 */}
        <Box w="max-content" minW="100%">
        {/* 顶层新增输入框：置顶显示，与最终渲染位置（depth 0 标题文字）对齐 */}
        {addParentId === 0 && !searchMode && (
          <Flex align="center" gap={1} ml={TITLE_LEFT_OFFSET} py={1}>
            <Input
              size="xs"
              placeholder="顶层章节标题"
              value={addText}
              autoFocus
              fontSize="sm"
              sx={inputSx}
              onChange={(e) => setAddText(e.target.value)}
              onBlur={() => {
                if (addText.trim() && onAdd) onAdd(0, addText.trim(), 1);
                setAddParentId(null);
              }}
              onKeyDown={(e) => {
                if (e.key === "Enter") (e.target as HTMLInputElement).blur();
                if (e.key === "Escape") setAddParentId(null);
              }}
            />
          </Flex>
        )}
        {nodes.length === 0 && addParentId === 0 && (
          <Flex align="center" justify="center" py={8}>
            <Text fontSize="xs" color="gray.400" textAlign="center">
              暂无大纲，输入标题后回车即可添加新章节
            </Text>
          </Flex>
        )}
        {nodes.length === 0 && addParentId !== 0 && (
          <Flex
            direction="column"
            align="center"
            justify="center"
            h="full"
            gap={3}
            py={10}
          >
            <IconButton
              aria-label="添加章节"
              size="md"
              variant="outline"
              borderRadius="full"
              borderColor="gray.200"
              color="gray.400"
              icon={<FiPlus size="18" />}
              _hover={{
                borderColor: "primary.400",
                color: "primary.600",
                bg: "primary.50",
              }}
              _active={{ bg: "primary.100", transform: "scale(0.95)" }}
              _focusVisible={{
                boxShadow: "0 0 0 2px var(--chakra-colors-primary-400)",
                outline: "none",
              }}
              onClick={() => {
                setAddParentId(0);
                setAddText("");
              }}
            />
            <Text fontSize="xs" color="gray.400" textAlign="center">
              暂无大纲，可点击添加新章节
            </Text>
          </Flex>
        )}
        {searchEmpty && (
          <Flex align="center" justify="center" py={10}>
            <Text fontSize="xs" color="gray.400" textAlign="center">
              未找到匹配的目录
            </Text>
          </Flex>
        )}
        {nodes.length > 0 && !searchEmpty && tree}
        </Box>
      </Box>
      <Box px={3} py={2} borderTop="1px solid" borderColor="gray.100">
        {isBlueprint ? (
          <Flex align="center" gap={3} fontSize="xs" color="gray.500">
            <Legend color="gold.500" label="必须项" />
            <Legend color="primary.500" label="AI 建议项" />
            <Legend color="gray.400" label="可选项" />
          </Flex>
        ) : (
          <Flex align="center" gap={3} fontSize="xs" color="gray.400">
            <Legend color="gray.400" label="待生成" />
            <Legend color="gold.600" label="生成中" />
            <Legend color="success.600" label="已生成" />
            <Legend color="error.600" label="失败" />
          </Flex>
        )}
      </Box>
    </Flex>
  );
}
