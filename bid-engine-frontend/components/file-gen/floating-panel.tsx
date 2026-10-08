"use client";

/* eslint-disable no-nested-ternary */

/* eslint-disable no-unused-vars */
/* Hallmark · component: panel-title-controls · genre: modern-minimal · theme: chakra-smart-bid (preserved tokens)
 * states: default · hover · focus-visible · active · disabled
 * contrast: pass (neutral.500 on neutral.50, primary.600 on white, white on primary.600)
 */
import {
  Box,
  Button,
  Flex,
  IconButton,
  Progress,
  Text,
  Tooltip,
  Divider,
} from "@chakra-ui/react";
import {
  FiDownload,
  FiPlay,
  FiStopCircle,
  FiZap,
  FiImage,
  FiMinimize2,
  FiMaximize2,
  FiMinus,
  FiBookmark,
  FiCheckCircle,
  FiCheckSquare,
  FiExternalLink,
} from "react-icons/fi";
import { TbFileTypeDocx, TbFileTypePdf } from "react-icons/tb";
import { LuSparkles } from "react-icons/lu";
import { motion } from "framer-motion";
import type { GenMode } from "@/service/bid-gen";
import { isDocumentRootOutline } from "./types";

type Props = {
  generating: boolean;
  generationPhase?: "idle" | "queued" | "running" | "reconnecting" | "stopping";
  previewDegraded?: boolean;
  progress: { current: number; total: number; pct: number };
  streamTitle?: string;
  length: string;
  onLengthChange: (v: string) => void;
  onGenerateAll: () => void;
  onCancel: () => void;
  onExportDocx: () => void;
  onExportPdf: () => void;
  /** 正在进行的导出类型：仅对应按钮渲染 loading（null/undefined = 空闲） */
  exportingType?: "docx" | "pdf" | null;
  /** 是否可生成审核项目（招标来源标书 + status=succeeded） */
  canReview?: boolean;
  /** 是否展示审核入口（仅招标来源标书：tender_file / analysis） */
  showReviewEntry?: boolean;
  /** 已生成的审核项目ID（>0 表示已生成，按钮切换为查看审核项目） */
  reviewProjectId?: number;
  /** 生成审核项目（详情页负责导出 DOCX 并调用 create-from-gen） */
  onCreateReview?: () => void;
  /** 跳转到已生成的审核项目 */
  onViewReview?: (id: number) => void;
  /** 生成审核项目进行中 */
  creatingReview?: boolean;
  materialCount?: number;
  onShowMaterials?: () => void;
  pinned?: boolean;
  onTogglePin?: () => void;
  dragControls?: any;
  compact?: boolean;
  onToggleCompact?: () => void;
  onCollapse?: () => void;
  canCollapse?: boolean;
  /** 当前选中章节（大纲导航中最近选中的章节） */
  activeChapter?: {
    id: number;
    parentId: number;
    title: string;
    genStatus: string;
    source: string;
  } | null;
  /** 章节级 AI 操作：write=生成本章/重新生成（直接触发），rewrite/expand/condense=打开弹层 */
  onChapterAiAction?: (mode: GenMode) => void;
  /** 当前章节是否含子章节（章节级写作会连同子章节一起写） */
  activeChapterHasChildren?: boolean;
};

// 导出文件类型品牌色（Word 蓝 / Acrobat 红）——语义色，用于专属文件类型图标识别
const DOCX_COLOR = "#2B579A";
const PDF_COLOR = "#E0403C";

// 篇幅档位：口径为全篇字数下限，与后端 concise/standard/detailed 三档一一对应
const LENGTH_OPTIONS = [
  { key: "concise", label: "精简", desc: "3000 字+" },
  { key: "standard", label: "标准", desc: "5000 字+" },
  { key: "detailed", label: "详细", desc: "8000 字+" },
];

export default function FloatingPanel(props: Props) {
  const {
    generating,
    generationPhase = "idle",
    previewDegraded,
    progress,
    streamTitle,
    length,
    onLengthChange,
    onGenerateAll,
    onCancel,
    onExportDocx,
    onExportPdf,
    canReview,
    showReviewEntry = true,
    reviewProjectId,
    onCreateReview,
    onViewReview,
    creatingReview,
    exportingType = null,
    materialCount,
    onShowMaterials,
    pinned,
    onTogglePin,
    dragControls,
    compact,
    onToggleCompact,
    onCollapse,
    canCollapse = true,
    activeChapter,
    onChapterAiAction,
    activeChapterHasChildren,
  } = props;
  const exporting = exportingType !== null;
  const activeIsRoot = isDocumentRootOutline(activeChapter);
  // 当前章节生成本章提示：含子章节时说明会连带整棵子树
  const activeChapterHint = activeIsRoot
    ? "目录根节点无需生成正文"
    : activeChapter?.genStatus === "succeeded"
      ? "该章节已生成，可用重新生成"
      : activeChapterHasChildren
        ? "生成该章节及其子章节正文"
        : "生成该章节正文";
  const phaseLabel =
    generationPhase === "queued"
      ? "排队中"
      : generationPhase === "reconnecting"
        ? "正在重新连接"
        : generationPhase === "stopping"
          ? "正在停止"
          : "正在生成";

  return (
    <motion.div
      initial={{ opacity: 0, x: 24 }}
      animate={{ opacity: 1, x: 0 }}
      transition={{ duration: 0.35, ease: "easeOut" }}
    >
      {compact && !generating ? (
        /* ===== 缩小模式 — 竖向药丸 ===== */
        <Flex
          direction="column"
          align="center"
          gap={2}
          w="48px"
          py={3}
          bg="workbench.paper"
          border="1px solid"
          borderColor="workbench.line"
          borderRadius="14px"
          boxShadow="xl"
          cursor="grab"
          _active={{ cursor: "grabbing" }}
          onPointerDown={(e: any) => dragControls?.start(e)}
          userSelect="none"
        >
          <Tooltip label="整篇生成" placement="left">
            <IconButton
              aria-label="整篇生成"
              size="sm"
              variant="ghost"
              color="primary.600"
              _hover={{ color: "primary.700", bg: "primary.50" }}
              icon={<FiZap size="16" />}
              onClick={onGenerateAll}
              isDisabled={generating}
            />
          </Tooltip>
          <Tooltip
            label={
              activeChapter
                ? activeIsRoot
                  ? "目录根节点无需生成正文"
                  : activeChapter.genStatus === "succeeded"
                  ? "章节 AI：重写 / 扩写 / 缩写"
                  : "章节 AI：生成本章"
                : "先在大纲中选择章节"
            }
            placement="left"
          >
            <IconButton
              aria-label="章节 AI"
              size="sm"
              variant="ghost"
              color="gold.600"
              _hover={{ color: "gold.700", bg: "gold.50" }}
              icon={<LuSparkles size="15" />}
              onClick={() =>
                onChapterAiAction?.(
                  activeChapter?.genStatus === "succeeded"
                    ? "rewrite"
                    : "write",
                )
              }
              isDisabled={generating || !activeChapter || activeIsRoot}
            />
          </Tooltip>
          <Divider borderColor="gray.100" w="6" />
          <Tooltip label="导出 DOCX" placement="left">
            <IconButton
              aria-label="导出 DOCX"
              size="sm"
              variant="ghost"
              color="gray.500"
              _hover={{ bg: "gray.50" }}
              icon={<TbFileTypeDocx size="19" color={DOCX_COLOR} />}
              onClick={onExportDocx}
              isDisabled={generating || exporting}
              isLoading={exportingType === "docx"}
            />
          </Tooltip>
          <Tooltip label="导出 PDF" placement="left">
            <IconButton
              aria-label="导出 PDF"
              size="sm"
              variant="ghost"
              color="gray.500"
              _hover={{ bg: "gray.50" }}
              icon={<TbFileTypePdf size="19" color={PDF_COLOR} />}
              onClick={onExportPdf}
              isDisabled={generating || exporting}
              isLoading={exportingType === "pdf"}
            />
          </Tooltip>
          <Divider borderColor="gray.100" w="6" />
          {/* 缩小模式下，图标向外 = 展开/最大化 */}
          <Tooltip label="展开面板" placement="left">
            <IconButton
              aria-label="展开面板"
              size="sm"
              variant="ghost"
              color="gray.400"
              _hover={{ color: "primary.600", bg: "primary.50" }}
              icon={<FiMaximize2 size="14" />}
              onClick={onToggleCompact}
            />
          </Tooltip>
        </Flex>
      ) : (
        /* ===== 展开模式 ===== */
        <Flex
          direction="column"
          gap={4}
          w="290px"
          position="relative"
          overflow="hidden"
          bg="rgba(255,255,255,0.92)"
          backdropFilter="blur(16px)"
          border="1px solid"
          borderColor="primary.100"
          borderRadius="2xl"
          boxShadow="0 16px 48px rgba(30,58,95,0.16)"
          p={4}
        >
          {/* 标题栏 — 可拖动区域 */}
          <Flex
            align="center"
            justify="space-between"
            mb={-2}
            cursor="grab"
            _active={{ cursor: "grabbing" }}
            onPointerDown={(e: any) => dragControls?.start(e)}
            userSelect="none"
          >
            <Flex align="center" gap={2}>
              <Flex
                w="7"
                h="7"
                borderRadius="8px"
                align="center"
                justify="center"
                bg="workbench.control"
                color="gold.300"
              >
                <FiZap />
              </Flex>
              <Text fontSize="sm" fontWeight="700" color="gray.800">
                功能面板
              </Text>
            </Flex>
            {/* 三按钮分组胶囊工具栏：缩小=蓝系悬停 / 隐藏=金系悬停 / 钉住=深蓝实底锚定态 */}
            <Flex
              align="center"
              gap="0.5"
              px="0.5"
              py="0.5"
              bg="rgba(248,250,252,0.85)"
              border="1px solid"
              borderColor="neutral.200"
              borderRadius="full"
              boxShadow="inset 0 1px 2px rgba(15,23,42,0.04)"
            >
              <Tooltip label="缩小">
                <IconButton
                  aria-label="缩小面板"
                  size="xs"
                  variant="ghost"
                  borderRadius="full"
                  color="neutral.500"
                  icon={<FiMinimize2 size="13" />}
                  onClick={onToggleCompact}
                  _hover={{ color: "primary.600", bg: "primary.50" }}
                  _active={{ transform: "scale(0.92)" }}
                  _focusVisible={{
                    boxShadow: "0 0 0 2px var(--chakra-colors-primary-400)",
                    outline: "none",
                  }}
                />
              </Tooltip>
              {/* 隐藏：钉住状态下禁用 */}
              <Tooltip label={canCollapse ? "折叠" : "钉住时不可折叠"}>
                <IconButton
                  aria-label="折叠面板"
                  size="xs"
                  variant="ghost"
                  borderRadius="full"
                  color="neutral.500"
                  icon={<FiMinus size="13" />}
                  onClick={onCollapse}
                  isDisabled={!canCollapse}
                  _hover={
                    canCollapse
                      ? { color: "gold.600", bg: "gold.50" }
                      : undefined
                  }
                  _active={{ transform: "scale(0.92)" }}
                  _focusVisible={{
                    boxShadow: "0 0 0 2px var(--chakra-colors-primary-400)",
                    outline: "none",
                  }}
                />
              </Tooltip>
              <Tooltip label={pinned ? "取消钉住" : "钉住面板"}>
                <IconButton
                  aria-label={pinned ? "取消钉住" : "钉住面板"}
                  size="xs"
                  variant="ghost"
                  borderRadius="full"
                  color={pinned ? "white" : "primary.600"}
                  bg={pinned ? "primary.600" : "primary.50"}
                  icon={<FiBookmark size="14" />}
                  onClick={onTogglePin}
                  _hover={
                    pinned
                      ? { bg: "primary.700" }
                      : { bg: "primary.100", color: "primary.700" }
                  }
                  _active={{ transform: "scale(0.92)" }}
                  _focusVisible={{
                    boxShadow: "0 0 0 2px var(--chakra-colors-primary-400)",
                    outline: "none",
                  }}
                />
              </Tooltip>
            </Flex>
          </Flex>
          {/* AI 生成区 */}
          <Box>
            {/* 篇幅选择 */}
            <Flex gap={1.5} mb={3}>
              {LENGTH_OPTIONS.map((opt) => (
                <Button
                  key={opt.key}
                  size="sm"
                  flex="1"
                  variant={length === opt.key ? "solid" : "outline"}
                  bg={length === opt.key ? "primary.600" : "white"}
                  color={length === opt.key ? "white" : "gray.600"}
                  borderColor="primary.200"
                  borderRadius="lg"
                  _hover={{ opacity: 0.9 }}
                  onClick={() => onLengthChange(opt.key)}
                  title={`篇幅：${opt.desc}`}
                >
                  {opt.label}
                </Button>
              ))}
            </Flex>

            <Button
              w="full"
              minH="44px"
              bg="workbench.control"
              color="workbench.paper"
              _hover={{
                bg: "workbench.controlRaised",
                transform: "translateY(-1px)",
              }}
              _active={{ transform: "translateY(0)" }}
              leftIcon={<FiPlay />}
              onClick={onGenerateAll}
              isDisabled={generating}
            >
              整篇生成
            </Button>
          </Box>

          {/* 章节 AI 区：针对当前选中章节生成/改写 */}
          <Box borderTop="1px solid" borderColor="gray.100" pt={3}>
            <Flex align="center" gap={2} mb={2.5}>
              <Flex
                w="7"
                h="7"
                borderRadius="lg"
                align="center"
                justify="center"
                bg="gold.50"
                color="gold.600"
              >
                <LuSparkles />
              </Flex>
              <Text fontSize="sm" fontWeight="700" color="gray.800" flex="1">
                章节 AI
              </Text>
            </Flex>
            <Tooltip
              label={
                activeChapter ? activeChapter.title : "先在大纲中选择一个章节"
              }
              openDelay={600}
            >
              <Text
                fontSize="xs"
                color={activeChapter ? "gray.600" : "gray.400"}
                noOfLines={1}
                mb={2.5}
              >
                {activeChapter ? activeChapter.title : "未选择章节"}
              </Text>
            </Tooltip>
            <Flex gap={2}>
              <Tooltip
                label={activeChapter ? activeChapterHint : "先在大纲中选择章节"}
                openDelay={600}
              >
                <Button
                  flex="1"
                  size="sm"
                  variant="outline"
                  borderColor="primary.200"
                  leftIcon={<FiZap />}
                  isDisabled={
                    generating ||
                    !activeChapter ||
                    activeIsRoot ||
                    activeChapter.genStatus === "succeeded"
                  }
                  onClick={() => onChapterAiAction?.("write")}
                >
                  生成本章
                </Button>
              </Tooltip>
              <Tooltip
                label={
                  activeIsRoot
                    ? "目录根节点无需改写"
                    : activeChapter?.genStatus === "succeeded"
                    ? "重写 / 扩写 / 缩写"
                    : "该章节暂无内容，请先生成本章"
                }
                openDelay={600}
              >
                <Button
                  flex="1"
                  size="sm"
                  variant="outline"
                  borderColor="gold.300"
                  color="gray.700"
                  leftIcon={<LuSparkles size="14" />}
                  isDisabled={
                    generating ||
                    !activeChapter ||
                    activeIsRoot ||
                    activeChapter.genStatus !== "succeeded"
                  }
                  onClick={() => onChapterAiAction?.("rewrite")}
                >
                  改写…
                </Button>
              </Tooltip>
            </Flex>
          </Box>

          {/* 导出区 */}
          <Box borderTop="1px solid" borderColor="gray.100" pt={3}>
            <Flex align="center" gap={2} mb={2.5}>
              <Flex
                w="7"
                h="7"
                borderRadius="lg"
                align="center"
                justify="center"
                bg="primary.50"
                color="primary.600"
              >
                <FiDownload />
              </Flex>
              <Text fontSize="sm" fontWeight="700" color="gray.800">
                导出
              </Text>
            </Flex>
            <Flex gap={2}>
              <Button
                flex="1"
                size="sm"
                variant="outline"
                borderColor="primary.200"
                leftIcon={<TbFileTypeDocx color={DOCX_COLOR} />}
                onClick={onExportDocx}
                isDisabled={generating || exporting}
                isLoading={exportingType === "docx"}
                loadingText="导出中…"
              >
                DOCX
              </Button>
              <Button
                flex="1"
                size="sm"
                variant="outline"
                borderColor="primary.200"
                leftIcon={<TbFileTypePdf color={PDF_COLOR} />}
                onClick={onExportPdf}
                isDisabled={generating || exporting}
                isLoading={exportingType === "pdf"}
                loadingText="导出中…"
              >
                PDF
              </Button>
            </Flex>
          </Box>

          {/* 审核区：交付前创建审核项目（仅招标来源标书展示） */}
          {showReviewEntry && (
          <Box borderTop="1px solid" borderColor="gray.100" pt={3}>
            <Flex align="center" gap={2} mb={2.5}>
              <Flex
                w="7"
                h="7"
                borderRadius="lg"
                align="center"
                justify="center"
                bg="success.50"
                color="success.600"
              >
                <FiCheckCircle />
              </Flex>
              <Text fontSize="sm" fontWeight="700" color="gray.800">
                审核
              </Text>
            </Flex>
            {reviewProjectId ? (
              <Button
                flex="1"
                size="sm"
                variant="outline"
                borderColor="success.200"
                colorScheme="success"
                leftIcon={<FiExternalLink />}
                onClick={() => onViewReview?.(reviewProjectId)}
              >
                查看审核项目
              </Button>
            ) : (
              <Tooltip
                label={
                  canReview
                    ? "生成审核项目（自动带出关联招标文件与当前标书）"
                    : "仅从招标文件创建且已完成的标书可生成审核项目"
                }
                hasArrow
              >
                <Button
                  flex="1"
                  size="sm"
                  colorScheme="success"
                  leftIcon={<FiCheckSquare />}
                  onClick={onCreateReview}
                  isDisabled={!canReview || generating}
                  isLoading={creatingReview}
                  loadingText="创建中…"
                >
                  生成审核项目
                </Button>
              </Tooltip>
            )}
          </Box>
          )}

          {/* 素材区 */}
          <Box borderTop="1px solid" borderColor="gray.100" pt={3}>
            <Flex
              align="center"
              gap={2}
              mb={2.5}
              cursor="pointer"
              onClick={onShowMaterials}
            >
              <Flex
                w="7"
                h="7"
                borderRadius="lg"
                align="center"
                justify="center"
                bg="gold.50"
                color="gold.600"
              >
                <FiImage />
              </Flex>
              <Text fontSize="sm" fontWeight="700" color="gray.800" flex="1">
                章节素材
              </Text>
              <Tooltip label="已插入素材图片数">
                <Flex
                  align="center"
                  justify="center"
                  minW="6"
                  h="6"
                  px={1}
                  borderRadius="full"
                  bg="gold.100"
                  color="gold.700"
                  fontSize="xs"
                  fontWeight="700"
                >
                  {materialCount ?? 0}
                </Flex>
              </Tooltip>
            </Flex>
          </Box>

          {/* 提示 */}
          <Text
            fontSize="xs"
            color="gray.400"
            borderTop="1px solid"
            borderColor="gray.100"
            pt={3}
          >
            生成期间编辑器自动锁定，完成后恢复编辑；素材将自动插入章节末尾
          </Text>

          {/* 生成中毛玻璃遮罩：整体覆盖面板，展示当前章节/进度 + 停止生成 */}
          {generating && (
            <Flex
              position="absolute"
              inset="0"
              direction="column"
              align="center"
              justify="center"
              gap={3}
              zIndex={2}
              bg="rgba(255,255,255,0.55)"
              backdropFilter="blur(12px)"
              px={4}
              pointerEvents="auto"
            >
              <Flex
                align="center"
                gap={2}
                fontSize="sm"
                fontWeight="600"
                color="gray.700"
              >
                <Box
                  w="2"
                  h="2"
                  borderRadius="full"
                  bg="gold.500"
                  animation="pulse 1.2s infinite"
                  flexShrink={0}
                />
                <Text noOfLines={2}>
                  {phaseLabel} · {streamTitle || "等待任务状态…"}
                </Text>
              </Flex>
              <Progress
                value={progress.pct}
                size="sm"
                w="full"
                colorScheme="gold"
                borderRadius="full"
                hasStripe
                isAnimated
              />
              <Text fontSize="xs" color="gray.500">
                已完成 {progress.current}/{progress.total} 章 · {progress.pct}%
              </Text>
              {previewDegraded && (
                <Text fontSize="xs" color="warning.700" textAlign="center">
                  实时预览异常，后台生成仍在继续，完成后将自动恢复正文
                </Text>
              )}
              <Button
                size="sm"
                variant="solid"
                colorScheme="error"
                leftIcon={<FiStopCircle />}
                onClick={onCancel}
                mt={1}
                isLoading={generationPhase === "stopping"}
                loadingText="正在停止"
                isDisabled={generationPhase === "stopping"}
              >
                停止生成
              </Button>
            </Flex>
          )}
        </Flex>
      )}
    </motion.div>
  );
}
