"use client";

/* eslint-disable no-nested-ternary */

/* Hallmark · macrostructure: Workbench · genre: modern-minimal · theme: BidEngine deep-sea/gold (preserved tokens)
 * 用途：系统管理 → 招标情报管理 → 情报管理。渲染口径与情报大厅一致（同一张卡片），
 * 额外提供发布、批量导入、置顶、隐藏、下架、恢复、编辑、删除等管理能力。
 * motion: hover lift + press scale（≤200ms） · 无虚构指标 · 窄屏单列
 * Hallmark · pre-emit critique: P5 H5 E5 S5 R5 V4
 */
import React, { useCallback, useEffect, useMemo, useState } from "react";
import {
  Badge,
  Box,
  Button,
  Collapse,
  Divider,
  Flex,
  Grid,
  HStack,
  ListItem,
  Skeleton,
  Stack,
  Text,
  UnorderedList,
  useDisclosure,
  useToast,
} from "@chakra-ui/react";
import {
  FiEdit3,
  FiEyeOff,
  FiPlus,
  FiRefreshCw,
  FiRotateCcw,
  FiStar,
  FiTrash2,
  FiUpload,
  FiXCircle,
} from "react-icons/fi";

import EmptyState from "@/components/common/empty-state";
import DeleteConfirmModal from "@/components/common/delete-confirm-modal";
import InfiniteScrollList from "@/components/common/infinite-scroll";
import ScrollToTopButton from "@/components/common/scroll-to-top";
import { useInfiniteList } from "@/hooks/use-infinite-list";
import IntelFilterBar, {
  EMPTY_FILTER_VALUE,
  countActiveFilters,
  toNoticeQuery,
  type IntelFilterValue,
} from "@/components/intel/filter-bar";
import IntelSelect, {
  FIELD_HEIGHT,
  type IntelSelectOption,
} from "@/components/intel/intel-select";
import IntelNoticeCard from "@/components/intel/notice-card";
import NoticeEditorModal, {
  type IntelEditableNotice,
  type NoticeFormValue,
  toNoticePayload,
} from "@/components/intel/admin/notice-editor-modal";
import NoticeImportModal from "@/components/intel/admin/notice-import-modal";
import HintToggle from "@/components/intel/admin/hint-toggle";
import {
  useDebouncedValue,
  useIntelAdminNotices,
  useIntelFilters,
  useIntelNoticeDelete,
  useIntelNoticeDetail,
  useIntelNoticePin,
  useIntelNoticeSave,
  useIntelNoticeStatus,
  type IntelAdminNotice,
  type IntelNoticeAdminQuery,
} from "@/service/intel";

const PAGE_SIZE = 10;

// 滑动分页（隐式分页）需要绑定页面自身的滚动容器（由 /system/intel 的 PageViewport 提供）
const MANAGER_SCROLL_ID = "intel-admin-scroll";

const STATUS_OPTIONS: IntelSelectOption[] = [
  { value: "", label: "全部状态" },
  { value: "normal", label: "在架" },
  { value: "hidden", label: "已隐藏" },
  { value: "archived", label: "已下架" },
];

const ORIGIN_OPTIONS: IntelSelectOption[] = [
  { value: "", label: "全部来源类型" },
  { value: "collect", label: "自动采集" },
  { value: "manual", label: "系统录入" },
];

/** 状态徽标：在架、隐藏、下架三种管理态。 */
function statusBadgeOf(status: string) {
  if (status === "hidden") {
    return (
      <Badge colorScheme="warning" variant="subtle" borderRadius="md">
        已隐藏
      </Badge>
    );
  }
  if (status === "archived") {
    return (
      <Badge colorScheme="gray" variant="subtle" borderRadius="md">
        已下架
      </Badge>
    );
  }
  if (status === "duplicate") {
    return (
      <Badge colorScheme="blue" variant="subtle" borderRadius="md">
        重复
      </Badge>
    );
  }
  if (status === "invalid") {
    return (
      <Badge colorScheme="red" variant="subtle" borderRadius="md">
        无效
      </Badge>
    );
  }
  return (
    <Badge colorScheme="green" variant="subtle" borderRadius="md">
      在架
    </Badge>
  );
}

/** 折叠态概览里的单个统计项。 */
function Stat({
  label,
  value,
  dot,
}: {
  label: string;
  value: number;
  dot?: string;
}) {
  return (
    <HStack spacing={1.5} align="center" whiteSpace="nowrap">
      {dot ? <Box w="6px" h="6px" borderRadius="full" bg={dot} /> : null}
      <Text as="span" fontSize="xs" color="workbench.muted">
        {label}
      </Text>
      <Text
        as="span"
        fontSize="sm"
        fontWeight="700"
        color="workbench.text"
        sx={{ fontVariantNumeric: "tabular-nums" }}
      >
        {value}
      </Text>
    </HStack>
  );
}

/** 单条公告的管理操作区：低频操作按“主操作 + 次操作”分组，窄屏自动换行。 */
function NoticeAdminActions({
  notice,
  hidden,
  archived,
  onPin,
  onRestore,
  onHide,
  onArchive,
  onEdit,
  onDelete,
}: {
  notice: IntelAdminNotice;
  hidden: boolean;
  archived: boolean;
  onPin: () => void;
  onRestore: () => void;
  onHide: () => void;
  onArchive: () => void;
  onEdit: () => void;
  onDelete: () => void;
}) {
  return (
    <>
      <Button
        size="sm"
        h="36px"
        variant={notice.pinned ? "solid" : "outline"}
        colorScheme={notice.pinned ? "primary" : undefined}
        borderColor="neutral.200"
        color={notice.pinned ? "white" : "neutral.600"}
        leftIcon={<FiStar aria-hidden />}
        onClick={onPin}
      >
        {notice.pinned ? "取消置顶" : "置顶"}
      </Button>
      {hidden || archived ? (
        <Button
          size="sm"
          h="36px"
          variant="outline"
          borderColor="neutral.200"
          color="neutral.600"
          leftIcon={<FiRotateCcw aria-hidden />}
          onClick={onRestore}
        >
          恢复
        </Button>
      ) : (
        <Button
          size="sm"
          h="36px"
          variant="outline"
          borderColor="neutral.200"
          color="neutral.600"
          leftIcon={<FiEyeOff aria-hidden />}
          onClick={onHide}
        >
          隐藏
        </Button>
      )}
      {!archived ? (
        <Button
          size="sm"
          h="36px"
          variant="outline"
          borderColor="neutral.200"
          color="neutral.600"
          leftIcon={<FiXCircle aria-hidden />}
          onClick={onArchive}
        >
          下架
        </Button>
      ) : null}
      <Button
        size="sm"
        h="36px"
        variant="outline"
        borderColor="neutral.200"
        color="neutral.600"
        leftIcon={<FiEdit3 aria-hidden />}
        onClick={onEdit}
      >
        编辑
      </Button>
      <Button
        size="sm"
        h="36px"
        variant="outline"
        borderColor="red.200"
        color="red.600"
        leftIcon={<FiTrash2 aria-hidden />}
        onClick={onDelete}
      >
        删除
      </Button>
    </>
  );
}

export default function NoticeManager() {
  const toast = useToast();
  const { filters, filtersLoading } = useIntelFilters();

  const [filterValue, setFilterValue] = useState<IntelFilterValue>({
    ...EMPTY_FILTER_VALUE,
  });
  const [status, setStatus] = useState("");
  const [origin, setOrigin] = useState("");
  const [selectedIds, setSelectedIds] = useState<number[]>([]);
  const [helpOpen, setHelpOpen] = useState(false);
  const [editingNotice, setEditingNotice] =
    useState<IntelEditableNotice | null>(null);
  const [deleteTargets, setDeleteTargets] = useState<IntelAdminNotice[]>([]);
  // 隐藏/下架是不可逆展示效果的管理动作，先弹二次确认再执行
  const [statusTargets, setStatusTargets] = useState<IntelAdminNotice[]>([]);
  const [statusAction, setStatusAction] = useState<"hidden" | "archived">(
    "hidden",
  );

  const editor = useDisclosure();
  const importer = useDisclosure();
  const deleteDialog = useDisclosure();
  const statusDialog = useDisclosure();

  const { adminNoticesSummary, adminNoticesLoading, fetchAdminNotices } =
    useIntelAdminNotices({}, false);
  const { noticeSaveLoading, fetchCreate, fetchUpdate } = useIntelNoticeSave();
  const { fetchStatus } = useIntelNoticeStatus();
  const { fetchPin } = useIntelNoticePin();
  const { noticeDeleteLoading, fetchDelete } = useIntelNoticeDelete();
  const { fetchDetail } = useIntelNoticeDetail();

  const appliedFilters = useDebouncedValue(filterValue, 350);

  /** 滑动分页的数据源：每次请求一页，由 useInfiniteList 负责累积与去重。 */
  const fetchPage = useCallback(
    async (targetPage: number) => {
      const query: IntelNoticeAdminQuery = {
        ...toNoticeQuery(appliedFilters, targetPage, PAGE_SIZE),
        ...(status ? { statuses: [status] } : {}),
        ...(origin ? { origins: [origin] } : {}),
      };
      const res = await fetchAdminNotices({ params: query });
      return {
        items: (res?.data?.data?.list || []) as IntelAdminNotice[],
        total: (res?.data?.data?.total || 0) as number,
      };
    },
    [appliedFilters, status, origin, fetchAdminNotices],
  );

  const {
    items,
    total,
    initialLoading,
    loadingMore,
    hasMore,
    error: loadError,
    loadFirstPage,
    loadMore,
  } = useInfiniteList<IntelAdminNotice>({
    fetcher: fetchPage,
    pageSize: PAGE_SIZE,
    resetDeps: [appliedFilters, status, origin],
  });

  const handleFilterChange = (next: IntelFilterValue) => {
    setFilterValue(next);
  };

  const handleStatusFilterChange = (next: string) => {
    setStatus(next);
  };

  const handleOriginFilterChange = (next: string) => {
    setOrigin(next);
  };

  // 列表刷新后剔除已不在当前页的选中项，避免对看不见的公告执行批量操作
  useEffect(() => {
    setSelectedIds((prev) => {
      if (prev.length === 0) return prev;
      const visible = new Set(items.map((item) => item.id));
      const next = prev.filter((id) => visible.has(id));
      return next.length === prev.length ? prev : next;
    });
  }, [items]);

  const activeCount =
    countActiveFilters(appliedFilters) + (status ? 1 : 0) + (origin ? 1 : 0);
  const summary = adminNoticesSummary;

  const notifyError = (error: any, fallback: string) => {
    toast({
      title: "操作失败",
      description:
        error?.response?.data?.message ||
        error?.data?.message ||
        error?.message ||
        fallback,
      status: "error",
      duration: 4000,
      isClosable: true,
    });
  };

  /** 统一处理写操作：先判定业务错误码，再提示并刷新列表。 */
  const runWrite = async (
    request: () => Promise<any>,
    success: { title: string; description?: string },
  ) => {
    try {
      const res = await request();
      const code = res?.data?.code;
      if (code !== undefined && code !== null && code !== 0) {
        throw new Error(res?.data?.message || "操作失败");
      }
      toast({
        title: success.title,
        description: success.description,
        status: "success",
        duration: 3000,
        isClosable: true,
      });
      return true;
    } catch (error: any) {
      notifyError(error, "请稍后重试");
      return false;
    }
  };

  /** 写操作成功后：清空选择、重新拉第 1 页并把滚动容器拉回顶部。 */
  const afterWrite = () => {
    setSelectedIds([]);
    loadFirstPage();
    if (typeof document !== "undefined") {
      const container = document.getElementById(MANAGER_SCROLL_ID);
      if (container) container.scrollTop = 0;
    }
  };

  /** 隐藏/下架前先弹二次确认。 */
  const askStatusChange = (
    targets: IntelAdminNotice[],
    next: "hidden" | "archived",
  ) => {
    if (targets.length === 0) return;
    setStatusTargets(targets);
    setStatusAction(next);
    statusDialog.onOpen();
  };

  const handleStatus = async (
    targets: IntelAdminNotice[],
    next: "normal" | "hidden" | "archived",
  ) => {
    const label =
      next === "hidden" ? "已隐藏" : next === "archived" ? "已下架" : "已恢复";
    const ok = await runWrite(
      () =>
        fetchStatus({
          data: { ids: targets.map((item) => item.id), status: next },
        }),
      {
        title: `操作完成：${label}`,
        description:
          next === "normal"
            ? "公告已回到情报大厅"
            : `共 ${targets.length} 条公告${label}，可在本页状态筛选中找回`,
      },
    );
    if (ok) afterWrite();
  };

  /** 二次确认后执行隐藏或下架。 */
  const handleStatusConfirm = async () => {
    const targets = statusTargets;
    statusDialog.onClose();
    setStatusTargets([]);
    if (targets.length === 0) return;
    await handleStatus(targets, statusAction);
  };

  const handlePin = async (targets: IntelAdminNotice[], pinned: boolean) => {
    const ok = await runWrite(
      () => fetchPin({ data: { ids: targets.map((item) => item.id), pinned } }),
      {
        title: pinned ? "已置顶" : "已取消置顶",
        description: pinned
          ? "置顶公告会排在情报大厅列表最前"
          : "公告恢复按发布时间排序",
      },
    );
    if (ok) afterWrite();
  };

  const handleDelete = async () => {
    const ids = deleteTargets.map((item) => item.id);
    const ok = await runWrite(() => fetchDelete({ data: { ids } }), {
      title: "已删除",
      description: `共删除 ${ids.length} 条情报，及其提醒与收藏记录`,
    });
    deleteDialog.onClose();
    setDeleteTargets([]);
    if (ok) afterWrite();
  };

  /**
   * 打开编辑器。编辑已有情报时先拉一次详情补全正文，
   * 否则保存时正文会被空值覆盖。
   */
  const openEditor = async (notice: IntelAdminNotice | null) => {
    if (!notice) {
      setEditingNotice(null);
      editor.onOpen();
      return;
    }
    let merged: IntelEditableNotice = notice;
    try {
      const res = await fetchDetail({ url: `/zb/intel/notices/${notice.id}` });
      const detail = res?.data?.data;
      if (detail) {
        merged = { ...notice, body_text: detail.body_text || "" };
      }
    } catch (error: any) {
      // 详情拉取失败不阻塞编辑：正文留空但用户填写后仍可保存
      notifyError(error, "读取公告正文失败，正文将以空值展示");
    }
    setEditingNotice(merged);
    editor.onOpen();
  };

  const handleSubmitNotice = async (value: NoticeFormValue) => {
    const editing = editingNotice;
    const ok = await runWrite(
      () =>
        editing
          ? fetchUpdate({ data: toNoticePayload(value, editing.id) })
          : fetchCreate({ data: toNoticePayload(value) }),
      {
        title: editing ? "情报已更新" : "情报已发布",
        description: editing
          ? "修改内容已对全体用户生效"
          : "新情报已在情报大厅可见（状态：在架）",
      },
    );
    if (ok) {
      editor.onClose();
      setEditingNotice(null);
      afterWrite();
    }
  };

  const selectedSet = useMemo(() => new Set(selectedIds), [selectedIds]);
  const selectedNotices = useMemo(
    () => items.filter((item) => selectedSet.has(item.id)),
    [items, selectedSet],
  );
  const allSelected = items.length > 0 && selectedIds.length === items.length;
  const toggleSelectAll = () =>
    setSelectedIds(allSelected ? [] : items.map((item) => item.id));
  const toggleSelect = (id: number, next: boolean) =>
    setSelectedIds((prev) =>
      next
        ? Array.from(new Set([...prev, id]))
        : prev.filter((item) => item !== id),
    );

  return (
    <Stack spacing={4}>
      <ScrollToTopButton targetId={MANAGER_SCROLL_ID} />
      {/* 操作条：默认折叠，只渲染概览统计与三个按钮 */}
      <Box
        bg="white"
        border="1px solid"
        borderColor="workbench.line"
        borderRadius="14px"
        boxShadow="0 10px 30px rgba(11, 27, 43, 0.06)"
        px={{ base: 3, md: 4 }}
        py={3}
      >
        <Flex
          align={{ base: "stretch", md: "center" }}
          justify="space-between"
          direction={{ base: "column", md: "row" }}
          gap={3}
        >
          <HStack spacing={2} minW={0} wrap="wrap" order={{ base: 2, md: 1 }}>
            <HStack spacing={3} wrap="wrap">
              <Stat label="共" value={summary?.total ?? total} />
              <Stat label="在架" value={summary?.normal ?? 0} dot="green.400" />
              <Stat
                label="隐藏"
                value={summary?.hidden ?? 0}
                dot="orange.400"
              />
              <Stat
                label="下架"
                value={summary?.archived ?? 0}
                dot="gray.400"
              />
              <Stat
                label="置顶"
                value={summary?.pinned ?? 0}
                dot="yellow.400"
              />
              <Stat
                label="系统录入"
                value={summary?.manual ?? 0}
                dot="blue.400"
              />
            </HStack>
          </HStack>

          <HStack
            spacing={3}
            order={{ base: 1, md: 2 }}
            flexShrink={0}
            w={{ base: "full", md: "auto" }}
          >
            <Button
              flex={{ base: 1, md: "none" }}
              h={FIELD_HEIGHT}
              variant="outline"
              borderColor="neutral.200"
              leftIcon={<FiRefreshCw aria-hidden />}
              isLoading={adminNoticesLoading}
              onClick={() => loadFirstPage()}
              whiteSpace="nowrap"
              _hover={{ borderColor: "primary.300", color: "primary.600" }}
              _active={{ transform: "scale(0.98)" }}
            >
              刷新
            </Button>
            <Button
              flex={{ base: 1, md: "none" }}
              h={FIELD_HEIGHT}
              variant="outline"
              borderColor="primary.200"
              color="primary.600"
              leftIcon={<FiUpload aria-hidden />}
              onClick={importer.onOpen}
              whiteSpace="nowrap"
              _hover={{ bg: "primary.50" }}
              _active={{ transform: "scale(0.98)" }}
            >
              批量导入
            </Button>
            <Button
              flex={{ base: 1, md: "none" }}
              h={FIELD_HEIGHT}
              colorScheme="primary"
              leftIcon={<FiPlus aria-hidden />}
              onClick={() => openEditor(null)}
              whiteSpace="nowrap"
              _active={{ transform: "scale(0.98)" }}
            >
              发布情报
            </Button>
            <HintToggle
              label="情报管理说明"
              expanded={helpOpen}
              onToggle={() => setHelpOpen((prev) => !prev)}
            />
          </HStack>
        </Flex>

        <Collapse in={helpOpen} animateOpacity>
          <Divider mt={3} mb={3} borderColor="neutral.100" />
          <UnorderedList
            spacing={2}
            pl={5}
            m={0}
            fontSize="sm"
            color="workbench.text"
          >
            <ListItem>
              列表口径与情报大厅一致，但这里同时展示隐藏与下架的公告；筛选条件同样自动生效。
            </ListItem>
            <ListItem>
              <Text as="span" fontWeight="600">
                置顶
              </Text>
              ：置顶公告排在情报大厅列表最前，适合重点机会。
            </ListItem>
            <ListItem>
              <Text as="span" fontWeight="600">
                隐藏
              </Text>
              ：临时不在情报大厅展示，内容仍然有效，随时可以恢复。
            </ListItem>
            <ListItem>
              <Text as="span" fontWeight="600">
                下架
              </Text>
              ：确认为误采或已失效的信息，不再展示，历史记录保留可追溯。
            </ListItem>
            <ListItem>
              <Text as="span" fontWeight="600">
                发布情报
              </Text>
              ：线下收集的信息可手工录入，来源标记为系统录入，并参与订阅匹配。
            </ListItem>
            <ListItem>
              <Text as="span" fontWeight="600">
                批量导入
              </Text>
              ：按模板上传表格，支持一次录入多条，导入批次号可用于按批次筛选。
            </ListItem>
            <ListItem>
              <Text as="span" fontWeight="600">
                删除
              </Text>
              ：连同提醒、收藏与 AI
              解读缓存一并删除，不可恢复，删除前会二次确认。
            </ListItem>
          </UnorderedList>
        </Collapse>
      </Box>

      <IntelFilterBar
        filters={filters}
        filtersLoading={filtersLoading}
        value={filterValue}
        onChange={handleFilterChange}
        onReset={() => {
          setFilterValue({ ...EMPTY_FILTER_VALUE });
          setStatus("");
          setOrigin("");
        }}
        loading={initialLoading || loadingMore}
        collapsible
        showFavoriteToggle={false}
        primaryExtras={
          <HStack spacing={3} wrap="wrap">
            <Box w={{ base: "full", sm: "150px" }} flexShrink={0}>
              <IntelSelect
                value={status}
                options={STATUS_OPTIONS}
                onChange={handleStatusFilterChange}
                ariaLabel="状态筛选"
                placeholder="全部状态"
              />
            </Box>
            <Box w={{ base: "full", sm: "170px" }} flexShrink={0}>
              <IntelSelect
                value={origin}
                options={ORIGIN_OPTIONS}
                onChange={handleOriginFilterChange}
                ariaLabel="来源类型筛选"
                placeholder="全部来源类型"
              />
            </Box>
          </HStack>
        }
      />

      {selectedIds.length > 0 ? (
        <Flex
          gap={3}
          wrap="wrap"
          align="center"
          justify="space-between"
          bg="primary.50"
          border="1px solid"
          borderColor="primary.200"
          borderRadius="12px"
          px={4}
          py={3}
        >
          <HStack spacing={3} wrap="wrap">
            <Text fontSize="sm" fontWeight="600" color="primary.800">
              已选择 {selectedIds.length} 条
            </Text>
            <Button
              size="sm"
              variant="ghost"
              h="32px"
              onClick={toggleSelectAll}
            >
              {allSelected ? "取消全选" : "全选已加载"}
            </Button>
            <Button
              size="sm"
              variant="ghost"
              h="32px"
              onClick={() => setSelectedIds([])}
            >
              清空选择
            </Button>
          </HStack>
          <HStack spacing={2} wrap="wrap">
            <Button
              size="sm"
              h="36px"
              variant="outline"
              borderColor="neutral.200"
              leftIcon={<FiStar aria-hidden />}
              onClick={() => handlePin(selectedNotices, true)}
            >
              批量置顶
            </Button>
            <Button
              size="sm"
              h="36px"
              variant="outline"
              borderColor="neutral.200"
              leftIcon={<FiEyeOff aria-hidden />}
              onClick={() => askStatusChange(selectedNotices, "hidden")}
            >
              批量隐藏
            </Button>
            <Button
              size="sm"
              h="36px"
              variant="outline"
              borderColor="neutral.200"
              leftIcon={<FiXCircle aria-hidden />}
              onClick={() => askStatusChange(selectedNotices, "archived")}
            >
              批量下架
            </Button>
            <Button
              size="sm"
              h="36px"
              variant="outline"
              borderColor="neutral.200"
              leftIcon={<FiRotateCcw aria-hidden />}
              onClick={() => handleStatus(selectedNotices, "normal")}
            >
              批量恢复
            </Button>
            <Button
              size="sm"
              h="36px"
              colorScheme="red"
              variant="outline"
              leftIcon={<FiTrash2 aria-hidden />}
              onClick={() => {
                setDeleteTargets(selectedNotices);
                deleteDialog.onOpen();
              }}
            >
              批量删除
            </Button>
          </HStack>
        </Flex>
      ) : items.length > 0 ? (
        <Button
          alignSelf="flex-start"
          size="sm"
          variant="ghost"
          color="workbench.muted"
          onClick={toggleSelectAll}
        >
          全选已加载（{items.length} 条）
        </Button>
      ) : null}

      {initialLoading && items.length === 0 ? (
        <Grid
          templateColumns="repeat(auto-fill, minmax(min(100%, 340px), 1fr))"
          gap={4}
          alignItems="stretch"
        >
          {[0, 1, 2, 3].map((key) => (
            <Skeleton key={key} height="220px" borderRadius="xl" />
          ))}
        </Grid>
      ) : loadError && items.length === 0 ? (
        <EmptyState
          title="情报加载失败"
          description={loadError}
          action={
            <Button colorScheme="primary" onClick={() => loadFirstPage()}>
              重新加载
            </Button>
          }
        />
      ) : items.length === 0 ? (
        <EmptyState
          title={activeCount > 0 ? "没有符合条件的情报" : "还没有情报数据"}
          description={
            activeCount > 0
              ? "试着放宽关键词、地区或状态条件后重试。"
              : "可以等待自动采集，或用发布情报、批量导入手工录入。"
          }
        />
      ) : (
        /* 滑动分页：滚到底部自动加载下一页，加载完显示“已加载全部” */
        <InfiniteScrollList
          dataLength={items.length}
          hasMore={hasMore && !loadError}
          loadMore={loadMore}
          scrollableTarget={MANAGER_SCROLL_ID}
          endMessage={
            loadError ? (
              <Flex justify="center" mt={4}>
                <Button
                  variant="outline"
                  borderColor="neutral.200"
                  onClick={() => loadMore()}
                >
                  加载更多失败，点击重试
                </Button>
              </Flex>
            ) : undefined
          }
        >
          <Grid
            templateColumns="repeat(auto-fill, minmax(min(100%, 340px), 1fr))"
            gap={4}
            alignItems="stretch"
          >
            {items.map((notice) => {
              const hidden = notice.status === "hidden";
              const archived = notice.status === "archived";
              return (
                <IntelNoticeCard
                  key={notice.id}
                  notice={notice}
                  detailBasePath="/system/intel/notices"
                  variant="admin"
                  showFavorite={false}
                  onToggleFavorite={() => {}}
                  statusBadge={statusBadgeOf(notice.status)}
                  meta={
                    notice.admin_note ? (
                      <Text fontSize="xs" color="workbench.muted">
                        管理备注：{notice.admin_note}
                      </Text>
                    ) : notice.pinned_at ? (
                      <Text fontSize="xs" color="workbench.muted">
                        置顶时间 {notice.pinned_at}
                      </Text>
                    ) : null
                  }
                  selectable
                  selected={selectedSet.has(notice.id)}
                  onSelectedChange={(next) => toggleSelect(notice.id, next)}
                  actions={
                    <NoticeAdminActions
                      notice={notice}
                      hidden={hidden}
                      archived={archived}
                      onPin={() => handlePin([notice], !notice.pinned)}
                      onRestore={() => handleStatus([notice], "normal")}
                      onHide={() => askStatusChange([notice], "hidden")}
                      onArchive={() => askStatusChange([notice], "archived")}
                      onEdit={() => openEditor(notice)}
                      onDelete={() => {
                        setDeleteTargets([notice]);
                        deleteDialog.onOpen();
                      }}
                    />
                  }
                />
              );
            })}
          </Grid>
        </InfiniteScrollList>
      )}

      {items.length > 0 ? (
        <Flex justify="center">
          <Text
            fontSize="sm"
            color="workbench.muted"
            sx={{ fontVariantNumeric: "tabular-nums" }}
          >
            当前筛选共 {total} 条，已加载 {items.length} 条
          </Text>
        </Flex>
      ) : null}

      <NoticeEditorModal
        isOpen={editor.isOpen}
        onClose={() => {
          editor.onClose();
          setEditingNotice(null);
        }}
        notice={editingNotice}
        filters={filters}
        onSubmit={handleSubmitNotice}
        submitting={noticeSaveLoading}
      />

      <NoticeImportModal
        isOpen={importer.isOpen}
        onClose={importer.onClose}
        onImported={() => afterWrite()}
      />

      <DeleteConfirmModal
        isOpen={deleteDialog.isOpen}
        onClose={() => {
          deleteDialog.onClose();
          setDeleteTargets([]);
        }}
        title="删除情报"
        description={
          deleteTargets.length > 1
            ? `将删除 ${deleteTargets.length} 条情报，并同步清理其提醒、收藏与 AI 解读缓存。删除后不可恢复。`
            : `将删除“${deleteTargets[0]?.title || ""}”，并同步清理其提醒、收藏与 AI 解读缓存。删除后不可恢复。`
        }
        isLoading={noticeDeleteLoading}
        handleConfirm={handleDelete}
      />

      {/* 隐藏 / 下架的二次确认：两个动作都会让公告从情报大厅消失，必须先确认 */}
      <DeleteConfirmModal
        isOpen={statusDialog.isOpen}
        onClose={() => {
          statusDialog.onClose();
          setStatusTargets([]);
        }}
        title={statusAction === "hidden" ? "隐藏情报" : "下架情报"}
        description={
          statusAction === "hidden"
            ? statusTargets.length > 1
              ? `将隐藏 ${statusTargets.length} 条情报，它们会从情报大厅消失，但内容仍然有效，可随时在“状态=已隐藏”里恢复。`
              : `将隐藏“${statusTargets[0]?.title || ""}”，它会从情报大厅消失，但内容仍然有效，可随时在“状态=已隐藏”里恢复。`
            : statusTargets.length > 1
              ? `将下架 ${statusTargets.length} 条情报，它们会从情报大厅消失并标记为已下架（适用于误采或已失效信息），历史记录保留可追溯。`
              : `将下架“${statusTargets[0]?.title || ""}”，它会从情报大厅消失并标记为已下架（适用于误采或已失效信息），历史记录保留可追溯。`
        }
        isLoading={false}
        handleConfirm={handleStatusConfirm}
      />
    </Stack>
  );
}
