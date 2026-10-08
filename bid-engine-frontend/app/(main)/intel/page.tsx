"use client";

/* eslint-disable no-nested-ternary */

/* Hallmark · macrostructure: Workbench · genre: modern-minimal · theme: BidEngine deep-sea/gold (preserved tokens)
 * audience: 投标/售前团队 · use: 从全网招标公告中筛出值得跟进的机会 · tone: technical, restrained
 * motion: hover lift + press scale（≤200ms） · 无虚构指标 · 窄屏单列
 * Hallmark · pre-emit critique: P5 H4 E5 S5 R5 V4
 */
import React, { useCallback, useState } from "react";
import {
  Box,
  Button,
  Flex,
  Grid,
  Skeleton,
  Text,
  useToast,
} from "@chakra-ui/react";
import { FiInbox } from "react-icons/fi";

import EmptyState from "@/components/common/empty-state";
import InfiniteScrollList from "@/components/common/infinite-scroll";
import ScrollToTopButton from "@/components/common/scroll-to-top";
import { useInfiniteList } from "@/hooks/use-infinite-list";
import { PageViewport } from "@/components/layout/responsive-page";
import { WorkspaceShell } from "@/components/analysis/bid-analysis-v3/workspace";
import { ModuleWorkbenchHeader } from "@/components/common/module-workbench.mjs";
import IntelFilterBar, {
  EMPTY_FILTER_VALUE,
  countActiveFilters,
  toNoticeQuery,
  type IntelFilterValue,
} from "@/components/intel/filter-bar";
import IntelNoticeCard from "@/components/intel/notice-card";
import {
  useDebouncedValue,
  useIntelFavorite,
  useIntelFilters,
  useIntelNotices,
  useIntelUnreadCount,
  type IntelNotice,
} from "@/service/intel";

const PAGE_SIZE = 12;

// 滑动分页（隐式分页）需要绑定页面自身的滚动容器
const HALL_SCROLL_ID = "intel-hall-scroll";

export default function IntelHallPage() {
  const toast = useToast();
  const [filterValue, setFilterValue] = useState<IntelFilterValue>({
    ...EMPTY_FILTER_VALUE,
  });
  const { filters, filtersLoading } = useIntelFilters();
  const { fetchNotices } = useIntelNotices({}, false);
  const { favoriteLoading, fetchFavorite } = useIntelFavorite();
  const { unread } = useIntelUnreadCount();

  const [favoriteBusyId, setFavoriteBusyId] = useState<number | null>(null);

  // 所有筛选条件都自动生效：统一防抖 350ms，避免用户每敲一个字就打一次接口。
  const appliedFilters = useDebouncedValue(filterValue, 350);

  /**
   * 滑动分页的数据源：每次请求一页，由 useInfiniteList 负责累积与去重。
   *
   * 筛选条件变化时 fetcher 身份变化 → hook 自动回到第 1 页重新加载。
   */
  const fetchPage = useCallback(
    async (targetPage: number) => {
      const res = await fetchNotices({
        params: toNoticeQuery(appliedFilters, targetPage, PAGE_SIZE),
      });
      return {
        items: (res?.data?.data?.list || []) as IntelNotice[],
        total: (res?.data?.data?.total || 0) as number,
      };
    },
    [fetchNotices, appliedFilters],
  );

  const {
    items,
    setItems,
    total,
    initialLoading,
    loadingMore,
    hasMore,
    error: loadError,
    loadFirstPage,
    loadMore,
  } = useInfiniteList<IntelNotice>({
    fetcher: fetchPage,
    pageSize: PAGE_SIZE,
    resetDeps: [appliedFilters],
  });

  const handleFilterChange = (next: IntelFilterValue) => {
    setFilterValue(next);
  };

  const activeCount = countActiveFilters(appliedFilters);

  const resetFilters = () => {
    setFilterValue({ ...EMPTY_FILTER_VALUE });
  };

  const handleToggleFavorite = async (notice: IntelNotice, next: boolean) => {
    setFavoriteBusyId(notice.id);
    try {
      await fetchFavorite({
        url: `/zb/intel/notices/${notice.id}/favorite`,
        data: { favorite: next },
      });
      if (appliedFilters.favoriteOnly && !next) {
        setItems((prev) => prev.filter((item) => item.id !== notice.id));
      } else {
        setItems((prev) =>
          prev.map((item) =>
            item.id === notice.id ? { ...item, favorited: next } : item,
          ),
        );
      }
    } catch (error: any) {
      toast({
        title: "操作失败",
        description: error?.response?.data?.message || "请稍后重试",
        status: "error",
        duration: 3000,
        isClosable: true,
      });
    } finally {
      setFavoriteBusyId(null);
    }
  };

  return (
    <PageViewport id={HALL_SCROLL_ID}>
      <ScrollToTopButton targetId={HALL_SCROLL_ID} />
      {/* 与招标解析 / 标书生成 / 标书审核统一的外壳：同底色、同内边距 */}
      <WorkspaceShell>
        <ModuleWorkbenchHeader title="招标情报站" titleSuffix={null} />

        <Flex
          mb={4}
          gap={4}
          wrap="wrap"
          fontSize="sm"
          color="workbench.muted"
          sx={{ fontVariantNumeric: "tabular-nums" }}
        >
          <Text>全站情报：{total} 条（当前筛选）</Text>
          <Text>我的未读提醒：{unread} 条</Text>
        </Flex>

        <IntelFilterBar
          filters={filters}
          filtersLoading={filtersLoading}
          value={filterValue}
          onChange={handleFilterChange}
          loading={initialLoading || loadingMore}
          onReset={resetFilters}
          collapsible
        />

        {initialLoading && items.length === 0 ? (
          <Grid
            templateColumns={{
              base: "1fr",
              md: "repeat(auto-fill, minmax(320px, 1fr))",
            }}
            gap={4}
          >
            {[0, 1, 2, 3].map((key) => (
              <Skeleton key={key} height="180px" borderRadius="14px" />
            ))}
          </Grid>
        ) : loadError && items.length === 0 ? (
          <EmptyState
            icon={<FiInbox aria-hidden />}
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
            icon={<FiInbox aria-hidden />}
            title={activeCount > 0 ? "没有符合条件的情报" : "还没有采集到情报"}
            description={
              activeCount > 0
                ? "试着放宽行业、地区或预算条件，或清空筛选后重试。"
                : "采集服务按每日多轮自动抓取，稍后再来看看。"
            }
            action={
              activeCount > 0 ? (
                <Button variant="outline" onClick={resetFilters}>
                  清空筛选
                </Button>
              ) : undefined
            }
          />
        ) : (
          /* 滑动分页：滚到底部自动加载下一页，加载完显示“已加载全部” */
          <InfiniteScrollList
            dataLength={items.length}
            hasMore={hasMore && !loadError}
            loadMore={loadMore}
            scrollableTarget={HALL_SCROLL_ID}
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
              {items.map((notice) => (
                <IntelNoticeCard
                  key={notice.id}
                  notice={notice}
                  detailFrom="hall"
                  onToggleFavorite={handleToggleFavorite}
                  favoriteLoading={
                    favoriteLoading && favoriteBusyId === notice.id
                  }
                />
              ))}
            </Grid>
          </InfiniteScrollList>
        )}

        <Box mt={4}>
          <Text fontSize="xs" color="workbench.muted">
            共 {total} 条 ·
            情报来自公开招标网站，仅供参考；投标前请以官网公告原文为准。
          </Text>
        </Box>
      </WorkspaceShell>
    </PageViewport>
  );
}
