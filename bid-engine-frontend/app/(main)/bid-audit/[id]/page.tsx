"use client";

import React, { useEffect } from "react";
import { useParams } from "next/navigation";
import { Box } from "@chakra-ui/react";
import AuditDetail from "@/components/audit/detail";
import { useSidebarCollapse } from "@/components/layout";

export default function BidAuditDetailPage() {
  const params = useParams();
  const id = (params?.id as string) || "";
  const { collapseForDetail, restoreSidebar } = useSidebarCollapse();

  // 进入详情页自动收起左侧菜单，为双栏审核工作区留出空间；离开时恢复进入前状态。
  useEffect(() => {
    collapseForDetail();
    return restoreSidebar;
  }, [collapseForDetail, restoreSidebar]);

  return (
    // 详情页内容（头部 + 任务阶段 + 总览 + 双栏工作区）可能超出视口高度，整页允许纵向滚动
    <Box
      h="100%"
      overflowY="auto"
      overflowX="hidden"
      className="thin-scrollbars"
      bg="workbench.canvas"
    >
      <AuditDetail projectId={id} />
    </Box>
  );
}
