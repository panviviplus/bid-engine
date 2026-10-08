"use client";

import MaterialTypeListPage from "@/components/material/type-list-page";
import {
  usePerformanceList, usePerformanceAdd, usePerformanceUpdate, usePerformanceDelete,
} from "@/service/material";

export default function PerformanceListPage() {
  const { fetchList, listLoading } = usePerformanceList();
  const { fetchAdd, addLoading } = usePerformanceAdd();
  const { fetchUpdate, updateLoading } = usePerformanceUpdate();
  const { fetchDelete, deleteLoading } = usePerformanceDelete();

  return (
    <MaterialTypeListPage
      type="performance"
      typeName="企业业绩"
      addButtonLabel="新增业绩"
      fileAccept=".jpg,.jpeg,.png,.pdf,.doc,.docx"
      uploadHint="支持：JPG/JPEG/PNG（≤5MB），PDF/DOC/DOCX（≤200MB）"
      descPlaceholder="例：XX智慧城市项目，合同金额 500 万元，2024 年 6 月完成"
      fetchList={fetchList}
      listLoading={listLoading}
      fetchAdd={fetchAdd}
      addLoading={addLoading}
      fetchUpdate={fetchUpdate}
      updateLoading={updateLoading}
      fetchDelete={fetchDelete}
      deleteLoading={deleteLoading}
    />
  );
}
