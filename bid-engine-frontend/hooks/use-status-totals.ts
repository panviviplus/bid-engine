"use client";

import axios from "axios";
import { useCallback, useEffect, useRef, useState } from "react";

// 统计条使用独立 axios 实例（与 providers 配置一致），避免与列表 hook 共享状态
const statsClient = axios.create({ baseURL: "/api", withCredentials: true });

export interface UseStatusTotalsOptions {
  /** 状态 key 列表，"" 表示全部 */
  statuses: string[];
  /** 当前搜索关键词（统计数字跟随关键词） */
  keyword?: string;
  /** list 接口路径，如 "/zb/v3/projects" */
  url: string;
  /** 变化时强制刷新（如创建/删除后自增） */
  refreshKey?: number;
  enabled?: boolean;
}

/**
 * 各状态真实总数：对每个状态用 list 接口（pageSize=1）并行取 total。
 * keyword / refreshKey 变化或主动 refresh 时刷新；不随当前状态筛选变化。
 * 请求竞态用 seq 版本号防护。
 */
export function useStatusTotals({
  statuses,
  keyword,
  url,
  refreshKey = 0,
  enabled = true,
}: UseStatusTotalsOptions) {
  const [totals, setTotals] = useState<Record<string, number>>({});
  const [loading, setLoading] = useState(false);
  const seqRef = useRef(0);
  const statusesKey = statuses.join(",");

  const refresh = useCallback(async () => {
    if (!enabled) return;
    const seq = seqRef.current + 1;
    seqRef.current = seq;
    setLoading(true);
    try {
      const results = await Promise.all(
        statuses.map(async (status) => {
          const res = await statsClient.get(url, {
            params: {
              pageNum: 1,
              pageSize: 1,
              status: status || undefined,
              name: keyword || undefined,
            },
          });
          return { status, total: res?.data?.data?.total ?? 0 };
        }),
      );
      if (seq !== seqRef.current) return;
      const next: Record<string, number> = {};
      results.forEach((r) => {
        next[r.status] = r.total;
      });
      setTotals(next);
    } catch {
      // 统计失败保留上一次数据，不打断列表
    } finally {
      if (seq === seqRef.current) setLoading(false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [statusesKey, keyword, url, enabled]);

  useEffect(() => {
    if (enabled) refresh();
  }, [enabled, refreshKey, refresh]);

  return { totals, totalsLoading: loading, refreshTotals: refresh };
}
