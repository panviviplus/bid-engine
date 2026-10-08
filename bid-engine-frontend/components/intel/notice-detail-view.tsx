"use client";

/* Hallmark · macrostructure: Workbench · genre: modern-minimal · theme: BidEngine deep-sea/gold (preserved tokens)
 * audience: 投标/售前团队 · use: 判断这条公告值不值得跟，并直接进入解析流程 · tone: technical, restrained
 * Hallmark · pre-emit critique: P5 H4 E5 S4 R5 V4
 */
import React, { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import {
  Badge,
  Box,
  Button,
  Divider,
  Flex,
  HStack,
  Skeleton,
  Stack,
  Tag,
  Text,
  Wrap,
  WrapItem,
  useToast,
} from "@chakra-ui/react";
import { FiExternalLink, FiStar, FiZap } from "react-icons/fi";

import EmptyState from "@/components/common/empty-state";
import { BackButton } from "@/components/common/header-controls";
import { MarkdownContent } from "@/components/analysis/bid-analysis-v3/detail/markdown-content";
import { NoticeDocumentContent } from "@/components/intel/notice-document-content";
import { formatInsightMeta } from "@/components/intel/notice-detail-meta.mjs";
import { PageViewport } from "@/components/layout/responsive-page";
import {
  DataSurface,
  WorkspaceShell,
} from "@/components/analysis/bid-analysis-v3/workspace";
import {
  useIntelFavorite,
  useIntelInsight,
  useIntelNoticeDetail,
} from "@/service/intel";
import { formatBudget, formatRegion } from "@/components/intel/notice-card";

/** 详情视图的“返回”目标：由承载它的路由决定，保证返回回得来路。 */
export type NoticeBackTarget = { href: string; label: string };

export const HALL_BACK_TARGET: NoticeBackTarget = {
  href: "/intel",
  label: "返回情报大厅",
};

/**
 * 情报管理里的公告详情。
 *
 * 独立路由 `/system/intel/notices/[id]`：这样左侧导航按路径就能定位到
 * “系统管理 → 招标情报管理”，不会因为详情页挂在 `/intel` 下而跳到情报大厅。
 */
export const ADMIN_BACK_TARGET: NoticeBackTarget = {
  href: "/system/intel?tab=notices",
  label: "返回情报管理",
};

export default function NoticeDetailView({
  noticeId,
  backTarget,
}: {
  noticeId: string;
  backTarget: NoticeBackTarget;
}) {
  const router = useRouter();
  const toast = useToast();
  const { detail, detailLoading, detailError, fetchDetail } =
    useIntelNoticeDetail(noticeId);
  const { favoriteLoading, fetchFavorite } = useIntelFavorite();
  const { insightLoading, fetchInsight } = useIntelInsight();
  const [insight, setInsight] = useState<{
    content_md: string;
    created_at: string;
  } | null>(null);

  useEffect(() => {
    if (detail?.insight) setInsight(detail.insight);
  }, [detail?.insight]);

  const handleFavorite = useCallback(
    async (next: boolean) => {
      if (!detail) return;
      try {
        await fetchFavorite({
          url: `/zb/intel/notices/${detail.id}/favorite`,
          data: { favorite: next },
        });
        fetchDetail();
      } catch (error: any) {
        toast({
          title: "操作失败",
          description: error?.response?.data?.message || "请稍后重试",
          status: "error",
          duration: 3000,
          isClosable: true,
        });
      }
    },
    [detail, fetchDetail, fetchFavorite, toast],
  );

  const handleInsight = useCallback(async () => {
    if (!detail) return;
    try {
      const res = await fetchInsight({
        url: `/zb/intel/notices/${detail.id}/insight`,
      });
      const data = res?.data?.data;
      if (data) {
        setInsight({
          content_md: data.content_md,
          created_at: data.created_at,
        });
      }
    } catch (error: any) {
      toast({
        title: "AI 解读失败",
        description:
          error?.response?.data?.message || error?.message || "请稍后重试",
        status: "error",
        duration: 4000,
        isClosable: true,
      });
    }
  }, [detail, fetchInsight, toast]);

  const handleStartParse = useCallback(() => {
    if (!detail) return;
    const search = new URLSearchParams({
      prefill_name: detail.title || "",
      prefill_url: detail.url || "",
      prefill_publisher: detail.publisher || "",
      prefill_source: "intel",
      prefill_notice_id: String(detail.id),
    });
    router.push(`/bid-analysis?${search.toString()}`);
  }, [detail, router]);

  if (detailLoading && !detail) {
    return (
      <PageViewport>
        <WorkspaceShell>
          <Stack spacing={4}>
            <Skeleton height="80px" borderRadius="14px" />
            <Skeleton height="240px" borderRadius="14px" />
          </Stack>
        </WorkspaceShell>
      </PageViewport>
    );
  }

  if (detailError || !detail) {
    return (
      <PageViewport>
        <WorkspaceShell>
          <EmptyState
            title="公告不存在或已被清理"
            description="情报按保留期自动清理，请返回列表查看其他公告。"
            action={
              <Button
                colorScheme="primary"
                onClick={() => router.push(backTarget.href)}
              >
                {backTarget.label}
              </Button>
            }
          />
        </WorkspaceShell>
      </PageViewport>
    );
  }

  return (
    <PageViewport>
      <WorkspaceShell>
        <Flex align="center" gap={3} mb={4}>
          <BackButton href={backTarget.href} label={backTarget.label} />
          <Text fontSize="sm" color="workbench.muted">
            {detail.source_name || detail.source_key}
          </Text>
        </Flex>

        <DataSurface p={{ base: 4, md: 6 }}>
          <Text
            as="h1"
            fontSize={{ base: "xl", md: "2xl" }}
            fontWeight="800"
            color="workbench.text"
            lineHeight="1.3"
            overflowWrap="anywhere"
          >
            {detail.title || "（无标题公告）"}
          </Text>

          <Wrap spacing={2} mt={3}>
            <WrapItem>
              <Badge colorScheme="primary" variant="subtle">
                {detail.notice_type_name || "其他"}
              </Badge>
            </WrapItem>
            {(detail.industry_names || []).map((name) => (
              <WrapItem key={name}>
                <Tag size="sm" variant="outline" borderColor="neutral.200">
                  {name}
                </Tag>
              </WrapItem>
            ))}
          </Wrap>

          <HStack
            spacing={5}
            mt={4}
            wrap="wrap"
            fontSize="sm"
            color="workbench.muted"
            sx={{ fontVariantNumeric: "tabular-nums" }}
          >
            <Text>预算：{formatBudget(detail)}</Text>
            <Text>地区：{formatRegion(detail)}</Text>
            <Text>发布时间：{detail.publish_date || "未标注"}</Text>
            <Text>投标截止：{detail.deadline_at || "未标注"}</Text>
          </HStack>

          {(detail.publisher || detail.agency || detail.project_code) && (
            <Stack spacing={1} mt={3} fontSize="sm" color="workbench.text">
              {detail.publisher && <Text>采购人：{detail.publisher}</Text>}
              {detail.agency && <Text>代理机构：{detail.agency}</Text>}
              {detail.project_code && (
                <Text>项目编号：{detail.project_code}</Text>
              )}
            </Stack>
          )}

          <HStack spacing={3} mt={5} wrap="wrap">
            <Button
              h="44px"
              colorScheme="primary"
              leftIcon={<FiZap aria-hidden />}
              onClick={handleStartParse}
              _active={{ transform: "scale(0.98)" }}
            >
              发起招标解析
            </Button>
            <Button
              h="44px"
              variant="outline"
              borderColor="neutral.200"
              leftIcon={<FiStar aria-hidden />}
              isLoading={favoriteLoading}
              onClick={() => handleFavorite(!detail.favorited)}
              _hover={{ borderColor: "gold.400", color: "gold.600" }}
            >
              {detail.favorited ? "已收藏" : "收藏"}
            </Button>
            <Button
              h="44px"
              as="a"
              href={detail.url}
              target="_blank"
              rel="noreferrer noopener"
              variant="outline"
              borderColor="neutral.200"
              rightIcon={<FiExternalLink aria-hidden />}
            >
              查看原文
            </Button>
          </HStack>
        </DataSurface>

        <DataSurface p={{ base: 4, md: 6 }} mt={4}>
          <Flex justify="space-between" align="center" gap={3} wrap="wrap">
            <Text fontWeight="700" color="workbench.text">
              AI 解读
            </Text>
            <Button
              h="44px"
              size="sm"
              variant="outline"
              borderColor="primary.200"
              color="primary.600"
              isLoading={insightLoading}
              loadingText="生成中"
              onClick={handleInsight}
              _hover={{ bg: "primary.50" }}
            >
              {insight ? "重新生成" : "生成解读"}
            </Button>
          </Flex>
          <Divider my={4} />
          {insight ? (
            <Box>
              <MarkdownContent content={insight.content_md} />
              <Text mt={3} fontSize="xs" color="workbench.muted">
                {formatInsightMeta(insight.created_at)}
              </Text>
            </Box>
          ) : (
            <Text fontSize="sm" color="workbench.muted">
              尚未生成解读。生成后会缓存到公告上，团队其他成员打开时直接复用。
            </Text>
          )}
        </DataSurface>

        <DataSurface p={{ base: 4, md: 6 }} mt={4}>
          <Text fontWeight="700" color="workbench.text" mb={3}>
            公告正文
          </Text>
          {detail.body_html || detail.body_markdown || detail.body_text ? (
            <NoticeDocumentContent
              html={detail.body_html}
              markdown={detail.body_markdown}
              text={detail.body_text}
            />
          ) : (
            <Text fontSize="sm" color="workbench.muted">
              未抓取到正文，请点击“查看原文”到源站查看。
            </Text>
          )}

          {detail.attachments?.length > 0 && (
            <>
              <Divider my={5} />
              <Text fontWeight="700" color="workbench.text" mb={2}>
                附件链接（仅链接，不下载）
              </Text>
              <Stack spacing={2}>
                {detail.attachments.map((item) => (
                  <Text
                    key={item.url}
                    as="a"
                    href={item.url}
                    target="_blank"
                    rel="noreferrer noopener"
                    fontSize="sm"
                    color="primary.600"
                    minH="32px"
                    display="inline-flex"
                    alignItems="center"
                    overflowWrap="anywhere"
                    _hover={{ textDecoration: "underline" }}
                  >
                    {item.name || item.url}
                  </Text>
                ))}
              </Stack>
            </>
          )}

          {detail.extract_warnings?.length > 0 && (
            <Text mt={5} fontSize="xs" color="workbench.muted">
              抽取告警：{detail.extract_warnings.join("；")}
            </Text>
          )}
        </DataSurface>
      </WorkspaceShell>
    </PageViewport>
  );
}
