"use client";

/* Hallmark · component: intel-source-editor · genre: modern-minimal · theme: BidEngine deep-sea/gold (preserved tokens)
 * states: default · hover · focus-visible · active · disabled · loading(保存) · error(JSON 非法) · success
 */
import React, { useEffect, useState } from "react";
import {
  Button,
  Flex,
  FormControl,
  FormErrorMessage,
  FormHelperText,
  FormLabel,
  Input,
  Modal,
  ModalBody,
  ModalCloseButton,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  NumberInput,
  NumberInputField,
  SimpleGrid,
  Stack,
  Switch,
  Text,
  Textarea,
} from "@chakra-ui/react";

import IntelSelect, {
  type IntelSelectOption,
} from "@/components/intel/intel-select";
import type { IntelSource } from "@/service/intel";

const FIELD_H = "44px";

/** 发现方式：与采集服务的三级发现策略一一对应。 */
const DISCOVERY_MODE_OPTIONS: IntelSelectOption[] = [
  { value: "api", label: "站点接口（api）", hint: "最稳" },
  { value: "list", label: "静态列表页（list）", hint: "默认" },
  { value: "browser", label: "无头浏览器（browser）", hint: "需渲染" },
];

const FOCUS_PROPS = {
  _focusVisible: {
    borderColor: "primary.500",
    boxShadow: "0 0 0 2px var(--chakra-colors-primary-200)",
    outline: "none",
  },
};

export type SourceFormValue = {
  name: string;
  homepageUrl: string;
  listUrl: string;
  category: string;
  region: string;
  industryHint: string;
  discoveryMode: string;
  needsBrowser: boolean;
  enabled: boolean;
  priority: string;
  params: string;
};

export function toSourceForm(item: IntelSource): SourceFormValue {
  return {
    name: item.name || "",
    homepageUrl: item.homepage_url || "",
    listUrl: item.list_url || "",
    category: item.category || "",
    region: item.region || "",
    industryHint: item.industry_hint || "",
    discoveryMode: item.discovery_mode || "list",
    needsBrowser: Boolean(item.needs_browser),
    enabled: Boolean(item.enabled),
    priority: String(item.priority ?? 100),
    params: item.params || "",
  };
}

export function toSourcePayload(value: SourceFormValue) {
  return {
    name: value.name.trim(),
    homepage_url: value.homepageUrl.trim(),
    list_url: value.listUrl.trim(),
    category: value.category.trim(),
    region: value.region.trim(),
    industry_hint: value.industryHint.trim(),
    discovery_mode: value.discoveryMode,
    needs_browser: value.needsBrowser,
    enabled: value.enabled,
    priority: Number(value.priority || 100),
    params: value.params.trim(),
  };
}

export default function SourceEditorModal({
  isOpen,
  onClose,
  source,
  onSubmit,
  submitting,
}: {
  isOpen: boolean;
  onClose: () => void;
  source: IntelSource | null;
  // eslint-disable-next-line no-unused-vars
  onSubmit: (value: SourceFormValue) => void | Promise<void>;
  submitting: boolean;
}) {
  const [form, setForm] = useState<SourceFormValue | null>(null);
  const [paramsError, setParamsError] = useState("");

  useEffect(() => {
    if (!isOpen || !source) return;
    setParamsError("");
    setForm(toSourceForm(source));
  }, [isOpen, source]);

  if (!form) return null;

  const patch = (part: Partial<SourceFormValue>) =>
    setForm((prev) => (prev ? { ...prev, ...part } : prev));

  const validateParams = (raw: string) => {
    const value = raw.trim();
    if (!value) {
      setParamsError("");
      return true;
    }
    try {
      const parsed = JSON.parse(value);
      if (typeof parsed !== "object" || Array.isArray(parsed)) {
        setParamsError("params 需为 JSON 对象");
        return false;
      }
      setParamsError("");
      return true;
    } catch {
      setParamsError("params 不是合法 JSON");
      return false;
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
          编辑采集源
          <Text mt={1} fontSize="sm" fontWeight="400" color="workbench.muted">
            源标识 {source?.source_key}
            ；修改列表地址与规则参数后建议先“探测”验证。
          </Text>
        </ModalHeader>
        <ModalCloseButton />
        <ModalBody>
          <Stack spacing={5}>
            <SimpleGrid columns={{ base: 1, md: 2 }} spacing={5}>
              <FormControl isRequired>
                <FormLabel fontSize="sm" fontWeight="600">
                  展示名称
                </FormLabel>
                <Input
                  h={FIELD_H}
                  value={form.name}
                  onChange={(event) => patch({ name: event.target.value })}
                  borderColor="neutral.200"
                  {...FOCUS_PROPS}
                />
              </FormControl>
              <FormControl>
                <FormLabel fontSize="sm" fontWeight="600">
                  网站性质
                </FormLabel>
                <Input
                  h={FIELD_H}
                  value={form.category}
                  onChange={(event) => patch({ category: event.target.value })}
                  placeholder="国家级 / 地方级 / 国央企 / 银行 / 高校"
                  borderColor="neutral.200"
                  {...FOCUS_PROPS}
                />
              </FormControl>
            </SimpleGrid>

            <FormControl isRequired>
              <FormLabel fontSize="sm" fontWeight="600">
                列表地址
              </FormLabel>
              <Input
                h={FIELD_H}
                value={form.listUrl}
                onChange={(event) => patch({ listUrl: event.target.value })}
                placeholder="https://..."
                borderColor="neutral.200"
                {...FOCUS_PROPS}
              />
            </FormControl>

            <FormControl>
              <FormLabel fontSize="sm" fontWeight="600">
                门户地址
              </FormLabel>
              <Input
                h={FIELD_H}
                value={form.homepageUrl}
                onChange={(event) => patch({ homepageUrl: event.target.value })}
                placeholder="https://..."
                borderColor="neutral.200"
                {...FOCUS_PROPS}
              />
            </FormControl>

            <SimpleGrid columns={{ base: 1, md: 3 }} spacing={5}>
              <FormControl>
                <FormLabel fontSize="sm" fontWeight="600">
                  地区
                </FormLabel>
                <Input
                  h={FIELD_H}
                  value={form.region}
                  onChange={(event) => patch({ region: event.target.value })}
                  placeholder="广东省"
                  borderColor="neutral.200"
                  {...FOCUS_PROPS}
                />
              </FormControl>
              <FormControl>
                <FormLabel fontSize="sm" fontWeight="600">
                  行业属性
                </FormLabel>
                <Input
                  h={FIELD_H}
                  value={form.industryHint}
                  onChange={(event) =>
                    patch({ industryHint: event.target.value })
                  }
                  placeholder="通用"
                  borderColor="neutral.200"
                  {...FOCUS_PROPS}
                />
              </FormControl>
              <FormControl>
                <FormLabel fontSize="sm" fontWeight="600">
                  优先级（数字越小越先）
                </FormLabel>
                <NumberInput
                  min={0}
                  max={9999}
                  value={form.priority}
                  onChange={(value) => patch({ priority: value })}
                >
                  <NumberInputField
                    h={FIELD_H}
                    borderColor="neutral.200"
                    {...FOCUS_PROPS}
                  />
                </NumberInput>
              </FormControl>
            </SimpleGrid>

            <SimpleGrid columns={{ base: 1, md: 3 }} spacing={5}>
              <FormControl>
                <FormLabel fontSize="sm" fontWeight="600">
                  发现方式
                </FormLabel>
                <IntelSelect
                  value={form.discoveryMode}
                  options={DISCOVERY_MODE_OPTIONS}
                  onChange={(next) => patch({ discoveryMode: next })}
                  ariaLabel="发现方式"
                  placeholder="选择发现方式"
                />
              </FormControl>
              <FormControl>
                <FormLabel fontSize="sm" fontWeight="600">
                  需要无头浏览器
                </FormLabel>
                <Flex align="center" minH={FIELD_H}>
                  <Switch
                    colorScheme="primary"
                    isChecked={form.needsBrowser}
                    onChange={(event) =>
                      patch({ needsBrowser: event.target.checked })
                    }
                    aria-label="需要无头浏览器"
                  />
                  <Text ml={3} fontSize="sm" color="workbench.muted">
                    {form.needsBrowser ? "是（渲染后再抓）" : "否（静态请求）"}
                  </Text>
                </Flex>
              </FormControl>
              <FormControl>
                <FormLabel fontSize="sm" fontWeight="600">
                  启用采集
                </FormLabel>
                <Flex align="center" minH={FIELD_H}>
                  <Switch
                    colorScheme="primary"
                    isChecked={form.enabled}
                    onChange={(event) =>
                      patch({ enabled: event.target.checked })
                    }
                    aria-label="启用采集"
                  />
                  <Text ml={3} fontSize="sm" color="workbench.muted">
                    {form.enabled ? "参与每日自动采集" : "暂停采集"}
                  </Text>
                </Flex>
              </FormControl>
            </SimpleGrid>

            <FormControl isInvalid={Boolean(paramsError)}>
              <FormLabel fontSize="sm" fontWeight="600">
                规则参数（params，可选）
              </FormLabel>
              <Textarea
                value={form.params}
                onChange={(event) => {
                  patch({ params: event.target.value });
                  if (paramsError) validateParams(event.target.value);
                }}
                onBlur={(event) => validateParams(event.target.value)}
                rows={3}
                fontFamily="mono"
                fontSize="sm"
                bg="white"
                placeholder='{"linkPattern":"/zbgg/\\\\d+","contentSelectors":[".article-content"],"maxItems":30}'
                _focusVisible={{
                  borderColor: "primary.500",
                  boxShadow: "0 0 0 2px var(--chakra-colors-primary-200)",
                  outline: "none",
                }}
              />
              {paramsError ? (
                <FormErrorMessage fontSize="xs">{paramsError}</FormErrorMessage>
              ) : (
                <FormHelperText fontSize="xs">
                  留空表示使用通用规则；支持的键：linkPattern、listLinkSelector、
                  listExcludePattern、contentSelectors、titleSelectors、maxItems、maxPages
                </FormHelperText>
              )}
            </FormControl>
          </Stack>
        </ModalBody>
        <ModalFooter gap={3}>
          <Button variant="ghost" h={FIELD_H} onClick={onClose}>
            取消
          </Button>
          <Button
            h={FIELD_H}
            colorScheme="primary"
            isLoading={submitting}
            loadingText="保存中"
            isDisabled={
              Boolean(paramsError) || !form.name.trim() || !form.listUrl.trim()
            }
            onClick={() => {
              if (!validateParams(form.params)) return;
              onSubmit(form);
            }}
            _active={{ transform: "scale(0.98)" }}
          >
            保存修改
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
}
