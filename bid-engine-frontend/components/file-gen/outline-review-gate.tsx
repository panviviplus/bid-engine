"use client";

/* eslint-disable no-nested-ternary */

/* eslint-disable no-unused-vars */
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  AlertDialog,
  AlertDialogBody,
  AlertDialogContent,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogOverlay,
  Box,
  Button,
  Flex,
  Text,
  useToast,
} from "@chakra-ui/react";
import { FiAlertTriangle, FiCheckCircle } from "react-icons/fi";

import OutlineNav from "@/components/file-gen/outline-nav";
import OutlineSidebar from "@/components/file-gen/outline-sidebar";
import BidEditor, { BidEditorHandle } from "@/components/file-gen/bid-editor";
import { reorderOutline } from "@/components/file-gen/outline-tree-utils";
import {
  useBidGenOutlineAdd,
  useBidGenOutlineUpdate,
  useBidGenOutlineDelete,
  useBidGenOutlineApply,
  useBidGenSaveDoc,
  useBidGenConfirmOutline,
} from "@/service/bid-gen";
import type { BidGenOutlineNode } from "@/components/file-gen/types";

type Props = {
  project: any;
  onBack: () => void;
  /** 确认成功后回调（父级刷新详情，进入编辑器） */
  onConfirmed: () => void;
};

export default function OutlineReviewGate({
  project,
  onBack,
  onConfirmed,
}: Props) {
  const toast = useToast();
  const projectId = Number(project?.id || 0);

  const [localOutline, setLocalOutline] = useState<BidGenOutlineNode[]>(
    project?.outline || [],
  );
  const [editorReady, setEditorReady] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const cancelRef = useRef<any>(null);
  const editorRef = useRef<BidEditorHandle>(null);
  // 结构快照串行队列：防止拖拽连续触发时并发写
  const applyChainRef = useRef<Promise<void>>(Promise.resolve());
  const localOutlineRef = useRef(localOutline);
  localOutlineRef.current = localOutline;

  const { fetchAdd } = useBidGenOutlineAdd();
  const { fetchUpdate } = useBidGenOutlineUpdate();
  const { fetchDelete } = useBidGenOutlineDelete();
  const { fetchApply } = useBidGenOutlineApply();
  const { fetchSave } = useBidGenSaveDoc();
  const { fetchConfirm } = useBidGenConfirmOutline();

  useEffect(() => {
    setLocalOutline(project?.outline || []);
  }, [project?.outline]);

  const docJSONObj = useMemo(() => {
    if (!project?.docJson) return null;
    try {
      return JSON.parse(project.docJson);
    } catch {
      return null;
    }
  }, [project?.docJson]);

  // ===== 大纲编辑（本地优先 + 实时驱动右侧文档预览）=====
  const handleAdd = useCallback(
    async (parentId: number, title: string, level: number) => {
      try {
        const res = await fetchAdd({
          data: { projectId, parentId, level, title },
        });
        if (res?.data?.code === 200) {
          const newId = Number(res?.data?.data?.id);
          if (!newId) return;
          setLocalOutline((prev) => {
            const max = prev
              .filter((n) => n.parentId === parentId)
              .reduce((m, n) => Math.max(m, n.sortOrder), 0);
            const node: BidGenOutlineNode = {
              id: newId,
              projectId,
              parentId,
              level,
              sortOrder: max + 100,
              title,
              clauseIds: "",
              materialIds: "",
              genStatus: "pending",
              source: "user",
              isRequiredFile: false,
              isAiSuggested: false,
            };
            return [...prev, node];
          });
          if (parentId === 0) {
            editorRef.current?.insertHeading(newId, level, title);
          } else {
            editorRef.current?.insertHeading(newId, level, title, {
              afterOutlineId: parentId,
            });
          }
          editorRef.current?.resetOutlineBaseline();
        } else {
          toast({
            title: res?.data?.msg || "添加失败",
            status: "error",
            duration: 3000,
          });
        }
      } catch (e: any) {
        toast({
          title: e?.response?.data?.msg || "添加失败",
          status: "error",
          duration: 3000,
        });
      }
    },
    [fetchAdd, projectId, toast],
  );

  const handleRename = useCallback(
    async (outlineId: number, title: string) => {
      try {
        const res = await fetchUpdate({ data: { id: outlineId, title } });
        if (res?.data?.code === 200) {
          setLocalOutline((prev) =>
            prev.map((n) => (n.id === outlineId ? { ...n, title } : n)),
          );
          editorRef.current?.renameHeading(outlineId, title);
          editorRef.current?.resetOutlineBaseline();
        }
      } catch {
        // ignore
      }
    },
    [fetchUpdate],
  );

  const handleDelete = useCallback(
    async (outlineId: number) => {
      try {
        const res = await fetchDelete({
          data: { id: outlineId, projectId },
        });
        if (res?.data?.code === 200) {
          toast({ title: "章节已删除", status: "success", duration: 2000 });
          setLocalOutline((prev) => {
            const ids = new Set<number>([outlineId]);
            let changed = true;
            while (changed) {
              changed = false;
              for (const n of prev) {
                if (ids.has(n.parentId) && !ids.has(n.id)) {
                  ids.add(n.id);
                  changed = true;
                }
              }
            }
            return prev.filter((n) => !ids.has(n.id));
          });
          editorRef.current?.removeChapter(outlineId);
          editorRef.current?.resetOutlineBaseline();
        }
      } catch {
        // ignore
      }
    },
    [fetchDelete, projectId, toast],
  );

  const enqueueApply = useCallback(
    (nodes: BidGenOutlineNode[]) => {
      applyChainRef.current = applyChainRef.current.then(async () => {
        try {
          const payload = {
            projectId,
            nodes: nodes.map((n) => ({
              id: n.id,
              parentId: n.parentId,
              level: n.level,
              sortOrder: n.sortOrder,
            })),
          };
          const res = await fetchApply({ data: payload });
          if (
            res?.data?.code === 200 &&
            Array.isArray(res?.data?.data?.outline)
          ) {
            setLocalOutline(res.data.data.outline);
          }
        } catch {
          // 静默：确认时会再兜底提交一次
        }
      });
      return applyChainRef.current;
    },
    [fetchApply, projectId],
  );

  const handleMove = useCallback(
    (dragId: number, targetParentId: number, nextId: number) => {
      const res = reorderOutline(
        localOutlineRef.current,
        dragId,
        targetParentId,
        nextId,
      );
      if (!res) return;
      setLocalOutline(res.nodes);
      editorRef.current?.moveChapter(dragId, {
        parentId: targetParentId,
        nextId,
        newLevel: res.dragLevel,
      });
      editorRef.current?.resetOutlineBaseline();
      enqueueApply(res.nodes);
    },
    [enqueueApply],
  );

  // ===== 确认流程 =====
  const handleConfirm = useCallback(async () => {
    setConfirming(true);
    try {
      // ① 等待在途拖拽 apply 完成，再兜底提交一次权威结构（失败则中止确认）
      const nodes = localOutlineRef.current;
      if (nodes.length > 0) {
        await applyChainRef.current.catch(() => {});
        const applyRes = await fetchApply({
          data: {
            projectId,
            nodes: nodes.map((n) => ({
              id: n.id,
              parentId: n.parentId,
              level: n.level,
              sortOrder: n.sortOrder,
            })),
          },
        });
        if (applyRes?.data?.code !== 200) {
          toast({
            title: applyRes?.data?.msg || "大纲结构保存失败",
            status: "error",
            duration: 3000,
          });
          setConfirming(false);
          return;
        }
      }
      // ② 保存编辑器当前文档（正文随章节保留）
      const docJson = editorRef.current?.getJSON();
      if (docJson) {
        const saveRes = await fetchSave({
          data: {
            projectId,
            docJson: JSON.stringify(docJson),
            docHtml: editorRef.current?.getHTML() || project?.docHtml || "",
          },
        });
        if (saveRes?.data?.code !== 200) {
          toast({
            title: saveRes?.data?.msg || "文档保存失败",
            status: "error",
            duration: 3000,
          });
          setConfirming(false);
          return;
        }
      }
      // ③ 确认大纲
      const res = await fetchConfirm({ data: { projectId } });
      if (res?.data?.code === 200) {
        toast({
          title: "大纲已确认，进入编辑器",
          status: "success",
          duration: 2500,
        });
        setConfirmOpen(false);
        onConfirmed();
      } else {
        toast({
          title: res?.data?.msg || "确认失败",
          status: "error",
          duration: 3000,
        });
        setConfirming(false);
      }
    } catch (e: any) {
      toast({
        title: e?.response?.data?.msg || "确认失败，请重试",
        status: "error",
        duration: 3000,
      });
      setConfirming(false);
    }
  }, [
    fetchApply,
    fetchSave,
    fetchConfirm,
    projectId,
    project?.docHtml,
    toast,
    onConfirmed,
  ]);

  if (!project) return null;

  return (
    <Flex
      direction="column"
      h="full"
      minH={0}
      gap={3}
      p={{ base: 2, md: 4 }}
      bg="workbench.canvas"
    >
      {/* 顶部标题栏 */}
      <Flex
        align="center"
        gap={3}
        px={{ base: 3, md: 6 }}
        py={{ base: 3, md: 4 }}
        border="1px solid"
        borderColor="workbench.controlRaised"
        borderRadius="16px"
        bg="workbench.control"
        color="workbench.paper"
        boxShadow="lg"
        flexShrink={0}
      >
        <Flex
          w="10"
          h="10"
          borderRadius="10px"
          align="center"
          justify="center"
          bg="gold.400"
          color="workbench.control"
          fontSize="xl"
          flexShrink={0}
        >
          <FiCheckCircle />
        </Flex>
        <Box minW={0}>
          <Text fontSize="lg" fontWeight="700" color="workbench.paper">
            大纲蓝图确认
          </Text>
          <Text fontSize="sm" color="whiteAlpha.700" overflowWrap="anywhere">
            AI 已根据
            {project.createType === "analysis"
              ? "招标解析蓝图"
              : project.createType === "tender_file"
                ? "招标文件"
                : project.createType === "blank"
                  ? "空白标书"
                  : "模板"}
            生成标书大纲结构，可调整后进入编辑器
          </Text>
        </Box>
      </Flex>

      {/* 主体：左蓝图树 + 右文档预览（调宽后窄屏横向滚动，避免压扁预览） */}
      <Flex
        flex="1"
        minH="0"
        overflowX="auto"
        border="1px solid"
        borderColor="workbench.line"
        borderRadius="14px"
        bg="workbench.paper"
      >
        <OutlineSidebar
          defaultWidth={340}
          storageKey="bidgen-blueprint-outline-width"
          fullWidthOnBase
          borderRight="1px solid"
          borderColor="gray.200"
          bg="white"
        >
          <OutlineNav
            nodes={localOutline}
            variant="blueprint"
            readOnly={!editorReady}
            onJump={() => {}}
            onAdd={handleAdd}
            onRename={handleRename}
            onDelete={handleDelete}
            onMove={handleMove}
          />
        </OutlineSidebar>
        <Box
          flex="1"
          minW="320px"
          bg="white"
          display={{ base: "none", md: "block" }}
        >
          {/* 确认前只读提示：正文不可编辑，仅编排大纲 */}
          <Flex
            align="center"
            gap={2}
            px={4}
            py={2}
            bg="warning.50"
            borderBottom="1px solid"
            borderColor="warning.200"
          >
            <FiAlertTriangle
              color="#D97706"
              size="14"
              style={{ flexShrink: 0 }}
            />
            <Text fontSize="sm" color="warning.700">
              大纲待确认 · 请先编排标书大纲，确认后即可进入编辑器
            </Text>
          </Flex>
          <BidEditor
            ref={editorRef}
            contentKey={`bidgen-${projectId}-gate`}
            contentJSON={docJSONObj || undefined}
            contentHTML={project.docHtml || undefined}
            outline={localOutline}
            readOnly
            onReady={() => setEditorReady(true)}
          />
        </Box>
      </Flex>

      {/* 底部操作栏 */}
      <Flex
        align="center"
        justify="flex-end"
        gap={3}
        px={{ base: 3, md: 6 }}
        py={3}
        direction={{ base: "column-reverse", sm: "row" }}
        border="1px solid"
        borderColor="workbench.line"
        borderRadius="12px"
        bg="workbench.paper"
        flexShrink={0}
      >
        {localOutline.length === 0 && (
          <Text mr={{ sm: "auto" }} fontSize="sm" color="warning.700">
            请先添加至少一个大纲章节
          </Text>
        )}
        <Button
          w={{ base: "full", sm: "auto" }}
          variant="ghost"
          onClick={onBack}
          isDisabled={confirming}
        >
          返回列表
        </Button>
        <Button
          w={{ base: "full", sm: "auto" }}
          minH="44px"
          bg="workbench.control"
          color="workbench.paper"
          _hover={{ bg: "workbench.controlRaised" }}
          isDisabled={confirming || !editorReady || localOutline.length === 0}
          onClick={() => setConfirmOpen(true)}
        >
          确认大纲，进入编辑器
        </Button>
      </Flex>

      {/* 二次确认 */}
      <AlertDialog
        isOpen={confirmOpen}
        leastDestructiveRef={cancelRef}
        onClose={() => {
          if (!confirming) setConfirmOpen(false);
        }}
        isCentered
      >
        <AlertDialogOverlay bg="blackAlpha.500" backdropFilter="blur(2px)">
          <AlertDialogContent borderRadius="14px">
            <AlertDialogHeader fontSize="lg" fontWeight="700">
              确认大纲？
            </AlertDialogHeader>
            <AlertDialogBody fontSize="sm" color="gray.600">
              确认后，AI 建议项与可选项将一并纳入标书大纲并进入编辑器；
              章节次序将锁定，仍可重命名、增删章节。
            </AlertDialogBody>
            <AlertDialogFooter gap={3}>
              <Button
                ref={cancelRef}
                variant="ghost"
                onClick={() => setConfirmOpen(false)}
                isDisabled={confirming}
              >
                再调整一下
              </Button>
              <Button
                minH="44px"
                bg="workbench.control"
                color="workbench.paper"
                _hover={{ bg: "workbench.controlRaised" }}
                isLoading={confirming}
                loadingText="确认中…"
                onClick={handleConfirm}
              >
                确认并进入编辑器
              </Button>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialogOverlay>
      </AlertDialog>
    </Flex>
  );
}
