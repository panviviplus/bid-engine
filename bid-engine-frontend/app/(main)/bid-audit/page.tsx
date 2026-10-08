"use client";

/* eslint-disable no-nested-ternary */

import React, { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import {
  Box,
  Button,
  Flex,
  Grid,
  Input,
  InputGroup,
  InputLeftElement,
  Skeleton,
  Text,
  useDisclosure,
  useToast,
} from "@chakra-ui/react";
import { FiSearch, FiPlus } from "react-icons/fi";

import StatusFilterBar from "@/components/common/status-filter-bar";
import EmptyState from "@/components/common/empty-state";
import InfiniteScrollList from "@/components/common/infinite-scroll";
import DeleteConfirmModal from "@/components/common/delete-confirm-modal";
import { useInfiniteList } from "@/hooks/use-infinite-list";
import { useStatusTotals } from "@/hooks/use-status-totals";
import ProjectCard from "@/components/audit/project-card";
import CreateAuditModal from "@/components/audit/create-modal";
import {
  useReviewList,
  useReviewDelete,
  useReviewRetryStage,
} from "@/service/audit";
import { ReviewProject, STAGE_LABELS } from "@/components/audit/types";
import { PageViewport } from "@/components/layout/responsive-page";
import {
  ModuleWorkbenchDeck,
  ModuleWorkbenchHeader,
} from "@/components/common/module-workbench.mjs";
import {
  DataSurface,
  WorkspaceShell,
} from "@/components/analysis/bid-analysis-v3/workspace";

const PAGE_SIZE = 12;
const LIST_URL = "/zb/review/project/list";
const SCROLL_ID = "bid-audit-scroll";

const STATUS_TABS = [
  { key: "", label: "全部", color: "primary.600" },
  { key: "running", label: "审核中", color: "info.500" },
  { key: "succeed", label: "已完成", color: "success.500" },
  { key: "failed", label: "失败", color: "error.500" },
  { key: "cancelled", label: "已取消", color: "neutral.500" },
];

const FLUID_GRID =
  "repeat(auto-fill, minmax(min(100%, clamp(280px, 24vw, 400px)), 1fr))";

export default function BidAuditListPage() {
  const toast = useToast();
  const router = useRouter();

  const [statusFilter, setStatusFilter] = useState("");
  const [keywordInput, setKeywordInput] = useState("");
  const [nameFilter, setNameFilter] = useState("");
  const [deckOpen, setDeckOpen] = useState(false);

  const [deleteTarget, setDeleteTarget] = useState<ReviewProject | null>(null);
  const {
    isOpen: createIsOpen,
    onOpen: createOnOpen,
    onClose: createOnClose,
  } = useDisclosure();
  const {
    isOpen: deleteIsOpen,
    onOpen: deleteOnOpen,
    onClose: deleteOnClose,
  } = useDisclosure();
  const [statsRefreshKey, setStatsRefreshKey] = useState(0);

  const { fetchList } = useReviewList({});
  const { deleteLoading, fetchDelete } = useReviewDelete();
  const { fetchRetryStage } = useReviewRetryStage();

  useEffect(() => {
    const t = window.setTimeout(() => setNameFilter(keywordInput.trim()), 400);
    return () => window.clearTimeout(t);
  }, [keywordInput]);

  const fetcher = useCallback(
    async (pageNum: number) => {
      const res = await fetchList({
        params: {
          pageNum,
          pageSize: PAGE_SIZE,
          status: statusFilter || undefined,
          name: nameFilter || undefined,
        },
      });
      return {
        items: (res?.data?.data?.list as ReviewProject[]) || [],
        total: res?.data?.data?.total || 0,
      };
    },
    [fetchList, statusFilter, nameFilter],
  );

  const {
    items,
    setItems,
    initialLoading,
    hasMore,
    error,
    loadFirstPage,
    loadMore,
    mergeRefresh,
  } = useInfiniteList<ReviewProject>({
    fetcher,
    pageSize: PAGE_SIZE,
    resetDeps: [statusFilter, nameFilter],
  });

  const { totals, refreshTotals } = useStatusTotals({
    statuses: STATUS_TABS.map((s) => s.key),
    keyword: nameFilter,
    url: LIST_URL,
    refreshKey: statsRefreshKey,
  });

  const hasRunning = useMemo(
    () => items.some((i) => i.status === "running"),
    [items],
  );
  useEffect(() => {
    if (!hasRunning) return;
    const timer = window.setInterval(() => {
      mergeRefresh();
      refreshTotals();
    }, 5000);
    return () => window.clearInterval(timer);
  }, [hasRunning, mergeRefresh, refreshTotals]);

  const refreshAfterMutation = useCallback(() => {
    setStatsRefreshKey((k) => k + 1);
    loadFirstPage();
  }, [loadFirstPage]);

  const handleConfirmDelete = useCallback(async () => {
    if (!deleteTarget) return;
    try {
      const res = await fetchDelete({
        params: { id: deleteTarget.id },
      });
      if (res?.data?.code === 200) {
        toast({ title: "删除成功", status: "success" });
        setDeleteTarget(null);
        deleteOnClose();
        refreshAfterMutation();
      } else {
        toast({ title: res?.data?.msg || "删除失败", status: "error" });
      }
    } catch (e: any) {
      toast({ title: e?.response?.data?.msg || "删除失败", status: "error" });
    }
  }, [
    deleteTarget,
    fetchDelete,
    toast,
    deleteOnClose,
    refreshAfterMutation,
  ]);

  // 卡片内重试：后端已把 failed + 残留 running 统一收敛为 retryable_stage
  const handleRetry = useCallback(
    async (project: ReviewProject) => {
      if (!project?.retryable_stage) return;
      try {
        const res = await fetchRetryStage({
          url: `/zb/review/project/retry-stage/${project.id}`,
          data: { stage: project.retryable_stage },
        });
        if (res?.data?.code === 200) {
          const payload = res?.data?.data || {};
          // 立即把卡片切回 running 并按基线进度重绘（进度条"重新来"），随后由轮询续上
          setItems((prev) =>
            prev.map((p) =>
              p.id === project.id
                ? {
                    ...p,
                    status: "running",
                    stage: payload.stage || project.retryable_stage,
                    progress: payload.progress ?? 0,
                    stage_status: payload.stage_status || p.stage_status,
                    retryable_stage: "",
                    last_error: "",
                  }
                : p,
            ),
          );
          toast({
            title: `已从${STAGE_LABELS[payload.stage] || payload.stage || "失败阶段"}重跑`,
            status: "success",
          });
          refreshTotals();
          mergeRefresh();
        } else {
          toast({ title: res?.data?.msg || "重试失败", status: "error" });
        }
      } catch (e: any) {
        toast({
          title: e?.response?.data?.msg || "重试失败",
          status: "error",
        });
      }
    },
    [fetchRetryStage, toast, refreshTotals, mergeRefresh, setItems],
  );

  return (
    <PageViewport
      w="full"
      id={SCROLL_ID}
      className="thin-scrollbars"
      bg="workbench.canvas"
    >
      <WorkspaceShell minH="100%">
        <ModuleWorkbenchHeader
          title="标书审核"
          activity={
            totals.running > 0
              ? `当前有 ${totals.running} 个项目审核中`
              : undefined
          }
        />

        <ModuleWorkbenchDeck
          title="在交付前，把风险逐项定位到原文"
          description="同时提交招标文件与投标文件，审核结果会按风险等级汇总，并保留原文定位依据。"
          expanded={deckOpen}
          onToggle={() => setDeckOpen((value) => !value)}
          action={
            <Flex gap={2} w={{ base: "full", md: "auto" }} direction={{ base: "column", md: "row" }}>
              <Button
                onClick={() => router.push("/bid-audit/rules")}
                minH="44px"
                px={5}
                w={{ base: "full", md: "auto" }}
                variant="outline"
                color="workbench.paper"
                borderColor="whiteAlpha.600"
                whiteSpace="nowrap"
                _hover={{ bg: "whiteAlpha.200" }}
                _focusVisible={{
                  outline: "2px solid",
                  outlineColor: "gold.300",
                  outlineOffset: "3px",
                }}
              >
                审核规则库
              </Button>
              <Button
                leftIcon={<FiPlus />}
                onClick={createOnOpen}
                minH="44px"
                px={6}
                w={{ base: "full", md: "auto" }}
                bg="workbench.paper"
                color="workbench.control"
                border="1px solid"
                borderColor="whiteAlpha.600"
                whiteSpace="nowrap"
                _hover={{ bg: "gold.50", transform: "translateY(-1px)" }}
                _active={{ transform: "translateY(0)" }}
                _focusVisible={{
                  outline: "2px solid",
                  outlineColor: "gold.300",
                  outlineOffset: "3px",
                }}
              >
                创建审核
              </Button>
            </Flex>
          }
        >
          {["合规性", "完整性", "竞争力", "暗标版式"].map((label) => (
            <Flex
              key={label}
              align="center"
              gap={1.5}
              px={2.5}
              py={1}
              borderRadius="full"
              bg="workbench.controlRaised"
            >
              <Box w="6px" h="6px" borderRadius="full" bg="gold.400" />
              <Text>{label}</Text>
            </Flex>
          ))}
        </ModuleWorkbenchDeck>

        <Flex
          mt={6}
          justify="space-between"
          align={{ base: "stretch", lg: "center" }}
          direction={{ base: "column", lg: "row" }}
          gap={3}
          mb={5}
        >
          <StatusFilterBar
            options={STATUS_TABS}
            value={statusFilter}
            counts={totals}
            onChange={setStatusFilter}
          />
          <InputGroup w={{ base: "full", lg: "320px" }} flexShrink={0}>
            <InputLeftElement h="44px" pointerEvents="none">
              <FiSearch color="var(--chakra-colors-workbench-muted)" />
            </InputLeftElement>
            <Input
              placeholder="搜索项目名称"
              value={keywordInput}
              onChange={(e) => setKeywordInput(e.target.value)}
              h="44px"
              pl="40px"
              borderRadius="10px"
              bg="workbench.paper"
              border="1px solid"
              borderColor="workbench.line"
              _hover={{ borderColor: "neutral.300" }}
              _focusVisible={{
                borderColor: "workbench.control",
                boxShadow: "none",
                outline: "2px solid",
                outlineColor: "gold.400",
                outlineOffset: "2px",
              }}
            />
          </InputGroup>
        </Flex>

        {initialLoading ? (
          <Grid templateColumns={FLUID_GRID} gap={4} alignItems="start">
            {Array.from({ length: 6 }).map((_, i) => (
              <DataSurface key={i} p={5} minH="330px">
                <Skeleton h="20px" w="42%" />
                <Skeleton mt={5} h="110px" borderRadius="11px" />
                <Skeleton mt={5} h="16px" w="72%" />
              </DataSurface>
            ))}
          </Grid>
        ) : error ? (
          <EmptyState title="加载失败" description={error} />
        ) : items.length === 0 ? (
          <EmptyState
            title="还没有审核项目"
            description="上传招标文件与投标文件，或从标书生成项目一键发起"
            action={
              <Button colorScheme="primary" size="sm" onClick={createOnOpen}>
                创建第一个审核
              </Button>
            }
          />
        ) : (
          <InfiniteScrollList
            dataLength={items.length}
            hasMore={hasMore}
            loadMore={loadMore}
            scrollableTarget={SCROLL_ID}
            loader={
              <Flex justify="center" py={4}>
                <Skeleton h="120px" w="60%" borderRadius="xl" />
              </Flex>
            }
          >
            {/* 卡片高度随内容变化（运行中带进度区块更高），按内容对齐避免拉伸出空白 */}
            <Grid templateColumns={FLUID_GRID} gap={4} alignItems="start">
              {items.map((p) => (
                <ProjectCard
                  key={p.id}
                  project={p}
                  onRetry={handleRetry}
                  onDelete={(proj) => {
                    setDeleteTarget(proj);
                    deleteOnOpen();
                  }}
                />
              ))}
            </Grid>
          </InfiniteScrollList>
        )}
      </WorkspaceShell>

      <CreateAuditModal
        isOpen={createIsOpen}
        onClose={createOnClose}
        onCreated={refreshAfterMutation}
      />
      <DeleteConfirmModal
        isOpen={deleteIsOpen}
        onClose={() => {
          setDeleteTarget(null);
          deleteOnClose();
        }}
        title="删除审核项目"
        description={
          <>
            确定删除
            <Text as="span" fontWeight="700" color="workbench.text">
              {deleteTarget?.name || ""}
            </Text>
            吗？将同时删除其检查项与操作记录，且不可恢复。
          </>
        }
        isLoading={deleteLoading}
        handleConfirm={handleConfirmDelete}
      />
    </PageViewport>
  );
}
