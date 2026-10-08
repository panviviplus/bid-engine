/** @param {Array<{ fields?: Array<{ extract_status?: string }> }>} categories */
export function summarizeFieldCoverage(categories = []) {
  const fields = categories.flatMap((category) => category?.fields || []);
  const pending = fields.filter(
    (field) => field?.extract_status === "not_found",
  ).length;
  return {
    extracted: fields.length - pending,
    pending,
    total: fields.length,
  };
}

// LLM 在抽取阶段可能输出“剔除/排除”类标记条款（如“剔除：正文片段”）。
// 这类条目不是真实条款，服务端新解析会过滤，前端对存量数据同样做渲染过滤。
export const CLAUSE_NOISE_TITLE_RE =
  /^(剔除|排除|删除|忽略|跳过|非条款|无关|目录)|正文片段/;

// 重要日期卡：判定字段是否属于时间类，以及值是否为占位符/无实际内容。
const DATE_LABEL_RE = /(时间|日期|截止|有效期|工期|日历天)/;
const DATE_PLACEHOLDER_RE =
  /^(无|不组织|未提供|暂无|详见.*)$|(?:_{2,}|×{2,}|\*{3,}|（\s*）|\(\s*\)|【\s*】|\[\s*\]|待填|待定|TBD|未填写|空白|无具体数值)/i;
const DATE_PRIORITY = [
  "投标截止",
  "开标",
  "答疑",
  "澄清",
  "报名截止",
  "报名开始",
  "获取招标文件",
  "招标公告发布",
  "踏勘",
  "投标有效期",
  "工期",
];

/**
 * 从字段分类中提炼重要日期。
 * 返回 { valid, placeholders }：
 * - valid：有具体数值的日期项（正常渲染）；
 * - placeholders：原文为占位符/空模板、无实际数值的日期项（渲染“缺少有效值”）。
 * 两者都为空时，前端应整卡隐藏。
 */
export function pickKeyDates(categories = []) {
  const valid = [];
  const placeholders = [];
  for (const category of categories) {
    for (const field of category?.fields || []) {
      if (!DATE_LABEL_RE.test(field?.display_name || "")) continue;
      const value = String(field?.values?.[0]?.display_value || "").trim();
      if (!value) continue;
      const item = {
        fieldKey: field.field_key,
        label: field.display_name,
        value,
      };
      if (DATE_PLACEHOLDER_RE.test(value)) {
        placeholders.push(item);
      } else {
        valid.push(item);
      }
    }
  }
  const rank = (label) => {
    const index = DATE_PRIORITY.findIndex((prefix) => label.includes(prefix));
    return index < 0 ? DATE_PRIORITY.length : index;
  };
  valid.sort((a, b) => rank(a.label) - rank(b.label));
  placeholders.sort((a, b) => rank(a.label) - rank(b.label));
  return {
    valid: valid.slice(0, 6),
    placeholders: placeholders.slice(0, 6),
  };
}

/**
 * 风险/待确认分组。
 * 新数据每项带 kind（risk|confirm）；旧数据 risks 为纯文本（无 kind），
 * 整组回退为 legacy 合并展示，不做启发式拆分。
 */
export function groupSummaryRisks(items = []) {
  const untyped = items.filter(
    (item) => item?.kind !== "risk" && item?.kind !== "confirm",
  );
  if (untyped.length > 0) {
    return { legacy: items, risks: [], confirms: [] };
  }
  return {
    legacy: [],
    risks: items.filter((item) => item.kind === "risk"),
    confirms: items.filter((item) => item.kind === "confirm"),
  };
}

const BLUEPRINT_STATUS_META = {
  load_failed: {
    badge: "读取失败",
    tone: "error",
    label: "标书蓝图状态读取失败",
  },
  not_generated: {
    badge: "待生成",
    tone: "neutral",
    label: "标书蓝图尚未生成",
  },
  invalidated: {
    badge: "已失效",
    tone: "warning",
    label: "标书蓝图需要重新生成",
  },
  pending: {
    badge: "等待中",
    tone: "info",
    label: "标书蓝图等待生成",
  },
  running: {
    badge: "生成中",
    tone: "info",
    label: "正在生成标书蓝图",
  },
  succeeded: {
    badge: "已生成",
    tone: "success",
    label: "标书蓝图已生成",
  },
  failed: {
    badge: "失败",
    tone: "error",
    label: "标书蓝图生成失败",
  },
};

/** @param {string | null | undefined} status */
export function blueprintStatusMeta(status) {
  if (!status) {
    return {
      badge: "读取中",
      tone: "neutral",
      label: "正在读取标书蓝图状态",
    };
  }
  return BLUEPRINT_STATUS_META[status] || BLUEPRINT_STATUS_META.not_generated;
}

function warningToneFor(warningCount, warningSeverity) {
  if (warningCount <= 0) return "neutral";
  if (warningSeverity === "critical") return "error";
  if (warningSeverity === "warning") return "warning";
  return "neutral";
}

/**
 * @param {{
 *   warningCount?: number;
 *   warningSeverity?: "critical" | "warning" | "info" | null;
 *   fieldCount?: number;
 *   clauseCount?: number;
 *   blueprintStatus?: string | null;
 * }} options
 */
export function buildDetailTabs({
  warningCount = 0,
  warningSeverity = null,
  fieldCount = 0,
  clauseCount = 0,
  blueprintStatus = null,
} = {}) {
  const blueprint = blueprintStatusMeta(blueprintStatus);
  const warningTone = warningToneFor(warningCount, warningSeverity);
  return [
    {
      key: "overview",
      label: "概览",
      // 概览页告警数无实际意义，不再展示数字徽标（保留 tone/statusLabel 供其他用途）。
      badge: "",
      tone: warningTone,
      statusLabel: `${warningCount} 条解析告警`,
    },
    {
      key: "fields",
      label: "字段",
      badge: String(fieldCount),
      tone: "neutral",
      statusLabel: `${fieldCount} 个已提取字段`,
    },
    {
      key: "clauses",
      label: "关键条款",
      badge: String(clauseCount),
      tone: "neutral",
      statusLabel: `${clauseCount} 条关键条款`,
    },
    {
      key: "blueprint",
      label: "标书蓝图",
      badge: blueprint.badge,
      tone: blueprint.tone,
      statusLabel: blueprint.label,
    },
  ];
}
