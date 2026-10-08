"use client";

/* eslint-disable no-nested-ternary */

/* Hallmark · component: intel-match-manager · genre: modern-minimal · theme: BidEngine workbench (preserved tokens)
 * 职责：订阅匹配任务的可视化作业面——优先级、取消、重新入队、删除、手动补扫。
 * states: default · hover · focus-visible · active · disabled · loading · empty · error
 * Hallmark · pre-emit critique: P5 H4 E5 S5 R5 V4
 */
import React, { useCallback, useMemo, useState } from "react";
import {
  Badge,
  Box,
  Button,
  Collapse,
  Divider,
  Flex,
  Grid,
  HStack,
  Input,
  ListItem,
  Modal,
  ModalBody,
  ModalCloseButton,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  Skeleton,
  Stack,
  Table,
  TableContainer,
  Tbody,
  Td,
  Text,
  Th,
  Thead,
  Tooltip,
  Tr,
  UnorderedList,
  useDisclosure,
  useToast,
} from "@chakra-ui/react";
import {
  FiArrowUp,
  FiPlay,
  FiRefreshCw,
  FiRotateCcw,
  FiTrash2,
  FiXCircle,
} from "react-icons/fi";

import EmptyState from "@/components/common/empty-state";
import DeleteConfirmModal from "@/components/common/delete-confirm-modal";
import IntelSelect, {
  type IntelSelectOption,
} from "@/components/intel/intel-select";
import HintToggle from "@/components/intel/admin/hint-toggle";
import {
  useIntelMatchCancel,
  useIntelMatchDelete,
  useIntelMatchOverview,
  useIntelMatchPriority,
  useIntelMatchRescan,
  useIntelMatchRetry,
  useIntelMatchTasks,
  type IntelMatchTask,
} from "@/service/intel";

const STATUS_OPTIONS: IntelSelectOption[] = [
  { value: "", label: "全部状态" },
  { value: "pending", label: "排队中" },
  { value: "running", label: "执行中" },
  { value: "success", label: "成功" },
  { value: "failed", label: "失败" },
  { value: "cancelled", label: "已取消" },
];

const TYPE_OPTIONS: IntelSelectOption[] = [
  { value: "", label: "全部类型" },
  { value: "collect_run", label: "采集后匹配" },
  { value: "subscription_backfill", label: "订阅回溯" },
  { value: "manual_notice", label: "手工发布匹配" },
  { value: "import_batch", label: "批量导入匹配" },
  { value: "manual_rescan", label: "手动补扫" },
];

/** 状态徽标配色：与既有管理页的功能色口径保持一致。 */
function statusMeta(status: string): { label: string; colorScheme: string } {
  switch (status) {
    case "pending":
      return { label: "排队中", colorScheme: "neutral" };
    case "running":
      return { label: "执行中", colorScheme: "info" };
    case "success":
      return { label: "成功", colorScheme: "success" };
    case "failed":
      return { label: "失败", colorScheme: "error" };
    case "cancelled":
      return { label: "已取消", colorScheme: "neutral" };
    default:
      return { label: status || "未知", colorScheme: "neutral" };
  }
}

/** 终态任务的操作文案：失败/成功可重试，取消后可重新入队。 */
function retryActionLabel(status: string): string {
  return status === "cancelled" ? "重新入队" : "重试";
}

function retryActionHint(status: string): string {
  if (status === "cancelled") {
    return "按原情报范围重新入队执行";
  }
  if (status === "success") {
    return "用同一批情报按当前匹配规则重新匹配";
  }
  return "重试失败任务，按原情报范围重新执行";
}

/** 情报范围的人类可读描述：范围在任务创建时固定，这里只做展示。 */
function scopeLabel(task: IntelMatchTask): string {
  switch (task.scope_kind) {
    case "run":
      return `采集批次 ${task.scope_ref}`;
    case "batch":
      return `导入批次 ${task.scope_ref}`;
    case "notice":
      return `单条情报 #${task.scope_ref}`;
    case "subscription":
      return `订阅回溯 #${task.scope_ref}`;
    case "window":
      return `${task.range_from || "-"} ~ ${task.range_to || "-"}`;
    default:
      return task.scope_ref || "-";
  }
}

function OverviewCard({
  label,
  value,
  tone,
}: {
  label: string;
  value: React.ReactNode;
  tone?: string;
}) {
  return (
    <Box
      bg="workbench.paper"
      border="1px solid"
      borderColor="workbench.line"
      borderRadius="12px"
      px={4}
      py={3}
    >
      <Text fontSize="xs" color="workbench.muted" letterSpacing="0.02em">
        {label}
      </Text>
      <Text
        mt={1}
        fontSize="xl"
        fontWeight="800"
        color={tone || "workbench.text"}
        sx={{ fontVariantNumeric: "tabular-nums" }}
      >
        {value}
      </Text>
    </Box>
  );
}

export default function MatchManager() {
  const toast = useToast();
  const [status, setStatus] = useState("");
  const [taskType, setTaskType] = useState("");
  const [cancelTarget, setCancelTarget] = useState<IntelMatchTask | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<IntelMatchTask | null>(null);
  const [rescanFrom, setRescanFrom] = useState("");
  const [rescanTo, setRescanTo] = useState("");
  // 业务说明默认折叠：折叠态只渲染筛选与操作，展开后才解释这个 tab 在做什么
  const [helpOpen, setHelpOpen] = useState(false);
  const cancelDialog = useDisclosure();
  const deleteDialog = useDisclosure();
  const rescanDialog = useDisclosure();

  const { overview, overviewLoading, refreshOverview } =
    useIntelMatchOverview();
  const { matchTasks, matchTasksLoading, matchTasksError, refreshMatchTasks } =
    useIntelMatchTasks(status, taskType);
  const { priorityLoading, fetchPriority } = useIntelMatchPriority();
  const { cancelLoading, fetchCancel } = useIntelMatchCancel();
  const { matchRetryLoading, fetchRetry } = useIntelMatchRetry();
  const { matchDeleteLoading, fetchDelete } = useIntelMatchDelete();
  const { rescanLoading, fetchRescan } = useIntelMatchRescan();

  const counts = useMemo(() => overview?.counts || {}, [overview]);

  const refreshAll = useCallback(() => {
    refreshMatchTasks();
    refreshOverview();
  }, [refreshMatchTasks, refreshOverview]);

  const notifyError = useCallback(
    (title: string, error: any) => {
      toast({
        title,
        description:
          error?.response?.data?.message || error?.message || "请稍后重试",
        status: "error",
        duration: 3500,
        isClosable: true,
      });
    },
    [toast],
  );

  const handlePriority = async (
    task: IntelMatchTask,
    next: "normal" | "high",
  ) => {
    try {
      await fetchPriority({
        url: `/zb/intel/matches/tasks/${task.task_no}/priority`,
        data: { priority: next },
      });
      toast({
        title: next === "high" ? "已提升为优先" : "已恢复为普通优先级",
        status: "success",
        duration: 2500,
        isClosable: true,
      });
      refreshAll();
    } catch (error: any) {
      notifyError("调整优先级失败", error);
    }
  };

  const handleCancel = async () => {
    if (!cancelTarget) return;
    try {
      await fetchCancel({
        url: `/zb/intel/matches/tasks/${cancelTarget.task_no}/cancel`,
        data: { reason: "管理员取消" },
      });
      toast({
        title:
          cancelTarget.status === "pending"
            ? "任务已取消"
            : "已请求取消，将在批次边界停止",
        status: "success",
        duration: 3000,
        isClosable: true,
      });
      refreshAll();
    } catch (error: any) {
      notifyError("取消失败", error);
    } finally {
      cancelDialog.onClose();
      setCancelTarget(null);
    }
  };

  const handleRetry = async (task: IntelMatchTask) => {
    try {
      const res = await fetchRetry({
        url: `/zb/intel/matches/tasks/${task.task_no}/retry`,
      });
      // 接口在业务校验失败时仍返回 HTTP 200，必须检查业务 code，
      // 否则会把“状态不允许重试”误报成入队成功。
      const code = res?.data?.code;
      if (code !== undefined && code !== 0) {
        toast({
          title: "重新入队失败",
          description: res?.data?.message || "当前任务状态不允许重新入队",
          status: "error",
          duration: 3500,
          isClosable: true,
        });
        return;
      }
      const newTaskNo = res?.data?.data?.task_no || "";
      toast({
        title: newTaskNo ? `已重新入队：${newTaskNo}` : "已重新入队",
        description: "已创建新任务并加入队列，可在列表顶部查看执行状态。",
        status: "success",
        duration: 3500,
        isClosable: true,
      });
      // 成功任务重跑后会新建一条 pending 任务；若当前筛选只看“成功”，
      // 新任务会被筛掉，看起来像没有执行，因此回到全部状态。
      if (status) {
        setStatus("");
        refreshOverview();
      } else {
        refreshAll();
      }
    } catch (error: any) {
      notifyError("重新入队失败", error);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    try {
      await fetchDelete({
        url: `/zb/intel/matches/tasks/${deleteTarget.task_no}`,
      });
      toast({
        title: "任务已删除",
        status: "success",
        duration: 2500,
        isClosable: true,
      });
      refreshAll();
    } catch (error: any) {
      notifyError("删除失败", error);
    } finally {
      deleteDialog.onClose();
      setDeleteTarget(null);
    }
  };

  const handleRescan = async () => {
    try {
      const res = await fetchRescan({
        data: { date_from: rescanFrom, date_to: rescanTo },
      });
      const code = res?.data?.code;
      if (code && code !== 200) {
        toast({
          title: "补扫未执行",
          description: res?.data?.message || "请检查时间范围",
          status: "error",
          duration: 3500,
          isClosable: true,
        });
        return;
      }
      toast({
        title: `已创建补扫任务，覆盖 ${
          res?.data?.data?.notice_total ?? 0
        } 条在架情报`,
        status: "success",
        duration: 3000,
        isClosable: true,
      });
      rescanDialog.onClose();
      refreshAll();
    } catch (error: any) {
      notifyError("创建补扫任务失败", error);
    }
  };

  return (
    <Stack spacing={4}>
      {/* 操作条：默认折叠，说明开关固定在最右，与其它子 tab 位置一致 */}
      <Box
        bg="white"
        border="1px solid"
        borderColor="workbench.line"
        borderRadius="14px"
        boxShadow="0 10px 30px rgba(11, 27, 43, 0.06)"
        px={{ base: 3, md: 4 }}
        py={3}
      >
        <Flex
          align={{ base: "stretch", lg: "center" }}
          direction={{ base: "column", lg: "row" }}
          justify="space-between"
          gap={3}
          wrap="wrap"
        >
          <HStack spacing={3} wrap="wrap" order={{ base: 2, lg: 1 }}>
            <Box w={{ base: "full", sm: "160px" }}>
              <IntelSelect
                value={status}
                options={STATUS_OPTIONS}
                onChange={setStatus}
                ariaLabel="匹配任务状态筛选"
                placeholder="全部状态"
              />
            </Box>
            <Box w={{ base: "full", sm: "180px" }}>
              <IntelSelect
                value={taskType}
                options={TYPE_OPTIONS}
                onChange={setTaskType}
                ariaLabel="匹配任务类型筛选"
                placeholder="全部类型"
              />
            </Box>
          </HStack>
          <HStack
            spacing={3}
            order={{ base: 1, lg: 2 }}
            flexShrink={0}
            w={{ base: "full", lg: "auto" }}
            justify={{ base: "space-between", lg: "flex-end" }}
          >
            <Button
              h="44px"
              variant="outline"
              borderColor="workbench.line"
              leftIcon={<FiRefreshCw aria-hidden />}
              onClick={refreshAll}
            >
              刷新
            </Button>
            <Button
              h="44px"
              colorScheme="primary"
              leftIcon={<FiPlay aria-hidden />}
              onClick={() => {
                setRescanFrom("");
                setRescanTo("");
                rescanDialog.onOpen();
              }}
            >
              手动补扫
            </Button>
            <HintToggle
              label="订阅匹配说明"
              expanded={helpOpen}
              onToggle={() => setHelpOpen((prev) => !prev)}
            />
          </HStack>
        </Flex>

        <Collapse in={helpOpen} animateOpacity>
          <Divider mt={3} mb={3} borderColor="neutral.100" />
          <UnorderedList
            spacing={2}
            pl={5}
            m={0}
            fontSize="sm"
            color="workbench.text"
          >
            <ListItem>
              这里管理的是订阅匹配任务：把入库情报与用户订阅规则比对，命中就生成站内提醒。
            </ListItem>
            <ListItem>
              每次采集结束后系统自动生成一条“采集后匹配”任务，情报范围固定为本次新增的那批；手工发布与批量导入也各自生成一条。
            </ListItem>
            <ListItem>
              用户新建订阅或首次启用时，系统会静默回溯近 30
              天的在架情报（“订阅回溯”），用户侧不感知；需要时可在这里点“手动补扫”按发布时间窗重跑。
            </ListItem>
            <ListItem>
              排队中的任务可以切换为“优先”，优先任务先于普通任务执行；“顺位”列显示它排在第几位执行。
            </ListItem>
            <ListItem>
              取消任务表示这批情报这次不生成任何提醒，也不会被自动重跑；需要时可按原情报范围重新入队。
            </ListItem>
            <ListItem>
              执行成功、失败或已取消的任务都可以按原情报范围重新执行；匹配规则更新后可用“重试”补算提醒。
            </ListItem>
            <ListItem>
              已结束的任务可以删除记录；排队中或执行中的任务必须先取消才能删除。
            </ListItem>
            <ListItem>
              提醒按“用户 + 订阅 + 情报”唯一，重复匹配不会重复提醒。
            </ListItem>
          </UnorderedList>
        </Collapse>
      </Box>

      <Grid
        templateColumns={{
          base: "repeat(2, minmax(0, 1fr))",
          md: "repeat(3, minmax(0, 1fr))",
          xl: "repeat(6, minmax(0, 1fr))",
        }}
        gap={3}
      >
        {overviewLoading && !overview ? (
          [0, 1, 2, 3, 4, 5].map((key) => (
            <Skeleton key={key} height="72px" borderRadius="12px" />
          ))
        ) : (
          <>
            <OverviewCard label="排队中" value={counts.pending || 0} />
            <OverviewCard
              label="执行中"
              value={counts.running || 0}
              tone="info.600"
            />
            <OverviewCard
              label="成功"
              value={counts.success || 0}
              tone="success.600"
            />
            <OverviewCard
              label="失败"
              value={counts.failed || 0}
              tone="error.600"
            />
            <OverviewCard label="已取消" value={counts.cancelled || 0} />
            <OverviewCard
              label="今日匹配提醒"
              value={overview?.today_alerts || 0}
              tone="primary.600"
            />
          </>
        )}
      </Grid>

      {matchTasksLoading && matchTasks.length === 0 ? (
        <Stack spacing={3}>
          {[0, 1, 2].map((key) => (
            <Skeleton key={key} height="72px" borderRadius="12px" />
          ))}
        </Stack>
      ) : matchTasksError && matchTasks.length === 0 ? (
        <EmptyState
          title="匹配任务加载失败"
          description="请稍后重试"
          action={
            <Button colorScheme="primary" onClick={() => refreshMatchTasks()}>
              重新加载
            </Button>
          }
        />
      ) : matchTasks.length === 0 ? (
        <EmptyState
          title="还没有匹配任务"
          description="每次采集（自动或手动）完成打标后系统会生成一个订阅匹配任务；订阅新建或首次启用时也会生成回溯任务。"
        />
      ) : (
        <Box
          bg="workbench.paper"
          border="1px solid"
          borderColor="workbench.line"
          borderRadius="14px"
          overflow="hidden"
        >
          <TableContainer>
            {/* 表头与内容必须同向对齐：数值列的表头同样 isNumeric，
                否则会出现“列名左对齐、数字右对齐”的错位（此前就是这个毛病）。 */}
            <Table size="sm" variant="simple">
              <Thead bg="workbench.canvas">
                <Tr>
                  <Th
                    minW="220px"
                    fontSize="xs"
                    color="workbench.muted"
                    borderColor="workbench.line"
                  >
                    任务
                  </Th>
                  <Th
                    minW="220px"
                    fontSize="xs"
                    color="workbench.muted"
                    borderColor="workbench.line"
                  >
                    匹配对象
                  </Th>
                  <Th
                    isNumeric
                    minW="110px"
                    fontSize="xs"
                    color="workbench.muted"
                    borderColor="workbench.line"
                  >
                    进度
                  </Th>
                  <Th
                    minW="170px"
                    fontSize="xs"
                    color="workbench.muted"
                    borderColor="workbench.line"
                  >
                    执行
                  </Th>
                  <Th
                    minW="200px"
                    fontSize="xs"
                    color="workbench.muted"
                    borderColor="workbench.line"
                  >
                    错误
                  </Th>
                  <Th
                    minW="220px"
                    fontSize="xs"
                    color="workbench.muted"
                    borderColor="workbench.line"
                  >
                    操作
                  </Th>
                </Tr>
              </Thead>
              <Tbody>
                {matchTasks.map((task) => {
                  const meta = statusMeta(task.status);
                  const terminal =
                    task.status === "success" ||
                    task.status === "failed" ||
                    task.status === "cancelled";
                  const isBackfill = task.scope_kind === "subscription";
                  return (
                    <Tr key={task.task_no} _hover={{ bg: "workbench.canvas" }}>
                      <Td verticalAlign="top" borderColor="workbench.line">
                        <Text
                          fontSize="sm"
                          fontWeight="600"
                          color="workbench.text"
                        >
                          {task.task_type_name}
                        </Text>
                        <Text
                          fontSize="xs"
                          color="workbench.muted"
                          fontFamily="mono"
                          wordBreak="break-all"
                        >
                          {task.task_no}
                        </Text>
                        <Text fontSize="xs" color="workbench.muted">
                          {task.created_at}
                          {task.retry_of ? ` · 重试自 ${task.retry_of}` : ""}
                        </Text>
                      </Td>
                      <Td verticalAlign="top" borderColor="workbench.line">
                        {isBackfill ? (
                          <>
                            <Text fontSize="sm" color="workbench.text">
                              {task.subscription_name || "订阅已删除"}
                              <Text as="span" color="workbench.muted">
                                {` #${task.subscription_id}`}
                              </Text>
                            </Text>
                            <Text
                              fontSize="xs"
                              color="workbench.muted"
                              mt={0.5}
                            >
                              用户：{task.user_name || `#${task.user_id}`}
                              {task.user_mobile ? ` · ${task.user_mobile}` : ""}
                            </Text>
                          </>
                        ) : (
                          <Text
                            fontSize="sm"
                            color="workbench.text"
                            wordBreak="break-word"
                          >
                            {scopeLabel(task)}
                          </Text>
                        )}
                      </Td>
                      <Td
                        isNumeric
                        verticalAlign="top"
                        borderColor="workbench.line"
                      >
                        <Text
                          fontSize="sm"
                          color="workbench.text"
                          sx={{ fontVariantNumeric: "tabular-nums" }}
                        >
                          {task.scanned_count}/{task.notice_total}
                        </Text>
                        <Text
                          fontSize="xs"
                          color="workbench.muted"
                          sx={{ fontVariantNumeric: "tabular-nums" }}
                        >
                          命中 {task.matched_notice_count} 条
                        </Text>
                        <Text
                          fontSize="xs"
                          color="workbench.muted"
                          sx={{ fontVariantNumeric: "tabular-nums" }}
                        >
                          提醒 {task.alert_count} 条
                        </Text>
                      </Td>
                      <Td verticalAlign="top" borderColor="workbench.line">
                        <Badge colorScheme={meta.colorScheme} variant="subtle">
                          {meta.label}
                        </Badge>
                        {task.status === "pending" ? (
                          <HStack spacing={2} mt={1.5} wrap="wrap">
                            <Button
                              size="xs"
                              h="30px"
                              px={2}
                              variant={
                                task.priority === "high" ? "solid" : "outline"
                              }
                              colorScheme={
                                task.priority === "high" ? "primary" : undefined
                              }
                              borderColor="workbench.line"
                              leftIcon={<FiArrowUp aria-hidden />}
                              isLoading={priorityLoading}
                              onClick={() =>
                                handlePriority(
                                  task,
                                  task.priority === "high" ? "normal" : "high",
                                )
                              }
                              _active={{ transform: "scale(0.98)" }}
                            >
                              {task.priority === "high" ? "优先" : "普通"}
                            </Button>
                            <Text fontSize="xs" color="workbench.muted">
                              {task.queue_order > 0
                                ? `第 ${task.queue_order} 位执行`
                                : "等待重试"}
                            </Text>
                          </HStack>
                        ) : (
                          <Text fontSize="xs" color="workbench.muted" mt={1}>
                            {task.priority === "high" ? "优先" : "普通"}
                            {task.cancel_requested ? " · 已请求取消" : ""}
                          </Text>
                        )}
                      </Td>
                      <Td
                        verticalAlign="top"
                        borderColor="workbench.line"
                        maxW="280px"
                      >
                        <Text
                          fontSize="xs"
                          color={
                            task.status === "failed"
                              ? "error.600"
                              : "workbench.muted"
                          }
                          wordBreak="break-word"
                          noOfLines={3}
                        >
                          {task.last_error || "—"}
                        </Text>
                      </Td>
                      <Td verticalAlign="top" borderColor="workbench.line">
                        <HStack spacing={2} flexWrap="nowrap">
                          {!terminal ? (
                            <Tooltip label="取消任务：该批情报不会给用户提醒">
                              <Button
                                aria-label={`取消任务 ${task.task_no}`}
                                size="sm"
                                h="36px"
                                px={3}
                                variant="outline"
                                borderColor="neutral.200"
                                color="neutral.600"
                                leftIcon={<FiXCircle aria-hidden />}
                                isLoading={cancelLoading}
                                onClick={() => {
                                  setCancelTarget(task);
                                  cancelDialog.onOpen();
                                }}
                                _hover={{ bg: "red.50", color: "red.600" }}
                              >
                                取消
                              </Button>
                            </Tooltip>
                          ) : (
                            <>
                              <Tooltip label={retryActionHint(task.status)}>
                                <Button
                                  aria-label={`${retryActionLabel(task.status)} ${task.task_no}`}
                                  size="sm"
                                  h="36px"
                                  px={3}
                                  variant="outline"
                                  borderColor="primary.200"
                                  color="primary.600"
                                  leftIcon={<FiRotateCcw aria-hidden />}
                                  isLoading={matchRetryLoading}
                                  onClick={() => handleRetry(task)}
                                  _hover={{ bg: "primary.50" }}
                                >
                                  {retryActionLabel(task.status)}
                                </Button>
                              </Tooltip>
                              <Tooltip label="删除任务记录（不可恢复）">
                                <Button
                                  aria-label={`删除任务 ${task.task_no}`}
                                  size="sm"
                                  h="36px"
                                  px={3}
                                  variant="outline"
                                  borderColor="neutral.200"
                                  color="red.600"
                                  leftIcon={<FiTrash2 aria-hidden />}
                                  isLoading={matchDeleteLoading}
                                  onClick={() => {
                                    setDeleteTarget(task);
                                    deleteDialog.onOpen();
                                  }}
                                  _hover={{ bg: "red.50", color: "red.600" }}
                                >
                                  删除
                                </Button>
                              </Tooltip>
                            </>
                          )}
                        </HStack>
                      </Td>
                    </Tr>
                  );
                })}
              </Tbody>
            </Table>
          </TableContainer>
        </Box>
      )}

      {/* 取消二次确认：必须写清“这批情报不会提醒用户”的后果 */}
      <DeleteConfirmModal
        isOpen={cancelDialog.isOpen}
        onClose={() => {
          cancelDialog.onClose();
          setCancelTarget(null);
        }}
        title="取消匹配任务"
        description={
          <>
            确定取消
            <Text as="span" fontWeight="700" color="workbench.text">
              {cancelTarget?.task_type_name}（{cancelTarget?.task_no}）
            </Text>
            吗？取消后
            <Text as="span" fontWeight="700" color="workbench.text">
              这批情报不会给任何用户生成提醒
            </Text>
            ，也不会被自动重跑；需要时可在任务记录里按原情报范围重新入队。
          </>
        }
        isLoading={cancelLoading}
        handleConfirm={handleCancel}
      />

      <DeleteConfirmModal
        isOpen={deleteDialog.isOpen}
        onClose={() => {
          deleteDialog.onClose();
          setDeleteTarget(null);
        }}
        title="删除匹配任务"
        description={
          <>
            确定删除
            <Text as="span" fontWeight="700" color="workbench.text">
              {deleteTarget?.task_type_name}（{deleteTarget?.task_no}）
            </Text>
            的记录吗？删除后这条任务记录不可恢复；已生成的提醒不受影响，排队中或执行中的任务需先取消才能删除。
          </>
        }
        isLoading={matchDeleteLoading}
        handleConfirm={handleDelete}
      />

      {/* 手动补扫：按时间窗对在架情报重新匹配一遍全量启用订阅 */}
      <Modal
        isOpen={rescanDialog.isOpen}
        onClose={rescanDialog.onClose}
        isCentered
      >
        <ModalOverlay />
        <ModalContent
          mx={3}
          maxW={{ base: "calc(100vw - 1.5rem)", sm: "32rem" }}
        >
          <ModalHeader fontSize="md">手动补扫</ModalHeader>
          <ModalCloseButton />
          <ModalBody>
            <Stack spacing={4}>
              <Text fontSize="sm" color="workbench.muted">
                按发布时间窗对在架情报重新执行一次订阅匹配。命中的提醒按唯一键去重，因此只有历史上确实漏掉的提醒会被补出来。
              </Text>
              <HStack spacing={3} align="flex-end" wrap="wrap">
                <Box>
                  <Text fontSize="xs" color="workbench.muted" mb={1}>
                    发布时间从
                  </Text>
                  <Input
                    h="44px"
                    type="date"
                    value={rescanFrom}
                    onChange={(event) => setRescanFrom(event.target.value)}
                    aria-label="补扫开始日期"
                  />
                </Box>
                <Box>
                  <Text fontSize="xs" color="workbench.muted" mb={1}>
                    到
                  </Text>
                  <Input
                    h="44px"
                    type="date"
                    value={rescanTo}
                    onChange={(event) => setRescanTo(event.target.value)}
                    aria-label="补扫结束日期"
                  />
                </Box>
              </HStack>
              <Text fontSize="xs" color="workbench.muted">
                单次补扫最多覆盖 2 万条情报，超出请缩小时间范围。
              </Text>
            </Stack>
          </ModalBody>
          <ModalFooter gap={3}>
            <Button variant="ghost" onClick={rescanDialog.onClose}>
              取消
            </Button>
            <Button
              colorScheme="primary"
              isLoading={rescanLoading}
              isDisabled={!rescanFrom || !rescanTo}
              onClick={handleRescan}
            >
              开始补扫
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </Stack>
  );
}
