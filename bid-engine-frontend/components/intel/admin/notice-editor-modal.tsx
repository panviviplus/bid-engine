"use client";

/* Hallmark · component: intel-notice-editor · genre: modern-minimal · theme: BidEngine deep-sea/gold (preserved tokens)
 * 用途：手工发布情报（来源=系统录入）与编辑已入库情报。
 * states: default · hover · focus-visible · active · disabled · loading(保存) · error(必填缺失)
 * responsive: 320px 单列 → 宽屏两列；Modal 内容区可滚动，避免小屏顶到底
 * Hallmark · pre-emit critique: P5 H5 E5 S5 R5 V4
 */
import React, { useEffect, useMemo, useState } from "react";
import {
  Button,
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
  NumberInputStepper,
  NumberIncrementStepper,
  NumberDecrementStepper,
  SimpleGrid,
  Stack,
  Text,
  Textarea,
} from "@chakra-ui/react";

import IntelSelect, {
  FIELD_HEIGHT,
  type IntelSelectOption,
} from "@/components/intel/intel-select";
import { SuggestTagField } from "@/components/intel/tag-field";
import type {
  IntelAdminNotice,
  IntelFilters,
  IntelNoticePayload,
} from "@/service/intel";

/**
 * 可编辑公告：在列表项之上补充正文。
 *
 * 列表接口不返回正文（mediumtext，逐行返回会拖慢列表），因此编辑前会按需拉一次详情，
 * 保证“只改标题”这类编辑不会把正文清空。
 */
export type IntelEditableNotice = IntelAdminNotice & { body_text?: string };

const FOCUS_PROPS = {
  _focusVisible: {
    borderColor: "primary.500",
    boxShadow: "0 0 0 2px var(--chakra-colors-primary-200)",
    outline: "none",
  },
};

export type NoticeFormValue = {
  title: string;
  publisher: string;
  agency: string;
  projectCode: string;
  noticeType: string;
  budgetWan: string;
  budgetText: string;
  regionProvince: string;
  regionCity: string;
  publishDate: string;
  deadlineAt: string;
  url: string;
  sourceName: string;
  industries: string[];
  bodyText: string;
  adminNote: string;
};

export const EMPTY_NOTICE_FORM: NoticeFormValue = {
  title: "",
  publisher: "",
  agency: "",
  projectCode: "",
  noticeType: "",
  budgetWan: "",
  budgetText: "",
  regionProvince: "",
  regionCity: "",
  publishDate: "",
  deadlineAt: "",
  url: "",
  sourceName: "",
  industries: [],
  bodyText: "",
  adminNote: "",
};

/** 日期时间转成 <input type="date"> 需要的 yyyy-MM-dd。 */
function toDateInput(value: string): string {
  if (!value) return "";
  return value.slice(0, 10);
}

/** 行业编码 → 展示名（未知编码原样保留，兼容自定义行业）。 */
function industryLabels(
  codes: string[],
  filters: IntelFilters | null,
): string[] {
  const nameMap = new Map(
    (filters?.industries || []).map((item) => [item.code, item.name]),
  );
  return codes.map((code) => nameMap.get(code) || code);
}

/** 管理端表单 → 接口载荷；预算由万元换算为元。 */
export function toNoticePayload(
  value: NoticeFormValue,
  id?: number,
): IntelNoticePayload {
  const wan = Number(value.budgetWan);
  const hasBudget =
    value.budgetWan.trim() !== "" && Number.isFinite(wan) && wan >= 0;
  return {
    ...(id ? { id } : {}),
    title: value.title.trim(),
    publisher: value.publisher.trim(),
    agency: value.agency.trim(),
    project_code: value.projectCode.trim(),
    notice_type: value.noticeType,
    budget_amount: hasBudget ? Math.round(wan * 10000) : null,
    budget_text: value.budgetText.trim(),
    region_province: value.regionProvince.trim(),
    region_city: value.regionCity.trim(),
    publish_date: value.publishDate,
    deadline_at: value.deadlineAt,
    url: value.url.trim(),
    source_name: value.sourceName.trim(),
    industries: value.industries,
    body_text: value.bodyText,
    admin_note: value.adminNote.trim(),
  };
}

export function toNoticeForm(
  notice: IntelEditableNotice,
  filters: IntelFilters | null,
): NoticeFormValue {
  return {
    title: notice.title || "",
    publisher: notice.publisher || "",
    agency: notice.agency || "",
    projectCode: notice.project_code || "",
    noticeType: notice.notice_type || "",
    budgetWan:
      notice.budget_amount && notice.budget_amount > 0
        ? String(Math.round(notice.budget_amount / 10000))
        : "",
    budgetText: notice.budget_text || "",
    regionProvince: notice.region_province || "",
    regionCity: notice.region_city || "",
    publishDate: toDateInput(notice.publish_date),
    deadlineAt: toDateInput(notice.deadline_at),
    url: notice.url || "",
    sourceName: notice.source_name || "",
    industries: industryLabels(notice.industries || [], filters),
    bodyText: notice.body_text || "",
    adminNote: notice.admin_note || "",
  };
}

export default function NoticeEditorModal({
  isOpen,
  onClose,
  notice,
  filters,
  onSubmit,
  submitting,
}: {
  isOpen: boolean;
  onClose: () => void;
  /** 传 null 表示新建（手工发布） */
  notice: IntelEditableNotice | null;
  filters: IntelFilters | null;
  // eslint-disable-next-line no-unused-vars
  onSubmit: (value: NoticeFormValue) => void | Promise<void>;
  submitting: boolean;
}) {
  const [form, setForm] = useState<NoticeFormValue>({ ...EMPTY_NOTICE_FORM });
  const [touched, setTouched] = useState(false);

  useEffect(() => {
    if (!isOpen) return;
    setTouched(false);
    setForm(notice ? toNoticeForm(notice, filters) : { ...EMPTY_NOTICE_FORM });
  }, [isOpen, notice, filters]);

  const patch = (part: Partial<NoticeFormValue>) =>
    setForm((prev) => ({ ...prev, ...part }));

  const noticeTypeOptions: IntelSelectOption[] = useMemo(
    () =>
      (filters?.notice_types || []).map((item) => ({
        value: item.code,
        label: item.name,
      })),
    [filters],
  );
  const industrySuggestions = useMemo(
    () => (filters?.industries || []).map((item) => item.name),
    [filters],
  );
  const regionSuggestions = useMemo(() => filters?.regions || [], [filters]);

  const titleInvalid = touched && !form.title.trim();

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      size="4xl"
      isCentered
      scrollBehavior="inside"
    >
      <ModalOverlay />
      <ModalContent maxH="90dvh" borderRadius="16px">
        <ModalHeader pb={2}>
          {notice ? "编辑情报" : "发布情报"}
          <Text mt={1} fontSize="sm" fontWeight="400" color="workbench.muted">
            {notice
              ? "保存后立即对全体用户生效；置顶、隐藏、下架在列表卡片上单独操作。"
              : "线下收集的招标信息可手工录入，来源统一标记为系统录入；未填行业时按标题自动打标。"}
          </Text>
        </ModalHeader>
        <ModalCloseButton />
        <ModalBody>
          <Stack spacing={5}>
            <FormControl isInvalid={titleInvalid} isRequired>
              <FormLabel fontSize="sm" fontWeight="600">
                公告标题
              </FormLabel>
              <Input
                h={FIELD_HEIGHT}
                value={form.title}
                onChange={(event) => patch({ title: event.target.value })}
                onBlur={() => setTouched(true)}
                placeholder="例如：某某市政务云平台建设项目公开招标公告"
                borderColor="neutral.200"
                {...FOCUS_PROPS}
              />
              {titleInvalid ? (
                <FormErrorMessage fontSize="xs">标题是必填项</FormErrorMessage>
              ) : (
                <FormHelperText fontSize="xs">
                  公告类型留空时按标题自动判定
                </FormHelperText>
              )}
            </FormControl>

            <SimpleGrid columns={{ base: 1, md: 2 }} spacing={5}>
              <FormControl>
                <FormLabel fontSize="sm" fontWeight="600">
                  采购人
                </FormLabel>
                <Input
                  h={FIELD_HEIGHT}
                  value={form.publisher}
                  onChange={(event) => patch({ publisher: event.target.value })}
                  placeholder="招标单位"
                  borderColor="neutral.200"
                  {...FOCUS_PROPS}
                />
              </FormControl>
              <FormControl>
                <FormLabel fontSize="sm" fontWeight="600">
                  代理机构
                </FormLabel>
                <Input
                  h={FIELD_HEIGHT}
                  value={form.agency}
                  onChange={(event) => patch({ agency: event.target.value })}
                  placeholder="招标代理机构"
                  borderColor="neutral.200"
                  {...FOCUS_PROPS}
                />
              </FormControl>
              <FormControl>
                <FormLabel fontSize="sm" fontWeight="600">
                  项目编号
                </FormLabel>
                <Input
                  h={FIELD_HEIGHT}
                  value={form.projectCode}
                  onChange={(event) =>
                    patch({ projectCode: event.target.value })
                  }
                  placeholder="例如 ZB-2026-0001"
                  borderColor="neutral.200"
                  {...FOCUS_PROPS}
                />
              </FormControl>
              <FormControl>
                <FormLabel fontSize="sm" fontWeight="600">
                  公告类型
                </FormLabel>
                <IntelSelect
                  value={form.noticeType}
                  options={noticeTypeOptions}
                  onChange={(next) => patch({ noticeType: next })}
                  placeholder="留空按标题自动判定"
                  ariaLabel="公告类型"
                  isClearable
                />
              </FormControl>
              <FormControl>
                <FormLabel fontSize="sm" fontWeight="600">
                  预算金额
                  <Text as="span" ml={1} color="neutral.400" fontWeight="400">
                    （万元）
                  </Text>
                </FormLabel>
                <NumberInput
                  value={form.budgetWan}
                  min={0}
                  step={100}
                  precision={0}
                  clampValueOnBlur={false}
                  onChange={(next) =>
                    patch({
                      budgetWan: String(next ?? "").replace(/[^\d]/g, ""),
                    })
                  }
                >
                  <NumberInputField
                    h={FIELD_HEIGHT}
                    placeholder="例如 860"
                    borderColor="neutral.200"
                    sx={{ fontVariantNumeric: "tabular-nums" }}
                    {...FOCUS_PROPS}
                  />
                  <NumberInputStepper>
                    <NumberIncrementStepper borderColor="neutral.200" />
                    <NumberDecrementStepper borderColor="neutral.200" />
                  </NumberInputStepper>
                </NumberInput>
              </FormControl>
              <FormControl>
                <FormLabel fontSize="sm" fontWeight="600">
                  预算原文
                </FormLabel>
                <Input
                  h={FIELD_HEIGHT}
                  value={form.budgetText}
                  onChange={(event) =>
                    patch({ budgetText: event.target.value })
                  }
                  placeholder="例如 预算金额 860 万元"
                  borderColor="neutral.200"
                  {...FOCUS_PROPS}
                />
              </FormControl>
              <FormControl>
                <FormLabel fontSize="sm" fontWeight="600">
                  省级地区
                </FormLabel>
                <Input
                  h={FIELD_HEIGHT}
                  value={form.regionProvince}
                  onChange={(event) =>
                    patch({ regionProvince: event.target.value })
                  }
                  placeholder="例如 广东省"
                  list="intel-region-options"
                  borderColor="neutral.200"
                  {...FOCUS_PROPS}
                />
                <datalist id="intel-region-options">
                  {regionSuggestions.map((item) => (
                    <option key={item} value={item} aria-label={item} />
                  ))}
                </datalist>
              </FormControl>
              <FormControl>
                <FormLabel fontSize="sm" fontWeight="600">
                  市级地区
                </FormLabel>
                <Input
                  h={FIELD_HEIGHT}
                  value={form.regionCity}
                  onChange={(event) =>
                    patch({ regionCity: event.target.value })
                  }
                  placeholder="例如 深圳市"
                  borderColor="neutral.200"
                  {...FOCUS_PROPS}
                />
              </FormControl>
              <FormControl>
                <FormLabel fontSize="sm" fontWeight="600">
                  发布时间
                </FormLabel>
                <Input
                  h={FIELD_HEIGHT}
                  type="date"
                  value={form.publishDate}
                  onChange={(event) =>
                    patch({ publishDate: event.target.value })
                  }
                  borderColor="neutral.200"
                  {...FOCUS_PROPS}
                />
              </FormControl>
              <FormControl>
                <FormLabel fontSize="sm" fontWeight="600">
                  投标截止时间
                </FormLabel>
                <Input
                  h={FIELD_HEIGHT}
                  type="date"
                  value={form.deadlineAt}
                  onChange={(event) =>
                    patch({ deadlineAt: event.target.value })
                  }
                  borderColor="neutral.200"
                  {...FOCUS_PROPS}
                />
              </FormControl>
              <FormControl>
                <FormLabel fontSize="sm" fontWeight="600">
                  来源链接
                </FormLabel>
                <Input
                  h={FIELD_HEIGHT}
                  value={form.url}
                  onChange={(event) => patch({ url: event.target.value })}
                  placeholder="https://（留空表示线下收集，无原站链接）"
                  borderColor="neutral.200"
                  {...FOCUS_PROPS}
                />
              </FormControl>
              <FormControl>
                <FormLabel fontSize="sm" fontWeight="600">
                  来源名称
                </FormLabel>
                <Input
                  h={FIELD_HEIGHT}
                  value={form.sourceName}
                  onChange={(event) =>
                    patch({ sourceName: event.target.value })
                  }
                  placeholder="留空为“系统录入”"
                  borderColor="neutral.200"
                  {...FOCUS_PROPS}
                />
              </FormControl>
            </SimpleGrid>

            <SuggestTagField
              label="行业（可自定义）"
              helperText="点选常用行业，或直接输入自定义行业名称后回车添加"
              placeholder="例如：智慧水务"
              value={form.industries}
              onChange={(next) => patch({ industries: next })}
              suggestions={industrySuggestions}
            />

            <FormControl>
              <FormLabel fontSize="sm" fontWeight="600">
                公告正文
              </FormLabel>
              <Textarea
                value={form.bodyText}
                onChange={(event) => patch({ bodyText: event.target.value })}
                rows={6}
                placeholder="粘贴公告正文或采购需求要点；支持 Markdown"
                borderColor="neutral.200"
                {...FOCUS_PROPS}
              />
              <FormHelperText fontSize="xs">
                正文会参与关键词检索与 AI 解读，建议粘贴完整公告内容
              </FormHelperText>
            </FormControl>

            <FormControl>
              <FormLabel fontSize="sm" fontWeight="600">
                管理备注
              </FormLabel>
              <Input
                h={FIELD_HEIGHT}
                value={form.adminNote}
                onChange={(event) => patch({ adminNote: event.target.value })}
                placeholder="仅管理员可见，例如录入来源、跟进结论"
                borderColor="neutral.200"
                {...FOCUS_PROPS}
              />
            </FormControl>
          </Stack>
        </ModalBody>
        <ModalFooter gap={3}>
          <Button variant="ghost" h={FIELD_HEIGHT} onClick={onClose}>
            取消
          </Button>
          <Button
            h={FIELD_HEIGHT}
            colorScheme="primary"
            isLoading={submitting}
            loadingText="保存中"
            onClick={() => {
              setTouched(true);
              if (!form.title.trim()) return;
              onSubmit(form);
            }}
            _active={{ transform: "scale(0.98)" }}
          >
            {notice ? "保存修改" : "发布情报"}
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
}
