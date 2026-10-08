import { useEffect } from "react";
import useAxios from "axios-hooks";

// ================================================================
// 标书审核模块（V2）接口
// ================================================================

// ===== 项目列表 =====
export const useReviewList = (params) => {
  const [{ data, loading, error }, fetchList] = useAxios(
    { url: "/zb/review/project/list", method: "GET", params },
    { useCache: false, manual: true },
  );
  return {
    listData: data?.data || { list: [], total: 0 },
    listError: error,
    listLoading: loading,
    fetchList,
  };
};

// ===== 上传创建（multipart）=====
export const useReviewCreate = () => {
  const [{ loading, error }, fetchCreate] = useAxios(
    { url: "/zb/review/project/create", method: "POST" },
    { manual: true },
  );
  return { createError: error, createLoading: loading, fetchCreate };
};

// ===== 从标书生成项目创建（multipart）=====
export const useReviewCreateFromGen = () => {
  const [{ loading, error }, fetchCreate] = useAxios(
    { url: "/zb/review/project/create-from-gen", method: "POST" },
    { manual: true },
  );
  return { createError: error, createLoading: loading, fetchCreate };
};

// ===== 项目详情 =====
export const useReviewDetail = (projectId) => {
  const [{ data, loading, error }, fetchDetail] = useAxios(
    {
      url: "/zb/review/project/detail",
      method: "GET",
      params: { project_id: projectId },
    },
    { useCache: false, manual: true },
  );
  useEffect(() => {
    if (projectId) {
      fetchDetail();
    }
  }, [projectId, fetchDetail]);
  return {
    detailData: data?.data || null,
    detailError: error,
    detailLoading: loading,
    fetchDetail,
  };
};

// ===== 删除项目 =====
export const useReviewDelete = () => {
  const [{ loading, error }, fetchDelete] = useAxios(
    { url: "/zb/review/project/delete", method: "DELETE" },
    { manual: true },
  );
  return { deleteError: error, deleteLoading: loading, fetchDelete };
};

// ===== 阶段重试 =====
export const useReviewRetryStage = () => {
  const [{ loading, error }, fetchRetryStage] = useAxios(
    { url: "", method: "POST" },
    { manual: true },
  );
  return { retryError: error, retryLoading: loading, fetchRetryStage };
};

// ===== 暗标评审开关 =====
export const useReviewAnonymousToggle = () => {
  const [{ loading, error }, fetchToggle] = useAxios(
    { url: "", method: "POST" },
    { manual: true },
  );
  return { anonymousError: error, anonymousLoading: loading, fetchToggle };
};

// ===== 清单项 =====
export const useReviewItemAdd = () => {
  const [{ loading, error }, fetchAdd] = useAxios(
    { url: "/zb/review/item/add", method: "POST" },
    { manual: true },
  );
  return { addError: error, addLoading: loading, fetchAdd };
};

export const useReviewItemUpdate = () => {
  const [{ loading, error }, fetchUpdate] = useAxios(
    { url: "/zb/review/item/update", method: "POST" },
    { manual: true },
  );
  return { updateError: error, updateLoading: loading, fetchUpdate };
};

export const useReviewItemRecheck = () => {
  const [{ loading, error }, fetchRecheck] = useAxios(
    { url: "/zb/review/item/recheck", method: "POST" },
    { manual: true },
  );
  return { recheckError: error, recheckLoading: loading, fetchRecheck };
};

export const useReviewItemDelete = () => {
  const [{ loading, error }, fetchDelete] = useAxios(
    { url: "/zb/review/item/delete", method: "DELETE" },
    { manual: true },
  );
  return { deleteItemError: error, deleteItemLoading: loading, fetchDelete };
};

// ===== 整改闭环 =====
export const useRemediationUpdate = () => {
  const [{ loading, error }, fetchUpdate] = useAxios(
    { url: "/zb/review/remediation/update", method: "POST" },
    { manual: true },
  );
  return { remediationError: error, remediationLoading: loading, fetchUpdate };
};

// ===== 企业规则库 =====
export const useReviewRules = (params) => {
  const [{ data, loading, error }, fetchRules] = useAxios(
    { url: "/zb/review/rule/list", method: "GET", params },
    { useCache: false, manual: true },
  );
  return {
    rulesData: data?.data || { list: [], total: 0 },
    rulesError: error,
    rulesLoading: loading,
    fetchRules,
  };
};

export const useRuleSave = () => {
  const [{ loading, error }, fetchSave] = useAxios(
    { url: "/zb/review/rule/save", method: "POST" },
    { manual: true },
  );
  return { saveError: error, saveLoading: loading, fetchSave };
};

export const useRuleDelete = () => {
  const [{ loading, error }, fetchDelete] = useAxios(
    { url: "/zb/review/rule/delete", method: "DELETE" },
    { manual: true },
  );
  return { ruleDeleteError: error, ruleDeleteLoading: loading, fetchDelete };
};

export const useRuleFromItem = () => {
  const [{ loading, error }, fetchSave] = useAxios(
    { url: "/zb/review/rule/from-item", method: "POST" },
    { manual: true },
  );
  return { ruleFromItemError: error, ruleFromItemLoading: loading, fetchSave };
};

// ===== 导出报告 =====
export const useReviewExportReport = () => {
  const [{ loading, error }, fetchExport] = useAxios(
    { url: "/zb/review/export/report", method: "POST" },
    { manual: true },
  );
  return { exportError: error, exportLoading: loading, fetchExport };
};
