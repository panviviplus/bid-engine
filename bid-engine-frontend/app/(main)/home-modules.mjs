export const HOME_MODULES = [
  {
    id: "tender_intelligence",
    label: "招标情报站",
    description: "从公开招标网站持续采集公告，按关键词与行业筛选，命中订阅即提醒。",
    href: "/intel",
    planned: false,
  },
  {
    id: "bid_analysis",
    label: "招标解析",
    description: "拆解资格条件、评分点、响应要求与否决风险。",
    statKey: "bid_analysis",
    href: "/bid-analysis",
    planned: false,
  },
  {
    id: "bid_decision_review",
    label: "投标审核",
    description: "参考已有资质、业绩与能力，判断参投资格和关键风险。",
    planned: true,
  },
  {
    id: "bid_generation",
    label: "标书生成",
    description: "沿着目录组织内容，复用投标素材形成可编辑初稿。",
    statKey: "bid_generation",
    href: "/file-gen",
    planned: false,
  },
  {
    id: "bid_review",
    label: "标书审核",
    description: "销售、技术与法务协同检查合规、质量与竞争力。",
    statKey: "bid_review",
    href: "/bid-audit",
    planned: false,
  },
  {
    id: "material",
    label: "素材库",
    description: "分类沉淀个人资质、业绩与模板，随用随取、持续复用。",
    statKey: "material",
    planned: false,
  },
];

export const HOME_MODULE_LABELS = {
  bid_analysis: "招标解析",
  bid_generation: "标书生成",
  bid_review: "标书审核",
  material: "素材库",
};

export const MATERIAL_LINKS = [
  { id: "qualification", label: "资质", href: "/material/qualification" },
  { id: "performance", label: "业绩", href: "/material/performance" },
  { id: "template", label: "模板", href: "/material/template" },
];
