"use client";

import MaterialTypeListPage from "@/components/material/type-list-page";
import {
  useTemplateList, useTemplateAdd, useTemplateUpdate, useTemplateDelete,
} from "@/service/material";

export default function TemplateListPage() {
  const { fetchList, listLoading } = useTemplateList();
  const { fetchAdd, addLoading } = useTemplateAdd();
  const { fetchUpdate, updateLoading } = useTemplateUpdate();
  const { fetchDelete, deleteLoading } = useTemplateDelete();

  return (
    <MaterialTypeListPage
      type="template"
      typeName="文档模板"
      addButtonLabel="新增素材"
      fileAccept=".pdf,.doc,.docx"
      uploadHint="支持：PDF/DOC/DOCX（≤200MB）"
      descPlaceholder="例：技术方案模板，适用于 XX 行业"
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
