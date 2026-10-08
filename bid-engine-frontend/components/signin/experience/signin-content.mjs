export const HERO_COPY =
  "从锁定值得投的机会，到读透招标文件、判断参投风险，再到生成、审核与交付标书——标擎让 AI 接住每一个关键环节！\n无论您手中是一份招标文件，还是一版标书初稿，都能从当下直接开工！";

export const HERO_LABELS = [
  "发现值得跟进的项目",
  "全面理解招标要求",
  "提前判断资格与风险",
  "协同完成标书交付",
  "持续沉淀投标知识与经验",
];

export const SCREEN_TWO_COPY = {
  title: "标擎，让 AI 融入投标的每一个环节！",
  body: "眼前的问题接得住，前面的成果带得走！\nAI把每一环的上下文吃透，让投标从头到尾闭环交付。",
};

export const SCREEN_THREE_COPY = {
  title: "您的进度，就是标擎接手的起点",
  body: "每一份已有成果，都是下一步的起点！\n标擎开放各环节独立接入，让您随时上手、顺势推进，直到交付！",
};

export const SIGNIN_STAGES = [
  {
    key: "intelligence",
    title: "招标情报站",
    shortTitle: "情报",
    action: "筛选机会",
    description: "结合关注范围与投标资产，整理值得跟进的招标机会。",
    planned: true,
    bubbles: ["不该错过好项目"],
  },
  {
    key: "analysis",
    title: "招标解析",
    shortTitle: "解析",
    action: "理解要求",
    description: "拆解资格条件、评分点、响应要求与否决风险。",
    bubbles: ["提炼关键信息", "不漏看硬要求"],
  },
  {
    key: "bid-review",
    title: "投标审核",
    shortTitle: "投审",
    action: "评估决策",
    description: "参考已有资质、业绩与能力，判断参投资格和关键风险。",
    planned: true,
    bubbles: ["能不能投？", "要不要投？"],
  },
  {
    key: "generation",
    title: "标书生成",
    shortTitle: "生成",
    action: "组织内容",
    description: "沿着目录组织内容，复用投标资产形成可编辑初稿。",
    bubbles: ["不必从零开始", "好素材快速成稿"],
  },
  {
    key: "document-review",
    title: "标书审核",
    shortTitle: "标审",
    action: "协同把关",
    description: "销售、技术与法务协同检查合规、质量与竞争力。",
    bubbles: ["高质量交付标书", "标书需经得起检验"],
  },
  {
    key: "delivery",
    title: "导出交付",
    shortTitle: "交付",
    action: "完成投标",
    description: "确认内容、附件与审核结果后导出投标文件。",
    bubbles: [],
  },
];

export const ENTRY_SCENARIOS = [
  {
    title: "还在寻找项目",
    action: "从招标情报站开始",
    startIndex: 0,
    eyebrow: "从机会发现开始",
    detail: "后续可以继续完成解析、决策、生成、审核和交付。",
  },
  {
    title: "已经拿到招标文件",
    action: "直接进入招标解析",
    startIndex: 1,
    eyebrow: "带着招标文件开始",
    detail: "前面的情报筛选无需重做，后续环节继续衔接。",
  },
  {
    title: "正在判断是否参投",
    action: "直接进入投标审核",
    startIndex: 2,
    eyebrow: "从参投决策开始",
    detail: "结合资质、业绩、人员与风险完成参投判断。",
  },
  {
    title: "已经确定参投",
    action: "直接进入标书生成",
    startIndex: 3,
    eyebrow: "从标书生产开始",
    detail: "调用投标资产形成目录、章节和可编辑初稿。",
  },
  {
    title: "已经有初版标书",
    action: "直接进入标书审核",
    startIndex: 4,
    eyebrow: "从交付前把关开始",
    detail: "邀请销售、技术与法务协同检查并优化，然后导出交付。",
  },
];

export const ASSET_TAGS = [
  "资质证照",
  "项目业绩",
  "人员履历",
  "方案段落",
  "证明材料",
];

export const VALUE_CARDS = [
  {
    title: "把时间留给差异化内容",
    body: "减少重复复制粘贴与格式整理，把精力放在得分目标和关键表达上。",
  },
  {
    title: "输出结果可追溯、可交接",
    body: "关键动作围绕协作、确认与导出组织，减少临近截止时间的返工。",
  },
  {
    title: "把不确定性前置",
    body: "把要求结构化、过程可视化、投标资产可复用化。",
  },
];

export const LEGAL_LABELS = [
  "法律声明",
  "Cookies 政策",
  "隐私政策",
  "廉正举报",
  "安全举报",
  "联系我们",
];
