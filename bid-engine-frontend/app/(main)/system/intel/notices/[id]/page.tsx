"use client";

/**
 * 系统管理 → 招标情报管理 → 情报管理 的公告详情路由。
 *
 * 与情报大厅共用同一套详情视图，但路由挂在 `/system/intel` 下：
 *   - 左侧导航按路径定位到“系统管理 → 招标情报管理”，不会跳到情报大厅；
 *   - 返回按钮回到情报管理子 tab，管理上下文不丢失。
 */
import React from "react";
import { useParams } from "next/navigation";

import NoticeDetailView, {
  ADMIN_BACK_TARGET,
} from "@/components/intel/notice-detail-view";

export default function AdminNoticeDetailPage() {
  const params = useParams();
  const noticeId = params?.id as string;
  return (
    <NoticeDetailView noticeId={noticeId} backTarget={ADMIN_BACK_TARGET} />
  );
}
