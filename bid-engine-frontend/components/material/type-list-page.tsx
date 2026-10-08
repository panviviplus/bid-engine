"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Box, Button, Flex, Heading, Text, Card, CardBody, IconButton,
  Tooltip, Modal, ModalOverlay, ModalContent, ModalHeader, ModalBody,
  ModalFooter, ModalCloseButton, FormControl, FormLabel, Input, Image,
  Textarea, SimpleGrid, Badge,
} from "@chakra-ui/react";
import { AddIcon, DeleteIcon, EditIcon, CloseIcon } from "@chakra-ui/icons";
import { useRouter } from "next/navigation";
import { usePagination } from "@ajna/pagination";

import CommonForm from "@/components/common-form";
import TableFooter from "@/components/common/table-footer";
import DeleteConfirmModal from "@/components/common/delete-confirm-modal";
import { useCustomToast } from "@/hooks/useCustomToast";
import { toFormData } from "@/utils";
import type { FormItemType } from "@/components/common-form";
import {
  PageContent,
  PageViewport,
} from "@/components/layout/responsive-page";

export interface TypeListPageProps {
  type: string;
  typeName: string;
  addButtonLabel: string;
  fileAccept: string;
  uploadHint: string;
  descPlaceholder: string;
  fetchList: (config: any) => Promise<any>;
  listLoading: boolean;
  fetchAdd: (config: any) => Promise<any>;
  addLoading: boolean;
  fetchUpdate: (config: any) => Promise<any>;
  updateLoading: boolean;
  fetchDelete: (config: any) => Promise<any>;
  deleteLoading: boolean;
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

export default function MaterialTypeListPage(props: TypeListPageProps) {
  const { type, typeName, addButtonLabel, fileAccept, uploadHint, descPlaceholder,
    fetchList, listLoading, fetchAdd, addLoading, fetchUpdate, updateLoading, fetchDelete, deleteLoading } = props;

  const showToast = useCustomToast();
  const router = useRouter();

  const { currentPage, setCurrentPage, setPageSize, pageSize, pagesCount, pages } = usePagination({
    total: 0,
    limits: { outer: 1, inner: 1 },
    initialState: { currentPage: 1, pageSize: 10 },
  });
  const [searchForm, setSearchForm] = useState<any>({});
  const [totalNum, setTotalNum] = useState(0);
  const [listItems, setListItems] = useState<any[]>([]);
  const [listApiError, setListApiError] = useState({ isError: false, message: "" });

  const [modalOpen, setModalOpen] = useState(false);
  const [modalMode, setModalMode] = useState<"add" | "edit">("add");
  const [modalInitial, setModalInitial] = useState<any>(null);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleteRow, setDeleteRow] = useState<any>(null);
  const [editRow, setEditRow] = useState<any>(null);

  const formItems: FormItemType[] = useMemo(
    () => [
      { name: "keyword", placeholder: "按名称、描述模糊筛选", type: "input", icon: true,
        props: { mr: 4, mb: 3, borderRadius: "full", h: "9", inputw: "18rem" } },
    ],
    [],
  );

  const refreshList = useCallback(
    (override?: any) => {
      return fetchList({
        params: { pageNum: currentPage, pageSize, ...(override || searchForm) },
      })
        .then((res: any) => {
          if (res?.data?.code === 0) {
            const nextList = res?.data?.data?.list || [];
            const nextTotal = res?.data?.data?.total || 0;
            setListApiError({ isError: false, message: "" });
            setListItems(nextList);
            setTotalNum(Number(nextTotal) || 0);
            return;
          }
          const msg = res?.data?.message || "查询失败";
          setListApiError({ isError: true, message: msg });
          setListItems([]);
          setTotalNum(0);
        })
        .catch(() => {
          setListApiError({ isError: true, message: "网络异常，请稍后重试" });
          setListItems([]);
          setTotalNum(0);
        });
    },
    [currentPage, pageSize, searchForm, fetchList],
  );

  useEffect(() => { refreshList(); }, [refreshList]);

  const handleFormChange = useCallback((values: any) => {
    setSearchForm(values || {});
    setCurrentPage(1);
  }, [setCurrentPage]);

  const handleAdd = () => {
    setModalMode("add");
    setModalInitial(null);
    setModalOpen(true);
  };

  const handleEdit = (row: any) => {
    setEditRow(row);
    setModalMode("edit");
    setModalInitial(row);
    setModalOpen(true);
  };

  const handleDelete = (row: any) => {
    setDeleteRow(row);
    setDeleteOpen(true);
  };

  const handleConfirmDelete = async () => {
    if (!deleteRow) return;
    const res = await fetchDelete({ url: `/material/${type}/delete/${deleteRow.id}` });
    if (res?.data?.code === 0) {
      showToast({ title: "删除成功", status: "success" });
      setDeleteOpen(false);
      setDeleteRow(null);
      refreshList({ ...searchForm, pageNum: 1, pageSize });
      return;
    }
    showToast({ title: res?.data?.message || "删除失败", status: "error" });
  };

  const handleSubmitModal = async ({ files, name, description }: any) => {
    if (modalMode === "add") {
      const formData = toFormData({ file: files, name, type, description });
      const res = await fetchAdd({ data: formData });
      if (res?.data?.code === 0) {
        showToast({ title: "新增成功", status: "success" });
        setModalOpen(false);
        refreshList({ ...searchForm, pageNum: 1, pageSize });
        return;
      }
      showToast({ title: res?.data?.message || "新增失败", status: "error" });
      return;
    }
    const id = modalInitial?.id;
    const res = await fetchUpdate({ url: `/material/${type}/update/${id}`, data: { name, description } });
    if (res?.data?.code === 0) {
      showToast({ title: "更新成功", status: "success" });
      setModalOpen(false);
      refreshList();
      return;
    }
    showToast({ title: res?.data?.message || "更新失败", status: "error" });
  };

  const itemActions = useMemo(
    () => (row: any) => (
      <Flex gap={2}>
        <Tooltip label="编辑" hasArrow>
          <IconButton aria-label="edit" icon={<EditIcon />} size="sm" variant="ghost"
            onClick={(e) => { e.stopPropagation(); handleEdit(row); }} />
        </Tooltip>
        <Tooltip label="删除" hasArrow>
          <IconButton aria-label="delete" icon={<DeleteIcon />} size="sm" variant="ghost"
            color="red.600" onClick={(e) => { e.stopPropagation(); handleDelete(row); }} />
        </Tooltip>
      </Flex>
    ),
    [],
  );

  return (
    <PageViewport className="thin-scrollbars">
      <PageContent py={{ base: 5, md: 6 }} bg="white" borderBottom="1px solid" borderColor="neutral.100">
        <Flex justify="space-between" align="center" wrap="wrap" gap={6}>
          <Box>
            <Heading fontSize="1.75rem" fontWeight="700" color="neutral.900">{typeName}</Heading>
            <Text color="neutral.500" fontSize="sm">素材管理</Text>
          </Box>
          <Flex gap={3}>
            <Button colorScheme="primary" leftIcon={<AddIcon />} shadow="md" onClick={handleAdd}>
              {addButtonLabel}
            </Button>
          </Flex>
        </Flex>
      </PageContent>

      <PageContent py={4}>
        <CommonForm formItems={formItems} formOptions={{}} onChange={handleFormChange} />

        <Card flex={1} overflow="hidden" shadow="sm" borderRadius="xl" bg="white" border="1px solid" borderColor="gray.100">
          <CardBody p={0} display="flex" flexDirection="column" h="full">
            <Flex px={4} py={3} align="center" borderBottom="1px solid" borderColor="gray.100">
              <Text fontSize="sm" fontWeight="semibold" color="gray.700">素材列表</Text>
              <Box flex={1} />
              <Text fontSize="xs" color="gray.400" mr={2}>共 {totalNum} 条</Text>
            </Flex>

            <Box flex={1} overflow="hidden" display="flex" flexDirection="column">
              <Box flex={1} overflowY="auto" p={4}>
                {listLoading ? (
                  <Text color="neutral.400" fontSize="sm" py={4}>加载中...</Text>
                ) : listItems.length ? (
                  <SimpleGrid
                    /* auto-fill + 有上限的轨道：条目少时卡片保持固定宽度，不会被拉满整行
                       （auto-fit + 1fr 会折叠空轨道并把剩余空间分给现有卡片） */
                    templateColumns="repeat(auto-fill, minmax(min(100%, 280px), 340px))"
                    alignItems="start"
                    justifyContent="start"
                    spacing={4}
                  >
                    {listItems.map((item: any) => (
                      <Card key={item.id} borderRadius="lg" shadow="sm" border="1px solid" borderColor="gray.100"
                        cursor="pointer" _hover={{ shadow: "md", borderColor: "primary.200" }}
                        onClick={() => router.push(`/material/${type}/${item.id}`)}>
                        <CardBody p={4}>
                          <Flex justify="space-between" align="flex-start" mb={2}>
                            <Text fontWeight="600" fontSize="sm" noOfLines={2} flex={1}>{item.name}</Text>
                            <Badge colorScheme="blue" variant="subtle" borderRadius="full" px={2} ml={2}>
                              {item.type_name || item.type}
                            </Badge>
                          </Flex>
                          <Text fontSize="xs" color="gray.500" noOfLines={2} mb={3}>
                            {item.description || "暂无描述"}
                          </Text>
                          <Flex justify="space-between" align="center">
                            <Text fontSize="xs" color="gray.400">{item.created_time || "-"}</Text>
                            <Flex gap={1}>{itemActions(item)}</Flex>
                          </Flex>
                        </CardBody>
                      </Card>
                    ))}
                  </SimpleGrid>
                ) : (
                  <Text color={listApiError.isError ? "red.500" : "neutral.400"} fontSize="sm" py={4}>
                    {listApiError.isError ? listApiError.message : "暂无数据"}
                  </Text>
                )}
              </Box>
            </Box>

            {pagesCount > 0 && (
              <TableFooter
                total={totalNum}
                pages={pages}
                pagesCount={pagesCount}
                currentPage={currentPage}
                setCurrentPage={setCurrentPage}
                pageSize={pageSize}
                setPageSize={setPageSize}
                handleCurrentPageChange={() => {}}
                hiddenTotal={false}
                hiddenSelect={false}
                hiddenInput={false}
                checkedFilesNum={0}
              />
            )}
          </CardBody>
        </Card>
      </PageContent>

      {/* ======== Add/Edit Modal ======== */}
      <AddEditModal
        isOpen={modalOpen} onClose={() => setModalOpen(false)}
        mode={modalMode} type={type} initial={modalInitial}
        fileAccept={fileAccept} uploadHint={uploadHint} descPlaceholder={descPlaceholder}
        onSubmit={handleSubmitModal} isSubmitting={modalMode === "add" ? addLoading : Boolean(updateLoading)}
      />

      <DeleteConfirmModal
        isOpen={deleteOpen} onClose={() => { setDeleteOpen(false); setDeleteRow(null); }}
        title="删除素材" description={<Text color="#000">确认删除素材{deleteRow?.name}？此操作不可恢复。</Text>}
        isLoading={Boolean(deleteLoading)} handleConfirm={handleConfirmDelete}
      />
    </PageViewport>
  );
}

/** Add/Edit Modal — inline component */
function AddEditModal({ isOpen, onClose, mode, type, initial, fileAccept, uploadHint, descPlaceholder, onSubmit, isSubmitting }: any) {
  const showToast = useCustomToast();
  const [files, setFiles] = useState<File[]>([]);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    if (!isOpen) return;
    setFiles([]);
    setName(initial?.name || "");
    setDescription(initial?.description || "");
  }, [isOpen, initial]);

  const handlePickFiles = (e: any) => {
    const next = Array.from(e?.target?.files || []) as File[];
    const { ok, msg } = validateFiles(next);
    if (!ok) { showToast({ title: msg, status: "error" }); e.target.value = ""; return; }
    setFiles(next);
  };

  const handleRemoveFile = (idx: number) => setFiles((prev: File[]) => prev.filter((_, i) => i !== idx));

  const nameRegex = /^[一-龥a-zA-Z0-9_-]+$/;
  const handleConfirm = async () => {
    if (!name.trim()) return showToast({ title: "请输入素材名称", status: "error" });
    if (!nameRegex.test(name.trim())) return showToast({ title: "素材名称仅支持汉字、字母、数字、中划线、下划线", status: "error" });
    if (description && description.length > 1000) return showToast({ title: "描述最多1000字", status: "error" });
    if (mode === "add" && !files.length) return showToast({ title: "请选择文件", status: "error" });
    await onSubmit({ files, name: name.trim(), description });
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} isCentered returnFocusOnClose={false} size="xl">
      <ModalOverlay />
      <ModalContent>
        <ModalHeader>{mode === "add" ? "新增素材" : "编辑素材"}</ModalHeader>
        <ModalCloseButton />
        <ModalBody>
          {mode === "add" && (
            <FormControl mb={4}>
              <FormLabel>文件上传</FormLabel>
              <input ref={fileInputRef} type="file" multiple accept={fileAccept}
                style={{ display: "none" }} onChange={handlePickFiles} />
              <Flex direction="column" align="center" justify="center" h="9rem"
                border="1px dashed" borderColor="blue.200" bg="blue.50" borderRadius="xl"
                cursor="pointer" onClick={() => fileInputRef.current?.click?.()} gap={2}>
                <Flex w="10" h="10" borderRadius="full" align="center" justify="center" bg="white" boxShadow="sm">
                  <Image src="/images/upload.png" alt="upload" boxSize="6" />
                </Flex>
                <Text fontSize="md" fontWeight="semibold" color="primary.600">点击上传</Text>
                <Text fontSize="xs" color="gray.500">{uploadHint}</Text>
              </Flex>
              {!!files.length && (
                <Box mt={3} border="1px solid" borderColor="gray.100" borderRadius="md">
                  {files.map((f: File, idx: number) => (
                    <Flex key={`${f.name}-${idx}`} px={3} py={2} align="center" justify="space-between"
                      borderBottom={idx === files.length - 1 ? "none" : "1px solid"} borderColor="gray.100">
                      <Text fontSize="sm" color="gray.800" noOfLines={1}>{f.name}</Text>
                      <IconButton aria-label="remove" icon={<CloseIcon />} size="sm" variant="ghost"
                        onClick={() => handleRemoveFile(idx)} />
                    </Flex>
                  ))}
                </Box>
              )}
            </FormControl>
          )}

          <FormControl mb={4}>
            <FormLabel mb="0" minW="6rem">素材名称</FormLabel>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="请输入素材名称" />
          </FormControl>

          <FormControl mb={4}>
            <FormLabel>描述</FormLabel>
            <Textarea value={description} onChange={(e) => setDescription(e.target.value)}
              placeholder={descPlaceholder} resize="vertical" maxLength={1000} />
            <Flex justify="flex-end">
              <Text fontSize="xs" color="gray.500">{(description || "").length}/1000</Text>
            </Flex>
          </FormControl>
        </ModalBody>
        <ModalFooter>
          <Button mr={3} variant="outline" onClick={onClose}>取消</Button>
          <Button colorScheme="primary" onClick={handleConfirm} isLoading={isSubmitting}>确认</Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
}
