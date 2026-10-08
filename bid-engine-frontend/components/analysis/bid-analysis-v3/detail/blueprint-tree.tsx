"use client";

/* Hallmark · component: blueprint-tree · genre: modern-minimal workbench · theme: chakra-smart-bid (preserved)
 * states: default · hover · focus-visible · active · disabled · loading · error · success
 * contrast: pass (semantic text ≥ 4.5:1 on paper)
 */

import { useMemo, useState } from "react";
import {
  Badge,
  Box,
  Collapse,
  Flex,
  HStack,
  IconButton,
  Input,
  Menu,
  MenuButton,
  MenuItem,
  MenuList,
  Text,
  Tooltip,
  useToast,
} from "@chakra-ui/react";
import {
  FiArrowDown,
  FiArrowUp,
  FiCheck,
  FiChevronDown,
  FiChevronRight,
  FiEdit3,
  FiMoreHorizontal,
  FiPlus,
  FiTrash2,
  FiX,
} from "react-icons/fi";

import { DataSurface } from "@/components/analysis/bid-analysis-v3/workspace";
import type { V3BlueprintNode } from "@/service/bid-analysis";

// 层级“脊柱”配色 —— 目录逐级缩进的引导线，AI 节点使用信号金区分。
const LEVEL_BORDER: Record<number, { color: string; width: string }> = {
  0: { color: "primary.300", width: "1.5px" },
  1: { color: "info.400", width: "1.5px" },
  2: { color: "neutral.300", width: "1px" },
  3: { color: "neutral.200", width: "1px" },
};
const AI_BORDER = { color: "gold.300", width: "1.5px" };

function parseIDList(value?: string) {
  try {
    const parsed = JSON.parse(value || "[]");
    return Array.isArray(parsed)
      ? parsed.map(Number).filter((id) => id > 0)
      : [];
  } catch {
    return [];
  }
}

type TreeMap = { roots: V3BlueprintNode[]; childrenMap: Map<number, V3BlueprintNode[]> };

function buildTree(nodes: V3BlueprintNode[]): TreeMap {
  if (!nodes?.length) return { roots: [], childrenMap: new Map() };
  const childrenMap = new Map<number, V3BlueprintNode[]>();
  for (const node of nodes) {
    if (!childrenMap.has(node.parent_id)) childrenMap.set(node.parent_id, []);
    childrenMap.get(node.parent_id)!.push(node);
  }
  childrenMap.forEach((list) => list.sort((a, b) => a.sort_order - b.sort_order));
  let roots = childrenMap.get(0) || [];
  if (!roots.length) roots = nodes.filter((node) => node.level === 1);
  roots.sort((a, b) => a.sort_order - b.sort_order);
  return { roots, childrenMap };
}

// 派生必须：节点自身或任意后代含必须提供标记。
function requiredDerivedMap(nodes: V3BlueprintNode[], tree: TreeMap) {
  const map = new Map<number, boolean>();
  const walk = (nodeId: number) => {
    const children = tree.childrenMap.get(nodeId) || [];
    let childHasRequired = false;
    for (const child of children) {
      walk(child.id);
      if (map.get(child.id)) childHasRequired = true;
    }
    const node = nodes.find((n) => n.id === nodeId);
    map.set(nodeId, !!(node?.is_required_file) || childHasRequired);
  };
  for (const root of tree.roots) walk(root.id);
  return map;
}

type TreeNodeProps = {
  node: V3BlueprintNode;
  childrenMap: Map<number, V3BlueprintNode[]>;
  required: boolean;
  actionsDisabled: boolean;
  mutating: boolean;
  onSaveTitle: (node: V3BlueprintNode, nextTitle: string) => Promise<void>;
  onAddChild: (parentId: number) => void;
  onMove: (nodeId: number, direction: "up" | "down") => Promise<void>;
  onAdopt: (nodeId: number) => Promise<void>;
  onRemove: (nodeId: number) => Promise<void>;
  onDelete: (nodeId: number) => Promise<void>;
  onOpenClause: (id: number) => void;
  onOpenEvidence: (ids: number[]) => void;
  depth?: number;
};

function TreeNode({
  node,
  childrenMap,
  required,
  actionsDisabled,
  mutating,
  onSaveTitle,
  onAddChild,
  onMove,
  onAdopt,
  onRemove,
  onDelete,
  onOpenClause,
  onOpenEvidence,
  depth = 0,
}: TreeNodeProps) {
  const toast = useToast();
  const children = childrenMap.get(node.id) || [];
  const hasChildren = children.length > 0;
  const [expanded, setExpanded] = useState(true);
  const [editing, setEditing] = useState(false);
  const [editTitle, setEditTitle] = useState(node.title);
  const [saving, setSaving] = useState(false);

  const clauseIds = useMemo(() => parseIDList(node.clause_ids_json), [node.clause_ids_json]);
  const evidenceIds = useMemo(() => parseIDList(node.evidence_ids_json), [node.evidence_ids_json]);

  const childRequired = useMemo(() => {
    const map = new Map<number, boolean>();
    const walk = (id: number): boolean => {
      const list = childrenMap.get(id) || [];
      if (!list.length) return false;
      let found = false;
      for (const child of list) {
        if (child.is_required_file || walk(child.id)) found = true;
      }
      return found;
    };
    for (const child of children) map.set(child.id, child.is_required_file || walk(child.id));
    return map;
  }, [children, childrenMap]);

  const border = node.is_ai_suggested ? AI_BORDER : LEVEL_BORDER[depth] || LEVEL_BORDER[3];
  const indent = depth * 20;
  const isAi = node.is_ai_suggested;
  const isAiPending = isAi && node.suggestion_status === "pending";
  const titleColor = isAi ? "gold.700" : required ? "error.700" : "workbench.text";
  const levelWeight = depth === 0 ? "750" : depth === 1 ? "700" : "620";
  const levelFontSize = depth <= 1 ? "sm" : "xs";

  const handleSaveTitle = async () => {
    const title = editTitle.trim();
    if (!title || title === node.title) {
      setEditing(false);
      return;
    }
    setSaving(true);
    try {
      await onSaveTitle(node, title);
      setEditing(false);
    } catch {
      toast({ title: "保存失败", status: "error", duration: 2000 });
    } finally {
      setSaving(false);
    }
  };

  const handleCancelEdit = () => {
    setEditTitle(node.title);
    setEditing(false);
  };

  return (
    <Box ml={`${indent}px`}>
      {/* 主行 —— 点击空白处折叠/展开 */}
      <Flex
        role="group"
        align="center"
        gap={1.5}
        minH="42px"
        px={2}
        py={1.5}
        pr={1.5}
        borderLeft={depth > 0 ? `${border.width} dashed` : "none"}
        borderColor={depth > 0 ? border.color : "transparent"}
        borderRadius="md"
        cursor={hasChildren ? "pointer" : "default"}
        transition="background-color 150ms cubic-bezier(0.16,1,0.3,1)"
        _hover={{
          bg: isAi ? "gold.50" : required ? "error.50" : "neutral.100",
        }}
        _focusWithin={{
          boxShadow: "inset 0 0 0 2px var(--chakra-colors-gold-300)",
        }}
        onClick={() => {
          if (hasChildren) setExpanded((value) => !value);
        }}
      >
        {/* 展开/折叠 */}
        <Box w="18px" textAlign="center" flexShrink={0}>
          {hasChildren ? (
            <IconButton
              aria-label={expanded ? "收起" : "展开"}
              icon={expanded ? <FiChevronDown size={12} /> : <FiChevronRight size={12} />}
              size="xs"
              variant="ghost"
              minW="16px"
              h="16px"
              _focusVisible={{ boxShadow: "0 0 0 2px var(--chakra-colors-gold-300)" }}
              onClick={(event) => {
                event.stopPropagation();
                setExpanded((value) => !value);
              }}
            />
          ) : depth > 0 ? (
            <Box as="span" color="neutral.300" fontSize="xs" lineHeight="16px">
              ·
            </Box>
          ) : null}
        </Box>

        {/* 必须/可选标记 */}
        <Text
          flexShrink={0}
          fontSize="2xs"
          fontWeight="medium"
          color={required ? "error.600" : "neutral.400"}
          userSelect="none"
          lineHeight="16px"
          title={required ? "本节点或子节点含必须标记" : "可选章节"}
        >
          {required ? "必须" : "可选"}
        </Text>

        {/* 标题 / 内联编辑 */}
        {editing ? (
          <Flex
            flex={1}
            minW={0}
            gap={1}
            align="center"
            onClick={(event) => event.stopPropagation()}
          >
            <Input
              size="xs"
              minH="34px"
              value={editTitle}
              autoFocus
              onChange={(event) => setEditTitle(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") handleSaveTitle();
                if (event.key === "Escape") handleCancelEdit();
              }}
            />
            <IconButton
              aria-label="保存标题"
              icon={<FiCheck size={12} />}
              size="xs"
              colorScheme="success"
              isLoading={saving}
              onClick={handleSaveTitle}
            />
            <IconButton
              aria-label="取消编辑"
              icon={<FiX size={12} />}
              size="xs"
              variant="ghost"
              onClick={handleCancelEdit}
            />
          </Flex>
        ) : (
          <Text
            flex={1}
            minW={0}
            fontSize={levelFontSize}
            fontWeight={levelWeight}
            color={titleColor}
            noOfLines={1}
          >
            {node.title}
          </Text>
        )}

        {/* 标签区 */}
        {!editing && (
          <HStack flexShrink={0} spacing={1} flexWrap="wrap" justify="flex-end">
            {required && (
              <Badge colorScheme="error" fontSize="2xs" variant="subtle" px={1.5} py={0} borderRadius="md">
                必须提供
              </Badge>
            )}
            {isAiPending && (
              <Tooltip label="点击采纳为正式章节">
                <Badge
                  colorScheme="gold"
                  fontSize="2xs"
                  variant="subtle"
                  px={1.5}
                  py={0}
                  borderRadius="md"
                  cursor="pointer"
                  flexShrink={0}
                  onClick={(event) => {
                    event.stopPropagation();
                    onAdopt(node.id);
                  }}
                  _hover={{ bg: "gold.200" }}
                >
                  AI建议
                </Badge>
              </Tooltip>
            )}
            {isAi && !isAiPending && (
              <Badge colorScheme="neutral" fontSize="2xs" variant="outline" px={1.5} py={0} borderRadius="md">
                已采纳
              </Badge>
            )}
            {node.is_user_added && (
              <Badge colorScheme="success" fontSize="2xs" variant="subtle" px={1.5} py={0} borderRadius="md">
                新增
              </Badge>
            )}
            {clauseIds.length > 0 && (
              <Tooltip label={`关联 ${clauseIds.length} 条招标要求`}>
                <Badge
                  colorScheme="info"
                  fontSize="2xs"
                  variant="subtle"
                  px={1.5}
                  py={0}
                  borderRadius="md"
                  cursor="pointer"
                  flexShrink={0}
                  onClick={(event) => {
                    event.stopPropagation();
                    if (clauseIds[0]) onOpenClause(clauseIds[0]);
                  }}
                  _hover={{ bg: "info.200" }}
                >
                  {clauseIds.length}条
                </Badge>
              </Tooltip>
            )}
            {evidenceIds.length > 0 && (
              <Tooltip label={`关联 ${evidenceIds.length} 处证据`}>
                <Badge
                  colorScheme="neutral"
                  fontSize="2xs"
                  variant="subtle"
                  px={1.5}
                  py={0}
                  borderRadius="md"
                  cursor="pointer"
                  flexShrink={0}
                  onClick={(event) => {
                    event.stopPropagation();
                    onOpenEvidence(evidenceIds);
                  }}
                  _hover={{ bg: "neutral.200" }}
                >
                  {evidenceIds.length}处
                </Badge>
              </Tooltip>
            )}
          </HStack>
        )}

        {/* 操作区 —— 悬浮出现，阻止冒泡 */}
        {!editing && !actionsDisabled && (
          <Flex
            flexShrink={0}
            gap={0}
            opacity={0.45}
            transition="opacity 150ms cubic-bezier(0.16,1,0.3,1)"
            _groupHover={{ opacity: 1 }}
            _hover={{ opacity: 1 }}
            onClick={(event) => event.stopPropagation()}
          >
            <Tooltip label="编辑标题">
              <IconButton
                aria-label="编辑标题"
                icon={<FiEdit3 size={11} />}
                size="xs"
                variant="ghost"
                minW="18px"
                h="18px"
                _focusVisible={{ boxShadow: "0 0 0 2px var(--chakra-colors-gold-300)" }}
                onClick={() => {
                  setEditTitle(node.title);
                  setEditing(true);
                }}
              />
            </Tooltip>
            <Tooltip label="添加子章节">
              <IconButton
                aria-label="添加子章节"
                icon={<FiPlus size={11} />}
                size="xs"
                variant="ghost"
                minW="18px"
                h="18px"
                _focusVisible={{ boxShadow: "0 0 0 2px var(--chakra-colors-gold-300)" }}
                onClick={() => onAddChild(node.id)}
              />
            </Tooltip>
            <Menu placement="bottom-end">
              <MenuButton
                as={IconButton}
                aria-label={`操作${node.title}`}
                icon={<FiMoreHorizontal size={12} />}
                size="xs"
                variant="ghost"
                minW="18px"
                h="18px"
                _focusVisible={{ boxShadow: "0 0 0 2px var(--chakra-colors-gold-300)" }}
              />
              <MenuList>
                <MenuItem
                  icon={<FiArrowUp />}
                  isDisabled={mutating}
                  onClick={() => onMove(node.id, "up")}
                >
                  上移
                </MenuItem>
                <MenuItem
                  icon={<FiArrowDown />}
                  isDisabled={mutating}
                  onClick={() => onMove(node.id, "down")}
                >
                  下移
                </MenuItem>
                {isAiPending && (
                  <MenuItem icon={<FiCheck />} isDisabled={mutating} onClick={() => onAdopt(node.id)}>
                    采纳建议
                  </MenuItem>
                )}
                {isAi && (
                  <MenuItem icon={<FiX />} isDisabled={mutating} onClick={() => onRemove(node.id)}>
                    移除建议
                  </MenuItem>
                )}
                {!node.is_locked && node.node_source !== "tender_required" && (
                  <MenuItem
                    icon={<FiTrash2 />}
                    color="error.600"
                    isDisabled={mutating}
                    onClick={() => onDelete(node.id)}
                  >
                    删除章节
                  </MenuItem>
                )}
              </MenuList>
            </Menu>
          </Flex>
        )}
      </Flex>

      {/* 递归子节点 */}
      {hasChildren && (
        <Collapse in={expanded} unmountOnExit>
          <Box>
            {children.map((child) => (
              <TreeNode
                key={child.id}
                node={child}
                childrenMap={childrenMap}
                required={!!childRequired.get(child.id)}
                actionsDisabled={actionsDisabled}
                mutating={mutating}
                onSaveTitle={onSaveTitle}
                onAddChild={onAddChild}
                onMove={onMove}
                onAdopt={onAdopt}
                onRemove={onRemove}
                onDelete={onDelete}
                onOpenClause={onOpenClause}
                onOpenEvidence={onOpenEvidence}
                depth={depth + 1}
              />
            ))}
          </Box>
        </Collapse>
      )}
    </Box>
  );
}

function StatBit({ label, value, color, bg }: { label: string; value: number; color: string; bg: string }) {
  return (
    <Text as="span" fontSize="xs" whiteSpace="nowrap" color="workbench.muted">
      {label}
      <Text as="span" fontWeight="650" color={color} bg={bg} px={1} py={0.5} borderRadius="4px" ml={0.5}>
        {value}
      </Text>
    </Text>
  );
}

export function BlueprintTree({
  nodes,
  actionsDisabled,
  mutating,
  onSaveTitle,
  onAddChild,
  onMove,
  onAdopt,
  onRemove,
  onDelete,
  onOpenClause,
  onOpenEvidence,
}: {
  nodes: V3BlueprintNode[];
  actionsDisabled: boolean;
  mutating: boolean;
  onSaveTitle: (node: V3BlueprintNode, nextTitle: string) => Promise<void>;
  onAddChild: (parentId: number) => void;
  onMove: (nodeId: number, direction: "up" | "down") => Promise<void>;
  onAdopt: (nodeId: number) => Promise<void>;
  onRemove: (nodeId: number) => Promise<void>;
  onDelete: (nodeId: number) => Promise<void>;
  onOpenClause: (id: number) => void;
  onOpenEvidence: (ids: number[]) => void;
}) {
  const [collapsed, setCollapsed] = useState(false);
  const tree = useMemo(() => buildTree(nodes), [nodes]);
  const requiredMap = useMemo(() => requiredDerivedMap(nodes, tree), [nodes, tree]);

  const requiredCount = nodes.filter((node) => node.is_required_file).length;
  const aiCount = nodes.filter((node) => node.is_ai_suggested && node.suggestion_status === "pending").length;
  const clauseCount = nodes.reduce(
    (sum, node) => sum + parseIDList(node.clause_ids_json).length,
    0,
  );
  const evidenceCount = nodes.reduce(
    (sum, node) => sum + parseIDList(node.evidence_ids_json).length,
    0,
  );

  return (
    <DataSurface p={0} overflow="hidden" boxShadow="none">
      <Flex
        px={{ base: 3, md: 4 }}
        py={2.5}
        align="center"
        justify="space-between"
        gap={3}
        borderBottom="1px solid"
        borderColor="workbench.line"
        cursor="pointer"
        transition="background-color 150ms cubic-bezier(0.16,1,0.3,1)"
        _hover={{ bg: "neutral.50" }}
        onClick={() => setCollapsed((value) => !value)}
      >
        <HStack gap={2} minW={0}>
          <Text fontSize="sm" fontWeight="740" color="workbench.text">
            目录层级
          </Text>
          {nodes.length > 0 && (
            <HStack gap={2} flexWrap="wrap">
              <StatBit label="节点" value={nodes.length} color="workbench.text" bg="neutral.100" />
              {requiredCount > 0 && (
                <StatBit label="必须" value={requiredCount} color="error.600" bg="error.50" />
              )}
              {aiCount > 0 && <StatBit label="AI建议" value={aiCount} color="gold.700" bg="gold.50" />}
              {clauseCount > 0 && (
                <StatBit label="条款" value={clauseCount} color="info.600" bg="info.50" />
              )}
              {evidenceCount > 0 && (
                <StatBit label="证据" value={evidenceCount} color="neutral.600" bg="neutral.100" />
              )}
            </HStack>
          )}
        </HStack>
        <IconButton
          aria-label={collapsed ? "展开目录" : "收起目录"}
          icon={collapsed ? <FiChevronRight size={16} /> : <FiChevronDown size={16} />}
          size="xs"
          variant="ghost"
          _focusVisible={{ boxShadow: "0 0 0 2px var(--chakra-colors-gold-300)" }}
          onClick={(event) => {
            event.stopPropagation();
            setCollapsed((value) => !value);
          }}
        />
      </Flex>

      <Collapse in={!collapsed} unmountOnExit>
        <Box p={2} maxH="70vh" overflowY="auto">
          {tree.roots.length === 0 ? (
            <Text fontSize="sm" color="workbench.muted" textAlign="center" py={8}>
              暂无目录节点，点击右上角“新增章节”开始搭建。
            </Text>
          ) : (
            tree.roots.map((root) => (
              <TreeNode
                key={root.id}
                node={root}
                childrenMap={tree.childrenMap}
                required={!!requiredMap.get(root.id)}
                actionsDisabled={actionsDisabled}
                mutating={mutating}
                onSaveTitle={onSaveTitle}
                onAddChild={onAddChild}
                onMove={onMove}
                onAdopt={onAdopt}
                onRemove={onRemove}
                onDelete={onDelete}
                onOpenClause={onOpenClause}
                onOpenEvidence={onOpenEvidence}
                depth={0}
              />
            ))
          )}
        </Box>
      </Collapse>
    </DataSurface>
  );
}
