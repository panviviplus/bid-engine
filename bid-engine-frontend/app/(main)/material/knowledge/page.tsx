"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Box,
  Button,
  Flex,
  Heading,
  Text,
  Card,
  CardBody,
  IconButton,
  Tooltip,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  ModalCloseButton,
  FormControl,
  FormLabel,
  Input,
  Image,
  Textarea,
  RadioGroup,
  Radio,
  Stack,
  SimpleGrid,
  ButtonGroup,
  Badge,
} from "@chakra-ui/react";
import { PageViewport } from "@/components/layout/responsive-page";
import { AddIcon, DeleteIcon, EditIcon, ViewIcon, CloseIcon, HamburgerIcon } from "@chakra-ui/icons";
import GridIcon from "@/components/common/grid-icon";
import { debounce } from "lodash";
import { useRouter, useSearchParams } from "next/navigation";
import { usePagination } from "@ajna/pagination";

import CommonForm from "@/components/common-form";
import TableWithCheckbox from "@/components/common/table-with-checkbox";
import TableFooter from "@/components/common/table-footer";
import ColumnConfig from "@/components/common/column-config";
import DeleteConfirmModal from "@/components/common/delete-confirm-modal";
import { useCustomToast } from "@/hooks/useCustomToast";
import { useAppContext } from "@/contexts/app-context";
import { formatOptions, toFormData } from "@/utils";
import type { FormItemType } from "@/components/common-form";

import {
  useMaterialTypes,
  useMaterialCompanies,
  useMaterialUsers,
  useMaterialList,
  useMaterialAdd,
  useMaterialUpdate,
  useMaterialDelete,
} from "@/service/material";

const TABLE_HEADERS = [
  { label: "序号", fixed: "left", width: "80px" },
  { label: "素材名称", width: "220px" },
  { label: "类型" },
  { label: "描述" },
  { label: "文件数量" },
  { label: "OCR", width: "80px" },
  { label: "所属公司" },
  { label: "创建人" },
  { label: "创建日期" },
  { label: "操作", fixed: "right", width: "150px" },
];

const MANDATORY_HEADERS = ["序号", "素材名称", "类型", "操作"];

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
    const ext = `.${f.name.split(".").pop() || ""}`;
    const g = fileGroupByExt(ext);
    if (!g) return { ok: false, msg: "不支持的文件类型" };
    if (g === "image" && f.size > 5 * 1024 * 1024) return { ok: false, msg: "图片大小不能超过5MB" };
    if (g === "doc" && f.size > 200 * 1024 * 1024) return { ok: false, msg: "文档大小不能超过200MB" };
  }
  return { ok: true, msg: "" };
}

function MaterialModal({
  isOpen,
  onClose,
  mode,
  typeOptions,
  initial,
  onSubmit,
  isSubmitting,
}: any) {
  const showToast = useCustomToast();
  const [files, setFiles] = useState<File[]>([]);
  const [name, setName] = useState("");
  const [type, setType] = useState("");
  const [description, setDescription] = useState("");
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    if (!isOpen) return;
    setFiles([]);
    setName(initial?.name || "");
    setType(initial?.type || "");
    setDescription(initial?.description || "");
  }, [isOpen, initial]);

  const fileAccept = useMemo(() => {
    if (type === "qualification") return ".jpg,.jpeg,.png,.pdf,.doc,.docx";
    if (type === "performance") return ".jpg,.jpeg,.png,.pdf,.doc,.docx";
    return ".pdf,.doc,.docx";
  }, [type]);

  const uploadHint = useMemo(() => {
    if (type === "qualification")
      return { main: "请上传营业执照、资质证书、许可证等", format: "支持：JPG/JPEG/PNG（≤5MB），PDF/DOC/DOCX（≤200MB）" };
    if (type === "performance")
      return { main: "请上传包含业绩信息的文件", format: "支持：JPG/JPEG/PNG（≤5MB），PDF/DOC/DOCX（≤200MB）" };
    return { main: "请上传投标文件模板", format: "支持：PDF/DOC/DOCX（≤200MB）" };
  }, [type]);

  const descPlaceholder = useMemo(() => {
    if (type === "qualification")
      return "例：ISO9001 质量管理体系认证，由XX机构颁发，有效期至 2028 年";
    if (type === "performance")
      return "例：XX智慧城市项目，合同金额 500 万元，2024 年 6 月完成";
    return "例：技术方案模板，适用于 XX 行业";
  }, [type]);

  const handlePickFiles = (e: any) => {
    const next = Array.from(e?.target?.files || []) as File[];
    const { ok, msg } = validateFiles(next);
    if (!ok) {
      showToast({ title: msg, status: "error" });
      e.target.value = "";
      return;
    }
    setFiles(next);
  };

  const handleRemoveFile = (idx: number) => {
    setFiles((prev) => prev.filter((_, i) => i !== idx));
  };

  const handleConfirm = async () => {
    if (!name.trim()) return showToast({ title: "请输入素材名称", status: "error" });
    if (!/^[一-龥a-zA-Z0-9_-]+$/.test(name.trim())) return showToast({ title: "素材名称仅支持汉字、字母、数字、中划线、下划线", status: "error" });
    if (!type) return showToast({ title: "请选择素材类型", status: "error" });
    if (description && description.length > 1000) return showToast({ title: "描述最多1000字", status: "error" });
    if (mode === "add" && !files.length) return showToast({ title: "请选择文件", status: "error" });
    const { ok, msg } = validateFiles(files);
    if (!ok) return showToast({ title: msg, status: "error" });
    await onSubmit({ files, name: name.trim(), type, description });
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} isCentered returnFocusOnClose={false} size="xl">
      <ModalOverlay />
      <ModalContent maxW="720px">
        <ModalHeader>{mode === "add" ? "新增素材" : "编辑素材"}</ModalHeader>
        <ModalCloseButton />
        <ModalBody>
          {mode === "add" && (
            <FormControl isRequired mb={4}>
              <FormLabel>文件上传</FormLabel>
              <Input
                ref={fileInputRef}
                type="file"
                multiple
                accept={fileAccept}
                display="none"
                onChange={handlePickFiles}
              />
              <Box
                border="1px dashed"
                borderColor="blue.200"
                borderRadius="xl"
                p={4}
                bg="blue.50"
                cursor="pointer"
                onClick={() => fileInputRef.current?.click?.()}
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
                  <Text fontSize="xs" color="gray.400">最多5个文件</Text>
                  <Text fontSize="xs" color="gray.400">{uploadHint.main}</Text>
                  <Text fontSize="xs" color="gray.400">{uploadHint.format}</Text>
                </Flex>
              </Box>
              {!!files.length && (
                <Box mt={3} border="1px solid" borderColor="gray.100" borderRadius="md">
                  {files.map((f, idx) => (
                    <Flex key={`${f.name}-${idx}`} px={3} py={2} align="center" justify="space-between" borderBottom={idx === files.length - 1 ? "none" : "1px solid"} borderColor="gray.100">
                      <Box>
                        <Text fontSize="sm" color="gray.800" noOfLines={1}>{f.name}</Text>
                        <Text fontSize="xs" color="gray.500">{bytesToSize(f.size)}</Text>
                      </Box>
                      <IconButton
                        aria-label="remove"
                        icon={<CloseIcon />}
                        size="sm"
                        variant="ghost"
                        onClick={() => handleRemoveFile(idx)}
                      />
                    </Flex>
                  ))}
                </Box>
              )}
            </FormControl>
          )}

          <FormControl isRequired mb={4}>
            <Flex align="center" gap={1}>
              <FormLabel mb="0" minW="6rem">素材名称</FormLabel>
              <Input flex={1} value={name} onChange={(e) => setName(e.target.value)} placeholder="请输入素材名称" />
            </Flex>
          </FormControl>

          <FormControl isRequired mb={4}>
            <Flex align="center" gap={1}>
              <FormLabel mb="0" minW="6rem">素材类型</FormLabel>
              <RadioGroup flex={1} value={type} onChange={setType}>
                <Stack direction="row" spacing={6}>
                  {typeOptions.map((t: any) => (
                    <Radio key={t.type} value={t.type}>{t.name}</Radio>
                  ))}
                </Stack>
              </RadioGroup>
            </Flex>
          </FormControl>

          <FormControl mb={2}>
            <FormLabel>描述</FormLabel>
            <Textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder={descPlaceholder}
              resize="vertical"
              maxLength={1000}
            />
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

function RenderColumnOperate({ row, onEdit, onDelete }: any) {
  const router = useRouter();
  return (
    <Flex gap={1} align="center">
      <Tooltip label="查看" hasArrow>
        <span>
          <IconButton
            aria-label="view"
            icon={<ViewIcon />}
            size="sm"
            variant="ghost"
            color="primary.600"
            onClick={() => router.push(`/material/knowledge/${row.id}`)}
          />
        </span>
      </Tooltip>
      <Tooltip label="编辑" hasArrow>
        <span>
          <IconButton
            aria-label="edit"
            icon={<EditIcon />}
            size="sm"
            variant="ghost"
            color="primary.600"
            onClick={() => onEdit(row)}
          />
        </span>
      </Tooltip>
      <Tooltip label="删除" hasArrow>
        <span>
          <IconButton
            aria-label="delete"
            icon={<DeleteIcon />}
            size="sm"
            variant="ghost"
            color="red.600"
            onClick={() => onDelete(row)}
          />
        </span>
      </Tooltip>
    </Flex>
  );
}

export default function KnowledgePage() {
  const showToast = useCustomToast();
  const { userProfile } = useAppContext();
  const isAdmin = !!(userProfile as any)?.isAdmin;

  const [viewMode, setViewMode] = useState<"table" | "card">("card");

  const { currentPage, setCurrentPage, pageSize, setPageSize, pagesCount, pages } = usePagination({
    total: 0,
    limits: { outer: 1, inner: 1 },
    initialState: { currentPage: 1, pageSize: 10 },
  });

  const [searchForm, setSearchForm] = useState<any>({});
  const [totalNum, setTotalNum] = useState(0);
  const [listItems, setListItems] = useState<any[]>([]);
  const [listApiError, setListApiError] = useState({ isError: false, message: "" });

  const searchParams = useSearchParams();
  const pageType = searchParams.get("type") || "";

  const addButtonLabel = useMemo(() => {
    if (pageType === "qualification") return "新增资质";
    if (pageType === "performance") return "新增业绩";
    return "新增素材";
  }, [pageType]);

  const { typeOptions, fetchTypes } = useMaterialTypes();
  const { companyOptions, fetchCompanies } = useMaterialCompanies();
  const { userOptions, fetchUsers } = useMaterialUsers();

  const { fetchList, listLoading } = useMaterialList();
  const { addLoading, fetchAdd } = useMaterialAdd();
  const { updateLoading, fetchUpdate } = useMaterialUpdate();
  const { deleteLoading, fetchDelete } = useMaterialDelete();

  const [visibleLabels, setVisibleLabels] = useState<string[]>(() => {
    const defaults = TABLE_HEADERS.map((h) => h.label);
    return defaults.filter((l) => !["所属公司", "创建人"].includes(l));
  });

  const [modalMode, setModalMode] = useState<"add" | "edit">("add");
  const [modalOpen, setModalOpen] = useState(false);
  const [modalInitial, setModalInitial] = useState<any>(null);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleteRow, setDeleteRow] = useState<any>(null);

  const formOptions = useMemo(() => {
    const options: any = {
      type: formatOptions(typeOptions || [], "name", "type"),
    };
    if (isAdmin) {
      options.companyId = formatOptions(companyOptions || [], "name", "company_id");
      options.creatorId = formatOptions(userOptions || [], "name", "user_id");
    }
    return options;
  }, [typeOptions, companyOptions, userOptions, isAdmin]);

  const formItems: FormItemType[] = useMemo(() => {
    const items: FormItemType[] = [
      {
        name: "keyword",
        placeholder: "按名称、描述模糊筛选",
        type: "input",
        icon: true,
        props: { mr: 4, mb: 3, borderRadius: "full", h: "9", inputw: "18rem" },
      },
      {
        name: "type",
        placeholder: "素材类型",
        type: "select",
        props: { mr: 4, mb: 3, borderRadius: "full", h: "9", w: "12rem" },
      },
    ];
    if (isAdmin) {
      items.push(
        // { name: "companyId", placeholder: "企业名称", type: "select", props: { mr: 4, mb: 3, borderRadius: "full", h: "9", w: "12rem" } },
        { name: "creatorId", placeholder: "创建人", type: "select", props: { mr: 4, mb: 3, borderRadius: "full", h: "9", w: "12rem" } },
      );
    }
    return items;
  }, [isAdmin]);

  useEffect(() => {
    fetchTypes();
    if (isAdmin) {
      fetchCompanies();
      fetchUsers();
    }
  }, [fetchTypes, fetchCompanies, fetchUsers, isAdmin]);

  const fetchListRef = useRef(fetchList);
  useEffect(() => {
    fetchListRef.current = fetchList;
  }, [fetchList]);

  const refreshList = useCallback(
    (override?: any) => {
      return fetchListRef.current({
        params: {
          pageNum: currentPage,
          pageSize,
          ...(override || searchForm),
        },
      })
        .then((res: any) => {
          const code = res?.data?.code;
          if (code === 0) {
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
    [currentPage, pageSize, searchForm],
  );

  useEffect(() => {
    refreshList();
  }, [refreshList]);

  const displayHeaders = useMemo(() => {
    const headers = TABLE_HEADERS.filter((h) => visibleLabels.includes(h.label) || MANDATORY_HEADERS.includes(h.label));
    const opIndex = headers.findIndex((h) => h.label === "操作");
    if (opIndex >= 0) {
      const next = { ...headers[opIndex] } as any;
      next.headerRender = () => (
        <Flex align="center" justify="space-between" width="100%">
          <Text>操作</Text>
          <ColumnConfig
            allHeaders={TABLE_HEADERS}
            visibleLabels={visibleLabels}
            onChange={setVisibleLabels}
            mandatoryLabels={MANDATORY_HEADERS}
          />
        </Flex>
      );
      headers[opIndex] = next;
    }
    return headers;
  }, [visibleLabels]);

  const handleEdit = useCallback((row: any) => {
    setModalMode("edit");
    setModalInitial(row);
    setModalOpen(true);
  }, []);

  const handleDelete = useCallback((row: any) => {
    setDeleteRow(row);
    setDeleteOpen(true);
  }, []);

  const tableData = useMemo(() => {
    return (listItems || []).map((item: any, index: number) => ({
      序号: (currentPage - 1) * pageSize + index + 1,
      素材名称: item.name,
      类型: item.type_name || item.type,
      描述: item.description || "",
      文件数量: item.file_count || 0,
      所属公司: item.company_name || "-",
      创建人: item.user_name || "-",
      创建日期: item.created_time || "",
      操作: (
        <RenderColumnOperate
          row={item}
          onEdit={handleEdit}
          onDelete={handleDelete}
        />
      ),
    }));
  }, [listItems, currentPage, pageSize, handleEdit, handleDelete]);

  const handleFormChange = useCallback((values: any) => {
    setSearchForm(values || {});
    setCurrentPage(1);
  }, [setCurrentPage]);

  const handleAdd = () => {
    setModalMode("add");
    setModalInitial(pageType ? { type: pageType } : null);
    setModalOpen(true);
  };

  const handleSubmitModal = async ({ files, name, type, description }: any) => {
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
    const res = await fetchUpdate({ url: pageType ? `/material/${pageType}/update/${id}` : `/material/update/${id}`, data: { name, description } });
    if (res?.data?.code === 0) {
      showToast({ title: "更新成功", status: "success" });
      setModalOpen(false);
      refreshList();
      return;
    }
    showToast({ title: res?.data?.message || "更新失败", status: "error" });
  };

  const handleConfirmDelete = async () => {
    const id = deleteRow?.id;
    const res = await fetchDelete({ url: pageType ? `/material/${pageType}/delete/${id}` : `/material/delete/${id}` });
    if (res?.data?.code === 0) {
      showToast({ title: "删除成功", status: "success" });
      setDeleteOpen(false);
      setDeleteRow(null);
      refreshList();
      return;
    }
    showToast({ title: res?.data?.message || "删除失败", status: "error" });
  };

  return (
    <PageViewport scroll={false}>
      <Flex
        h="full"
        minH={0}
        direction="column"
        bg="transparent"
        p={{ base: 3, md: 6, xl: 8, "2xl": 10 }}
      >
        <Flex
          justify="space-between"
          align={{ base: "stretch", lg: "center" }}
          direction={{ base: "column", lg: "row" }}
          gap={4}
          mb={6}
        >
          <Box flex={1}>
            <CommonForm
              isClearBtn={false}
              formItems={formItems}
              formOptions={formOptions}
              onChange={debounce(handleFormChange, 600)}
              isLoading={listLoading}
              props={{ mb: -4 }}
            />
          </Box>
          <Flex align="center" gap={3} wrap="wrap">
            <ButtonGroup isAttached variant="outline" size="sm">
              <Tooltip label="列表视图" hasArrow>
                <IconButton
                  aria-label="列表视图"
                  icon={<HamburgerIcon />}
                  onClick={() => setViewMode("table")}
                  isActive={viewMode === "table"}
                />
              </Tooltip>
              <Tooltip label="卡片视图" hasArrow>
                <IconButton
                  aria-label="卡片视图"
                  icon={<GridIcon />}
                  onClick={() => setViewMode("card")}
                  isActive={viewMode === "card"}
                />
              </Tooltip>
            </ButtonGroup>
            <Button colorScheme="primary" leftIcon={<AddIcon />} shadow="md" onClick={handleAdd}>
              {addButtonLabel}
            </Button>
          </Flex>
        </Flex>

        <Card flex={1} overflow="hidden" shadow="sm" borderRadius="xl" bg="white"  border="1px solid" borderColor="gray.100">
          <CardBody p={0} display="flex" flexDirection="column" h="full">
            <Box flex={1} overflow="hidden" display="flex" flexDirection="column">
              {viewMode === "table" ? (
                <TableWithCheckbox
                  wordsLibrary="true"
                  headers={displayHeaders}
                  isLoading={listLoading}
                  isError={listApiError.isError}
                  total={totalNum}
                  bodyData={tableData}
                  pages={pages}
                  pageSize={pageSize}
                  pagesCount={pagesCount}
                  setPageSize={setPageSize}
                  currentPage={currentPage}
                  setCurrentPage={setCurrentPage}
                  checkedItems={[]}
                  setCheckedItems={() => {}}
                  useCheckBox={false}
                  minBodyRows={5}
                  errorMessage={listApiError.message}
                  renderHeaderButton=""
                  onChangeChecked=""
                />
              ) : (
                <Box flex={1} overflowY="auto" p={4} className="thin-scrollbars">
                  <SimpleGrid
                    minChildWidth={{ base: "100%", md: "18rem", xl: "20rem" }}
                    spacing={4}
                  >
                    {(listItems || []).map((item: any) => (
                      <Card
                        key={item.id}
                        border="1px solid"
                        borderColor="gray.100"
                        borderRadius="xl"
                        shadow="sm"
                        bg="white"
                        _hover={{ shadow: "md", transform: "translateY(-2px)" }}
                        transition="all 0.2s"
                      >
                        <CardBody p={4}>
                          <Flex direction="column" gap={3}>
                            <Flex justify="space-between" align="start">
                              <Tooltip label={item.name} hasArrow>
                                <Text fontWeight="bold" fontSize="md" noOfLines={1} color="gray.800" flex={1}>
                                  {item.name}
                                </Text>
                              </Tooltip>
                              <Badge colorScheme="blue" variant="subtle" borderRadius="full" px={2}>
                                {item.type_name || item.type}
                              </Badge>
                            </Flex>
                            
                            <Text fontSize="sm" color="gray.500" noOfLines={2} minH="44px" lineHeight="1.5">
                              {item.description || "暂无描述"}
                            </Text>

                            <Flex justify="space-between" align="center" fontSize="xs" color="gray.400">
                              <Text>文件: {item.file_count || 0}</Text>
                              <Text>{item.created_time?.split(" ")[0]}</Text>
                            </Flex>

                            <Flex justify="flex-end" pt={1} mt={1} borderTop="1px solid" borderColor="gray.50" gap={2}>
                              <RenderColumnOperate
                                row={item}
                                onEdit={handleEdit}
                                onDelete={handleDelete}
                              />
                            </Flex>
                          </Flex>
                        </CardBody>
                      </Card>
                    ))}
                  </SimpleGrid>
                  {pagesCount > 0 && (
                    <Box mt={4}>
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
                    </Box>
                  )}
                </Box>
              )}
            </Box>
          </CardBody>
        </Card>
      </Flex>

      <MaterialModal
        isOpen={modalOpen}
        onClose={() => setModalOpen(false)}
        mode={modalMode}
        typeOptions={typeOptions || []}
        initial={modalInitial}
        onSubmit={handleSubmitModal}
        isSubmitting={modalMode === "add" ? addLoading : updateLoading}
      />

      <DeleteConfirmModal
        isOpen={deleteOpen}
        onClose={() => setDeleteOpen(false)}
        title={deleteRow ? "确认要删除该素材吗？" : ""}
        description={
          <Box>
            <Text color="#000">确认要删除该素材吗？</Text>
            <Text color="red.500" mt={2}>注意：删除操作是不可逆的，确认后将无法恢复。</Text>
          </Box>
        }
        isLoading={deleteLoading}
        handleConfirm={handleConfirmDelete}
      />
    </PageViewport>
  );
}
