"use client";

/**
 * 情报大厅的公告详情路由。
 *
 * 返回目标按来路解析：情报大厅 / 订阅与提醒。情报管理有自己的详情路由
 * （`/system/intel/notices/[id]`），避免左侧导航被带到情报大厅。
 */
import React, { useEffect, useState } from "react";
import { useParams } from "next/navigation";

import NoticeDetailView, {
  HALL_BACK_TARGET,
  type NoticeBackTarget,
} from "@/components/intel/notice-detail-view";

/**
 * 订阅与提醒页的返回目标。
 *
 * 从提醒抽屉进详情时会带上 `subscription=<id>`，返回后据此自动重开该订阅的抽屉，
 * 用户不必再手动找一遍刚才看的订阅。
 */
function subscriptionsBackTarget(subscriptionId: string): NoticeBackTarget {
  const id = Number(subscriptionId);
  const href =
    Number.isFinite(id) && id > 0
      ? `/intel/subscriptions?subscription=${id}`
      : "/intel/subscriptions";
  return { href, label: "返回订阅与提醒" };
}

/** 来自各个入口的返回目标；`hall` 为默认值。 */
const FROM_TARGETS: Record<
  string,
  // eslint-disable-next-line no-unused-vars
  (_subscriptionId: string) => NoticeBackTarget
> = {
  hall: () => HALL_BACK_TARGET,
  // 提醒中心已并入订阅与提醒；alerts 保留为历史链接的兼容别名
  subscriptions: subscriptionsBackTarget,
  alerts: subscriptionsBackTarget,
  // 兼容历史链接（管理端详情已迁到 /system/intel/notices）
  "system-intel": () => ({
    href: "/system/intel?tab=notices",
    label: "返回情报管理",
  }),
};

export default function IntelNoticeDetailPage() {
  const params = useParams();
  const noticeId = params?.id as string;
  // 首屏用默认目标（与 SSR 一致，避免 hydration 不一致），挂载后按来路纠正
  const [backTarget, setBackTarget] =
    useState<NoticeBackTarget>(HALL_BACK_TARGET);
  useEffect(() => {
    if (typeof window === "undefined") return;
    const search = new URLSearchParams(window.location.search);
    const from = search.get("from") || "";
    const resolve = FROM_TARGETS[from];
    setBackTarget(
      resolve ? resolve(search.get("subscription") || "") : HALL_BACK_TARGET,
    );
    // noticeId 变化意味着又进了一次详情页，需要按新的来路重新解析
  }, [noticeId]);

  return <NoticeDetailView noticeId={noticeId} backTarget={backTarget} />;
}
