"use client";

import React, {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
} from "react";
import {
  Box,
  Flex,
  Text,
  IconButton,
  Tooltip,
  Badge,
} from "@chakra-ui/react";
import { ChevronLeftIcon, ChevronRightIcon } from "@chakra-ui/icons";
import {
  IoIosAddCircleOutline,
  IoIosRemoveCircleOutline,
} from "react-icons/io";
import { FiFile } from "react-icons/fi";
import { useReducedMotion } from "framer-motion";
import PDFViewer from "@/components/common/viewer/pdf-viewer";
import { ReviewFile, TraceRef } from "./types";

export interface SourcePdfHandle {
  jumpToTrace: (refs: TraceRef[], side: "tender" | "bid") => void;
}

/* Hallmark · component: preview-pane-toolbar (bid-audit) · genre: modern-minimal · theme: chakra-smart-bid (preserved tokens)
 * states: default · hover · focus-visible · active · disabled（缩放/翻页随 PDF 状态禁用）
 */
interface PdfToolbarSlots {
  ZoomOut: React.FC<{
    children?: (p: {
      enableShortcuts: boolean;
      isDisabled?: boolean;
      onClick: () => void;
    }) => React.ReactNode;
  }>;
  Zoom: React.FC;
  ZoomIn: React.FC<{
    children?: (p: {
      enableShortcuts: boolean;
      isDisabled?: boolean;
      onClick: () => void;
    }) => React.ReactNode;
  }>;
  GoToPreviousPage: React.FC<{
    children?: (p: { isDisabled: boolean; onClick: () => void }) => React.ReactNode;
  }>;
  GoToNextPage: React.FC<{
    children?: (p: { isDisabled: boolean; onClick: () => void }) => React.ReactNode;
  }>;
  CurrentPageLabel: React.FC<{
    children?: (p: {
      currentPage: number;
      numberOfPages: number;
      pageLabel: string;
    }) => React.ReactNode;
  }>;
}

/* Hallmark · component: fold-to-edge-icon (bid-audit preview) · genre: modern-minimal · theme: chakra-smart-bid (preserved tokens)
 * states: 静态图标，8 态由外层 IconButton 承载（default · hover · focus-visible · active）
 * 语义：双右箭头 » = 面板向右缘折叠（业界成熟的折叠到边缘手势）
 */
function FoldToEdgeIcon() {
  return (
    <svg
      width="15"
      height="15"
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden
      focusable="false"
    >
      <path d="M4.6 4.8 L7.6 8 L4.6 11.2" />
      <path d="M9.2 4.8 L12.2 8 L9.2 11.2" />
    </svg>
  );
}

/** PDF 功能按钮（放大/缩小/翻页）——与招/投标文件标题同一行，窄屏自动换行 */
function PdfToolbar({ slots }: { slots: PdfToolbarSlots }) {
  const {
    ZoomOut,
    Zoom,
    ZoomIn,
    GoToPreviousPage,
    GoToNextPage,
    CurrentPageLabel,
  } = slots;
  const btnProps = {
    size: "xs" as const,
    variant: "ghost" as const,
    color: "neutral.500" as const,
    h: "26px",
    w: "26px",
    minW: 0,
    borderRadius: "md",
    transition: "all 0.16s cubic-bezier(0.16, 1, 0.3, 1)",
    _hover: { color: "primary.600", bg: "neutral.100" },
    _active: { color: "primary.700", bg: "primary.50", transform: "scale(0.94)" },
    _focusVisible: {
      boxShadow: "0 0 0 2px var(--chakra-colors-primary-400)",
      outline: "none",
    },
  };
  return (
    <Flex
      align="center"
      gap={0.5}
      flexShrink={0}
      pl={2}
      ml={2}
      borderLeft="1px solid"
      borderColor="neutral.100"
    >
      <ZoomOut>
        {(props) => (
          <IconButton
            aria-label="缩小"
            icon={<IoIosRemoveCircleOutline size={16} />}
            isDisabled={props.isDisabled}
            onClick={props.onClick}
            {...btnProps}
          />
        )}
      </ZoomOut>
      <Zoom />
      <ZoomIn>
        {(props) => (
          <IconButton
            aria-label="放大"
            icon={<IoIosAddCircleOutline size={16} />}
            isDisabled={props.isDisabled}
            onClick={props.onClick}
            {...btnProps}
          />
        )}
      </ZoomIn>
      <GoToPreviousPage>
        {(props) => (
          <IconButton
            aria-label="上一页"
            icon={<ChevronLeftIcon fontSize="18px" />}
            isDisabled={props.isDisabled}
            onClick={props.onClick}
            {...btnProps}
          />
        )}
      </GoToPreviousPage>
      <CurrentPageLabel>
        {(props) => (
          <Text
            fontSize="11px"
            color="neutral.500"
            minW="32px"
            textAlign="center"
            flexShrink={0}
            userSelect="none"
          >
            {props.currentPage + 1} / {props.numberOfPages}
          </Text>
        )}
      </CurrentPageLabel>
      <GoToNextPage>
        {(props) => (
          <IconButton
            aria-label="下一页"
            icon={<ChevronRightIcon fontSize="18px" />}
            isDisabled={props.isDisabled}
            onClick={props.onClick}
            {...btnProps}
          />
        )}
      </GoToNextPage>
    </Flex>
  );
}

function Pane({
  title,
  color,
  files,
  buildUrl,
  activeIndex,
  onFileChange,
  collapsed,
  basis,
  onCollapsedChange,
  jumpRef,
  searchRef,
}: {
  title: string;
  color: string;
  files: ReviewFile[];
  buildUrl: (fileId: number) => string;
  activeIndex: number;
  onFileChange: (i: number) => void;
  collapsed: boolean;
  basis: string;
  onCollapsedChange: (v: boolean) => void;
  jumpRef: React.MutableRefObject<((page: number) => void) | null>;
  searchRef: React.MutableRefObject<((kw: string) => void) | null>;
}) {
  const reduceMotion = useReducedMotion();
  // 折叠动画：比旧版(0.32s)稍慢，0.5s 渐收进右缘耳朵；reduced-motion 降级 ≤150ms 透明度
  const duration = reduceMotion ? "0.15s" : "0.5s";
  const easing = reduceMotion ? "ease" : "cubic-bezier(0.4, 0, 0.2, 1)";

  const [toolbar, setToolbar] = useState<PdfToolbarSlots | null>(null);

  const activeFile = files[activeIndex] || files[0];

  return (
    <Box
      h="full"
      overflow="hidden"
      flex={`0 0 ${basis}`}
      minW="0"
      flexShrink={0}
      borderLeft="1px solid"
      borderColor="neutral.200"
      bg="neutral.100"
      transition={`flex-basis ${duration} ${easing}, min-width ${duration} ${easing}`}
    >
      {/* 内容保持挂载：折叠时仅做向右缘收拢 + 渐隐的 transform/opacity 过渡，展开即时恢复 */}
      <Flex
        direction="column"
        h="full"
        minW={0}
        opacity={collapsed ? 0 : 1}
        transform={collapsed ? "translateX(28px) scaleX(0.92)" : "none"}
        transformOrigin="right center"
        pointerEvents={collapsed ? "none" : "auto"}
        transition={
          reduceMotion
            ? "opacity 0.15s ease"
            : `opacity 0.35s cubic-bezier(0.4, 0, 0.2, 1), transform 0.5s cubic-bezier(0.4, 0, 0.2, 1)`
        }
      >
        {/* 栏头：折叠钮 + 标题 + 文件切换 + PDF 功能按钮（窄屏自动换行，行距 6px） */}
        <Flex
          align="center"
          gap={2}
          rowGap={1.5}
          px={2.5}
          py={1.5}
          bg="white"
          borderBottom="1px solid"
          borderColor="neutral.200"
          flexShrink={0}
          flexWrap="wrap"
        >
          <Tooltip label={`收起${title}`}>
            <IconButton
              aria-label={`收起${title}`}
              icon={<FoldToEdgeIcon />}
              size="xs"
              variant="ghost"
              color="neutral.400"
              onClick={() => onCollapsedChange(true)}
              _hover={{ color: "primary.600", bg: "neutral.100" }}
              _active={{
                color: "primary.700",
                bg: "primary.50",
                transform: "scale(0.94)",
              }}
              _focusVisible={{
                boxShadow: "0 0 0 2px var(--chakra-colors-primary-400)",
                outline: "none",
              }}
            />
          </Tooltip>
          <Text fontSize="12px" fontWeight="700" color={color} flexShrink={0}>
            {title}
          </Text>
          {files.length > 1 ? (
            <Flex
              gap={1}
              overflowX="auto"
              className="thin-scrollbars"
              flex={1}
              minW={0}
            >
              {files.map((f, i) => (
                <Badge
                  key={f.id}
                  as="button"
                  variant="subtle"
                  bg={i === activeIndex ? color : "neutral.100"}
                  color={i === activeIndex ? "white" : "neutral.500"}
                  fontSize="10px"
                  borderRadius="full"
                  px={2}
                  py={0.5}
                  cursor="pointer"
                  whiteSpace="nowrap"
                  maxW="160px"
                  overflow="hidden"
                  textOverflow="ellipsis"
                  title={f.file_name}
                  onClick={() => onFileChange(i)}
                >
                  {f.file_name}
                </Badge>
              ))}
            </Flex>
          ) : (
            <Box flex={1} minW={0} />
          )}
          {toolbar && <PdfToolbar slots={toolbar} />}
        </Flex>

        {/* 原文 PDF 主体 */}
        <Box flex={1} minH={0} position="relative" bg="neutral.200">
          {activeFile ? (
            <PDFViewer
              url={buildUrl(activeFile.id)}
              highlightAreas={[]}
              onPagesContainerReady={undefined}
              onJumpToPageRef={jumpRef}
              onSearchRef={searchRef}
              pdfH="100%"
              showHighlights={false}
              externalToolbar={setToolbar}
            />
          ) : (
            <Flex
              h="full"
              align="center"
              justify="center"
              color="neutral.400"
              fontSize="xs"
              direction="column"
              gap={1}
            >
              <FiFile size={18} />
              <Text>暂无{title}文件</Text>
            </Flex>
          )}
        </Box>
      </Flex>
    </Box>
  );
}

interface Props {
  tenderFiles: ReviewFile[];
  bidFiles: ReviewFile[];
  buildUrl: (fileId: number) => string;
  tenderCollapsed: boolean;
  bidCollapsed: boolean;
  onTenderCollapsedChange: (v: boolean) => void;
  onBidCollapsedChange: (v: boolean) => void;
}

interface PendingTrace {
  side: "tender" | "bid";
  fileId: number;
  page: number;
  quote: string;
}

const SourcePdfPanel = forwardRef<SourcePdfHandle, Props>(function SourcePdfPanel(
  {
    tenderFiles,
    bidFiles,
    buildUrl,
    tenderCollapsed,
    bidCollapsed,
    onTenderCollapsedChange,
    onBidCollapsedChange,
  },
  ref,
) {
  const [tenderIndex, setTenderIndex] = useState(0);
  const [bidIndex, setBidIndex] = useState(0);
  const [pending, setPending] = useState<PendingTrace | null>(null);

  const tenderJumpRef = useRef<((p: number) => void) | null>(null);
  const tenderSearchRef = useRef<((k: string) => void) | null>(null);
  const bidJumpRef = useRef<((p: number) => void) | null>(null);
  const bidSearchRef = useRef<((k: string) => void) | null>(null);

  // 文件变化时重置活动文件（列表来自详情刷新）
  useEffect(() => {
    setTenderIndex(0);
    setBidIndex(0);
  }, [tenderFiles.length, bidFiles.length]);

  useImperativeHandle(ref, () => ({
    jumpToTrace: (refs: TraceRef[], side: "tender" | "bid") => {
      if (!refs?.length) return;
      const first = refs[0];
      const files = side === "tender" ? tenderFiles : bidFiles;
      // 溯源跳转时自动展开对应面板
      if (side === "tender") onTenderCollapsedChange(false);
      else onBidCollapsedChange(false);
      const idx = files.findIndex((f) => f.id === first.fileId);
      const resolvedIndex = idx >= 0 ? idx : 0;
      if (side === "tender") setTenderIndex(resolvedIndex);
      else setBidIndex(resolvedIndex);
      setPending({
        side,
        fileId: first.fileId,
        page: first.page,
        quote: first.quote,
      });
    },
  }));

  // 溯源定位：等待 PDF 切换/加载后跳页 + 高亮
  useEffect(() => {
    if (!pending) return;
    const { side, page, quote } = pending;
    const jump = side === "tender" ? tenderJumpRef : bidJumpRef;
    const search = side === "tender" ? tenderSearchRef : bidSearchRef;
    const t1 = setTimeout(() => {
      if (page > 0) jump.current?.(page);
      const t2 = setTimeout(() => {
        if (quote) search.current?.(quote);
        setPending(null);
      }, 400);
      return () => clearTimeout(t2);
    }, 450);
    return () => clearTimeout(t1);
  }, [pending]);

  // 面板宽度用显式 flex-basis（50/50 ↔ 0/100），保证折叠宽度过渡真实生效
  const tenderBasis = tenderCollapsed ? "0px" : bidCollapsed ? "100%" : "50%";
  const bidBasis = bidCollapsed ? "0px" : tenderCollapsed ? "100%" : "50%";

  return (
    <Flex h="full" minW={0}>
      <Pane
        title="招标原文"
        color="var(--chakra-colors-primary-600)"
        files={tenderFiles}
        buildUrl={buildUrl}
        activeIndex={tenderIndex}
        onFileChange={setTenderIndex}
        collapsed={tenderCollapsed}
        basis={tenderBasis}
        onCollapsedChange={onTenderCollapsedChange}
        jumpRef={tenderJumpRef}
        searchRef={tenderSearchRef}
      />
      <Pane
        title="投标原文"
        color="var(--chakra-colors-info-600)"
        files={bidFiles}
        buildUrl={buildUrl}
        activeIndex={bidIndex}
        onFileChange={setBidIndex}
        collapsed={bidCollapsed}
        basis={bidBasis}
        onCollapsedChange={onBidCollapsedChange}
        jumpRef={bidJumpRef}
        searchRef={bidSearchRef}
      />
    </Flex>
  );
});

export default SourcePdfPanel;
