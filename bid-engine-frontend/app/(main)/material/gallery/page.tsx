"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import {
  AspectRatio,
  Box,
  Button,
  Card,
  CardBody,
  Collapse,
  Flex,
  Heading,
  IconButton,
  Image,
  Input,
  Modal,
  ModalBody,
  ModalCloseButton,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  SimpleGrid,
  Spinner,
  Switch,
  Text,
  Tooltip,
  Checkbox,
} from "@chakra-ui/react";
import { PageViewport } from "@/components/layout/responsive-page";
import {
  AttachmentIcon,
  ChevronDownIcon,
  ChevronUpIcon,
  CloseIcon,
  DeleteIcon,
} from "@chakra-ui/icons";
import { useRouter } from "next/navigation";

import { useCustomToast } from "@/hooks/useCustomToast";
import DeleteConfirmModal from "@/components/common/delete-confirm-modal";
import {
  useMaterialGallery,
  useMaterialGalleryDelete,
  useMaterialGalleryMove,
  useMaterialGalleryUpload,
} from "@/service/material";

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

function encodeObjectKeyForPath(objectKey: string) {
  return String(objectKey || "")
    .split("/")
    .map((s) => encodeURIComponent(s))
    .join("/");
}

function LoadingDots({ label = "加载中" }: { label?: string }) {
  const [dots, setDots] = useState("");
  useEffect(() => {
    const timer = setInterval(() => {
      setDots((prev) => (prev.length >= 3 ? "" : `${prev}.`));
    }, 400);
    return () => clearInterval(timer);
  }, []);
  return (
    <Text fontSize="sm" color="gray.500" noOfLines={1}>
      {label}{dots}
    </Text>
  );
}

function UploadModal({
  isOpen,
  onClose,
  onUploaded,
  isUploading,
  uploadInputRef,
  selectedFiles,
  setSelectedFiles,
  keepOriginalName,
  setKeepOriginalName,
  onPickFiles,
}: any) {
  return (
    <Modal isOpen={isOpen} onClose={onClose} isCentered returnFocusOnClose={false} size="xl">
      <ModalOverlay />
      <ModalContent maxW="720px">
        <ModalHeader>上传图片</ModalHeader>
        <ModalCloseButton />
        <ModalBody>
          <Input
            ref={uploadInputRef}
            type="file"
            multiple
            accept=".jpg,.jpeg,.png"
            display="none"
            onChange={onPickFiles}
          />
          <Box
            border="1px dashed"
            borderColor="blue.200"
            borderRadius="xl"
            p={4}
            bg="blue.50"
            cursor="pointer"
            onClick={() => uploadInputRef.current?.click?.()}
          >
            <Flex direction="column" align="center" gap={2}>
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
              <Text fontSize="lg" fontWeight="semibold" color="primary.600">点击上传</Text>
              <Text fontSize="xs" color="gray.500">支持 JPG/JPEG/PNG，可批量上传</Text>
              <Text fontSize="xs" color="gray.500">单张≤5MB，一次≤5张</Text>
            </Flex>
          </Box>

          <Flex mt={4} align="center" gap={4}>
            <Text fontSize="sm" color="gray.700" fontWeight="medium">保留图片原名称？</Text>
            <Switch isChecked={keepOriginalName} onChange={(e) => setKeepOriginalName(e.target.checked)} />
          </Flex>

          {!!selectedFiles.length && (
            <Box mt={4} border="1px solid" borderColor="gray.100" borderRadius="md">
              {selectedFiles.map((f: File, idx: number) => (
                <Flex
                  key={`${f.name}-${idx}`}
                  px={3}
                  py={2}
                  align="center"
                  justify="space-between"
                  borderBottom={idx === selectedFiles.length - 1 ? "none" : "1px solid"}
                  borderColor="gray.100"
                >
                  <Box minW={0} flex={1}>
                    <Text fontSize="sm" color="gray.800" noOfLines={1}>{f.name}</Text>
                    <Text fontSize="xs" color="gray.500">{bytesToSize(f.size)}</Text>
                  </Box>
                  <IconButton
                    aria-label="remove"
                    icon={<CloseIcon />}
                    size="sm"
                    variant="ghost"
                    onClick={() => setSelectedFiles((prev: File[]) => prev.filter((_, i) => i !== idx))}
                  />
                </Flex>
              ))}
            </Box>
          )}
        </ModalBody>
        <ModalFooter>
          <Button mr={3} variant="outline" onClick={onClose}>取消</Button>
          <Button
            colorScheme="primary"
            onClick={onUploaded}
            isLoading={isUploading}
            isDisabled={!selectedFiles.length}
          >
            确认上传
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
}

function MaterialGalleryGroup({
  group,
  isOpen,
  onToggle,
  selectedIds,
  onToggleSelect,
  onDeleteClick,
  onDropToGroup,
  onDragStartCard,
  onDragEndCard,
  dragOver,
  onDragOverGroup,
  onDragLeaveGroup,
}: any) {
  const router = useRouter();
  const images = group?.images || [];
  const materialId = Number(group?.material_id || 0);
  const materialName = String(group?.material_name || "");

  const apiBase = "/api";

  const countText = useMemo(() => {
    return `共 ${images.length} 张`;
  }, [images.length]);

  return (
    <Box
      border="1px solid"
      borderColor={dragOver ? "primary.300" : "gray.100"}
      borderRadius="xl"
      bg="white"
      overflow="hidden"
      onDragOver={(e) => {
        e.preventDefault();
        onDragOverGroup();
      }}
      onDragLeave={() => onDragLeaveGroup()}
      onDrop={(e) => {
        e.preventDefault();
        e.stopPropagation();
        onDropToGroup(e);
      }}
    >
      <Flex
        px={4}
        py={3}
        align="center"
        bg="gray.50"
        borderBottom="1px solid"
        borderColor="gray.100"
        cursor="pointer"
        onClick={onToggle}
      >
        <Box minW={0} flex={1}>
          <Text fontSize="sm" fontWeight="semibold" color="gray.800" noOfLines={1}>
            {materialName || `知识库#${materialId}`}
          </Text>
          {!!countText && (
            <Text fontSize="xs" color="gray.500" mt={0.5}>
              {countText}
            </Text>
          )}
        </Box>
        <Tooltip label="删除" hasArrow>
          <span>
            <IconButton
              aria-label="delete"
              icon={<DeleteIcon />}
              size="sm"
              variant="ghost"
              color="red.600"
              isDisabled={!selectedIds?.length}
              onClick={(e) => {
                e.stopPropagation();
                onDeleteClick();
              }}
            />
          </span>
        </Tooltip>
        <IconButton
          aria-label={isOpen ? "收起" : "展开"}
          icon={isOpen ? <ChevronUpIcon /> : <ChevronDownIcon />}
          size="sm"
          variant="ghost"
          onClick={(e) => {
            e.stopPropagation();
            onToggle();
          }}
        />
      </Flex>

      <Collapse in={isOpen} animateOpacity>
        <Box p={4}>
          {!images.length ? (
            <Text fontSize="sm" color="gray.500">
              暂无图片
            </Text>
          ) : (
            <SimpleGrid
              minChildWidth={{ base: "8.5rem", md: "13rem", xl: "15rem" }}
              spacing={4}
            >
              {images.map((img: any) => {
                const name = String(img?.name || "");
                const sizeText = bytesToSize(Number(img?.size || 0));
                const objectKey = String(img?.object_key || "");
                const thumbUrl = objectKey
                  ? `${apiBase}/material/file/download/${encodeObjectKeyForPath(objectKey)}`
                  : "";
                const bottomLine = name ? `${name}  ${sizeText}` : "";
                const checked = !!selectedIds?.includes?.(img?.id);
                const handleClick = () => {
                  if (materialId > 0) {
                    router.push(`/material/knowledge/${materialId}?imageId=${img?.id}`);
                    return;
                  }
                };
                const card = (
                  <Card
                    key={img?.id}
                    cursor="pointer"
                    overflow="hidden"
                    borderRadius="lg"
                    border="1px solid"
                    borderColor="gray.100"
                    _hover={{ borderColor: "primary.200", boxShadow: "sm" }}
                    onClick={handleClick}
                    pos="relative"
                    draggable
                    onDragStart={(e) => onDragStartCard(e, materialId, img?.id)}
                    onDragEnd={onDragEndCard}
                  >
                    <Box pos="absolute" top={2} left={2} zIndex={2} onClick={(e) => e.stopPropagation()}>
                      <Checkbox
                        isChecked={checked}
                        onChange={() => onToggleSelect(img?.id)}
                        bg="white"
                        borderRadius="md"
                        px={1}
                        py={1}
                      />
                    </Box>
                    <AspectRatio ratio={4 / 3}>
                      <Box bg="gray.50">
                        {thumbUrl ? (
                          <Image
                            src={thumbUrl}
                            alt={name || "image"}
                            w="100%"
                            h="100%"
                            objectFit="cover"
                          />
                        ) : null}
                      </Box>
                    </AspectRatio>
                    <Box px={3} py={2}>
                      {name ? (
                        <Flex align="center" gap={2} minW={0}>
                          <Text flex="1" minW={0} fontSize="sm" color="gray.800" noOfLines={1}>
                            {name}
                          </Text>
                          <Text
                            flex="0 0 auto"
                            minW="72px"
                            fontSize="sm"
                            color="gray.600"
                            textAlign="right"
                          >
                            {sizeText}
                          </Text>
                        </Flex>
                      ) : (
                        <LoadingDots />
                      )}
                    </Box>
                  </Card>
                );
                if (!name) return card;
                return (
                  <Tooltip key={img?.id} label={bottomLine} hasArrow>
                    {card}
                  </Tooltip>
                );
              })}
            </SimpleGrid>
          )}
        </Box>
      </Collapse>
    </Box>
  );
}

export default function GalleryPage() {
  const showToast = useCustomToast();
  const { galleryGroups, galleryLoading, fetchGallery } = useMaterialGallery();
  const { galleryUploadLoading, fetchGalleryUpload } = useMaterialGalleryUpload();
  const { galleryDeleteLoading, fetchGalleryDelete } = useMaterialGalleryDelete();
  const { galleryMoveLoading, fetchGalleryMove } = useMaterialGalleryMove();
  const [expanded, setExpanded] = useState<number[]>([0]);
  const uploadInputRef = useRef<HTMLInputElement | null>(null);
  const [uploadOpen, setUploadOpen] = useState(false);
  const [selectedFiles, setSelectedFiles] = useState<File[]>([]);
  const [keepOriginalName, setKeepOriginalName] = useState(false);
  const [selectedByGroup, setSelectedByGroup] = useState<Record<number, number[]>>({});
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleteGroupId, setDeleteGroupId] = useState<number | null>(null);
  const [dragOverGroupId, setDragOverGroupId] = useState<number | null>(null);

  useEffect(() => {
    fetchGallery()
      .then((res: any) => {
        if (res?.data?.code !== 0) {
          showToast({ title: res?.data?.message || "查询失败", status: "error" });
        }
      })
      .catch(() => showToast({ title: "网络异常，请稍后重试", status: "error" }));
  }, [fetchGallery, showToast]);

  const groups = useMemo(() => {
    return galleryGroups || [];
  }, [galleryGroups]);

  const toggle = (id: number) => {
    setExpanded((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));
  };

  const toggleSelect = (groupId: number, imageId: number) => {
    if (!imageId) return;
    setSelectedByGroup((prev) => {
      const existing = prev[groupId] || [];
      const next = existing.includes(imageId)
        ? existing.filter((x) => x !== imageId)
        : [...existing, imageId];
      return { ...prev, [groupId]: next };
    });
  };

  const openDeleteForGroup = (groupId: number) => {
    const ids = selectedByGroup[groupId] || [];
    if (!ids.length) return;
    setDeleteGroupId(groupId);
    setDeleteOpen(true);
  };

  const confirmDelete = async () => {
    const groupId = deleteGroupId;
    if (groupId == null) return;
    const ids = selectedByGroup[groupId] || [];
    if (!ids.length) return;
    const res = await fetchGalleryDelete({ data: { image_ids: ids } });
    if (res?.data?.code === 0) {
      showToast({ title: "删除成功", status: "success" });
      setSelectedByGroup((prev) => ({ ...prev, [groupId]: [] }));
      setDeleteOpen(false);
      setDeleteGroupId(null);
      await fetchGallery();
      return;
    }
    showToast({ title: res?.data?.message || "删除失败", status: "error" });
  };

  const onDragStartCard = (e: any, fromGroupId: number, imageId: number) => {
    if (!imageId) return;
    const payload = { imageId, fromGroupId };
    setDragOverGroupId(null);
    e.dataTransfer?.setData?.("application/json", JSON.stringify(payload));
    e.dataTransfer.effectAllowed = "move";
  };

  const onDragEndCard = () => {
    setDragOverGroupId(null);
  };

  const onDropToGroup = async (e: any, toGroupId: number) => {
    const raw = e.dataTransfer?.getData?.("application/json") || "";
    let payload: any = null;
    try {
      payload = raw ? JSON.parse(raw) : null;
    } catch {
      payload = null;
    }
    const imageId = Number(payload?.imageId || 0);
    const fromGroupId = Number(payload?.fromGroupId || 0);
    if (!imageId) return;
    if (fromGroupId === toGroupId) return;
    if (galleryMoveLoading) return;

    const res = await fetchGalleryMove({ data: { image_id: imageId, to_material_id: toGroupId } });
    if (res?.data?.code === 0) {
      showToast({ title: "已移动到目标分组", status: "success" });
      setSelectedByGroup((prev) => {
        const next = { ...prev };
        next[fromGroupId] = (next[fromGroupId] || []).filter((x) => x !== imageId);
        next[toGroupId] = (next[toGroupId] || []).filter((x) => x !== imageId);
        return next;
      });
      await fetchGallery();
      setDragOverGroupId(null);
      return;
    }
    showToast({ title: res?.data?.message || "移动失败", status: "error" });
    setDragOverGroupId(null);
  };

  const validateUploadFiles = (files: File[]) => {
    if (!files.length) return { ok: false, msg: "请选择图片" };
    if (files.length > 5) return { ok: false, msg: "一次最多上传5张图片" };
    for (const f of files) {
      const ext = `.${f.name.split(".").pop() || ""}`.toLowerCase();
      if (![".jpg", ".jpeg", ".png"].includes(ext)) return { ok: false, msg: "仅支持jpg/jpeg/png图片" };
      if (f.size > 5 * 1024 * 1024) return { ok: false, msg: "单张图片不能超过5MB" };
    }
    return { ok: true, msg: "" };
  };

  const handlePickUpload = async (e: any) => {
    const next = Array.from(e?.target?.files || []) as File[];
    const { ok, msg } = validateUploadFiles(next);
    if (!ok) {
      showToast({ title: msg, status: "error" });
      e.target.value = "";
      return;
    }
    setSelectedFiles(next);
    e.target.value = "";
  };

  const submitUpload = async () => {
    const { ok, msg } = validateUploadFiles(selectedFiles);
    if (!ok) {
      showToast({ title: msg, status: "error" });
      return;
    }
    const fd = new FormData();
    selectedFiles.forEach((f) => fd.append("file", f));
    fd.append("keepOriginalName", String(keepOriginalName));
    const res = await fetchGalleryUpload({ data: fd });
    if (res?.data?.code === 0) {
      showToast({ title: "上传成功", status: "success" });
      setUploadOpen(false);
      setSelectedFiles([]);
      setKeepOriginalName(false);
      await fetchGallery();
      setExpanded((prev) => (prev.includes(0) ? prev : [0, ...prev]));
      return;
    }
    showToast({ title: res?.data?.message || "上传失败", status: "error" });
  };

  return (
    <PageViewport scroll={false}>
      <Flex
        h="full"
        minH={0}
        direction="column"
        bg="transparent"
        p={{ base: 3, md: 6, xl: 8, "2xl": 10 }}
        gap={4}
      >
        <Card flex={1} overflow="hidden" shadow="sm" borderRadius="xl" bg="whiteAlpha.800" backdropFilter="blur(10px)" border="1px solid" borderColor="gray.100">
          <CardBody p={6} display="flex" flexDirection="column" h="full" overflowY="auto">
            <Flex align="center" justify="space-between" mb={4}>
              <Box>
                {/* Header removed */}
              </Box>
              <Box>
                <Button
                  colorScheme="primary"
                  leftIcon={<AttachmentIcon />}
                  onClick={() => setUploadOpen(true)}
                  isLoading={galleryUploadLoading}
                >
                  上传
                </Button>
              </Box>
            </Flex>

            {galleryLoading ? (
              <Flex align="center" gap={2} color="gray.500">
                <Spinner size="sm" />
                <Text fontSize="sm">加载中...</Text>
              </Flex>
            ) : (
              <Flex direction="column" gap={4}>
                {!groups.length ? (
                  <Text fontSize="sm" color="gray.500">
                    暂无图片
                  </Text>
                ) : (
                  groups.map((g: any) => {
                    const mid = Number(g?.material_id || 0);
                    const isOpen = expanded.includes(mid);
                    return (
                      <MaterialGalleryGroup
                        key={mid}
                        group={g}
                        isOpen={isOpen}
                        onToggle={() => toggle(mid)}
                        selectedIds={selectedByGroup[mid] || []}
                        onToggleSelect={(imageId: number) => toggleSelect(mid, imageId)}
                        onDeleteClick={() => openDeleteForGroup(mid)}
                        dragOver={dragOverGroupId === mid}
                        onDragOverGroup={() => setDragOverGroupId(mid)}
                        onDragLeaveGroup={() => {
                          if (dragOverGroupId === mid) setDragOverGroupId(null);
                        }}
                        onDragStartCard={onDragStartCard}
                        onDragEndCard={onDragEndCard}
                        onDropToGroup={(e: any) => onDropToGroup(e, mid)}
                      />
                    );
                  })
                )}
              </Flex>
            )}
          </CardBody>
        </Card>

        <UploadModal
          isOpen={uploadOpen}
          onClose={() => {
            setUploadOpen(false);
            setSelectedFiles([]);
            setKeepOriginalName(false);
          }}
          onUploaded={submitUpload}
          isUploading={galleryUploadLoading}
          uploadInputRef={uploadInputRef}
          selectedFiles={selectedFiles}
          setSelectedFiles={setSelectedFiles}
          keepOriginalName={keepOriginalName}
          setKeepOriginalName={setKeepOriginalName}
          onPickFiles={handlePickUpload}
        />

        <DeleteConfirmModal
          isOpen={deleteOpen}
          onClose={() => {
            setDeleteOpen(false);
            setDeleteGroupId(null);
          }}
          title="确认要删除图片吗？"
          description={
            <Box>
              <Text color="#000">
                确认要删除图片吗？（共选中
                {(deleteGroupId == null ? 0 : (selectedByGroup[deleteGroupId] || []).length)}张）
              </Text>
              <Text color="red.500" mt={2}>
                注意：删除操作是不可逆的，确认后将无法恢复。
              </Text>
            </Box>
          }
          isLoading={galleryDeleteLoading}
          handleConfirm={confirmDelete}
        />
      </Flex>
    </PageViewport>
  );
}
