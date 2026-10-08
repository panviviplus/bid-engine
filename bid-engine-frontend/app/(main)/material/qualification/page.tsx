"use client";

import MaterialTypeListPage from "@/components/material/type-list-page";
import {
  useQualificationList, useQualificationAdd, useQualificationUpdate, useQualificationDelete,
} from "@/service/material";

export default function QualificationListPage() {
  const { fetchList, listLoading } = useQualificationList();
  const { fetchAdd, addLoading } = useQualificationAdd();
  const { fetchUpdate, updateLoading } = useQualificationUpdate();
  const { fetchDelete, deleteLoading } = useQualificationDelete();

  return (
    <MaterialTypeListPage
      type="qualification"
      typeName="企业资质"
      addButtonLabel="新增资质"
      fileAccept=".jpg,.jpeg,.png,.pdf,.doc,.docx"
      uploadHint="支持：JPG/JPEG/PNG（≤5MB），PDF/DOC/DOCX（≤200MB）"
      descPlaceholder="例：ISO9001 质量管理体系认证，由XX机构颁发，有效期至 2028 年"
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
