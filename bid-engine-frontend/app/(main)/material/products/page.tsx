"use client";

import PageHeader from "@/components/common/page-header";
import Placeholder from "@/components/common/placeholder";
import {
  PageContent,
  PageViewport,
} from "@/components/layout/responsive-page";

export default function ProductsPage() {
  return (
    <PageViewport bg="workbench.canvas">
      <PageContent py={{ base: 5, md: 6 }}>
        <PageHeader
          title="产品库"
          description="管理投标产品清单，AI 生成标书时自动引用匹配产品参数"
        />
        <Placeholder
          title="产品库 — 即将上线"
          description="上传产品清单（名称、型号、参数），AI 生成标书时将自动引用匹配的产品信息，确保技术参数准确一致。"
          icon="coming-soon"
        />
      </PageContent>
    </PageViewport>
  );
}
