"use client";

/* eslint-disable no-nested-ternary */

/* Hallmark · macrostructure: Workbench · genre: modern-minimal · theme: BidEngine workbench (preserved tokens)
 * audience: 投标/售前团队 · use: 用订阅声明关注点，命中后的提醒就地查看与处理 · tone: technical, restrained
 * Hallmark · pre-emit critique: P5 H4 E5 S5 R5 V4
 */
import React, { useCallback, useEffect, useState } from "react";
import {
  Badge,
  Box,
  Button,
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
import { FiBell, FiCheck, FiPlus, FiTrash2, FiEdit3 } from "react-icons/fi";

import EmptyState from "@/components/common/empty-state";
import DeleteConfirmModal from "@/components/common/delete-confirm-modal";
import { PageViewport } from "@/components/layout/responsive-page";
import { WorkspaceShell } from "@/components/analysis/bid-analysis-v3/workspace";
import { ModuleWorkbenchHeader } from "@/components/common/module-workbench.mjs";
import AlertDrawer from "@/components/intel/alert-drawer";
import SubscriptionEditor, {
  toSubscriptionPayload,
  type SubscriptionFormValue,
} from "@/components/intel/subscription-editor";
import {
  useIntelAlertStatus,
  useIntelFilters,
  useIntelSubscriptionDelete,
  useIntelSubscriptionSave,
  useIntelSubscriptions,
  useIntelUnreadCount,
  type IntelSubscription,
} from "@/service/intel";

export default function IntelSubscriptionsPage() {
  const toast = useToast();
  const { filters } = useIntelFilters();
  const { subscriptions, subscriptionsLoading, refreshSubscriptions } =
    useIntelSubscriptions();
  const { saveLoading, fetchSave, fetchUpdate } = useIntelSubscriptionSave();
  const { deleteLoading, fetchDelete } = useIntelSubscriptionDelete();
  const { statusLoading, fetchStatus } = useIntelAlertStatus();
  const { unread, refreshUnread } = useIntelUnreadCount();
  const { isOpen, onOpen, onClose } = useDisclosure();
  const alertDrawer = useDisclosure();
  const deleteDialog = useDisclosure();
  const [editing, setEditing] = useState<IntelSubscription | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<IntelSubscription | null>(
    null,
  );
  const [activeSubId, setActiveSubId] = useState(0);
  // 从情报详情返回时带 ?subscription=<id>，等订阅列表就绪后自动重开抽屉
  const [pendingSubId, setPendingSubId] = useState(0);

  useEffect(() => {
    if (typeof window === "undefined") return;
    const id = Number(
      new URLSearchParams(window.location.search).get("subscription"),
    );
    if (Number.isFinite(id) && id > 0) setPendingSubId(id);
  }, []);

  useEffect(() => {
    if (!pendingSubId || subscriptionsLoading) return;
    const target = subscriptions.find((item) => item.id === pendingSubId);
    if (target) {
      setActiveSubId(target.id);
      alertDrawer.onOpen();
    }
    // 无论是否找到都只处理一次，避免列表刷新时反复弹抽屉
    setPendingSubId(0);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pendingSubId, subscriptionsLoading, subscriptions]);

  const activeSubscription =
    subscriptions.find((item) => item.id === activeSubId) ?? null;

  const openAlerts = (item: IntelSubscription) => {
    setActiveSubId(item.id);
    alertDrawer.onOpen();
  };

  /** 提醒侧发生变更后：订阅卡片未读数与导航角标都要跟着刷新。 */
  const handleAlertsChanged = useCallback(() => {
    refreshSubscriptions();
    refreshUnread();
  }, [refreshSubscriptions, refreshUnread]);

  const handleSubmit = async (value: SubscriptionFormValue) => {
    const payload = toSubscriptionPayload(value);
    try {
      if (editing) {
        await fetchUpdate({
          url: `/zb/intel/subscriptions/${editing.id}`,
          data: payload,
        });
      } else {
        await fetchSave({ url: "/zb/intel/subscriptions", data: payload });
      }
      toast({
        title: editing ? "订阅已更新" : "订阅已创建",
        status: "success",
        duration: 2500,
        isClosable: true,
      });
      onClose();
      setEditing(null);
      refreshSubscriptions();
    } catch (error: any) {
      toast({
        title: "保存失败",
        description:
          error?.response?.data?.message || error?.message || "请稍后重试",
        status: "error",
        duration: 3500,
        isClosable: true,
      });
    }
  };

  const handleToggle = async (item: IntelSubscription, enabled: boolean) => {
    try {
      await fetchSave({
        url: "/zb/intel/subscriptions/enable",
        data: { id: item.id, enabled },
      });
      refreshSubscriptions();
    } catch (error: any) {
      toast({
        title: "操作失败",
        description: error?.response?.data?.message || "请稍后重试",
        status: "error",
        duration: 3000,
        isClosable: true,
      });
    }
  };

  const handleDelete = async (item: IntelSubscription) => {
    try {
      await fetchDelete({ url: `/zb/intel/subscriptions/${item.id}` });
      toast({
        title: "订阅已删除",
        status: "success",
        duration: 2500,
        isClosable: true,
      });
      // 删除的正是抽屉里的订阅时同步关掉抽屉，避免留下空壳
      if (activeSubId === item.id) {
        alertDrawer.onClose();
        setActiveSubId(0);
      }
      refreshSubscriptions();
      refreshUnread();
    } catch (error: any) {
      toast({
        title: "删除失败",
        description: error?.response?.data?.message || "请稍后重试",
        status: "error",
        duration: 3000,
        isClosable: true,
      });
    } finally {
      deleteDialog.onClose();
      setDeleteTarget(null);
    }
  };

  const markAllRead = async () => {
    try {
      await fetchStatus({ data: { all: true, is_read: true } });
      refreshSubscriptions();
      refreshUnread();
      toast({
        title: "已全部标记为已读",
        status: "success",
        duration: 2500,
        isClosable: true,
      });
    } catch (error: any) {
      toast({
        title: "操作失败",
        description: error?.response?.data?.message || "请稍后重试",
        status: "error",
        duration: 3000,
        isClosable: true,
      });
    }
  };

  return (
    <PageViewport>
      <WorkspaceShell>
        <ModuleWorkbenchHeader
          title="订阅与提醒"
          titleSuffix={null}
          activity={
            <HStack spacing={3} wrap="wrap" justify="flex-end">
              <Button
                h="44px"
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
              <Button
                h="44px"
                colorScheme="primary"
                leftIcon={<FiPlus aria-hidden />}
                onClick={() => {
                  setEditing(null);
                  onOpen();
                }}
                _active={{ transform: "scale(0.98)" }}
              >
                新建订阅
              </Button>
            </HStack>
          }
        />

        <Flex
          mb={5}
          gap={4}
          wrap="wrap"
          align="center"
          fontSize="sm"
          color="workbench.muted"
          sx={{ fontVariantNumeric: "tabular-nums" }}
        >
          <Text>我的订阅：{subscriptions.length} 条</Text>
          <Text>未读提醒：{unread} 条</Text>
          <Text display={{ base: "none", md: "block" }}>
            订阅声明关注点，采集到的新公告命中后会生成提醒
          </Text>
        </Flex>

        {subscriptionsLoading ? (
          <Stack spacing={3}>
            {[0, 1, 2].map((key) => (
              <Skeleton key={key} height="112px" borderRadius="14px" />
            ))}
          </Stack>
        ) : subscriptions.length === 0 ? (
          <EmptyState
            title="还没有订阅"
            description="把关注的关键词、行业、地区固化成订阅，新公告命中后会在订阅卡片里生成提醒。"
            action={
              <Button
                colorScheme="primary"
                onClick={() => {
                  setEditing(null);
                  onOpen();
                }}
              >
                新建订阅
              </Button>
            }
          />
        ) : (
          <Stack spacing={3}>
            {subscriptions.map((item) => (
              <Box
                key={item.id}
                bg="workbench.paper"
                borderRadius="14px"
                borderWidth="1px"
                borderColor="workbench.line"
                boxShadow="0 10px 30px rgba(11, 27, 43, 0.06)"
                p={{ base: 4, md: 5 }}
                cursor="pointer"
                transition="border-color .15s ease-out, box-shadow .15s ease-out, transform .15s ease-out"
                _hover={{
                  borderColor: "primary.200",
                  boxShadow: "0 12px 28px rgba(11, 27, 43, 0.10)",
                  transform: "translateY(-1px)",
                }}
                onClick={(event) => {
                  // 卡片内的开关/按钮各自处理，其余区域点击即打开提醒抽屉
                  const target = event.target as HTMLElement;
                  if (target.closest("a,button,[role='switch']")) return;
                  openAlerts(item);
                }}
              >
                <Flex justify="space-between" align="flex-start" gap={3}>
                  <Box minW={0}>
                    <HStack spacing={2}>
                      <Text fontWeight="700" color="workbench.text">
                        {item.name || "未命名订阅"}
                      </Text>
                      <Badge
                        colorScheme={item.enabled ? "success" : "neutral"}
                        variant="subtle"
                      >
                        {item.enabled ? "启用中" : "已停用"}
                      </Badge>
                    </HStack>
                    <Wrap spacing={2} mt={2}>
                      {(item.keywords || []).map((keyword) => (
                        <WrapItem key={keyword}>
                          <Tag
                            size="sm"
                            bg="primary.50"
                            color="primary.700"
                            border="none"
                          >
                            {keyword}
                          </Tag>
                        </WrapItem>
                      ))}
                      {(item.industry_names || []).map((name) => (
                        <WrapItem key={name}>
                          <Tag
                            size="sm"
                            variant="outline"
                            borderColor="workbench.line"
                            color="workbench.muted"
                          >
                            {name}
                          </Tag>
                        </WrapItem>
                      ))}
                      {(item.regions || []).map((region) => (
                        <WrapItem key={region}>
                          <Tag
                            size="sm"
                            variant="outline"
                            borderColor="workbench.line"
                            color="workbench.muted"
                          >
                            {region}
                          </Tag>
                        </WrapItem>
                      ))}
                    </Wrap>
                    <Text mt={2} fontSize="sm" color="workbench.muted">
                      匹配方式：
                      {item.match_mode === "all" ? "全部命中" : "任一命中"}
                      {item.budget_min || item.budget_max
                        ? ` · 预算 ${item.budget_min ?? "-"} ~ ${item.budget_max ?? "-"} 元`
                        : ""}
                      {` · 累计命中 ${item.matched_count} 次`}
                    </Text>
                  </Box>

                  {/* 开关与编辑/删除自成一组：阻止冒泡，避免操作卡片内的控件时又打开抽屉 */}
                  <Stack
                    align="flex-end"
                    spacing={2}
                    flexShrink={0}
                    onClick={(event) => event.stopPropagation()}
                  >
                    <Switch
                      colorScheme="primary"
                      isChecked={item.enabled}
                      onChange={(event) =>
                        handleToggle(item, event.target.checked)
                      }
                      aria-label={item.enabled ? "停用订阅" : "启用订阅"}
                    />
                    <HStack spacing={2}>
                      <Button
                        h="44px"
                        size="sm"
                        variant="ghost"
                        leftIcon={<FiEdit3 aria-hidden />}
                        onClick={() => {
                          setEditing(item);
                          onOpen();
                        }}
                        _hover={{ bg: "primary.50", color: "primary.700" }}
                        _focusVisible={{
                          outline: "2px solid",
                          outlineColor: "gold.400",
                          outlineOffset: "2px",
                        }}
                      >
                        编辑
                      </Button>
                      <Button
                        h="44px"
                        size="sm"
                        variant="ghost"
                        color="neutral.500"
                        leftIcon={<FiTrash2 aria-hidden />}
                        isLoading={deleteLoading}
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
                  </Stack>
                </Flex>

                <Flex
                  mt={3}
                  pt={3}
                  borderTop="1px solid"
                  borderColor="workbench.line"
                  justify="space-between"
                  align="center"
                  gap={3}
                  wrap="wrap"
                >
                  <Button
                    h="44px"
                    size="sm"
                    variant="outline"
                    borderColor="workbench.line"
                    color="workbench.text"
                    leftIcon={<FiBell aria-hidden />}
                    rightIcon={
                      item.unread_count > 0 ? (
                        <Badge
                          borderRadius="full"
                          px={2}
                          fontSize="xs"
                          bg="primary.500"
                          color="white"
                          sx={{ fontVariantNumeric: "tabular-nums" }}
                        >
                          {item.unread_count}
                        </Badge>
                      ) : undefined
                    }
                    aria-haspopup="dialog"
                    aria-expanded={
                      alertDrawer.isOpen && activeSubId === item.id
                    }
                    onClick={() => openAlerts(item)}
                    _hover={{
                      borderColor: "primary.300",
                      color: "primary.700",
                    }}
                    _active={{ transform: "scale(0.98)" }}
                    _focusVisible={{
                      outline: "2px solid",
                      outlineColor: "gold.400",
                      outlineOffset: "2px",
                    }}
                  >
                    查看命中提醒
                  </Button>
                  <Text fontSize="xs" color="workbench.muted">
                    {item.last_matched_at
                      ? `最近命中 ${item.last_matched_at}`
                      : "还没有命中记录"}
                  </Text>
                </Flex>
              </Box>
            ))}
          </Stack>
        )}

        <SubscriptionEditor
          isOpen={isOpen}
          onClose={() => {
            onClose();
            setEditing(null);
          }}
          filters={filters}
          editing={editing}
          submitting={saveLoading}
          onSubmit={handleSubmit}
        />

        <AlertDrawer
          key={activeSubId}
          subscription={activeSubscription}
          isOpen={alertDrawer.isOpen}
          onClose={() => {
            alertDrawer.onClose();
            setActiveSubId(0);
          }}
          onChanged={handleAlertsChanged}
        />

        <DeleteConfirmModal
          isOpen={deleteDialog.isOpen}
          onClose={() => {
            deleteDialog.onClose();
            setDeleteTarget(null);
          }}
          title="删除订阅"
          description={
            <>
              确定删除订阅
              <Text as="span" fontWeight="700" color="workbench.text">
                {deleteTarget?.name || "未命名订阅"}
              </Text>
              吗？删除后该规则不再生效，它产生的提醒也会一并清理，且不可恢复。
            </>
          }
          isLoading={deleteLoading}
          handleConfirm={() => {
            if (deleteTarget) handleDelete(deleteTarget);
          }}
        />
      </WorkspaceShell>
    </PageViewport>
  );
}
