import useAxios from "axios-hooks";

export type HomeModuleKey =
  | "bid_analysis"
  | "bid_generation"
  | "bid_review"
  | "material"
  | string;

export type HomePeriod = "today" | "week" | "month" | "year";

export interface HomeModuleStat {
  count: number;
  attention_count: number;
  breakdown?: Record<string, number>;
}

/** 招标情报站卡片：全平台共享库 + 当前用户个人关注数。 */
export interface HomeIntelStat {
  today_new: number;
  unread_alerts: number;
  subscription_count: number;
}

export interface HomeStatsData {
  period: HomePeriod;
  modules: Record<HomeModuleKey, HomeModuleStat>;
  tender_intel?: HomeIntelStat;
}

export interface HomeWorkItem {
  module: HomeModuleKey;
  id: number;
  name: string;
  status: string;
  stage: string;
  progress: number;
  updated_at: string;
  warning_count: number;
  error_count: number;
  attention_type?: "failed" | "risk" | "action_required" | "running";
}

export interface HomeLLMConfigStatus {
  modules: Record<HomeModuleKey, boolean>;
  missing_modules: HomeModuleKey[];
}

export const useHomeStats = (period: HomePeriod) => {
  const [{ data, loading, error }, refresh] = useAxios(
    { url: "/home/stats", method: "GET", params: { period } },
    { useCache: false },
  );
  return {
    statsData: data?.data as HomeStatsData | undefined,
    statsLoading: loading,
    statsError: error,
    refreshStats: refresh,
  };
};

export const useHomeRecentWork = (limit = 6) => {
  const [{ data, loading, error }, refresh] = useAxios(
    { url: "/home/recent-work", method: "GET", params: { limit } },
    { useCache: false },
  );
  return {
    recentItems: (data?.data?.items || []) as HomeWorkItem[],
    recentLoading: loading,
    recentError: error,
    refreshRecent: refresh,
  };
};

export const useHomeAttention = (limit = 6) => {
  const [{ data, loading, error }, refresh] = useAxios(
    { url: "/home/attention", method: "GET", params: { limit } },
    { useCache: false },
  );
  return {
    attentionItems: (data?.data?.items || []) as HomeWorkItem[],
    attentionLoading: loading,
    attentionError: error,
    refreshAttention: refresh,
  };
};

export const useHomeLLMConfigStatus = () => {
  const [{ data, loading, error }, refresh] = useAxios(
    { url: "/home/llm-config-status", method: "GET" },
    { useCache: false },
  );
  return {
    llmStatus: data?.data as HomeLLMConfigStatus | undefined,
    llmStatusLoading: loading,
    llmStatusError: error,
    refreshLLMStatus: refresh,
  };
};
