"use client";

import React, {
  useState,
  useEffect,
  useMemo,
  useCallback,
  useRef,
} from "react";
import {
  AlertDialog,
  AlertDialogBody,
  AlertDialogContent,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogOverlay,
  Badge,
  Box,
  Button,
  Center,
  Flex,
  HStack,
  Spinner,
  Text,
  Tooltip,
  VStack,
} from "@chakra-ui/react";
import {
  CheckCircleIcon,
  DeleteIcon,
  QuestionOutlineIcon,
  WarningIcon,
} from "@chakra-ui/icons";
import { FiSettings } from "react-icons/fi";
import useAxios from "axios-hooks";
import { useCustomToast } from "@/hooks/useCustomToast";
import PageHeader from "@/components/common/page-header";
import ModelConfigForm from "@/components/layout/ModelConfigForm";
import {
  PageContent,
  PageViewport,
} from "@/components/layout/responsive-page";

type ConfirmState =
  | { type: "module"; key: string; name: string }
  | { type: "clear-all" };

const EMPTY_FIELD = {
  base_url: "",
  api_key: "",
  model: "",
  endpoint_path: "/chat/completions",
  context_window_tokens: 32768,
  max_output_tokens: 8192,
};

// 是否完全未填写（base_url/api_key/model 均空；endpoint_path 有默认值不参与）
function isAllEmpty(field) {
  if (!field) return true;
  return !field.base_url && !field.api_key && !field.model;
}

// 四项配置是否完整（base_url / api_key / model / endpoint_path 均非空）
function isModuleConfigComplete(field) {
  if (!field) return false;
  return (
    !!field.base_url &&
    !!field.api_key &&
    !!field.model &&
    !!field.endpoint_path
  );
}

function contextBudgetError(field) {
  if (!field || isAllEmpty(field)) return "";
  const context = Number(field.context_window_tokens);
  const output = Number(field.max_output_tokens);
  if (!Number.isInteger(context) || !Number.isInteger(output)) {
    return "上下文窗口和最大输出必须是整数";
  }
  if (context < 8192 || output < 512 || output + 4096 > context) {
    return "上下文至少 8192，输出至少 512，并需预留 4096 tokens 输入与安全余量";
  }
  return "";
}

type TestResultState = {
  status: "success" | "error";
  message: string;
  model?: string;
  latencyMs?: number;
};

// 测试连接结果条（成功绿 / 失败红，卡片内联展示）
function TestResultBar({ result }: { result: TestResultState }) {
  if (result.status === "success") {
    return (
      <Flex
        align="center"
        gap={2}
        px={3}
        py={2}
        borderRadius="md"
        bg="success.50"
        border="1px solid"
        borderColor="success.200"
        color="success.700"
        fontSize="sm"
      >
        <CheckCircleIcon boxSize={4} flexShrink={0} />
        <Text>
          连接成功 · 模型 {result.model || "—"} · 耗时 {result.latencyMs ?? "—"}{" "}
          ms
        </Text>
      </Flex>
    );
  }
  return (
    <Flex
      align="flex-start"
      gap={2}
      px={3}
      py={2}
      borderRadius="md"
      bg="error.50"
      border="1px solid"
      borderColor="error.200"
      color="error.700"
      fontSize="sm"
    >
      <WarningIcon boxSize={4} flexShrink={0} mt={0.5} />
      <Text>连接失败：{result.message}</Text>
    </Flex>
  );
}

// 清空确认弹窗内的"不会滥用"提示块
function RetainHintBlock() {
  return (
    <Flex
      align="flex-start"
      gap={2}
      mt={4}
      px={3}
      py={2.5}
      borderRadius="lg"
      bg="warning.50"
      border="1px solid"
      borderColor="warning.200"
    >
      <WarningIcon boxSize={4} color="warning.500" flexShrink={0} mt={0.5} />
      <Text fontSize="sm" color="warning.700" fontWeight="500">
        标擎承诺不会滥用您的模型配置，建议保留，方便后续使用！
      </Text>
    </Flex>
  );
}

export default function LLMConfigPage() {
  const showToast = useCustomToast();
  const confirmRef = useRef(null);

  // --- fetch ---
  const [{ data: modulesData }] = useAxios({
    url: "/system/modules",
    method: "GET",
  });
  const [{ data: configData, loading: loadingConfig }, refetch] = useAxios({
    url: "/system/llm-config",
    method: "GET",
  });

  const [, saveOne] = useAxios(
    { url: "/system/llm-config", method: "PUT" },
    { manual: true },
  );
  const [, deleteOne] = useAxios(
    { url: "/system/llm-config", method: "DELETE" },
    { manual: true },
  );
  const [, testAI] = useAxios(
    { url: "/system/test-ai", method: "POST" },
    { manual: true },
  );

  // --- state ---
  const modulesList = useMemo(() => modulesData?.data || [], [modulesData]);
  const [global, setGlobal] = useState({ ...EMPTY_FIELD });
  const [modules, setModules] = useState({});
  const [busy, setBusy] = useState<string | null>(null);
  const [confirm, setConfirm] = useState<ConfirmState | null>(null);
  const [testingKey, setTestingKey] = useState<string | null>(null);
  const [testResult, setTestResult] = useState<
    Record<string, TestResultState | null>
  >({});

  // --- init from API ---
  // 模块表单始终展示该模块自身的配置：已保存（含部分填写）按 DB 回显，未配置则为空，不预填全局副本。
  useEffect(() => {
    if (!configData?.data || !modulesList.length) return;
    const d = configData.data as any;
    const g = d.all || { ...EMPTY_FIELD };
    setGlobal({ ...EMPTY_FIELD, ...g });

    const modMap = {};
    modulesList.forEach(({ key }) => {
      modMap[key] = d[key] ? { ...EMPTY_FIELD, ...d[key] } : { ...EMPTY_FIELD };
    });
    setModules(modMap);
  }, [configData, modulesList]);

  // --- 保存单个模块（含全局 all）：全空=清空，否则覆盖 ---
  const saveOneModule = useCallback(
    async (key: string, field: any) => {
      const budgetError = contextBudgetError(field);
      if (budgetError) {
        showToast({ status: "error", title: "上下文预算无效", description: budgetError });
        return;
      }
      setBusy(key);
      try {
        if (isAllEmpty(field)) {
          await deleteOne({ url: `/system/llm-config/${key}` });
        } else {
          await saveOne({ url: `/system/llm-config/${key}`, data: field });
        }
        if (key !== "all" && isAllEmpty(field)) {
          setModules((prev) => ({ ...prev, [key]: { ...EMPTY_FIELD } }));
        }
        showToast({ status: "success", title: "保存成功" });
      } catch {
        showToast({ status: "error", title: "保存失败，请稍后再试" });
      } finally {
        setBusy(null);
      }
    },
    [saveOne, deleteOne, showToast],
  );

  // --- 保存全局（兜底配置：仅写 all 行；各模块独立配置保持不变） ---
  const saveGlobal = useCallback(async () => {
    const budgetError = contextBudgetError(global);
    if (budgetError) {
      showToast({ status: "error", title: "上下文预算无效", description: budgetError });
      return;
    }
    setBusy("all");
    try {
      if (isAllEmpty(global)) {
        await deleteOne({ url: "/system/llm-config/all" });
      } else {
        await saveOne({ url: "/system/llm-config/all", data: global });
      }
      refetch();
      showToast({ status: "success", title: "保存成功" });
    } catch {
      showToast({ status: "error", title: "保存失败，请稍后再试" });
    } finally {
      setBusy(null);
    }
  }, [global, saveOne, deleteOne, refetch, showToast]);

  // --- 清空单个模块（DELETE /:module） ---
  const clearModule = useCallback(
    async (key: string) => {
      const name =
        key === "all"
          ? "全局配置"
          : modulesList.find((m) => m.key === key)?.name || key;
      setConfirm({ type: "module", key, name });
    },
    [modulesList],
  );

  const confirmClearModule = useCallback(async () => {
    if (!confirm || confirm.type === "clear-all") return;
    const { key } = confirm;
    setBusy(key);
    setConfirm(null);
    try {
      await deleteOne({ url: `/system/llm-config/${key}` });
      if (key === "all") {
        setGlobal({ ...EMPTY_FIELD });
      } else {
        setModules((prev) => ({ ...prev, [key]: { ...EMPTY_FIELD } }));
      }
      showToast({ status: "success", title: "已清空" });
    } catch {
      showToast({ status: "error", title: "清空失败，请稍后再试" });
    } finally {
      setBusy(null);
    }
  }, [confirm, deleteOne, showToast]);

  // --- 一键清空全部（DELETE /system/llm-config） ---
  const confirmClearAll = useCallback(async () => {
    setBusy("clear-all");
    setConfirm(null);
    try {
      await deleteOne({ url: "/system/llm-config" });
      refetch();
      showToast({ status: "success", title: "已清空全部配置" });
    } catch {
      showToast({ status: "error", title: "清空失败，请稍后再试" });
    } finally {
      setBusy(null);
    }
  }, [deleteOne, refetch, showToast]);

  // --- 测试连接（POST /system/test-ai，使用当前表单未保存的值） ---
  const testConnection = useCallback(
    async (key: string, field: any) => {
      if (testingKey) return;
      const budgetError = contextBudgetError(field);
      if (budgetError) {
        setTestResult((prev) => ({ ...prev, [key]: { status: "error", message: budgetError } }));
        return;
      }
      setTestingKey(key);
      setTestResult((prev) => ({ ...prev, [key]: null }));
      try {
        const res = await testAI({
          url: "/system/test-ai",
          method: "POST",
          data: { module: key, ...field },
          timeout: 35000,
        });
        if (res?.data?.code === 0) {
          const d = res?.data?.data || {};
          setTestResult((prev) => ({
            ...prev,
            [key]: {
              status: "success",
              message: "连接成功",
              model: d.model,
              latencyMs: d.latency_ms,
            },
          }));
        } else {
          setTestResult((prev) => ({
            ...prev,
            [key]: {
              status: "error",
              message: res?.data?.message || "测试失败，请稍后重试",
            },
          }));
        }
      } catch (e: any) {
        setTestResult((prev) => ({
          ...prev,
          [key]: {
            status: "error",
            message:
              e?.response?.data?.message || "无法连接到服务器，请稍后重试",
          },
        }));
      } finally {
        setTestingKey(null);
      }
    },
    [testingKey, testAI],
  );

  // --- helpers ---
  const moduleStatus = (key) =>
    isModuleConfigComplete(modules[key]) ? "已单独配置" : "继承全局配置";

  return (
    <PageViewport
      w="full"
      className="thin-scrollbars"
      bg="neutral.50"
    >
      <PageContent py={{ base: 5, md: 6 }}>
        <PageHeader
          title="模型配置"
          description="为标擎各功能模块配置可用的 LLM 模型与 API Key"
        />

        {loadingConfig && !configData?.data ? (
          <Center py={16}>
            <Spinner color="primary.500" />
          </Center>
        ) : (
          <Box
            display="grid"
            gridTemplateColumns="repeat(auto-fit, minmax(min(100%, 32rem), 1fr))"
            gap={5}
            alignItems="start"
          >
            {/* 全局配置 */}
            <Box
              p={5}
              borderRadius="xl"
              bg="white"
              border="1px solid"
              borderColor="neutral.100"
              transition="all 0.2s"
              _hover={{
                borderColor: "primary.200",
                boxShadow: "0 4px 12px rgba(0,0,0,0.06)",
              }}
            >
              <Flex align="center" justify="space-between" mb={4}>
                <HStack spacing={2}>
                  <Flex
                    w="8"
                    h="8"
                    borderRadius="lg"
                    align="center"
                    justify="center"
                    bg="primary.50"
                    color="primary.600"
                  >
                    <FiSettings size="16" />
                  </Flex>
                  <Text fontSize="sm" fontWeight="700" color="neutral.800">
                    全局配置
                  </Text>
                </HStack>
              </Flex>
              <VStack spacing={3}>
                <ModelConfigForm
                  values={global}
                  onChange={setGlobal}
                  showLabels
                  required
                />
              </VStack>
              {testResult.all && <TestResultBar result={testResult.all} />}
              <Flex align="center" justify="space-between" mt={4}>
                <Button
                  size="sm"
                  variant="ghost"
                  color="neutral.400"
                  leftIcon={<DeleteIcon />}
                  isDisabled={!!busy || !!testingKey}
                  onClick={() => clearModule("all")}
                  borderRadius="lg"
                  _hover={{ color: "error.600", bg: "error.50" }}
                >
                  清空
                </Button>
                <HStack spacing={3}>
                  <Button
                    size="sm"
                    variant="outline"
                    colorScheme="primary"
                    isLoading={testingKey === "all"}
                    isDisabled={
                      !!busy || (testingKey !== null && testingKey !== "all")
                    }
                    onClick={() => testConnection("all", global)}
                    borderRadius="lg"
                  >
                    测试连接
                  </Button>
                  <Button
                    size="sm"
                    colorScheme="primary"
                    isLoading={busy === "all"}
                    isDisabled={!!busy || !!testingKey}
                    onClick={saveGlobal}
                    borderRadius="lg"
                  >
                    保存
                  </Button>
                </HStack>
              </Flex>
            </Box>

            {/* 各功能模块独立配置（空/不完整时自动使用全局配置兜底） */}
            {modulesList.map(({ key, name }) => (
              <Box
                key={key}
                p={5}
                borderRadius="xl"
                bg="white"
                border="1px solid"
                borderColor="neutral.100"
                transition="all 0.2s"
                _hover={{
                  borderColor: "gold.400",
                  boxShadow: "0 8px 24px rgba(30,58,95,0.10)",
                }}
                position="relative"
                overflow="hidden"
              >
                <Box
                  position="absolute"
                  top={0}
                  left={0}
                  right={0}
                  h="3px"
                  bgGradient="linear(to-r, primary.600, gold.500)"
                  opacity={isModuleConfigComplete(modules[key]) ? 1 : 0.2}
                />
                <Flex align="center" justify="space-between" mb={4}>
                  <HStack spacing={2}>
                    <Text fontSize="sm" fontWeight="700" color="neutral.800">
                      {name}
                    </Text>
                    <Badge
                      fontSize="xs"
                      colorScheme={
                        isModuleConfigComplete(modules[key]) ? "green" : "gray"
                      }
                      borderRadius="full"
                      variant="subtle"
                    >
                      {moduleStatus(key)}
                    </Badge>
                    <Tooltip
                      hasArrow
                      placement="top"
                      label={
                        isModuleConfigComplete(modules[key])
                          ? `${name}已独立配置，优先使用本模块配置`
                          : `${name}未完整配置，将使用全局配置兜底；若全局也为空则该模块 AI 能力暂不可用`
                      }
                      bg="neutral.800"
                      color="white"
                      fontSize="xs"
                      borderRadius="lg"
                      px={3}
                      py={2}
                    >
                      <Box
                        as="span"
                        cursor="help"
                        display="inline-flex"
                        alignItems="center"
                      >
                        <QuestionOutlineIcon
                          boxSize={3.5}
                          color="neutral.400"
                        />
                      </Box>
                    </Tooltip>
                  </HStack>
                </Flex>
                <VStack spacing={3}>
                  <ModelConfigForm
                    values={modules[key] || { ...EMPTY_FIELD }}
                    onChange={(v) =>
                      setModules((prev) => ({ ...prev, [key]: v }))
                    }
                    showLabels
                  />
                </VStack>
                {testResult[key] && <TestResultBar result={testResult[key]} />}
                <Flex align="center" justify="space-between" mt={4}>
                  <Button
                    size="sm"
                    variant="ghost"
                    color="neutral.400"
                    leftIcon={<DeleteIcon />}
                    isDisabled={!!busy || !!testingKey}
                    onClick={() => clearModule(key)}
                    borderRadius="lg"
                    _hover={{ color: "error.600", bg: "error.50" }}
                  >
                    清空
                  </Button>
                  <HStack spacing={3}>
                    <Button
                      size="sm"
                      variant="outline"
                      colorScheme="primary"
                      isLoading={testingKey === key}
                      isDisabled={
                        !!busy || (testingKey !== null && testingKey !== key)
                      }
                      onClick={() =>
                        testConnection(key, modules[key] || { ...EMPTY_FIELD })
                      }
                      borderRadius="lg"
                    >
                      测试连接
                    </Button>
                    <Button
                      size="sm"
                      colorScheme="primary"
                      isLoading={busy === key}
                      isDisabled={!!busy || !!testingKey}
                      onClick={() => saveOneModule(key, modules[key])}
                      borderRadius="lg"
                    >
                      保存
                    </Button>
                  </HStack>
                </Flex>
              </Box>
            ))}

            {/* 底部操作区 */}
            <Flex
              gridColumn="1 / -1"
              justify="flex-start"
              align="center"
              pt={2}
            >
              <Button
                variant="outline"
                colorScheme="red"
                size="md"
                leftIcon={<DeleteIcon />}
                onClick={() => setConfirm({ type: "clear-all" })}
                isDisabled={!!busy || !!testingKey}
                borderRadius="lg"
              >
                清空全部配置
              </Button>
            </Flex>
          </Box>
        )}
      </PageContent>

      {/* 确认弹窗：清空单模块 / 清空全部 */}
      <AlertDialog
        isOpen={!!confirm}
        leastDestructiveRef={confirmRef}
        onClose={() => setConfirm(null)}
      >
        <AlertDialogOverlay>
          <AlertDialogContent borderRadius="xl">
            <AlertDialogHeader
              fontSize="lg"
              fontWeight="700"
              color="neutral.900"
            >
              {confirm?.type === "clear-all"
                ? "清空全部模型配置"
                : "清空模块配置"}
            </AlertDialogHeader>
            <AlertDialogBody>
              {confirm?.type === "clear-all" ? (
                <Text color="neutral.600">
                  将删除全部模型配置（含 API Key），且不可恢复。确定清空吗？
                </Text>
              ) : (
                <Text color="neutral.600">
                  {confirm?.key === "all" ? (
                    <>
                      将删除{" "}
                      <Text as="span" color="primary.600" fontWeight="600">
                        {" "}
                        全局配置{" "}
                      </Text>{" "}
                      的模型配置（含 API
                      Key），所有未独立配置的功能模块将无法使用 AI
                      能力。确定清空吗？
                    </>
                  ) : (
                    <>
                      将删除{" "}
                      <Text as="span" color="primary.600" fontWeight="600">
                        {" "}
                        {confirm?.name}{" "}
                      </Text>{" "}
                      的模型配置（含 API
                      Key），该模块将回退使用全局配置。确定清空吗？
                    </>
                  )}
                </Text>
              )}
              <RetainHintBlock />
            </AlertDialogBody>
            <AlertDialogFooter>
              <Button
                ref={confirmRef}
                onClick={() => setConfirm(null)}
                borderRadius="lg"
                variant="ghost"
              >
                取消
              </Button>
              <Button
                colorScheme="red"
                ml={3}
                isLoading={!!busy}
                onClick={
                  confirm?.type === "clear-all"
                    ? confirmClearAll
                    : confirmClearModule
                }
                borderRadius="lg"
              >
                清空
              </Button>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialogOverlay>
      </AlertDialog>
    </PageViewport>
  );
}
