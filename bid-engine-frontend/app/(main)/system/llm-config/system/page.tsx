"use client";

/* eslint-disable no-nested-ternary */

/* Hallmark · macrostructure: Workbench · genre: modern-minimal · theme: BidEngine deep-sea/gold (preserved tokens)
 * audience: 系统超管 · use: 维护平台级后台任务使用的全局模型配置 · tone: technical, austere
 * 交互：按 sort 升序取第一条“字段完整”的配置；失败自动降级到下一候选。
 * Hallmark · pre-emit critique: P5 H4 E5 S5 R5 V4
 */
import React, { useState } from "react";
import {
  Badge,
  Box,
  Button,
  Divider,
  Flex,
  FormControl,
  FormLabel,
  HStack,
  Input,
  Modal,
  ModalBody,
  ModalCloseButton,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  Skeleton,
  Stack,
  Text,
  useDisclosure,
  useToast,
} from "@chakra-ui/react";
import { FiEdit3, FiPlus, FiTrash2, FiZap } from "react-icons/fi";

import EmptyState from "@/components/common/empty-state";
import { PageContent, PageViewport } from "@/components/layout/responsive-page";
import { ModuleWorkbenchHeader } from "@/components/common/module-workbench.mjs";
import {
  useSystemLLMConfigDelete,
  useSystemLLMConfigSave,
  useSystemLLMConfigTest,
  useSystemLLMConfigs,
  type SystemLLMConfig,
} from "@/service/intel";

const FIELD_H = "44px";

type ConfigForm = {
  name: string;
  baseUrl: string;
  apiKey: string;
  model: string;
  endpointPath: string;
  contextWindowTokens: string;
  maxOutputTokens: string;
  sort: string;
};

const EMPTY_FORM: ConfigForm = {
  name: "",
  baseUrl: "",
  apiKey: "",
  model: "",
  endpointPath: "/chat/completions",
  contextWindowTokens: "",
  maxOutputTokens: "",
  sort: "100",
};

export default function SystemLLMConfigPage() {
  const toast = useToast();
  const { configs, configsLoading, refreshConfigs } = useSystemLLMConfigs();
  const { saveLoading, fetchCreate, fetchUpdate } = useSystemLLMConfigSave();
  const { deleteLoading, fetchDelete } = useSystemLLMConfigDelete();
  const { testLoading, fetchTest } = useSystemLLMConfigTest();
  const { isOpen, onOpen, onClose } = useDisclosure();
  const [editing, setEditing] = useState<SystemLLMConfig | null>(null);
  const [form, setForm] = useState<ConfigForm>({ ...EMPTY_FORM });

  const patch = (part: Partial<ConfigForm>) =>
    setForm((prev) => ({ ...prev, ...part }));

  const openCreate = () => {
    setEditing(null);
    setForm({ ...EMPTY_FORM });
    onOpen();
  };

  const openEdit = (item: SystemLLMConfig) => {
    setEditing(item);
    setForm({
      name: item.name || "",
      baseUrl: item.base_url || "",
      apiKey: "",
      model: item.model || "",
      endpointPath: item.endpoint_path || "/chat/completions",
      contextWindowTokens: item.context_window_tokens
        ? String(item.context_window_tokens)
        : "",
      maxOutputTokens: item.max_output_tokens
        ? String(item.max_output_tokens)
        : "",
      sort: item.sort ? String(item.sort) : "100",
    });
    onOpen();
  };

  const buildPayload = () => ({
    name: form.name.trim(),
    base_url: form.baseUrl.trim(),
    api_key: form.apiKey.trim(),
    model: form.model.trim(),
    endpoint_path: form.endpointPath.trim() || "/chat/completions",
    context_window_tokens: form.contextWindowTokens
      ? Number(form.contextWindowTokens)
      : 0,
    max_output_tokens: form.maxOutputTokens ? Number(form.maxOutputTokens) : 0,
    sort: form.sort ? Number(form.sort) : 100,
  });

  const handleSubmit = async () => {
    const payload = buildPayload();
    if (!payload.base_url || !payload.model) {
      toast({
        title: "请填写 API 地址与模型名称",
        status: "warning",
        duration: 2500,
        isClosable: true,
      });
      return;
    }
    try {
      if (editing) {
        await fetchUpdate({
          url: `/sys/llm-config/${editing.id}`,
          data: payload,
        });
      } else {
        await fetchCreate({ url: "/sys/llm-config", data: payload });
      }
      toast({
        title: editing ? "配置已更新" : "配置已新增",
        status: "success",
        duration: 2500,
        isClosable: true,
      });
      onClose();
      refreshConfigs();
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

  const handleTest = async () => {
    try {
      const res = await fetchTest({
        data: {
          id: editing?.id || 0,
          ...buildPayload(),
        },
      });
      const data = res?.data?.data;
      toast({
        title: "连通性测试通过",
        description: `模型 ${data?.model || "-"} · 延迟 ${data?.latency_ms ?? "-"} ms`,
        status: "success",
        duration: 4000,
        isClosable: true,
      });
    } catch (error: any) {
      toast({
        title: "连通性测试失败",
        description:
          error?.response?.data?.message || error?.message || "请检查配置",
        status: "error",
        duration: 5000,
        isClosable: true,
      });
    }
  };

  const handleDelete = async (item: SystemLLMConfig) => {
    try {
      await fetchDelete({ url: `/sys/llm-config/${item.id}` });
      toast({
        title: "配置已删除",
        status: "success",
        duration: 2000,
        isClosable: true,
      });
      refreshConfigs();
    } catch (error: any) {
      toast({
        title: "删除失败",
        description: error?.response?.data?.message || "请稍后重试",
        status: "error",
        duration: 3000,
        isClosable: true,
      });
    }
  };

  return (
    <PageViewport bg="workbench.canvas">
      <PageContent py={{ base: 5, md: 6 }}>
        <ModuleWorkbenchHeader
          title="系统模型配置"
          titleSuffix={null}
          activity={
            <Button
              h="44px"
              colorScheme="primary"
              leftIcon={<FiPlus aria-hidden />}
              onClick={openCreate}
              _active={{ transform: "scale(0.98)" }}
            >
              新增配置
            </Button>
          }
        />

        <Box
          bg="primary.50"
          borderRadius="lg"
          borderLeftWidth="4px"
          borderLeftColor="primary.400"
          p={4}
          mb={5}
        >
          <Text fontSize="sm" color="primary.800">
            这份配置供平台级后台任务使用（例如招标情报站的公告打标），与个人“模型配置”互不影响。
            后台任务按 sort
            升序取第一条字段完整的配置；调用失败时自动降级到下一候选。
          </Text>
        </Box>

        {configsLoading ? (
          <Stack spacing={3}>
            {[0, 1].map((key) => (
              <Skeleton key={key} height="120px" borderRadius="xl" />
            ))}
          </Stack>
        ) : configs.length === 0 ? (
          <EmptyState
            title="还没有全局模型配置"
            description="未配置时后台任务会回退到 conf/llm-config.yml 中的默认模型；建议至少配置一条可用配置。"
            action={
              <Button colorScheme="primary" onClick={openCreate}>
                新增配置
              </Button>
            }
          />
        ) : (
          <Stack spacing={3}>
            {configs.map((item, index) => (
              <Box
                key={item.id}
                bg="white"
                borderRadius="xl"
                borderWidth="1px"
                borderColor={item.complete ? "neutral.100" : "orange.200"}
                p={{ base: 4, md: 5 }}
              >
                <Flex
                  justify="space-between"
                  align="flex-start"
                  gap={3}
                  wrap="wrap"
                >
                  <Box minW={0}>
                    <HStack spacing={2} wrap="wrap">
                      <Text fontWeight="700" color="workbench.text">
                        {item.name || `配置 #${item.id}`}
                      </Text>
                      <Badge
                        colorScheme={
                          index === 0 && item.complete ? "success" : "neutral"
                        }
                        variant="subtle"
                      >
                        {index === 0 && item.complete
                          ? "当前生效"
                          : `sort=${item.sort}`}
                      </Badge>
                      {!item.complete && (
                        <Badge colorScheme="orange" variant="subtle">
                          字段不完整，会被跳过
                        </Badge>
                      )}
                    </HStack>
                    <Text
                      mt={2}
                      fontSize="sm"
                      color="workbench.muted"
                      overflowWrap="anywhere"
                    >
                      {item.base_url} · {item.model} ·{" "}
                      {item.endpoint_path || "/chat/completions"}
                    </Text>
                    <Text mt={1} fontSize="sm" color="workbench.muted">
                      API Key：{item.api_key || "未设置"} · 更新时间：
                      {item.update_time || "-"}
                    </Text>
                  </Box>
                  <HStack spacing={2}>
                    <Button
                      h="44px"
                      size="sm"
                      variant="ghost"
                      leftIcon={<FiEdit3 aria-hidden />}
                      onClick={() => openEdit(item)}
                      _hover={{ bg: "primary.50", color: "primary.700" }}
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
                      onClick={() => handleDelete(item)}
                      _hover={{ bg: "red.50", color: "red.600" }}
                    >
                      删除
                    </Button>
                  </HStack>
                </Flex>
              </Box>
            ))}
          </Stack>
        )}

        <Modal isOpen={isOpen} onClose={onClose} size="lg" isCentered>
          <ModalOverlay />
          <ModalContent maxH="90dvh" overflowY="auto">
            <ModalHeader>
              {editing ? "编辑全局模型配置" : "新增全局模型配置"}
            </ModalHeader>
            <ModalCloseButton />
            <ModalBody>
              <Stack spacing={4}>
                <FormControl>
                  <FormLabel fontSize="sm">配置名称</FormLabel>
                  <Input
                    h={FIELD_H}
                    value={form.name}
                    onChange={(event) => patch({ name: event.target.value })}
                    placeholder="例如：DeepSeek 主力"
                    _focusVisible={{
                      borderColor: "primary.500",
                      boxShadow: "0 0 0 2px var(--chakra-colors-primary-200)",
                    }}
                  />
                </FormControl>
                <FormControl isRequired>
                  <FormLabel fontSize="sm">API 地址（base_url）</FormLabel>
                  <Input
                    h={FIELD_H}
                    value={form.baseUrl}
                    onChange={(event) => patch({ baseUrl: event.target.value })}
                    placeholder="https://api.deepseek.com"
                    _focusVisible={{
                      borderColor: "primary.500",
                      boxShadow: "0 0 0 2px var(--chakra-colors-primary-200)",
                    }}
                  />
                </FormControl>
                <FormControl isRequired>
                  <FormLabel fontSize="sm">
                    API Key{editing ? "（留空则不修改）" : ""}
                  </FormLabel>
                  <Input
                    h={FIELD_H}
                    type="password"
                    value={form.apiKey}
                    onChange={(event) => patch({ apiKey: event.target.value })}
                    placeholder={editing ? "留空保持原值" : "sk-..."}
                    _focusVisible={{
                      borderColor: "primary.500",
                      boxShadow: "0 0 0 2px var(--chakra-colors-primary-200)",
                    }}
                  />
                </FormControl>
                <HStack spacing={4} wrap="wrap">
                  <FormControl flex={1} minW="180px" isRequired>
                    <FormLabel fontSize="sm">模型名称</FormLabel>
                    <Input
                      h={FIELD_H}
                      value={form.model}
                      onChange={(event) => patch({ model: event.target.value })}
                      placeholder="deepseek-v4-flash"
                      _focusVisible={{
                        borderColor: "primary.500",
                        boxShadow: "0 0 0 2px var(--chakra-colors-primary-200)",
                      }}
                    />
                  </FormControl>
                  <FormControl flex={1} minW="180px">
                    <FormLabel fontSize="sm">端点路径</FormLabel>
                    <Input
                      h={FIELD_H}
                      value={form.endpointPath}
                      onChange={(event) =>
                        patch({ endpointPath: event.target.value })
                      }
                      placeholder="/chat/completions"
                      _focusVisible={{
                        borderColor: "primary.500",
                        boxShadow: "0 0 0 2px var(--chakra-colors-primary-200)",
                      }}
                    />
                  </FormControl>
                </HStack>
                <HStack spacing={4} wrap="wrap">
                  <FormControl maxW="200px">
                    <FormLabel fontSize="sm">上下文窗口</FormLabel>
                    <Input
                      h={FIELD_H}
                      inputMode="numeric"
                      value={form.contextWindowTokens}
                      onChange={(event) =>
                        patch({ contextWindowTokens: event.target.value })
                      }
                      placeholder="留空使用默认"
                      _focusVisible={{
                        borderColor: "primary.500",
                        boxShadow: "0 0 0 2px var(--chakra-colors-primary-200)",
                      }}
                    />
                  </FormControl>
                  <FormControl maxW="200px">
                    <FormLabel fontSize="sm">单次输出上限</FormLabel>
                    <Input
                      h={FIELD_H}
                      inputMode="numeric"
                      value={form.maxOutputTokens}
                      onChange={(event) =>
                        patch({ maxOutputTokens: event.target.value })
                      }
                      placeholder="留空使用默认"
                      _focusVisible={{
                        borderColor: "primary.500",
                        boxShadow: "0 0 0 2px var(--chakra-colors-primary-200)",
                      }}
                    />
                  </FormControl>
                  <FormControl maxW="160px">
                    <FormLabel fontSize="sm">sort（越小越优先）</FormLabel>
                    <Input
                      h={FIELD_H}
                      inputMode="numeric"
                      value={form.sort}
                      onChange={(event) => patch({ sort: event.target.value })}
                      _focusVisible={{
                        borderColor: "primary.500",
                        boxShadow: "0 0 0 2px var(--chakra-colors-primary-200)",
                      }}
                    />
                  </FormControl>
                </HStack>
              </Stack>
            </ModalBody>
            <Divider />
            <ModalFooter gap={3} flexWrap="wrap">
              <Button
                h={FIELD_H}
                variant="outline"
                borderColor="neutral.200"
                leftIcon={<FiZap aria-hidden />}
                isLoading={testLoading}
                loadingText="测试中"
                onClick={handleTest}
                _hover={{ borderColor: "primary.300", color: "primary.600" }}
              >
                连通性测试
              </Button>
              <Button variant="ghost" h={FIELD_H} onClick={onClose}>
                取消
              </Button>
              <Button
                h={FIELD_H}
                colorScheme="primary"
                isLoading={saveLoading}
                loadingText="保存中"
                onClick={handleSubmit}
                _active={{ transform: "scale(0.98)" }}
              >
                保存
              </Button>
            </ModalFooter>
          </ModalContent>
        </Modal>
      </PageContent>
    </PageViewport>
  );
}
