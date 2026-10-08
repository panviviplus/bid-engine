const PAGE_TYPE = {
  CAMPANY: "CP",
  TEAM: "T",
  PEOPLE: "P",
};

const MODAL_TYPE = {
  new: "new",
  edit: "edit",
};

const TABLE_SYS_TEAM_HEADERS = [
  { label: "序号" },
  { label: "团队名称" },
  { label: "项目类型" },
  { label: "所属公司" },
  { label: "团队负责人" },
  { label: "联系电话" },
  { label: "团队人员" },
  { label: "已投案例" },
  { label: "投标模板" },
  { label: "操作" },
];
const TABLE__SYS_PEOPLE_HEADERS = [
  { label: "序号" },
  { label: "姓名" },
  { label: "手机号码" },
  { label: "用户角色" },
  { label: "创建时间" },
  { label: "账号状态" },
  { label: "操作" },
];

const GEN_GEN_CREATE_TYPE = [
  { label: "制作中", value: "running" },
  { label: "已完成", value: "succeed" },
  { label: "失败", value: "failed" },
];
const GEN_PROJECT_TYPE = [
  { label: "半导体", value: "半导体" },
  { label: "其他", value: "其他" },
  { label: "免税", value: "免税" },
  { label: "TFT", value: "TFT" },
  { label: "基础建设", value: "基础建设" },
  { label: "能源化工", value: "能源化工" },
  { label: "装备制造", value: "装备制造" },
  { label: "冶金矿业", value: "冶金矿业" },
];
const PROJECT_TYPE = [
  { label: "半导体", value: "semiconductor" },
  { label: "其他", value: "other" },
  { label: "免税", value: "tax-free" },
  { label: "TFT", value: "tft" },
  { label: "基础建设", value: "infrastructure" },
  { label: "能源化工", value: "energy" },
  { label: "装备制造", value: "equipment" },
  { label: "冶金矿业", value: "metallurgy" },
];
const ACCOUNT_STATUS = [
  { label: "启用", value: 1 },
  { label: "禁用", value: 0 },
];
const PEO_TYPE = [
  { label: "团队成员", value: "member" },
  { label: "团队负责人", value: "owner" },
];

const FEEDBACK_TYPE = [
  { label: "功能建议", value: "suggest" },
  { label: "使用问题", value: "question" },
  { label: "其他", value: "other" },
];
const PERFORMCE_TYPE = [
  { label: "运单", value: "0" },
  { label: "其他", value: "1" },
];

const PAGE_GEN_CREATE_TYPE = {
  direct: "direct",
  history_file: "history_file",
  analysis: "analysis",
  history_template: "history_template",
};

// yeji-status
const YEJI_STATUS = {
  running: "running",
  succeed: "succeed",
  failed: "failed",
};

export {
  PAGE_TYPE,
  MODAL_TYPE,
  TABLE_SYS_TEAM_HEADERS,
  TABLE__SYS_PEOPLE_HEADERS,
  GEN_GEN_CREATE_TYPE,
  ACCOUNT_STATUS,
  PEO_TYPE,
  GEN_PROJECT_TYPE,
  PROJECT_TYPE,
  FEEDBACK_TYPE,
  PAGE_GEN_CREATE_TYPE,
  YEJI_STATUS,
  PERFORMCE_TYPE,
};
