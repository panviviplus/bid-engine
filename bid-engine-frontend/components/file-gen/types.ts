// 标书生成模块类型定义

export type BidGenCreateType = "tender_file" | "template" | "blank" | "analysis";

export type BidGenStatus =
  | "parsing"
  | "outline_review"
  | "draft"
  | "generating"
  | "succeeded"
  | "failed";

export interface BidGenOutlineNode {
  id: number;
  projectId: number;
  parentId: number;
  level: number;
  sortOrder: number;
  title: string;
  clauseIds: string;
  materialIds: string;
  genStatus: "pending" | "generating" | "succeeded" | "failed";
  source: string;
  isRequiredFile: boolean;
  isAiSuggested: boolean;
}

export interface BidGenTaskResp {
  id: number;
  projectId: number;
  taskType: string;
  status: string;
  progress: number;
  currentOutlineId: number;
  completedCount: number;
  totalCount: number;
  errorMsg: string;
  lengthTier?: string;
  genMode?: string;
  attemptCount?: number;
}

export function isDocumentRootOutline(
  node?: Pick<BidGenOutlineNode, "parentId" | "title" | "source"> | null,
) {
  return Boolean(
    node && node.parentId === 0 && node.source === "system_root",
  );
}

export interface BidGenProjectItem {
  id: number;
  name: string;
  createType: BidGenCreateType;
  status: BidGenStatus;
  progress: number;
  stage: string;
  sourceFileName: string;
  stageStatus?: Record<string, string>;
  createdTime: number;
  updatedTime: number;
  outlineCount: number;
  succeededCount: number;
  /** 已生成的审核项目ID（>0 表示已生成审核项目） */
  reviewProjectId: number;
}

export interface BidGenProjectDetail {
  id: number;
  name: string;
  createType: BidGenCreateType;
  status: BidGenStatus;
  progress: number;
  stage: string;
  stageStatus?: Record<string, string>;
  sourceFileName: string;
  sourceFileUrl: string;
  sourceFileObject: string;
  docJson: string;
  docHtml: string;
  outline: BidGenOutlineNode[];
  runningTask?: BidGenTaskResp;
  lastError: string;
  createdTime: number;
  updatedTime: number;
  /** 已生成的审核项目ID（>0 表示已生成审核项目） */
  reviewProjectId: number;
}

// 创建模式
export type CreateMode = "tender" | "template" | "blank";
