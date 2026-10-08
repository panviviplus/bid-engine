export const NAVIGATION_LIST = [
  {
    label: "招标情报站",
    icon: "nav-intel",
    children: [
      {
        label: "情报大厅",
        href: "/intel",
      },
      {
        label: "订阅与提醒",
        href: "/intel/subscriptions",
        badge: "intel_unread",
      },
    ],
  },
  {
    label: "招标解析",
    icon: "nav-analysis",
    href: "/bid-analysis",
  },
  {
    label: "投标文件生成",
    icon: "nav-gen",
    href: "/file-gen",
  },
  {
    label: "投标文件审核",
    icon: "nav-audit",
    href: "/bid-audit",
  },
  {
    label: "素材库",
    icon: "nav-material",
    children: [
      {
        label: "企业资质",
        href: "/material/qualification",
      },
      {
        label: "企业业绩",
        href: "/material/performance",
      },
      {
        label: "文档模板",
        href: "/material/template",
      },
    ],
  },
  {
    label: "系统管理",
    icon: "nav-system",
    children: [
      {
        label: "模型配置",
        href: "/system/llm-config",
      },
      {
        label: "系统模型配置",
        href: "/system/llm-config/system",
      },
      {
        label: "招标情报管理",
        href: "/system/intel",
      },
    ],
  },
];
