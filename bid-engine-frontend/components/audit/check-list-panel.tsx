"use client";

/* Hallmark · component: check-list-panel (bid-audit V2) · genre: modern-minimal · theme: BidEngine deep-sea/gold
 * states: default · hover · focus-visible · active · loading · empty · disabled(not-completed)
 * Hallmark · pre-emit critique: P5 H5 E5 S5 R5 V4
 */

import React, { useCallback, useMemo, useState } from "react";
import {
  Badge,
  Box,
  Button,
  Flex,
  FormControl,
  FormLabel,
  Input,
  Modal,
  ModalBody,
  ModalCloseButton,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  Select,
  Text,
  Textarea,
  useDisclosure,
  useToast,
} from "@chakra-ui/react";
import { FiFilter, FiPlus } from "react-icons/fi";
import DeleteConfirmModal from "@/components/common/delete-confirm-modal";
import {
  ChecklistItem,
  DIMENSION_META,
  DIMENSION_ORDER,
  SEVERITY_META,
  TraceRef,
  isAlertFinding,
} from "./types";
import CheckItemCard from "./check-item-card";

interface Props {
  items: ChecklistItem[];
  loading?: boolean;
  /** 审核未成功完成（running/failed）：列表区仅提示，不展示检查项与添加按钮 */
  notCompleted?: boolean;
  highlightItemId?: number | null;
  onTrace: (refs: TraceRef[], side: "tender" | "bid") => void;
  onUpdate: (
    itemId: number,
    reviewStatus: string,
    note: string,
  ) => Promise<void>;
  onRecheck: (itemId: number) => Promise<void>;
  onDelete: (itemId: number) => Promise<void>;
  onSaveRule: (itemId: number) => Promise<void>;
  onAddItem: (data: {
    dimension: string;
    category: string;
    title: string;
    requirement: string;
    expected_evidence: string;
    severity: string;
  }) => Promise<void>;
}

const TAB_ALL = "__all__";

export default function CheckListPanel({
  items,
  loading,
  notCompleted,
  highlightItemId,
  onTrace,
  onUpdate,
  onRecheck,
  onDelete,
  onSaveRule,
  onAddItem,
}: Props) {
  const [tab, setTab] = useState<string>(TAB_ALL);
  const [onlyAlert, setOnlyAlert] = useState(false);
  const { isOpen, onOpen, onClose } = useDisclosure();
  const toast = useToast();

  const [addDimension, setAddDimension] = useState("compliance");
  const [addTitle, setAddTitle] = useState("");
  const [addCategory, setAddCategory] = useState("自定义");
  const [addRequirement, setAddRequirement] = useState("");
  const [addEvidence, setAddEvidence] = useState("");
  const [addSeverity, setAddSeverity] = useState("medium");
  const [adding, setAdding] = useState(false);

  // 删除二次确认：统一使用应用内弹窗（不使用浏览器原生 confirm）
  const [deleteTarget, setDeleteTarget] = useState<ChecklistItem | null>(null);
  const [deleting, setDeleting] = useState(false);
  const {
    isOpen: deleteIsOpen,
    onOpen: deleteOnOpen,
    onClose: deleteOnClose,
  } = useDisclosure();

  const counts = useMemo(() => {
    const c: Record<string, number> = {};
    for (const it of items) c[it.dimension] = (c[it.dimension] || 0) + 1;
    return c;
  }, [items]);

  const alertCount = useMemo(
    () => items.filter((it) => isAlertFinding(it.finding?.status)).length,
    [items],
  );

  const filtered = useMemo(() => {
    let list = items;
    if (tab !== TAB_ALL) list = list.filter((it) => it.dimension === tab);
    if (onlyAlert)
      list = list.filter((it) => isAlertFinding(it.finding?.status));
    return list;
  }, [items, tab, onlyAlert]);

  const submitAdd = async () => {
    if (!addTitle.trim()) {
      toast({ title: "请输入检查项名称", status: "warning" });
      return;
    }
    setAdding(true);
    try {
      await onAddItem({
        dimension: addDimension,
        category: addCategory.trim() || "自定义",
        title: addTitle.trim(),
        requirement: addRequirement.trim(),
        expected_evidence: addEvidence.trim(),
        severity: addSeverity,
      });
      toast({ title: "已添加", status: "success" });
      setAddTitle("");
      setAddRequirement("");
      setAddEvidence("");
      onClose();
    } catch (e: any) {
      toast({ title: e?.message || "添加失败", status: "error" });
    } finally {
      setAdding(false);
    }
  };

  const requestDelete = useCallback(
    (itemId: number) => {
      setDeleteTarget(items.find((it) => it.id === itemId) || null);
      deleteOnOpen();
    },
    [items, deleteOnOpen],
  );

  const confirmDelete = useCallback(async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await onDelete(deleteTarget.id);
      toast({ title: "已删除", status: "success", duration: 1500 });
      setDeleteTarget(null);
      deleteOnClose();
    } catch (e: any) {
      toast({ title: e?.message || "删除失败", status: "error" });
    } finally {
      setDeleting(false);
    }
  }, [deleteTarget, onDelete, toast, deleteOnClose]);

  return (
    <Box h="full" display="flex" flexDirection="column">
      {/* 顶部：维度 Tab + 仅看告警 */}
      <Box
        px={3}
        pt={3}
        pb={2}
        borderBottom="1px solid"
        borderColor="neutral.100"
        bg="white"
      >
        <Flex gap={1.5} align="center" flexWrap="wrap">
          <TabChip
            active={tab === TAB_ALL}
            onClick={() => setTab(TAB_ALL)}
            label={`全部 ${items.length}`}
          />
          {DIMENSION_ORDER.map((dim) => (
            <TabChip
              key={dim}
              active={tab === dim}
              onClick={() => setTab(dim)}
              label={`${DIMENSION_META[dim].label} ${counts[dim] || 0}`}
              color={DIMENSION_META[dim].color}
            />
          ))}
          <Button
            size="xs"
            variant={onlyAlert ? "solid" : "outline"}
            colorScheme="error"
            leftIcon={<FiFilter size={11} />}
            ml="auto"
            onClick={() => setOnlyAlert((v) => !v)}
            _focusVisible={{
              outline: "2px solid",
              outlineColor: "gold.400",
              outlineOffset: "2px",
            }}
          >
            仅看告警 {alertCount}
          </Button>
        </Flex>
      </Box>

      {/* 列表 */}
      <Box
        flex={1}
        overflowY="auto"
        p={3}
        className="thin-scrollbars"
        bg="workbench.canvas"
      >
        {notCompleted ? (
          <Flex direction="column" align="center" gap={2} py={12}>
            <Text fontSize="sm" color="neutral.400">
              审核未完成，暂无清单
            </Text>
            <Text fontSize="xs" color="neutral.400">
              审核完成后会按合规性/完整性/竞争力逐条给出结论与证据
            </Text>
          </Flex>
        ) : loading ? (
          <Text fontSize="sm" color="neutral.400" textAlign="center" py={10}>
            加载中…
          </Text>
        ) : filtered.length === 0 ? (
          <Flex direction="column" align="center" gap={2} py={12}>
            <Text fontSize="sm" color="neutral.400">
              {items.length === 0
                ? "暂无检查项，审核完成后自动生成"
                : "没有符合条件的检查项"}
            </Text>
            <Button size="xs" leftIcon={<FiPlus />} onClick={onOpen}>
              添加检查项
            </Button>
          </Flex>
        ) : (
          <Flex direction="column" gap={2}>
            {filtered.map((it) => (
              <CheckItemCard
                key={it.id}
                item={it}
                highlighted={highlightItemId === it.id}
                onTrace={onTrace}
                onUpdate={onUpdate}
                onRecheck={onRecheck}
                onDelete={requestDelete}
                onSaveRule={onSaveRule}
              />
            ))}
            <Button
              size="sm"
              variant="ghost"
              leftIcon={<FiPlus />}
              onClick={onOpen}
              alignSelf="flex-start"
            >
              添加检查项
            </Button>
          </Flex>
        )}
      </Box>

      {/* 添加检查项 */}
      <Modal
        isOpen={isOpen}
        onClose={onClose}
        isCentered
        size="md"
        scrollBehavior="inside"
      >
        <ModalOverlay bg="blackAlpha.500" />
        <ModalContent mx={3} borderRadius="14px" maxH="calc(100dvh - 2rem)">
          <ModalHeader
            fontSize="md"
            borderBottom="1px solid"
            borderColor="neutral.100"
          >
            添加自定义检查项
          </ModalHeader>
          <ModalCloseButton />
          <ModalBody py={4}>
            <Flex direction="column" gap={3}>
              <FormControl>
                <FormLabel fontSize="xs" color="neutral.500" mb={1}>
                  审核维度
                </FormLabel>
                <Select
                  size="sm"
                  value={addDimension}
                  onChange={(e) => setAddDimension(e.target.value)}
                >
                  {DIMENSION_ORDER.map((dim) => (
                    <option key={dim} value={dim}>
                      {DIMENSION_META[dim].label}
                    </option>
                  ))}
                </Select>
              </FormControl>
              <FormControl>
                <FormLabel fontSize="xs" color="neutral.500" mb={1}>
                  检查项名称 *
                </FormLabel>
                <Input
                  size="sm"
                  value={addTitle}
                  onChange={(e) => setAddTitle(e.target.value)}
                  placeholder="如：售后服务承诺函是否盖章"
                />
              </FormControl>
              <Flex gap={3}>
                <FormControl flex={1}>
                  <FormLabel fontSize="xs" color="neutral.500" mb={1}>
                    分类
                  </FormLabel>
                  <Input
                    size="sm"
                    value={addCategory}
                    onChange={(e) => setAddCategory(e.target.value)}
                    placeholder="如：资格材料"
                  />
                </FormControl>
                <FormControl flex={1}>
                  <FormLabel fontSize="xs" color="neutral.500" mb={1}>
                    风险级别
                  </FormLabel>
                  <Select
                    size="sm"
                    value={addSeverity}
                    onChange={(e) => setAddSeverity(e.target.value)}
                  >
                    {Object.entries(SEVERITY_META).map(([key, meta]) => (
                      <option key={key} value={key}>
                        {meta.label}
                      </option>
                    ))}
                  </Select>
                </FormControl>
              </Flex>
              <FormControl>
                <FormLabel fontSize="xs" color="neutral.500" mb={1}>
                  判定口径 / 招标依据
                </FormLabel>
                <Textarea
                  size="sm"
                  value={addRequirement}
                  onChange={(e) => setAddRequirement(e.target.value)}
                  placeholder="（可选）该项的招标依据或检查口径"
                  rows={3}
                />
              </FormControl>
              <FormControl>
                <FormLabel fontSize="xs" color="neutral.500" mb={1}>
                  期望证据
                </FormLabel>
                <Input
                  size="sm"
                  value={addEvidence}
                  onChange={(e) => setAddEvidence(e.target.value)}
                  placeholder="（可选）如：盖章的承诺函扫描件"
                />
              </FormControl>
            </Flex>
          </ModalBody>
          <ModalFooter borderTop="1px solid" borderColor="neutral.100">
            <Button variant="ghost" mr={3} onClick={onClose}>
              取消
            </Button>
            <Button
              colorScheme="primary"
              size="sm"
              isLoading={adding}
              onClick={submitAdd}
            >
              添加
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      <DeleteConfirmModal
        isOpen={deleteIsOpen}
        onClose={() => {
          setDeleteTarget(null);
          deleteOnClose();
        }}
        title="删除检查项"
        description={
          <>
            确定删除检查项
            <Text as="span" fontWeight="700" color="workbench.text">
              {deleteTarget?.title || ""}
            </Text>
            吗？删除后不可恢复。
          </>
        }
        isLoading={deleting}
        handleConfirm={confirmDelete}
      />
    </Box>
  );
}

function TabChip({
  label,
  active,
  onClick,
  color,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
  color?: string;
}) {
  return (
    <Badge
      as="button"
      onClick={onClick}
      variant={active ? "solid" : "subtle"}
      bg={active ? "primary.600" : "neutral.100"}
      color={active ? "white" : color || "neutral.600"}
      fontSize="11px"
      borderRadius="full"
      px={2.5}
      py={1}
      cursor="pointer"
      transition="all 0.15s"
      _hover={{ opacity: 0.85 }}
      _focusVisible={{
        outline: "2px solid",
        outlineColor: "gold.400",
        outlineOffset: "2px",
      }}
    >
      {label}
    </Badge>
  );
}
