/* eslint-disable no-unused-vars */
import { useEffect, useState } from "react";
import useAxios from "axios-hooks";

// ===== 模块 LLM 配置存在性检测（纯 DB 查询，无 LLM 调用，不消耗 Token）=====
export const useLlmConfigExist = (module: string) => {
  const [{ data, loading, error }, fetchExist] = useAxios(
    {
      url: "/system/llm-config/exist",
      method: "GET",
      params: { module },
    },
    { useCache: false, manual: true },
  );
  const [exists, setExists] = useState<boolean | null>(null);

  useEffect(() => {
    if (!module) return;
    let cancelled = false;
    fetchExist()
      .then((res: any) => {
        if (cancelled) return;
        setExists(Boolean(res?.data?.data?.exists));
      })
      .catch(() => {
        if (cancelled) return;
        setExists(null); // 检测失败不打扰用户
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [module]);

  return { exists, checking: loading, checkError: error };
};

// ===== 获取模块 LLM 配置（脱敏）=====
export const useLlmConfigs = () => {
  const [{ data, loading, error }, fetchConfigs] = useAxios(
    { url: "/system/llm-config", method: "GET" },
    { useCache: false, manual: true },
  );
  return {
    configs: data?.data || null,
    configLoading: loading,
    configError: error,
    fetchConfigs,
  };
};
