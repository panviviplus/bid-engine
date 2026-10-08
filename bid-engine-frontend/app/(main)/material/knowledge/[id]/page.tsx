"use client";

/* eslint-disable no-await-in-loop */
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Badge,
  Box,
  Button,
  Card,
  CardBody,
  Drawer,
  DrawerBody,
  DrawerCloseButton,
  DrawerContent,
  DrawerHeader,
  DrawerOverlay,
  Flex,
  IconButton,
  Image,
  Modal,
  ModalBody,
  ModalCloseButton,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  SimpleGrid,
  Spinner,
  Text,
  Tooltip,
  useDisclosure,
} from "@chakra-ui/react";
import {
  AddIcon,
  ChevronDownIcon,
  ChevronUpIcon,
  DeleteIcon,
  DownloadIcon,
  CloseIcon,
  ViewIcon,
} from "@chakra-ui/icons";
import { FiArrowLeft } from "react-icons/fi";
import { useRouter, usePathname } from "next/navigation";
import { SpecialZoomLevel } from "@react-pdf-viewer/core";

import DeleteConfirmModal from "@/components/common/delete-confirm-modal";
import PDFViewer from "@/components/common/viewer/pdf-viewer";
import { useCustomToast } from "@/hooks/useCustomToast";

import {
  useMaterialDetail,
  useMaterialFileAdd,
  useMaterialFileDelete,
  useMaterialImageDelete,
  useMaterialImageSortOrder,
  useMaterialFileSortOrder,
  useMaterialOcrResults,
  useMaterialOcrRetry,
} from "@/service/material";

const EMPTY_FILES: any[] = [];

function bytesToSize(bytes: number) {
  if (!bytes || bytes <= 0) return "0B";
  const units = ["B", "KB", "MB", "GB"];
  let idx = 0;
  let size = bytes;
  while (size >= 1024 && idx < units.length - 1) {
    size /= 1024;
    idx += 1;
  }
  return `${size.toFixed(idx === 0 ? 0 : 2)}${units[idx]}`;
}

function fileGroupByExt(ext: string) {
  const e = ext.toLowerCase();
  if ([".jpg", ".jpeg", ".png"].includes(e)) return "image";
  if ([".doc", ".docx", ".pdf"].includes(e)) return "doc";
  return "";
}

function validateFiles(files: File[]) {
  if (!files?.length) return { ok: true, msg: "" };
  if (files.length > 5) return { ok: false, msg: "单次不超过5个文件" };
  for (const f of files) {
    const ext = `.${f.name.split(".").pop() || ""}`.toLowerCase();
    const group = fileGroupByExt(ext);
    if (!group) return { ok: false, msg: "不支持的文件类型" };
    if (group === "image" && f.size > 5 * 1024 * 1024)
      return { ok: false, msg: "图片大小不能超过5MB" };
    if (group === "doc" && f.size > 200 * 1024 * 1024)
      return { ok: false, msg: "文档大小不能超过200MB" };
  }
  return { ok: true, msg: "" };
}

function encodeObjectKeyForPath(objectKey: string) {
  return String(objectKey || "")
    .split("/")
    .map((s) => encodeURIComponent(s))
    .join("/");
}

function FileRow({
  item,
  checked,
  onToggle,
  onSelect,
  isActive,
  dragGroup,
  draggingId,
  dragOverId,
  onDragStart,
  onDragOver,
  onDrop,
  onDragEnd,
}: any) {
  return (
    <Flex
      px={2}
      py={2}
      align="center"
      borderRadius="md"
      bg={isActive ? "workbench.canvas" : "transparent"}
      border={dragOverId === item.id ? "1px dashed" : "1px solid"}
      borderColor={dragOverId === item.id ? "primary.300" : "transparent"}
      _hover={{ bg: "workbench.canvas" }}
      cursor="pointer"
      onClick={() => onSelect(item)}
      onDragOver={(e) => onDragOver(e, dragGroup, item.id)}
      onDrop={(e) => onDrop(e, dragGroup, item.id)}
    >
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => {
          e.stopPropagation();
          onToggle(dragGroup, item.id);
        }}
        onClick={(e) => e.stopPropagation()}
      />
      <Box ml={2} flex={1} minW={0}>
        <Text fontSize="sm" color="workbench.text" noOfLines={1}>
          {item.name}
        </Text>
        <Text fontSize="xs" color="workbench.muted">
          {bytesToSize(item.size)}
        </Text>
      </Box>
      <Box
        draggable
        onDragStart={(e) => onDragStart(e, dragGroup, item.id)}
        onDragEnd={onDragEnd}
        onClick={(e) => e.stopPropagation()}
      >
        <Image
          src="/images/drag.png"
          alt="drag"
          boxSize="16px"
          opacity={draggingId === item.id ? 0.9 : 0.6}
        />
      </Box>
    </Flex>
  );
}

/** 素材信息字段：标签在上、值在下，供基础信息卡片的栅格使用 */
function InfoField({
  label,
  value,
  valueNode,
  emphasize = false,
  children,
}: {
  label: string;
  value?: any;
  valueNode?: any;
  emphasize?: boolean;
  children?: any;
}) {
  const content = children ?? valueNode ?? (
    <Tooltip label={String(value ?? "-")} hasArrow>
      <Text
        fontSize={emphasize ? "md" : "sm"}
        fontWeight={emphasize ? "700" : "500"}
        color="workbench.text"
        lineHeight="1.5"
        noOfLines={emphasize ? 1 : 2}
      >
        {value ?? "-"}
      </Text>
    </Tooltip>
  );

  return (
    <Box minW={0}>
      <Text fontSize="11px" fontWeight="600" color="workbench.muted" mb={1}>
        {label}
      </Text>
      <Box minW={0}>{content}</Box>
    </Box>
  );
}

export default function KnowledgeDetailPage({ params }: any) {
  const showToast = useCustomToast();
  const router = useRouter();
  const pathname = usePathname();
  const apiBase = "/api";

  const materialId = Number(params?.id || 0);

  // 根据路径确定素材类型，使用类型专属 detail API
  const materialTypeFromPath = useMemo(() => {
    if (pathname.includes("/qualification/")) return "qualification";
    if (pathname.includes("/performance/")) return "performance";
    if (pathname.includes("/template/")) return "template";
    return "";
  }, [pathname]);

  const detailApiUrl = useMemo(() => {
    if (materialTypeFromPath) {
      return `/material/${materialTypeFromPath}/detail/${materialId}`;
    }
    return `/material/detail/${materialId}`;
  }, [materialTypeFromPath, materialId]);

  // 返回目标：按素材类型回到各自的列表页，无法判定时回到素材库默认入口
  const listRoute = materialTypeFromPath
    ? `/material/${materialTypeFromPath}`
    : "/material/knowledge";
  const listLabel =
    {
      qualification: "企业资质",
      performance: "企业业绩",
      template: "文档模板",
    }[materialTypeFromPath as string] || "素材库";

  const { detailData, detailLoading, fetchDetail } = useMaterialDetail();
  const { fileAddLoading, fetchFileAdd } = useMaterialFileAdd();
  const { fileDeleteLoading, fetchFileDelete } = useMaterialFileDelete();
  const { imageDeleteLoading, fetchImageDelete } = useMaterialImageDelete();
  const { fetchImageSortOrder } = useMaterialImageSortOrder();
  const { fetchFileSortOrder } = useMaterialFileSortOrder();
  const { ocrResults, fetchOcrResults } = useMaterialOcrResults();
  const { ocrRetryLoading, fetchOcrRetry } = useMaterialOcrRetry();

  const {
    isOpen: addFileOpen,
    onOpen: onAddFileOpen,
    onClose: onAddFileClose,
  } = useDisclosure();
  const {
    isOpen: deleteOpen,
    onOpen: onDeleteOpen,
    onClose: onDeleteClose,
  } = useDisclosure();
  const {
    isOpen: lightboxOpen,
    onOpen: onLightboxOpen,
    onClose: onLightboxClose,
  } = useDisclosure();
  const {
    isOpen: previewOpen,
    onOpen: onPreviewOpen,
    onClose: onPreviewClose,
  } = useDisclosure();
  const [checkedImages, setCheckedImages] = useState<number[]>([]);
  const [checkedDocs, setCheckedDocs] = useState<number[]>([]);
  const [pendingFiles, setPendingFiles] = useState<File[]>([]);
  const [baseCollapsed, setBaseCollapsed] = useState(false);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [imageList, setImageList] = useState<any[]>([]);
  const [docList, setDocList] = useState<any[]>([]);
  const [dragging, setDragging] = useState<{
    group: "image" | "doc";
    id: number;
  } | null>(null);
  const [dragOverId, setDragOverId] = useState<number | null>(null);

  const [lightboxUrl, setLightboxUrl] = useState("");
  const [lightboxName, setLightboxName] = useState("");
  // 右侧预览区状态：文档走后端转换后的 PDF（Blob），图片直接走下载地址
  const [previewFile, setPreviewFile] = useState<any>(null);
  const [previewState, setPreviewState] = useState<{
    status: "idle" | "loading" | "ready" | "error";
    url?: string;
    message?: string;
  }>({ status: "idle" });

  const fetchDetailRef = useRef(fetchDetail);
  useEffect(() => {
    fetchDetailRef.current = fetchDetail;
  }, [fetchDetail]);

  const refreshDetail = useCallback(() => {
    if (!materialId) return Promise.resolve();
    return fetchDetailRef
      .current({ url: detailApiUrl })
      .then((res: any) => {
        if (res?.data?.code !== 0) {
          showToast({
            title: res?.data?.message || "查询失败",
            status: "error",
          });
        }
      })
      .catch(() =>
        showToast({ title: "网络异常，请稍后重试", status: "error" }),
      );
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [materialId]);

  useEffect(() => {
    if (!materialId) {
      showToast({ title: "知识库ID不合法", status: "error" });
      router.replace("/material/knowledge");
      return;
    }
    refreshDetail();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [materialId, refreshDetail, router]);

  useEffect(() => {
    if (!addFileOpen) return;
    setPendingFiles([]);
  }, [addFileOpen]);

  const imageFiles = detailData?.image_files || EMPTY_FILES;
  const docFiles = detailData?.doc_files || EMPTY_FILES;

  useEffect(() => {
    setImageList(imageFiles);
  }, [imageFiles]);

  useEffect(() => {
    setDocList(docFiles);
  }, [docFiles]);

  const handleOpenLightbox = (file: any) => {
    const key = file.object_key || "";
    setLightboxUrl(
      `${apiBase}/material/file/download/${encodeObjectKeyForPath(key)}`,
    );
    setLightboxName(file.name || "");
    onLightboxOpen();
  };

  const previewUrlRef = useRef<string>("");

  const releasePreviewUrl = useCallback(() => {
    if (previewUrlRef.current) {
      URL.revokeObjectURL(previewUrlRef.current);
      previewUrlRef.current = "";
    }
  }, []);

  useEffect(() => releasePreviewUrl, [releasePreviewUrl]);

  /**
   * 文档预览：一次性取回 PDF Blob 再交给阅读器。
   * 后端已把 .doc/.docx 转成 PDF 并做缓存，前端只发一次请求，
   * 避免 pdf.js 的分段请求反复触发后端转换。
   */
  const loadDocPreview = useCallback(
    async (file: any) => {
      setPreviewFile({ ...file, kind: "doc" });
      setPreviewState({ status: "loading" });
      releasePreviewUrl();
      onPreviewOpen();
      try {
        const res = await fetch(`${apiBase}/material/file/preview/${file.id}`, {
          credentials: "include",
        });
        if (!res.ok) {
          throw new Error(
            res.status === 401 || res.status === 403
              ? "没有权限预览该文件，请重新登录后再试"
              : `预览失败（HTTP ${res.status}）`,
          );
        }
        const blob = await res.blob();
        if (!blob.size) throw new Error("文件内容为空，无法预览");
        const url = URL.createObjectURL(blob);
        previewUrlRef.current = url;
        setPreviewState({ status: "ready", url });
      } catch (e: any) {
        setPreviewState({
          status: "error",
          message: e?.message || "预览失败，请稍后重试",
        });
      }
    },
    [releasePreviewUrl, onPreviewOpen],
  );

  const handleSelectImage = useCallback(
    (file: any) => {
      releasePreviewUrl();
      setPreviewFile({ ...file, kind: "image" });
      setPreviewState({
        status: "ready",
        url: `${apiBase}/material/file/download/${encodeObjectKeyForPath(
          file.object_key || "",
        )}?v=${file.id}`,
      });
      onPreviewOpen();
    },
    [releasePreviewUrl, onPreviewOpen],
  );

  const handleClosePreview = useCallback(() => {
    onPreviewClose();
    releasePreviewUrl();
    setPreviewState({ status: "idle" });
  }, [onPreviewClose, releasePreviewUrl]);

  const persistSortOrder = useCallback(
    async (group: "image" | "doc", ids: number[]) => {
      const payload = { material_id: materialId, file_ids: ids };
      const res =
        group === "image"
          ? await fetchImageSortOrder({ data: payload })
          : await fetchFileSortOrder({ data: payload });
      if (res?.data?.code === 0) {
        showToast({ title: "排序已保存", status: "success" });
        await refreshDetail();
        return;
      }
      showToast({ title: res?.data?.message || "排序失败", status: "error" });
      await refreshDetail();
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [fetchFileSortOrder, fetchImageSortOrder, materialId, refreshDetail],
  );

  const onDragStartRow = useCallback(
    (e: any, group: "image" | "doc", id: number) => {
      e.dataTransfer?.setData?.("text/plain", String(id));
      e.dataTransfer.effectAllowed = "move";
      setDragging({ group, id });
    },
    [],
  );

  const onDragOverRow = useCallback(
    (e: any, group: "image" | "doc", id: number) => {
      if (!dragging || dragging.group !== group) return;
      e.preventDefault();
      if (dragging.id === id) {
        setDragOverId(null);
        return;
      }
      setDragOverId(id);
    },
    [dragging],
  );

  const onDropRow = useCallback(
    async (e: any, group: "image" | "doc", targetId: number) => {
      if (!dragging || dragging.group !== group) return;
      e.preventDefault();
      const fromId = dragging.id;
      if (fromId === targetId) return;

      const list = group === "image" ? imageList : docList;
      const fromIdx = list.findIndex((it: any) => it.id === fromId);
      const toIdx = list.findIndex((it: any) => it.id === targetId);
      if (fromIdx < 0 || toIdx < 0) return;

      const next = [...list];
      const [moved] = next.splice(fromIdx, 1);
      next.splice(toIdx, 0, moved);

      if (group === "image") setImageList(next);
      else setDocList(next);

      setDragOverId(null);
      setDragging(null);

      await persistSortOrder(
        group,
        next.map((it: any) => it.id),
      );
    },
    [docList, dragging, imageList, persistSortOrder],
  );

  const onDragEndRow = useCallback(() => {
    setDragOverId(null);
    setDragging(null);
  }, []);

  const toggleChecked = (group: "image" | "doc", id: number) => {
    if (group === "image") {
      setCheckedImages((prev) =>
        prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id],
      );
      return;
    }
    setCheckedDocs((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id],
    );
  };

  const handleDeleteFiles = () => {
    if (!checkedImages.length && !checkedDocs.length) {
      showToast({ title: "请先选择文件", status: "error" });
      return;
    }
    onDeleteOpen();
  };

  const confirmDelete = async () => {
    const docIds = [...checkedDocs];
    const imageIds = [...checkedImages];
    for (const id of docIds) {
      const res = await fetchFileDelete({ url: `/material/file/delete/${id}` });
      if (res?.data?.code !== 0) {
        showToast({ title: res?.data?.message || "删除失败", status: "error" });
        return;
      }
    }
    for (const id of imageIds) {
      const res = await fetchImageDelete({
        url: `/material/image/delete/${id}`,
      });
      if (res?.data?.code !== 0) {
        showToast({ title: res?.data?.message || "删除失败", status: "error" });
        return;
      }
    }
    showToast({ title: "删除成功", status: "success" });
    setCheckedImages([]);
    setCheckedDocs([]);
    onDeleteClose();
    await refreshDetail();
  };

  const handlePickFiles = (e: any) => {
    const next = Array.from(e?.target?.files || []) as File[];
    const { ok, msg } = validateFiles(next);
    if (!ok) {
      showToast({ title: msg, status: "error" });
      e.target.value = "";
      return;
    }
    setPendingFiles(next);
  };

  const handleRemovePendingFile = (idx: number) => {
    setPendingFiles((prev) => prev.filter((_, i) => i !== idx));
  };

  const submitAddFile = async () => {
    if (!pendingFiles.length) {
      showToast({ title: "请选择文件", status: "error" });
      return;
    }
    const fd = new FormData();
    pendingFiles.forEach((f) => fd.append("file", f));
    const res = await fetchFileAdd({
      url: `/material/file/add/${materialId}`,
      data: fd,
    });
    if (res?.data?.code === 0) {
      showToast({ title: "新增成功", status: "success" });
      setPendingFiles([]);
      onAddFileClose();
      await refreshDetail();
      return;
    }
    showToast({ title: res?.data?.message || "新增失败", status: "error" });
  };

  return (
    <>
      <Flex
        h="full"
        minH={0}
        direction="column"
        bg="workbench.canvas"
        p={{ base: 3, md: 5, xl: 6 }}
      >
        {/* 顶部信息条：返回 + 素材名称 + 元信息 + 文件操作 */}
        <Flex align="center" gap={3} mb={4} minW={0} flexWrap="wrap">
          <IconButton
            aria-label={`返回${listLabel}列表`}
            icon={<FiArrowLeft />}
            variant="ghost"
            size="sm"
            minW="44px"
            minH="44px"
            color="workbench.text"
            _hover={{ bg: "white" }}
            _active={{ bg: "neutral.100" }}
            _focusVisible={{
              outline: "2px solid",
              outlineColor: "gold.400",
              outlineOffset: "2px",
            }}
            onClick={() => router.push(listRoute)}
          />
          <Box flex={1} minW={0}>
            <Flex align="center" gap={2} minW={0}>
              <Text
                as="h1"
                fontSize={{ base: "md", md: "lg" }}
                fontWeight="700"
                color="workbench.text"
                noOfLines={1}
                minW={0}
              >
                {detailData?.name || "-"}
              </Text>
              <Badge
                colorScheme="blue"
                variant="subtle"
                borderRadius="full"
                px={2}
                py={0.5}
                fontSize="10px"
                flexShrink={0}
              >
                {detailData?.type_name || detailData?.type || "-"}
              </Badge>
            </Flex>
            <Text fontSize="xs" color="workbench.muted" noOfLines={1}>
              {detailData?.user_name || "-"} · {detailData?.company_name || "-"}{" "}
              · {detailData?.created_time || "-"}
            </Text>
          </Box>
          <Flex gap={2} flexShrink={0}>
            <Button
              size="sm"
              leftIcon={<AddIcon />}
              bg="workbench.control"
              color="workbench.paper"
              _hover={{ bg: "workbench.controlRaised" }}
              _focusVisible={{
                outline: "2px solid",
                outlineColor: "gold.400",
                outlineOffset: "2px",
              }}
              onClick={onAddFileOpen}
            >
              新增文件
            </Button>
            <Button
              size="sm"
              variant="outline"
              borderColor="error.200"
              color="error.600"
              leftIcon={<DeleteIcon />}
              isDisabled={!checkedImages.length && !checkedDocs.length}
              _hover={{ bg: "error.50" }}
              _focusVisible={{
                outline: "2px solid",
                outlineColor: "gold.400",
                outlineOffset: "2px",
              }}
              onClick={handleDeleteFiles}
            >
              删除选中
            </Button>
          </Flex>
        </Flex>

        <Card
          shadow="sm"
          borderRadius="xl"
          bg="white"
          border="1px solid"
          borderColor="neutral.100"
          mb={4}
        >
          {/* 卡片头：标题 + 折叠开关 */}
          <Flex
            align="center"
            justify="space-between"
            gap={3}
            px={{ base: 4, md: 5 }}
            py={4}
            minW={0}
            borderBottom={baseCollapsed ? "none" : "1px solid"}
            borderColor="neutral.100"
          >
            <Text fontSize="sm" fontWeight="700" color="workbench.text">
              素材信息
            </Text>
            <IconButton
              aria-label={baseCollapsed ? "展开素材信息" : "收起素材信息"}
              icon={baseCollapsed ? <ChevronDownIcon /> : <ChevronUpIcon />}
              size="sm"
              variant="ghost"
              color="workbench.muted"
              _hover={{ bg: "workbench.canvas", color: "workbench.text" }}
              _focusVisible={{
                outline: "2px solid",
                outlineColor: "gold.400",
                outlineOffset: "2px",
              }}
              onClick={() => setBaseCollapsed((v) => !v)}
            />
          </Flex>

          {!baseCollapsed && (
            <CardBody px={{ base: 4, md: 5 }} py={{ base: 4, md: 5 }}>
              {/* 字段栅格：窄屏单列，宽屏按内容自适应列数 */}
              <Box
                display="grid"
                gridTemplateColumns="repeat(auto-fit, minmax(min(100%, 12rem), 1fr))"
                gap={{ base: 4, md: 5 }}
              >
                <Box minW={0} gridColumn={{ base: "auto", lg: "span 2" }}>
                  <InfoField
                    label="素材名称"
                    value={detailData?.name || "-"}
                    emphasize
                  />
                </Box>

                <InfoField
                  label="类型"
                  valueNode={
                    <Badge
                      colorScheme="blue"
                      variant="subtle"
                      borderRadius="full"
                      px={2}
                      py={0.5}
                      fontSize="10px"
                      maxW="100%"
                    >
                      {detailData?.type_name || detailData?.type || "-"}
                    </Badge>
                  }
                />

                <InfoField
                  label="创建人"
                  value={detailData?.user_name || "-"}
                />

                <InfoField
                  label="所属公司"
                  value={detailData?.company_name || "-"}
                />

                <InfoField
                  label="创建日期"
                  valueNode={
                    <Text
                      fontSize="sm"
                      fontWeight="500"
                      color="workbench.text"
                      sx={{ fontVariantNumeric: "tabular-nums" }}
                    >
                      {detailData?.created_time || "-"}
                    </Text>
                  }
                />
              </Box>

              <Box
                mt={{ base: 4, md: 5 }}
                pt={{ base: 4, md: 5 }}
                borderTop="1px solid"
                borderColor="neutral.100"
              >
                <InfoField label="描述">
                  <Text
                    fontSize="sm"
                    lineHeight="1.75"
                    color="workbench.text"
                    whiteSpace="pre-wrap"
                  >
                    {detailData?.description || "-"}
                  </Text>
                </InfoField>
              </Box>
            </CardBody>
          )}
        </Card>

        {/* OCR 解析结果 */}
        {detailData?.type !== "template" && ocrResults?.length > 0 && (
          <Card
            shadow="sm"
            borderRadius="xl"
            bg="white"
            border="1px solid"
            borderColor="neutral.100"
            mb={4}
          >
            <CardBody>
              <Flex align="center" justify="space-between" mb={3}>
                <Text fontWeight="semibold" fontSize="md">
                  OCR 解析结果
                </Text>
                <Button
                  size="xs"
                  variant="outline"
                  isLoading={ocrRetryLoading}
                  onClick={async () => {
                    await fetchOcrRetry({
                      url: `/material/ocr/retry/${materialId}`,
                    });
                    showToast({ title: "OCR 已重新开始", status: "info" });
                    setTimeout(
                      () =>
                        fetchOcrResults({
                          params: { material_id: materialId },
                        }),
                      2000,
                    );
                  }}
                >
                  重新解析
                </Button>
              </Flex>
              {ocrResults.map((r: any) => (
                <Box
                  key={r.id}
                  mb={3}
                  border="1px solid"
                  borderColor="neutral.100"
                  borderRadius="md"
                  p={3}
                >
                  <Flex align="center" gap={2} mb={2}>
                    <Box
                      w="8px"
                      h="8px"
                      borderRadius="full"
                      bg={
                        r.status === "done"
                          ? "green.400"
                          : r.status === "processing"
                            ? "yellow.400"
                            : r.status === "failed"
                              ? "red.400"
                              : "gray.400"
                      }
                    />
                    <Text fontSize="sm" fontWeight="medium">
                      {r.status === "done"
                        ? "解析完成"
                        : r.status === "processing"
                          ? "解析中"
                          : r.status === "failed"
                            ? "解析失败"
                            : "等待中"}
                    </Text>
                    {r.error_message && (
                      <Text fontSize="xs" color="red.500">
                        {r.error_message}
                      </Text>
                    )}
                  </Flex>
                  {r.status === "done" &&
                    r.structured_fields &&
                    (() => {
                      try {
                        const fields =
                          typeof r.structured_fields === "string"
                            ? JSON.parse(r.structured_fields)
                            : r.structured_fields;
                        const entries = Object.entries(fields || {});
                        if (!entries.length) return null;
                        return (
                          <Box mb={2}>
                            <Text fontSize="xs" color="workbench.muted" mb={1}>
                              结构化信息
                            </Text>
                            <Box as="table" width="100%" fontSize="sm">
                              <Box as="tbody">
                                {entries.map(([k, v]: [string, any]) => (
                                  <Box
                                    as="tr"
                                    key={k}
                                    borderBottom="1px solid"
                                    borderColor="gray.50"
                                  >
                                    <Box
                                      as="td"
                                      py={1}
                                      pr={3}
                                      color="workbench.muted"
                                      whiteSpace="nowrap"
                                    >
                                      {k}
                                    </Box>
                                    <Box
                                      as="td"
                                      py={1}
                                      color="workbench.text"
                                      wordBreak="break-all"
                                    >
                                      {String(v || "-")}
                                    </Box>
                                  </Box>
                                ))}
                              </Box>
                            </Box>
                          </Box>
                        );
                      } catch {
                        return null;
                      }
                    })()}
                  {r.status === "done" && r.labels && (
                    <Flex gap={1} flexWrap="wrap">
                      {String(r.labels)
                        .split(",")
                        .filter(Boolean)
                        .map((l: string, i: number) => (
                          <Badge
                            key={i}
                            colorScheme="blue"
                            variant="subtle"
                            fontSize="xs"
                          >
                            {l.trim()}
                          </Badge>
                        ))}
                    </Flex>
                  )}
                </Box>
              ))}
            </CardBody>
          </Card>
        )}

        {/* 无 OCR 数据提示 */}
        {detailData?.type !== "template" &&
          (!ocrResults || ocrResults.length === 0) && (
            <Card
              shadow="sm"
              borderRadius="xl"
              bg="yellow.50"
              border="1px solid"
              borderColor="yellow.200"
              mb={4}
            >
              <CardBody py={3}>
                <Text fontSize="sm" color="yellow.700">
                  OCR 解析尚未完成，上传文件后将自动触发解析。
                </Text>
              </CardBody>
            </Card>
          )}

        {/* 文件：工具栏 + 列表 */}
        <Card
          flex={1}
          minH={0}
          display="flex"
          flexDirection="column"
          shadow="sm"
          borderRadius="xl"
          bg="white"
          border="1px solid"
          borderColor="neutral.100"
        >
          <Flex
            align="center"
            justify="space-between"
            gap={3}
            px={4}
            py={3}
            borderBottom="1px solid"
            borderColor="neutral.100"
          >
            <Text fontSize="sm" fontWeight="600" color="workbench.text">
              文件（图片 {imageFiles.length} · 文档 {docFiles.length}）
            </Text>
            <Text fontSize="xs" color="workbench.muted">
              勾选可批量删除 · 拖拽可排序
            </Text>
          </Flex>

          <Box
            flex={1}
            overflowY="auto"
            minH={0}
            p={4}
            className="thin-scrollbars"
          >
            {/* ======== 企业资质：图片画廊 ======== */}
            {detailData?.type === "qualification" && (
              <>
                {imageList.length > 0 ? (
                  <SimpleGrid
                    minChildWidth={{
                      base: "8.5rem",
                      md: "13rem",
                      xl: "15rem",
                    }}
                    spacing={4}
                    mb={6}
                  >
                    {imageList.map((f: any) => (
                      <Box
                        key={f.id}
                        borderRadius="lg"
                        overflow="hidden"
                        border="1px solid"
                        borderColor={
                          dragOverId === f.id ? "primary.300" : "neutral.100"
                        }
                        bg="white"
                        shadow="sm"
                        cursor="pointer"
                        _hover={{
                          shadow: "md",
                          borderColor: "primary.200",
                        }}
                        onClick={() => handleSelectImage(f)}
                      >
                        <Box
                          position="relative"
                          pt="75%"
                          bg="workbench.canvas"
                          overflow="hidden"
                        >
                          <Image
                            src={`${apiBase}/material/file/download/${encodeObjectKeyForPath(f.object_key || "")}?v=${f.id}`}
                            alt={f.name}
                            position="absolute"
                            top={0}
                            left={0}
                            w="full"
                            h="full"
                            objectFit="cover"
                          />
                          <Box position="absolute" top={1} left={1}>
                            <input
                              type="checkbox"
                              checked={checkedImages.includes(f.id)}
                              onChange={(e) => {
                                e.stopPropagation();
                                toggleChecked("image", f.id);
                              }}
                              onClick={(e) => e.stopPropagation()}
                            />
                          </Box>
                        </Box>
                        <Box p={2}>
                          <Text
                            fontSize="xs"
                            color="workbench.text"
                            noOfLines={1}
                            fontWeight="medium"
                          >
                            {f.name}
                          </Text>
                          <Text fontSize="xs" color="workbench.muted">
                            {bytesToSize(f.size)}
                          </Text>
                        </Box>
                      </Box>
                    ))}
                  </SimpleGrid>
                ) : (
                  <Text fontSize="sm" color="workbench.muted" mb={6}>
                    暂无图片文件
                  </Text>
                )}

                {docList.length > 0 && (
                  <>
                    <Text
                      fontSize="sm"
                      fontWeight="medium"
                      color="gray.600"
                      mb={2}
                    >
                      文档（{docList.length}）
                    </Text>
                    <Flex direction="column" gap={1} mb={4}>
                      {docList.map((f: any) => (
                        <FileRow
                          key={f.id}
                          item={f}
                          checked={checkedDocs.includes(f.id)}
                          onToggle={toggleChecked}
                          onSelect={() => loadDocPreview(f)}
                          isActive={previewFile?.id === f.id}
                          dragGroup="doc"
                          draggingId={
                            dragging?.group === "doc" ? dragging.id : null
                          }
                          dragOverId={dragOverId}
                          onDragStart={onDragStartRow}
                          onDragOver={onDragOverRow}
                          onDrop={onDropRow}
                          onDragEnd={onDragEndRow}
                        />
                      ))}
                    </Flex>
                  </>
                )}
              </>
            )}

            {/* ======== 企业业绩：文档为主 ======== */}
            {detailData?.type === "performance" && (
              <>
                {docList.length > 0 ? (
                  <Flex direction="column" gap={1} mb={6}>
                    {docList.map((f: any) => (
                      <FileRow
                        key={f.id}
                        item={f}
                        checked={checkedDocs.includes(f.id)}
                        onToggle={toggleChecked}
                        onSelect={() => loadDocPreview(f)}
                        isActive={previewFile?.id === f.id}
                        dragGroup="doc"
                        draggingId={
                          dragging?.group === "doc" ? dragging.id : null
                        }
                        dragOverId={dragOverId}
                        onDragStart={onDragStartRow}
                        onDragOver={onDragOverRow}
                        onDrop={onDropRow}
                        onDragEnd={onDragEndRow}
                      />
                    ))}
                  </Flex>
                ) : (
                  <Text fontSize="sm" color="workbench.muted" mb={6}>
                    暂无文档文件，请点击上方"新增文件"上传
                  </Text>
                )}

                {imageList.length > 0 && (
                  <>
                    <Text
                      fontSize="sm"
                      fontWeight="medium"
                      color="gray.600"
                      mb={2}
                    >
                      图片（{imageList.length}）
                    </Text>
                    <SimpleGrid
                      minChildWidth={{
                        base: "8.5rem",
                        md: "13rem",
                        xl: "15rem",
                      }}
                      spacing={4}
                      mb={4}
                    >
                      {imageList.map((f: any) => (
                        <Box
                          key={f.id}
                          borderRadius="lg"
                          overflow="hidden"
                          border="1px solid"
                          borderColor="neutral.100"
                          bg="white"
                          shadow="sm"
                          cursor="pointer"
                          _hover={{
                            shadow: "md",
                            borderColor: "primary.200",
                          }}
                          onClick={() => handleSelectImage(f)}
                        >
                          <Box
                            position="relative"
                            pt="75%"
                            bg="workbench.canvas"
                            overflow="hidden"
                          >
                            <Image
                              src={`${apiBase}/material/file/download/${encodeObjectKeyForPath(f.object_key || "")}?v=${f.id}`}
                              alt={f.name}
                              position="absolute"
                              top={0}
                              left={0}
                              w="full"
                              h="full"
                              objectFit="cover"
                            />
                          </Box>
                          <Box p={2}>
                            <Text
                              fontSize="xs"
                              color="workbench.text"
                              noOfLines={1}
                            >
                              {f.name}
                            </Text>
                            <Text fontSize="xs" color="workbench.muted">
                              {bytesToSize(f.size)}
                            </Text>
                          </Box>
                        </Box>
                      ))}
                    </SimpleGrid>
                  </>
                )}
              </>
            )}

            {/* ======== 文档模板：仅文档 ======== */}
            {detailData?.type === "template" && (
              <>
                {docList.length > 0 ? (
                  <Flex direction="column" gap={1} mb={6}>
                    {docList.map((f: any) => (
                      <FileRow
                        key={f.id}
                        item={f}
                        checked={checkedDocs.includes(f.id)}
                        onToggle={toggleChecked}
                        onSelect={() => loadDocPreview(f)}
                        isActive={previewFile?.id === f.id}
                        dragGroup="doc"
                        draggingId={
                          dragging?.group === "doc" ? dragging.id : null
                        }
                        dragOverId={dragOverId}
                        onDragStart={onDragStartRow}
                        onDragOver={onDragOverRow}
                        onDrop={onDropRow}
                        onDragEnd={onDragEndRow}
                      />
                    ))}
                  </Flex>
                ) : (
                  <Text fontSize="sm" color="workbench.muted" mb={6}>
                    暂无文档文件，请点击上方"新增文件"上传
                  </Text>
                )}
              </>
            )}
          </Box>
        </Card>
      </Flex>

      {/* ======== 预览抽屉：点击文件后从右侧打开 ======== */}
      <Drawer
        isOpen={previewOpen}
        onClose={handleClosePreview}
        placement="right"
        size="lg"
      >
        <DrawerOverlay bg="blackAlpha.500" />
        <DrawerContent
          bg="workbench.paper"
          borderLeft="1px solid"
          borderColor="workbench.line"
          boxShadow="xl"
        >
          <DrawerCloseButton
            color="workbench.muted"
            _hover={{ bg: "workbench.canvas", color: "workbench.text" }}
            _focusVisible={{
              outline: "2px solid",
              outlineColor: "gold.400",
              outlineOffset: "2px",
            }}
          />
          <DrawerHeader
            px={{ base: 4, md: 5 }}
            py={4}
            borderBottom="1px solid"
            borderColor="workbench.line"
            bg="workbench.paper"
          >
            <Flex align="center" gap={3} pr={8} minW={0}>
              <Box minW={0} flex={1}>
                <Text
                  fontSize="sm"
                  fontWeight="700"
                  color="workbench.text"
                  noOfLines={1}
                >
                  {previewFile?.name || "预览"}
                </Text>
                <Text
                  mt={0.5}
                  fontSize="11px"
                  fontWeight={400}
                  color="workbench.muted"
                >
                  {previewFile
                    ? `${
                        previewFile.kind === "image" ? "图片" : "文档"
                      } · ${bytesToSize(previewFile.size)}`
                    : ""}
                </Text>
              </Box>
              {previewFile && (
                <Flex gap={1} flexShrink={0}>
                  {previewFile.kind === "image" && (
                    <Tooltip label="放大查看" hasArrow>
                      <IconButton
                        aria-label="放大查看"
                        icon={<ViewIcon />}
                        size="sm"
                        variant="ghost"
                        color="workbench.muted"
                        _hover={{
                          bg: "workbench.canvas",
                          color: "workbench.text",
                        }}
                        onClick={() => handleOpenLightbox(previewFile)}
                      />
                    </Tooltip>
                  )}
                  <Tooltip label="下载文件" hasArrow>
                    <IconButton
                      aria-label="下载文件"
                      icon={<DownloadIcon />}
                      size="sm"
                      variant="ghost"
                      color="workbench.muted"
                      _hover={{
                        bg: "workbench.canvas",
                        color: "workbench.text",
                      }}
                      onClick={() =>
                        window.open(
                          `${apiBase}/material/file/download/${encodeObjectKeyForPath(
                            previewFile.object_key || "",
                          )}`,
                          "_blank",
                        )
                      }
                    />
                  </Tooltip>
                </Flex>
              )}
            </Flex>
          </DrawerHeader>

          <DrawerBody p={0} bg="workbench.canvas" position="relative">
            {previewState.status === "loading" && (
              <Flex
                h="full"
                direction="column"
                align="center"
                justify="center"
                gap={3}
                aria-live="polite"
              >
                <Spinner color="gold.400" />
                <Text fontSize="sm" color="workbench.muted">
                  正在转换并加载文档…
                </Text>
                <Text fontSize="xs" color="workbench.muted">
                  .doc/.docx 首次打开需要服务端转换，通常几秒内完成
                </Text>
              </Flex>
            )}

            {previewState.status === "error" && (
              <Flex
                h="full"
                direction="column"
                align="center"
                justify="center"
                gap={3}
                px={8}
                textAlign="center"
                aria-live="polite"
              >
                <Text fontSize="sm" fontWeight="600" color="error.600">
                  预览失败
                </Text>
                <Text fontSize="xs" color="workbench.muted">
                  {previewState.message}
                </Text>
                <Flex gap={2}>
                  <Button
                    size="sm"
                    variant="outline"
                    borderColor="workbench.line"
                    onClick={() =>
                      previewFile?.kind === "doc"
                        ? loadDocPreview(previewFile)
                        : previewFile && handleSelectImage(previewFile)
                    }
                  >
                    重试
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() =>
                      previewFile &&
                      window.open(
                        `${apiBase}/material/file/download/${encodeObjectKeyForPath(
                          previewFile.object_key || "",
                        )}`,
                        "_blank",
                      )
                    }
                  >
                    下载原文件
                  </Button>
                </Flex>
              </Flex>
            )}

            {previewState.status === "ready" &&
              previewState.url &&
              previewFile?.kind === "image" && (
                <Flex
                  h="full"
                  w="full"
                  p={{ base: 4, md: 6 }}
                  align="center"
                  justify="center"
                >
                  <Image
                    src={previewState.url}
                    alt={previewFile?.name || "预览"}
                    maxW="full"
                    maxH="full"
                    objectFit="contain"
                    bg="workbench.paper"
                    borderRadius="10px"
                    boxShadow="sm"
                  />
                </Flex>
              )}

            {previewState.status === "ready" &&
              previewState.url &&
              previewFile?.kind !== "image" && (
                <PDFViewer
                  url={previewState.url}
                  pdfH="100%"
                  defaultZoom={SpecialZoomLevel.ActualSize}
                  showHighlights={false}
                  highlightAreas={[]}
                />
              )}
          </DrawerBody>
        </DrawerContent>
      </Drawer>

      {/* ======== Lightbox Modal ======== */}
      <Modal
        isOpen={lightboxOpen}
        onClose={onLightboxClose}
        size="full"
        isCentered
      >
        <ModalOverlay bg="blackAlpha.800" />
        <ModalContent bg="transparent" maxW="90vw" maxH="90vh" boxShadow="none">
          <ModalCloseButton color="white" zIndex={10} />
          <ModalBody
            display="flex"
            flexDirection="column"
            alignItems="center"
            justifyContent="center"
            p={0}
          >
            <Image
              src={lightboxUrl}
              alt={lightboxName}
              maxH="80vh"
              maxW="full"
              objectFit="contain"
              borderRadius="md"
            />
            <Flex mt={4} align="center" gap={4}>
              <Text color="white" fontSize="sm">
                {lightboxName}
              </Text>
              <IconButton
                aria-label="download"
                icon={<DownloadIcon />}
                size="sm"
                colorScheme="whiteAlpha"
                variant="solid"
                onClick={() => window.open(lightboxUrl, "_blank")}
              />
            </Flex>
          </ModalBody>
        </ModalContent>
      </Modal>

      {/* ======== AddFile Modal ======== */}
      <Modal
        isOpen={addFileOpen}
        onClose={onAddFileClose}
        isCentered
        returnFocusOnClose={false}
      >
        <ModalOverlay />
        <ModalContent>
          <ModalHeader>新增文件</ModalHeader>
          <ModalCloseButton />
          <ModalBody>
            <input
              ref={fileInputRef}
              type="file"
              multiple
              accept={
                detailData?.type === "template"
                  ? ".pdf,.doc,.docx"
                  : ".jpg,.jpeg,.png,.pdf,.doc,.docx"
              }
              style={{ display: "none" }}
              onChange={handlePickFiles}
            />
            <Flex
              direction="column"
              align="center"
              justify="center"
              h="9.5rem"
              border="1px dashed"
              borderColor="blue.200"
              bg="blue.50"
              borderRadius="xl"
              cursor="pointer"
              onClick={() => fileInputRef.current?.click?.()}
              gap={2}
            >
              <Flex
                w="10"
                h="10"
                borderRadius="full"
                align="center"
                justify="center"
                bg="white"
                boxShadow="sm"
              >
                <Image src="/images/upload.png" alt="upload" boxSize="6" />
              </Flex>
              <Text fontSize="md" fontWeight="semibold" color="primary.600">
                点击上传
              </Text>
              <Text fontSize="xs" color="workbench.muted">
                最多5个文件，可混合上传
              </Text>
              <Text fontSize="xs" color="workbench.muted">
                支持：JPG/JPEG/PNG（≤5MB），PDF/DOC/DOCX（≤200MB）
              </Text>
            </Flex>
            {!!pendingFiles.length && (
              <Box
                mt={3}
                border="1px solid"
                borderColor="neutral.100"
                borderRadius="md"
              >
                {pendingFiles.map((f, idx) => (
                  <Flex
                    key={`${f.name}-${idx}`}
                    px={3}
                    py={2}
                    align="center"
                    justify="space-between"
                    borderBottom={
                      idx === pendingFiles.length - 1 ? "none" : "1px solid"
                    }
                    borderColor="neutral.100"
                  >
                    <Box minW={0}>
                      <Text fontSize="sm" color="workbench.text" noOfLines={1}>
                        {f.name}
                      </Text>
                      <Text fontSize="xs" color="workbench.muted">
                        {bytesToSize(f.size)}
                      </Text>
                    </Box>
                    <IconButton
                      aria-label="remove"
                      icon={<CloseIcon />}
                      size="sm"
                      variant="ghost"
                      onClick={() => handleRemovePendingFile(idx)}
                    />
                  </Flex>
                ))}
              </Box>
            )}
          </ModalBody>
          <ModalFooter>
            <Button mr={3} variant="outline" onClick={onAddFileClose}>
              取消
            </Button>
            <Button
              colorScheme="primary"
              onClick={submitAddFile}
              isLoading={fileAddLoading}
            >
              确认
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      <DeleteConfirmModal
        isOpen={deleteOpen}
        onClose={onDeleteClose}
        title=""
        description={
          <Box>
            <Text color="#000">
              确认要删除文件吗？（共选中
              {checkedImages.length + checkedDocs.length}个文件）
            </Text>
            <Text color="red.500" mt={2}>
              注意：删除操作是不可逆的，确认后将无法恢复。
            </Text>
          </Box>
        }
        isLoading={fileDeleteLoading || imageDeleteLoading}
        handleConfirm={confirmDelete}
      />
    </>
  );
}
