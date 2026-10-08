"use client";

import { useCallback, useEffect, useRef, useState } from "react";

export interface InfiniteListFetcherResult<T> {
  items: T[];
  total: number;
}

export interface UseInfiniteListOptions<T> {
  /** 拉取第 pageNum 页（1-based），返回该页 items 与总条数 */
  // eslint-disable-next-line no-unused-vars
  fetcher: (_pageNum: number) => Promise<InfiniteListFetcherResult<T>>;
  pageSize?: number;
  /** 这些值变化时重置到第 1 页并重新加载（如筛选条件） */
  resetDeps?: unknown[];
  enabled?: boolean;
}

/**
 * 统一错误文案口径：招标解析用 `msg`，招标情报站等新模块用 `message`。
 * 这里两者都认，避免列表把后端真实原因吞掉换成兜底文案。
 */
function resolveErrorMessage(error: any, fallback: string): string {
  return (
    error?.response?.data?.msg ||
    error?.response?.data?.message ||
    error?.message ||
    fallback
  );
}

/**
 * 长画布无限滚动列表状态机（招标解析 / 标书生成共用）：
 * - 首次 / 筛选变化 → 加载第 1 页并重置累积（loadedIds 去重集合同步重置）
 * - loadMore → 追加下一页，按 id 去重
 * - mergeRefresh → 轮询刷新已加载全部页并按 id 合并（保持滚动位置，不重置）
 * 竞态防护：seq 版本号，reset 后过期请求的结果直接丢弃。
 */
export function useInfiniteList<T extends { id: number | string }>({
  fetcher,
  pageSize = 12,
  resetDeps = [],
  enabled = true,
}: UseInfiniteListOptions<T>) {
  const [items, setItems] = useState<T[]>([]);
  const [total, setTotal] = useState(0);
  const [pageNum, setPageNum] = useState(1);
  const [initialLoading, setInitialLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadedIdsRef = useRef<Set<string | number>>(new Set());
  const seqRef = useRef(0);
  const pageNumRef = useRef(1);
  const pageSizeRef = useRef(pageSize);
  pageSizeRef.current = pageSize;

  const loadFirstPage = useCallback(async () => {
    const seq = seqRef.current + 1;
    seqRef.current = seq;
    setInitialLoading(true);
    setError(null);
    try {
      const res = await fetcher(1);
      if (seq !== seqRef.current) return;
      loadedIdsRef.current = new Set(res.items.map((i) => i.id));
      setItems(res.items);
      setTotal(res.total);
      setPageNum(1);
      pageNumRef.current = 1;
      setHasMore(
        res.items.length >= pageSizeRef.current && res.total > res.items.length,
      );
    } catch (e: any) {
      if (seq !== seqRef.current) return;
      setError(resolveErrorMessage(e, "加载失败，请稍后重试"));
      setItems([]);
      setTotal(0);
      setHasMore(false);
    } finally {
      if (seq === seqRef.current) setInitialLoading(false);
    }
  }, [fetcher]);

  // 首次加载 + 筛选重置
  useEffect(() => {
    if (!enabled) return;
    loadFirstPage();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, loadFirstPage, ...resetDeps]);

  const loadMore = useCallback(async () => {
    if (loadingMore || !hasMore) return;
    const seq = seqRef.current;
    const nextPage = pageNumRef.current + 1;
    setLoadingMore(true);
    // 重试时先清掉上一次的失败文案，成功后不应继续显示错误态
    setError(null);
    try {
      const res = await fetcher(nextPage);
      if (seq !== seqRef.current) return;
      const added = res.items.filter((i) => !loadedIdsRef.current.has(i.id));
      added.forEach((i) => loadedIdsRef.current.add(i.id));
      setItems((prev) => [...prev, ...added]);
      setTotal(res.total);
      setPageNum(nextPage);
      pageNumRef.current = nextPage;
      setHasMore(
        res.items.length >= pageSizeRef.current &&
          loadedIdsRef.current.size < res.total,
      );
    } catch (e: any) {
      if (seq !== seqRef.current) return;
      setError(resolveErrorMessage(e, "加载更多失败，请稍后重试"));
    } finally {
      if (seq === seqRef.current) setLoadingMore(false);
    }
  }, [loadingMore, hasMore, fetcher]);

  // 轮询刷新：拉取已加载的全部页并按 id 合并状态/进度
  const mergeRefresh = useCallback(async () => {
    const seq = seqRef.current;
    const pages = pageNumRef.current;
    if (pages <= 0) return;
    try {
      const results = await Promise.all(
        Array.from({ length: pages }, (_, i) => fetcher(i + 1)),
      );
      if (seq !== seqRef.current) return;
      const byId = new Map<string | number, T>();
      results.forEach((r) => r.items.forEach((it) => byId.set(it.id, it)));
      setItems((prev) => prev.map((p) => byId.get(p.id) ?? p));
      const first = results[0];
      if (first) setTotal(first.total);
    } catch {
      // 轮询失败静默保留现有数据
    }
  }, [fetcher]);

  return {
    items,
    setItems,
    total,
    pageNum,
    initialLoading,
    loadingMore,
    hasMore,
    error,
    loadFirstPage,
    loadMore,
    mergeRefresh,
  };
}
