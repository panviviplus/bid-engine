import assert from "node:assert/strict";
import test from "node:test";

import {
  blueprintStatusMeta,
  buildDetailTabs,
  groupSummaryRisks,
  pickKeyDates,
  summarizeFieldCoverage,
} from "./model.mjs";

test("summarizes extracted and pending fields across every category", () => {
  const categories = [
    {
      fields: [
        { extract_status: "succeeded" },
        { extract_status: "not_found" },
      ],
    },
    {
      fields: [
        { extract_status: "succeeded" },
        { extract_status: "not_found" },
        { extract_status: "not_found" },
      ],
    },
  ];

  assert.deepEqual(summarizeFieldCoverage(categories), {
    extracted: 2,
    pending: 3,
    total: 5,
  });
});

test("builds the four detail tabs with warning counts and blueprint failure", () => {
  const tabs = buildDetailTabs({
    warningCount: 78,
    warningSeverity: "critical",
    fieldCount: 5,
    clauseCount: 0,
    blueprintStatus: "failed",
  });

  assert.deepEqual(
    tabs.map(({ key, badge, tone, statusLabel }) => ({
      key,
      badge,
      tone,
      statusLabel,
    })),
    [
      {
        key: "overview",
        badge: "",
        tone: "error",
        statusLabel: "78 条解析告警",
      },
      {
        key: "fields",
        badge: "5",
        tone: "neutral",
        statusLabel: "5 个已提取字段",
      },
      {
        key: "clauses",
        badge: "0",
        tone: "neutral",
        statusLabel: "0 条关键条款",
      },
      {
        key: "blueprint",
        badge: "失败",
        tone: "error",
        statusLabel: "标书蓝图生成失败",
      },
    ],
  );
});

test("keeps blueprint loading and success labels explicit", () => {
  assert.deepEqual(blueprintStatusMeta(null), {
    badge: "读取中",
    tone: "neutral",
    label: "正在读取标书蓝图状态",
  });
  assert.deepEqual(blueprintStatusMeta("succeeded"), {
    badge: "已生成",
    tone: "success",
    label: "标书蓝图已生成",
  });
});

test("distinguishes a blueprint status request failure from generation failure", () => {
  assert.deepEqual(blueprintStatusMeta("load_failed"), {
    badge: "读取失败",
    tone: "error",
    label: "标书蓝图状态读取失败",
  });
});

test("splits key dates into valid values and masked placeholders", () => {
  const categories = [
    {
      fields: [
        {
          field_key: "bid_deadline",
          display_name: "投标截止时间",
          values: [{ display_value: "2025-08-15 09:30:00" }],
        },
        {
          field_key: "bid_validity_period",
          display_name: "投标有效期",
          values: [{ display_value: "报价截止日后的 _______ 个工作日" }],
        },
        {
          field_key: "work_period",
          display_name: "工期",
          values: [{ display_value: "未填写" }],
        },
        {
          field_key: "site_visit_time",
          display_name: "踏勘时间",
          values: [],
        },
      ],
    },
  ];
  const { valid, placeholders } = pickKeyDates(categories);
  assert.deepEqual(
    valid.map((item) => item.label),
    ["投标截止时间"],
  );
  assert.deepEqual(
    placeholders.map((item) => item.label),
    ["投标有效期", "工期"],
  );
});

test("key dates stay empty when no date fields are extracted", () => {
  const categories = [
    {
      fields: [
        {
          field_key: "project_name",
          display_name: "项目名称",
          values: [{ display_value: "某项目" }],
        },
      ],
    },
  ];
  assert.deepEqual(pickKeyDates(categories), { valid: [], placeholders: [] });
});

test("groups typed risks and confirms while legacy items fall back combined", () => {
  const typed = [
    {
      index: 0,
      text: "理赔时限严",
      kind: "risk",
      label: "履约风险",
      resolved: false,
    },
    {
      index: 1,
      text: "定义模糊",
      kind: "confirm",
      label: "定义模糊",
      resolved: false,
    },
  ];
  assert.deepEqual(groupSummaryRisks(typed), {
    legacy: [],
    risks: [typed[0]],
    confirms: [typed[1]],
  });

  const legacy = [
    { index: 0, text: "历史风险文本", resolved: false },
    { index: 1, text: "历史待确认文本", resolved: false },
  ];
  const grouped = groupSummaryRisks(legacy);
  assert.deepEqual(grouped, { legacy, risks: [], confirms: [] });
});
