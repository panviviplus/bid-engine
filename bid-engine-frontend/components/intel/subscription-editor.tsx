"use client";

/* Hallmark · component: intel-subscription-editor · genre: modern-minimal · theme: BidEngine deep-sea/gold (preserved tokens)
 * states: default · hover · focus-visible · active · disabled · loading(自然语言解析/保存) · error · success
 * 交互取向：自然语言只是“草稿助手”，必须先由用户确认再落库。
 */
import React, { useEffect, useMemo, useState } from "react";
import {
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
  Radio,
  RadioGroup,
  SimpleGrid,
  Stack,
  Text,
  Textarea,
  useToast,
} from "@chakra-ui/react";

import type {
  IntelFilters,
  IntelSubscription,
  IntelSubscriptionDraft,
} from "@/service/intel";
import { useIntelSubscriptionParse } from "@/service/intel";
import { OptionTagField, SuggestTagField } from "@/components/intel/tag-field";

const FIELD_H = "44px";

export type SubscriptionFormValue = {
  name: string;
  keywords: string;
  matchMode: string;
  industries: string[];
  regions: string[];
  noticeTypes: string[];
  budgetMin: string;
  budgetMax: string;
  enabled: boolean;
};

export const EMPTY_SUBSCRIPTION_FORM: SubscriptionFormValue = {
  name: "",
  keywords: "",
  matchMode: "any",
  industries: [],
  regions: [],
  noticeTypes: [],
  budgetMin: "",
  budgetMax: "",
  enabled: true,
};

/** 行业/地区在多选控件里都以“字符串数组”维护：已知行业存 code，自定义存文本。 */
export function toSubscriptionForm(
  item: IntelSubscription,
  knownIndustryCodes: string[] = [],
): SubscriptionFormValue {
  const known = new Set(knownIndustryCodes);
  return {
    name: item.name || "",
    keywords: (item.keywords || []).join("、"),
    matchMode: item.match_mode || "any",
    // 已知 code 保留编码（匹配更精确），其余视为用户自定义行业文本
    industries: (item.industries || []).map((code, index) =>
      known.has(code) ? code : (item.industry_names || [])[index] || code,
    ),
    regions: item.regions || [],
    noticeTypes: item.notice_types || [],
    budgetMin: item.budget_min ? String(item.budget_min) : "",
    budgetMax: item.budget_max ? String(item.budget_max) : "",
    enabled: item.enabled,
  };
}

/** 表单 → 接口请求体。 */
export function toSubscriptionPayload(value: SubscriptionFormValue) {
  const splitList = (raw: string) =>
    raw
      .split(/[、,，\s]+/)
      .map((item) => item.trim())
      .filter(Boolean);
  return {
    name: value.name.trim(),
    keywords: splitList(value.keywords),
    match_mode: value.matchMode,
    industries: value.industries,
    regions: value.regions,
    notice_types: value.noticeTypes,
    budget_min: value.budgetMin ? Number(value.budgetMin) : null,
    budget_max: value.budgetMax ? Number(value.budgetMax) : null,
    enabled: value.enabled,
  };
}

export default function SubscriptionEditor({
  isOpen,
  onClose,
  filters,
  editing,
  onSubmit,
  submitting,
}: {
  isOpen: boolean;
  onClose: () => void;
  filters: IntelFilters | null;
  editing: IntelSubscription | null;
  // eslint-disable-next-line no-unused-vars
  onSubmit: (value: SubscriptionFormValue) => void | Promise<void>;
  submitting: boolean;
}) {
  const toast = useToast();
  const [form, setForm] = useState<SubscriptionFormValue>(
    EMPTY_SUBSCRIPTION_FORM,
  );
  const [nlText, setNlText] = useState("");
  const { parseLoading, fetchParse } = useIntelSubscriptionParse();

  useEffect(() => {
    if (!isOpen) return;
    setNlText("");
    setForm(
      editing
        ? toSubscriptionForm(
            editing,
            (filters?.industries || []).map((item) => item.code),
          )
        : { ...EMPTY_SUBSCRIPTION_FORM },
    );
  }, [isOpen, editing, filters]);

  const patch = (part: Partial<SubscriptionFormValue>) =>
    setForm((prev) => ({ ...prev, ...part }));

  const hasCondition = useMemo(() => {
    const payload = toSubscriptionPayload(form);
    return (
      payload.keywords.length > 0 ||
      payload.industries.length > 0 ||
      payload.regions.length > 0 ||
      payload.notice_types.length > 0 ||
      payload.budget_min !== null ||
      payload.budget_max !== null
    );
  }, [form]);

  const handleParse = async () => {
    if (!nlText.trim()) {
      toast({
        title: "请先描述你想关注的项目",
        status: "warning",
        duration: 2500,
        isClosable: true,
      });
      return;
    }
    try {
      const res = await fetchParse({ data: { text: nlText.trim() } });
      const draft = res?.data?.data as IntelSubscriptionDraft | undefined;
      if (!draft) throw new Error("解析结果为空");
      setForm((prev) => ({
        ...prev,
        name: draft.name || prev.name,
        keywords: (draft.keywords || []).join("、"),
        matchMode: draft.match_mode || prev.matchMode,
        industries: draft.industries || [],
        regions: draft.regions || [],
        noticeTypes: draft.notice_types || [],
        budgetMin: draft.budget_min ? String(draft.budget_min) : "",
        budgetMax: draft.budget_max ? String(draft.budget_max) : "",
      }));
      toast({
        title: "已生成草稿",
        description: draft.summary || "请确认条件后保存",
        status: "success",
        duration: 3500,
        isClosable: true,
      });
    } catch (error: any) {
      toast({
        title: "解析失败",
        description:
          error?.response?.data?.message || error?.message || "请稍后重试",
        status: "error",
        duration: 4000,
        isClosable: true,
      });
    }
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      size="3xl"
      isCentered
      scrollBehavior="inside"
    >
      <ModalOverlay />
      <ModalContent maxH="88dvh">
        <ModalHeader pb={2}>
          {editing ? "编辑订阅" : "新建订阅"}
          <Text mt={1} fontSize="sm" fontWeight="400" color="workbench.muted">
            关键词命中或行业命中即进入候选，再按地区、公告类型与预算收窄；未填写的条件表示不限。
            命中的新情报会生成提醒，可在该订阅卡片上查看，条件随时可改或停用。
          </Text>
        </ModalHeader>
        <ModalCloseButton />
        <ModalBody>
          <Stack spacing={6}>
            {/* ① 自然语言草稿：先解析，再由用户确认 */}
            <Box
              bg="workbench.canvas"
              borderRadius="lg"
              borderWidth="1px"
              borderColor="neutral.100"
              p={{ base: 4, md: 5 }}
            >
              <FormControl>
                <FormLabel fontSize="sm" fontWeight="600">
                  用自然语言描述（可选）
                </FormLabel>
                <Textarea
                  value={nlText}
                  onChange={(event) => setNlText(event.target.value)}
                  placeholder="例如：帮我盯华东地区预算 100 万以上的政务信息化项目"
                  rows={2}
                  bg="white"
                  resize="vertical"
                  _focusVisible={{
                    borderColor: "primary.500",
                    boxShadow: "0 0 0 2px var(--chakra-colors-primary-200)",
                    outline: "none",
                  }}
                />
              </FormControl>
              <Flex
                mt={3}
                justify="space-between"
                align="center"
                gap={3}
                wrap="wrap"
              >
                <Text fontSize="xs" color="workbench.muted">
                  解析结果会填入下方表单，确认后才保存
                </Text>
                <Button
                  h={FIELD_H}
                  variant="outline"
                  borderColor="primary.200"
                  color="primary.600"
                  isLoading={parseLoading}
                  loadingText="解析中"
                  onClick={handleParse}
                  _hover={{ bg: "primary.50" }}
                  _active={{ transform: "scale(0.98)" }}
                >
                  解析为条件
                </Button>
              </Flex>
            </Box>

            {/* ② 基础信息 */}
            <SimpleGrid columns={{ base: 1, md: 2 }} spacing={5}>
              <FormControl>
                <FormLabel fontSize="sm" fontWeight="600">
                  订阅名称
                </FormLabel>
                <Input
                  h={FIELD_H}
                  value={form.name}
                  onChange={(event) => patch({ name: event.target.value })}
                  placeholder="留空将根据关键词自动命名"
                  borderColor="neutral.200"
                  _focusVisible={{
                    borderColor: "primary.500",
                    boxShadow: "0 0 0 2px var(--chakra-colors-primary-200)",
                    outline: "none",
                  }}
                />
              </FormControl>

              <FormControl>
                <FormLabel fontSize="sm" fontWeight="600">
                  关键词匹配方式
                </FormLabel>
                <RadioGroup
                  value={form.matchMode}
                  onChange={(value) => patch({ matchMode: value })}
                >
                  <HStack spacing={6} minH={FIELD_H}>
                    <Radio value="any">任一命中</Radio>
                    <Radio value="all">全部命中</Radio>
                  </HStack>
                </RadioGroup>
              </FormControl>
            </SimpleGrid>

            <FormControl>
              <FormLabel fontSize="sm" fontWeight="600">
                关键词
              </FormLabel>
              <Input
                h={FIELD_H}
                value={form.keywords}
                onChange={(event) => patch({ keywords: event.target.value })}
                placeholder="多个关键词用、或逗号分隔，例如：知识库、智能问答"
                borderColor="neutral.200"
                _focusVisible={{
                  borderColor: "primary.500",
                  boxShadow: "0 0 0 2px var(--chakra-colors-primary-200)",
                  outline: "none",
                }}
              />
            </FormControl>

            <Divider />

            {/* ③ 行业与地区：均可自由输入 */}
            <SimpleGrid columns={{ base: 1, md: 2 }} spacing={5}>
              <SuggestTagField
                label="行业（可选）"
                helperText="可点常用项，也可直接输入自定义行业后回车添加"
                placeholder="输入行业名称，如：智慧水务"
                value={form.industries}
                onChange={(next) => patch({ industries: next })}
                suggestions={(filters?.industries || []).map(
                  (item) => item.name,
                )}
              />
              <SuggestTagField
                label="地区（可选）"
                helperText="可点常用省份，也可输入城市或区县后回车添加"
                placeholder="输入地区，如：深圳市"
                value={form.regions}
                onChange={(next) => patch({ regions: next })}
                suggestions={filters?.regions || []}
              />
            </SimpleGrid>

            <OptionTagField
              label="公告类型（可选）"
              helperText="不选表示不限类型；变更、终止类公告也会按所选类型命中"
              options={filters?.notice_types || []}
              value={form.noticeTypes}
              onChange={(next) => patch({ noticeTypes: next })}
            />

            <Divider />

            {/* ④ 预算区间 */}
            <SimpleGrid columns={{ base: 1, md: 2 }} spacing={5}>
              <FormControl>
                <FormLabel fontSize="sm" fontWeight="600">
                  预算下限（元）
                </FormLabel>
                <Input
                  h={FIELD_H}
                  inputMode="numeric"
                  value={form.budgetMin}
                  onChange={(event) => patch({ budgetMin: event.target.value })}
                  placeholder="例如：1000000"
                  borderColor="neutral.200"
                  _focusVisible={{
                    borderColor: "primary.500",
                    boxShadow: "0 0 0 2px var(--chakra-colors-primary-200)",
                    outline: "none",
                  }}
                />
              </FormControl>
              <FormControl>
                <FormLabel fontSize="sm" fontWeight="600">
                  预算上限（元）
                </FormLabel>
                <Input
                  h={FIELD_H}
                  inputMode="numeric"
                  value={form.budgetMax}
                  onChange={(event) => patch({ budgetMax: event.target.value })}
                  placeholder="留空表示不限"
                  borderColor="neutral.200"
                  _focusVisible={{
                    borderColor: "primary.500",
                    boxShadow: "0 0 0 2px var(--chakra-colors-primary-200)",
                    outline: "none",
                  }}
                />
              </FormControl>
            </SimpleGrid>
          </Stack>
        </ModalBody>
        <ModalFooter gap={3}>
          <Button variant="ghost" onClick={onClose} h={FIELD_H}>
            取消
          </Button>
          <Button
            h={FIELD_H}
            colorScheme="primary"
            isLoading={submitting}
            loadingText="保存中"
            isDisabled={!hasCondition}
            onClick={() => onSubmit(form)}
            _active={{ transform: "scale(0.98)" }}
          >
            {editing ? "保存修改" : "创建订阅"}
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
}
