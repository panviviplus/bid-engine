import test from "node:test";
import assert from "node:assert/strict";

import {
  HERO_COPY,
  HERO_LABELS,
  SCREEN_TWO_COPY,
  SCREEN_THREE_COPY,
  SIGNIN_STAGES,
} from "./signin-content.mjs";

test("keeps the approved V10 hero and section copy frozen", () => {
  assert.equal(
    HERO_COPY,
    "从锁定值得投的机会，到读透招标文件、判断参投风险，再到生成、审核与交付标书——标擎让 AI 接住每一个关键环节！\n无论您手中是一份招标文件，还是一版标书初稿，都能从当下直接开工！",
  );
  assert.deepEqual(HERO_LABELS, [
    "发现值得跟进的项目",
    "全面理解招标要求",
    "提前判断资格与风险",
    "协同完成标书交付",
    "持续沉淀投标知识与经验",
  ]);
  assert.deepEqual(SCREEN_TWO_COPY, {
    title: "标擎，让 AI 融入投标的每一个环节！",
    body: "眼前的问题接得住，前面的成果带得走！\nAI把每一环的上下文吃透，让投标从头到尾闭环交付。",
  });
  assert.deepEqual(SCREEN_THREE_COPY, {
    title: "您的进度，就是标擎接手的起点",
    body: "每一份已有成果，都是下一步的起点！\n标擎开放各环节独立接入，让您随时上手、顺势推进，直到交付！",
  });
});

test("keeps the approved stage order and bubble messages", () => {
  assert.deepEqual(
    SIGNIN_STAGES.map(({ title, planned, bubbles }) => ({
      title,
      planned: Boolean(planned),
      bubbles,
    })),
    [
      { title: "招标情报站", planned: true, bubbles: ["不该错过好项目"] },
      {
        title: "招标解析",
        planned: false,
        bubbles: ["提炼关键信息", "不漏看硬要求"],
      },
      {
        title: "投标审核",
        planned: true,
        bubbles: ["能不能投？", "要不要投？"],
      },
      {
        title: "标书生成",
        planned: false,
        bubbles: ["不必从零开始", "好素材快速成稿"],
      },
      {
        title: "标书审核",
        planned: false,
        bubbles: ["高质量交付标书", "标书需经得起检验"],
      },
      { title: "导出交付", planned: false, bubbles: [] },
    ],
  );
});

test("does not expose internal design jargon in user-facing copy", () => {
  const text = JSON.stringify({
    HERO_COPY,
    HERO_LABELS,
    SCREEN_TWO_COPY,
    SCREEN_THREE_COPY,
    SIGNIN_STAGES,
  });
  assert.doesNotMatch(text, /Pipeline|Pipline|适配 To B|示例视图|企业资产底座/);
});
