"use client";

/* eslint-disable no-nested-ternary */

/* Hallmark · component: intel-alert-drawer · genre: modern-minimal · theme: BidEngine workbench (preserved tokens)
 * states: default · hover · focus-visible · active · disabled · loading · empty · error
 * 交互：点击提醒卡片即标记已读并进入情报详情；“设为未读”“删除提醒”为卡片内显式操作。
 * 说明：合并“我的订阅 / 提醒中心”后，提醒按订阅收敛在这个右侧抽屉里查看。
 * Hallmark · pre-emit critique: P5 H4 E5 S5 R5 V4
 */
import React, { useCallback, useState } from "react";
import { useRouter } from "next/navigation";
import {
  Badge,
  Box,
  Button,
  Drawer,
  DrawerBody,
  DrawerCloseButton,
  DrawerContent,
  DrawerHeader,
  DrawerOverlay,
  Flex,
  HStack,
  Skeleton,
  Stack,
  Switch,
  Tag,
  Text,
  Wrap,
  WrapItem,
  useDisclosure,
  useToast,
} from "@chakra-ui/react";
import NextLink from "next/link";
import { FiBell, FiCheck, FiTrash2 } from "react-icons/fi";

import EmptyState from "@/components/common/empty-state";
import DeleteConfirmModal from "@/components/common/delete-confirm-modal";
import InfiniteScrollList from "@/components/common/infinite-scroll";
import { useInfiniteList } from "@/hooks/use-infinite-list";
import {
  EMPTY_ALERTS,
  useIntelAlertDelete,
  useIntelAlerts,
  useIntelAlertStatus,
  useIntelUnreadCount,
  type IntelAlert,
  type IntelSubscription,
} from "@/service/intel";

const PAGE_SIZE = 20;

/** 抽屉正文的滚动容器 id：无限滚动与回到顶部都要绑定到它。 */
export const ALERT_DRAWER_SCROLL_ID = "intel-alert-drawer-scroll";

/**
 * 订阅卡片的提醒抽屉。
 *
 * 打开时按订阅拉取提醒并滚动加载；点击卡片标记已读并跳详情，
 * 详情页以 `from=subscriptions&subscription=<id>` 回传，返回后自动重开本抽屉。
 */
export default function AlertDrawer({
  subscription,
  isOpen,
  onClose,
  onChanged,
}: {
  subscription: IntelSubscription | null;
  isOpen: boolean;
  onClose: () => void;
  /** 提醒状态变更后的回调：刷新订阅列表未读数与导航角标 */
  onChanged?: () => void;
}) {
  const toast = useToast();
  const router = useRouter();
  const subId = subscription?.id ?? 0;
  const [unreadOnly, setUnreadOnly] = useState(false);
  const deleteDialog = useDisclosure();
  const [deleteTarget, setDeleteTarget] = useState<IntelAlert | null>(null);

  const { fetchAlerts } = useIntelAlerts();
  const { statusLoading, fetchStatus } = useIntelAlertStatus();
  const { deleteLoading, fetchDelete } = useIntelAlertDelete();
  // 抽屉关闭时退回全局键，避免为一个没人看的订阅持续轮询
  const { unread, refreshUnread } = useIntelUnreadCount(
    30000,
    isOpen ? subId : undefined,
  );

  const fetchPage = useCallback(
    async (pageNum: number) => {
      const res = await fetchAlerts({
        params: {
          subscription_id: subId,
          unread_only: unreadOnly,
          pageNum,
          pageSize: PAGE_SIZE,
        },
      });
      return {
        items: (res?.data?.data?.list ?? EMPTY_ALERTS) as IntelAlert[],
        total: (res?.data?.data?.total || 0) as number,
      };
    },
    [fetchAlerts, subId, unreadOnly],
  );

  const {
    items,
    setItems,
    initialLoading,
    loadingMore,
    hasMore,
    error,
    loadFirstPage,
    loadMore,
  } = useInfiniteList<IntelAlert>({
    fetcher: fetchPage,
    pageSize: PAGE_SIZE,
    resetDeps: [subId, unreadOnly],
    enabled: isOpen && subId > 0,
  });

  /** 变更后同步抽屉头部的订阅未读与父级（订阅卡片 / 导航角标）。 */
  const syncCounters = useCallback(() => {
    refreshUnread();
    onChanged?.();
  }, [refreshUnread, onChanged]);

  const markRead = useCallback(
    async (alert: IntelAlert) => {
      if (alert.is_read) return;
      try {
        await fetchStatus({ data: { ids: [alert.id], is_read: true } });
        // 本地就地更新：避免整表重载造成滚动位置跳回顶部
        setItems((prev) =>
          unreadOnly
            ? prev.filter((item) => item.id !== alert.id)
            : prev.map((item) =>
                item.id === alert.id ? { ...item, is_read: true } : item,
              ),
        );
        syncCounters();
      } catch {
        // 标记失败不打断“看情报”的主流程，下次打开抽屉会以服务端状态为准
      }
    },
    [fetchStatus, setItems, syncCounters, unreadOnly],
  );

  const markUnread = useCallback(
    async (alert: IntelAlert) => {
      try {
        await fetchStatus({ data: { ids: [alert.id], is_read: false } });
        setItems((prev) =>
          prev.map((item) =>
            item.id === alert.id ? { ...item, is_read: false } : item,
          ),
        );
        syncCounters();
      } catch (err: any) {
        toast({
          title: "操作失败",
          description: err?.response?.data?.message || "请稍后重试",
          status: "error",
          duration: 3000,
          isClosable: true,
        });
      }
    },
    [fetchStatus, setItems, syncCounters, toast],
  );

  const markAllRead = useCallback(async () => {
    try {
      // 只把这条订阅的提醒标记已读，不波及其他订阅
      await fetchStatus({
        data: { all: true, is_read: true, subscription_id: subId },
      });
      // 全部已读影响整份列表，直接回第一页最稳妥
      loadFirstPage();
      syncCounters();
    } catch (err: any) {
      toast({
        title: "操作失败",
        description: err?.response?.data?.message || "请稍后重试",
        status: "error",
        duration: 3000,
        isClosable: true,
      });
    }
  }, [fetchStatus, loadFirstPage, syncCounters, subId, toast]);

  const handleDelete = useCallback(async () => {
    if (!deleteTarget) return;
    try {
      await fetchDelete({ data: { ids: [deleteTarget.id] } });
      setItems((prev) => prev.filter((item) => item.id !== deleteTarget.id));
      syncCounters();
      toast({
        title: "提醒已删除",
        status: "success",
        duration: 2500,
        isClosable: true,
      });
    } catch (err: any) {
      toast({
        title: "删除失败",
        description: err?.response?.data?.message || "请稍后重试",
        status: "error",
        duration: 3000,
        isClosable: true,
      });
    } finally {
      deleteDialog.onClose();
      setDeleteTarget(null);
    }
  }, [deleteTarget, fetchDelete, setItems, syncCounters, toast, deleteDialog]);

  /** 打开情报详情：先标记已读，再带回来路与订阅 ID 跳转。 */
  const openNotice = useCallback(
    async (alert: IntelAlert) => {
      await markRead(alert);
      router.push(
        `/intel/${alert.notice_id}?from=subscriptions&subscription=${subId}`,
      );
    },
    [markRead, router, subId],
  );

  const detailHref = (alert: IntelAlert) =>
    `/intel/${alert.notice_id}?from=subscriptions&subscription=${subId}`;

  return (
    <>
      <Drawer isOpen={isOpen} onClose={onClose} placement="right">
        <DrawerOverlay bg="blackAlpha.400" />
        <DrawerContent
          bg="workbench.paper"
          borderLeft="1px solid"
          borderColor="workbench.line"
          boxShadow="xl"
          w={{ base: "100vw", md: "50vw" }}
          maxW={{ base: "100vw", md: "50vw" }}
        >
          <DrawerCloseButton
            color="workbench.muted"
            _hover={{ bg: "workbench.canvas", color: "workbench.text" }}
            _focusVisible={{
              outline: "2px solid",
              outlineColor: "gold.400",
              outlineOffset: "2px",
            }}
          />
          <DrawerHeader
            px={{ base: 4, md: 5 }}
            py={4}
            borderBottom="1px solid"
            borderColor="workbench.line"
          >
            <Box pr={8} minW={0}>
              <Text
                fontSize="sm"
                fontWeight="700"
                color="workbench.text"
                overflowWrap="anywhere"
              >
                {subscription?.name || "未命名订阅"} · 命中提醒
              </Text>
              <Text
                mt={1}
                fontSize="xs"
                color="workbench.muted"
                sx={{ fontVariantNumeric: "tabular-nums" }}
              >
                当前未读 {unread} 条
              </Text>
            </Box>
            <Flex
              mt={3}
              align="center"
              justify="space-between"
              gap={3}
              wrap="wrap"
            >
              <HStack spacing={2} minW={0}>
                <Switch
                  size="sm"
                  colorScheme="primary"
                  isChecked={unreadOnly}
                  onChange={(event) => setUnreadOnly(event.target.checked)}
                  aria-label="只看未读提醒"
                />
                <Text fontSize="sm" color="workbench.muted" whiteSpace="nowrap">
                  只看未读
                </Text>
              </HStack>
              <Button
                h="44px"
                size="sm"
                variant="outline"
                borderColor="workbench.line"
                color="workbench.muted"
                leftIcon={<FiCheck aria-hidden />}
                isLoading={statusLoading}
                isDisabled={unread === 0}
                onClick={markAllRead}
                _active={{ transform: "scale(0.98)" }}
                _focusVisible={{
                  outline: "2px solid",
                  outlineColor: "gold.400",
                  outlineOffset: "2px",
                }}
              >
                全部已读
              </Button>
            </Flex>
          </DrawerHeader>

          <DrawerBody
            id={ALERT_DRAWER_SCROLL_ID}
            px={{ base: 4, md: 5 }}
            py={4}
          >
            {initialLoading ? (
              <Stack spacing={3}>
                {[0, 1, 2].map((key) => (
                  <Skeleton key={key} height="112px" borderRadius="12px" />
                ))}
              </Stack>
            ) : error && items.length === 0 ? (
              <EmptyState
                icon={<FiBell aria-hidden />}
                title="提醒加载失败"
                description={error}
                action={
                  <Button colorScheme="primary" onClick={() => loadFirstPage()}>
                    重新加载
                  </Button>
                }
              />
            ) : items.length === 0 ? (
              <EmptyState
                icon={<FiBell aria-hidden />}
                title={unreadOnly ? "没有未读提醒" : "还没有命中提醒"}
                description="采集到的新公告命中这条订阅的条件后，会在这里出现提醒。"
              />
            ) : (
              <InfiniteScrollList
                dataLength={items.length}
                hasMore={hasMore && !error}
                loadMore={loadMore}
                scrollableTarget={ALERT_DRAWER_SCROLL_ID}
                endMessage={
                  error ? (
                    <Flex justify="center" mt={4}>
                      <Button
                        variant="outline"
                        borderColor="workbench.line"
                        onClick={() => loadMore()}
                      >
                        加载更多失败，点击重试
                      </Button>
                    </Flex>
                  ) : undefined
                }
              >
                <Stack spacing={3}>
                  {items.map((item) => (
                    <Box
                      key={item.id}
                      bg="workbench.paper"
                      border="1px solid"
                      borderColor={
                        item.is_read ? "workbench.line" : "primary.200"
                      }
                      borderLeftWidth={item.is_read ? "1px" : "4px"}
                      borderLeftColor={
                        item.is_read ? "workbench.line" : "primary.500"
                      }
                      borderRadius="12px"
                      p={{ base: 3.5, md: 4 }}
                      cursor="pointer"
                      transition="border-color .15s ease-out, box-shadow .15s ease-out"
                      _hover={{
                        borderColor: "primary.300",
                        boxShadow: "0 8px 20px rgba(11, 27, 43, 0.08)",
                      }}
                      onClick={(event) => {
                        // 卡片内的链接与按钮各自处理，避免一次点击触发两次跳转
                        const target = event.target as HTMLElement;
                        if (target.closest("a,button,[role='switch']")) return;
                        openNotice(item);
                      }}
                    >
                      <HStack spacing={2} mb={1.5} wrap="wrap">
                        <Badge
                          colorScheme={item.is_read ? "neutral" : "primary"}
                          variant="subtle"
                        >
                          {item.is_read ? "已读" : "未读"}
                        </Badge>
                        <Text
                          fontSize="xs"
                          color="workbench.muted"
                          sx={{ fontVariantNumeric: "tabular-nums" }}
                        >
                          {item.created_at}
                        </Text>
                      </HStack>

                      <Text
                        as={NextLink}
                        href={detailHref(item)}
                        fontWeight="700"
                        color="workbench.text"
                        lineHeight="1.4"
                        overflowWrap="anywhere"
                        _hover={{
                          color: "primary.600",
                          textDecoration: "underline",
                        }}
                        _focusVisible={{
                          outline: "2px solid",
                          outlineColor: "gold.400",
                          outlineOffset: "2px",
                          borderRadius: "md",
                        }}
                        onClick={() => {
                          markRead(item);
                        }}
                      >
                        {item.notice_title || "（无标题公告）"}
                      </Text>

                      <Wrap spacing={2} mt={2}>
                        {(item.matched_keywords || []).map((keyword) => (
                          <WrapItem key={keyword}>
                            <Tag
                              size="sm"
                              variant="outline"
                              borderColor="workbench.line"
                              color="workbench.muted"
                            >
                              {keyword}
                            </Tag>
                          </WrapItem>
                        ))}
                      </Wrap>

                      <Text
                        mt={2}
                        fontSize="sm"
                        color="workbench.muted"
                        overflowWrap="anywhere"
                      >
                        {item.matched_reason}
                      </Text>

                      <Flex
                        mt={3}
                        justify="space-between"
                        align="center"
                        gap={2}
                        wrap="wrap"
                      >
                        <Text fontSize="xs" color="workbench.muted">
                          点击卡片查看情报详情
                        </Text>
                        <HStack spacing={2}>
                          {item.is_read ? (
                            <Button
                              h="44px"
                              size="sm"
                              variant="ghost"
                              color="workbench.muted"
                              onClick={() => markUnread(item)}
                              _hover={{
                                bg: "workbench.canvas",
                                color: "workbench.text",
                              }}
                              _focusVisible={{
                                outline: "2px solid",
                                outlineColor: "gold.400",
                                outlineOffset: "2px",
                              }}
                            >
                              设为未读
                            </Button>
                          ) : null}
                          <Button
                            h="44px"
                            size="sm"
                            variant="ghost"
                            color="neutral.500"
                            leftIcon={<FiTrash2 aria-hidden />}
                            onClick={() => {
                              setDeleteTarget(item);
                              deleteDialog.onOpen();
                            }}
                            _hover={{ bg: "red.50", color: "red.600" }}
                            _focusVisible={{
                              outline: "2px solid",
                              outlineColor: "gold.400",
                              outlineOffset: "2px",
                            }}
                          >
                            删除
                          </Button>
                        </HStack>
                      </Flex>
                    </Box>
                  ))}
                </Stack>
              </InfiniteScrollList>
            )}

            {loadingMore ? (
              <Text
                mt={3}
                fontSize="xs"
                color="workbench.muted"
                textAlign="center"
              >
                正在加载更多提醒…
              </Text>
            ) : null}
          </DrawerBody>
        </DrawerContent>
      </Drawer>

      <DeleteConfirmModal
        isOpen={deleteDialog.isOpen}
        onClose={() => {
          deleteDialog.onClose();
          setDeleteTarget(null);
        }}
        title="删除提醒"
        description="删除后这条提醒不再出现在抽屉里，且不可恢复；对应的情报仍保留在情报大厅。"
        isLoading={deleteLoading}
        handleConfirm={handleDelete}
      />
    </>
  );
}
