"use client";

import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  Badge,
  Box,
  Button,
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
  Skeleton,
  Text,
  Textarea,
  Tooltip,
  useDisclosure,
} from "@chakra-ui/react";
import { PageViewport } from "@/components/layout/responsive-page";
import {
  ModuleWorkbenchDeck,
  ModuleWorkbenchHeader,
} from "@/components/common/module-workbench.mjs";
import {
  DataSurface,
  WorkspaceShell,
} from "@/components/analysis/bid-analysis-v3/workspace";
import {
  FiAlertTriangle,
  FiCheck,
  FiEye,
  FiFilter,
  FiInbox,
  FiPlus,
  FiRotateCcw,
  FiTrash2,
} from "react-icons/fi";
import InfiniteScrollList from "@/components/common/infinite-scroll";
import { debounce } from "lodash";
import { env } from "next-runtime-env";

import CommonForm from "@/components/common-form";
import DeleteConfirmModal from "@/components/common/delete-confirm-modal";
import { CompactFeedbackCardFrame } from "@/components/file-feedback/compact-card-frame.mjs";
import { FeedbackCardGrid } from "@/components/file-feedback/feedback-card-grid.mjs";
import { resolveFeedbackStatusMeta } from "@/components/file-feedback/feedback-status.mjs";
import { resolveFeedbackCardViewState } from "@/components/file-feedback/card-view-state.mjs";
import { useInfiniteList } from "@/hooks/use-infinite-list";
import { useCustomToast } from "@/hooks/useCustomToast";
import { useAppContext } from "@/contexts/app-context";
import { FEEDBACK_TYPE } from "@/utils/constants";
import {
  useAddFeedbackRecord,
  useDeleteFeedbackRecord,
  useFeedbackOptions,
  useFeedbackRecords,
  useGetFeedbackRecord,
  useUpdateFeedbackStatus,
} from "@/service/file-feedback";

type FeedbackRecordItem = {
  id: number;
  userName: string;
  mobile: string;
  type: string;
  description: string;
  createTime: string;
  status: number | string;
};

const FEEDBACK_PAGE_SIZE = 12;
const FEEDBACK_SCROLL_ID = "feedback-list-scroll";

function buildTypeOptions(types: string[]) {
  const map = new Map<string, string>();
  FEEDBACK_TYPE.forEach((t) => map.set(t.value, t.label));
  return (types || []).map((t) => ({ label: map.get(t) || t, value: t }));
}

function getTypeLabel(type: string) {
  const found = FEEDBACK_TYPE.find((t) => t.value === type);
  return found?.label || type;
}

/** 卡片操作按钮共用焦点环：2px、金色、2px offset，键盘导航时立即可见 */
const CARD_ACTION_FOCUS = {
  outline: "2px solid",
  outlineColor: "gold.400",
  outlineOffset: "2px",
} as const;

/** 视觉尺寸 32px，命中区通过 ::before 扩到 44px，触屏可点且不撑高行 */
const CARD_ACTION_HIT_AREA = {
  content: '""',
  position: "absolute",
  inset: "-6px",
  borderRadius: "10px",
} as const;

function StatusToggleButton({
  isProcessed,
  onClick,
  isLoading,
}: {
  isProcessed: boolean;
  onClick: () => void;
  isLoading: boolean;
}) {
  return (
    <Button
      size="sm"
      h="32px"
      px={3}
      borderRadius="8px"
      position="relative"
      _before={CARD_ACTION_HIT_AREA}
      fontSize="xs"
      fontWeight="700"
      bg={isProcessed ? "neutral.100" : "success.50"}
      color={isProcessed ? "neutral.700" : "success.700"}
      border="1px solid"
      borderColor={isProcessed ? "neutral.200" : "success.200"}
      leftIcon={isProcessed ? <FiRotateCcw size={13} /> : <FiCheck size={13} />}
      isLoading={isLoading}
      loadingText={isProcessed ? "恢复中" : "提交中"}
      onClick={onClick}
      aria-pressed={isProcessed}
      _hover={{ bg: isProcessed ? "neutral.200" : "success.100" }}
      _active={{
        bg: isProcessed ? "neutral.200" : "success.100",
        transform: "translateY(1px)",
      }}
      _disabled={{ opacity: 0.6, cursor: "not-allowed" }}
      _focusVisible={CARD_ACTION_FOCUS}
    >
      {isProcessed ? "恢复待处理" : "标记已处理"}
    </Button>
  );
}

function buildImageUrl(objectKey: string) {
  const base = env("NEXT_PUBLIC_SYSTEM_SERVER") || "";
  const q = encodeURIComponent(objectKey || "");
  return `${base}/feedback/image?objectKey=${q}`;
}

function AddFeedbackModal({
  isOpen,
  onClose,
  typeOptions,
  onSuccess,
}: {
  isOpen: boolean;
  onClose: () => void;
  typeOptions: Array<{ label: string; value: string }>;
  onSuccess: () => void;
}) {
  const showToast = useCustomToast();
  const { addRecordLoading, fetchAddFeedbackRecord } = useAddFeedbackRecord();

  const [type, setType] = useState<string>("");
  const [description, setDescription] = useState<string>("");
  const [files, setFiles] = useState<Array<{ file: File; url: string }>>([]);
  const inputRef = useRef<HTMLInputElement | null>(null);

  const reset = useCallback(() => {
    setType("");
    setDescription("");
    setFiles((prev) => {
      prev.forEach((it) => URL.revokeObjectURL(it.url));
      return [];
    });
  }, []);

  useEffect(() => {
    if (!isOpen) reset();
  }, [isOpen, reset]);

  const validateAndAppendFiles = useCallback(
    (incoming: File[]) => {
      const next = [...files];
      for (const f of incoming) {
        if (!f) continue;
        const extOk = /image\/(png|jpeg|jpg)/i.test(f.type);
        if (!extOk) {
          showToast({ title: "仅支持 PNG/JPG/JPEG 图片", status: "error" });
          continue;
        }
        if (f.size > 5 * 1024 * 1024) {
          showToast({ title: "单张图片不能超过 5MB", status: "error" });
          continue;
        }
        if (next.length >= 5) {
          showToast({ title: "最多上传 5 张图片", status: "error" });
          break;
        }
        next.push({ file: f, url: URL.createObjectURL(f) });
      }
      setFiles(next);
    },
    [files, showToast],
  );

  const handlePaste = useCallback(
    (e: React.ClipboardEvent<HTMLTextAreaElement>) => {
      const items = e.clipboardData?.items || [];
      const pasted: File[] = [];
      for (let i = 0; i < items.length; i += 1) {
        const item = items[i];
        if (item.kind !== "file") continue;
        const f = item.getAsFile();
        if (!f) continue;
        if (!/^image\//i.test(f.type)) continue;
        pasted.push(f);
      }
      if (pasted.length) {
        validateAndAppendFiles(pasted);
      }
    },
    [validateAndAppendFiles],
  );

  const handlePickFiles = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const selected = Array.from(e.target.files || []);
      if (selected.length) validateAndAppendFiles(selected);
      e.target.value = "";
    },
    [validateAndAppendFiles],
  );

  const handleSubmit = async () => {
    if (!type) {
      showToast({ title: "请选择反馈类型", status: "error" });
      return;
    }
    if (!description.trim()) {
      showToast({ title: "请描述您的反馈内容", status: "error" });
      return;
    }

    const form = new FormData();
    form.append("type", type);
    form.append("description", description.trim());
    files.forEach((it) => form.append("file", it.file));

    const res = await fetchAddFeedbackRecord({
      data: form,
      headers: { "Content-Type": "multipart/form-data" },
    });
    if (res?.data?.code === 0) {
      showToast({
        title: "反馈已提交，当前状态为待处理",
        status: "success",
      });
      onClose();
      onSuccess();
      return;
    }
    showToast({ title: res?.data?.message || "提交失败", status: "error" });
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      isCentered
      returnFocusOnClose={false}
    >
      <ModalOverlay bg="blackAlpha.600" />
      <ModalContent
        w={{ base: "calc(100% - 1.5rem)", md: "46rem" }}
        maxW={{ base: "calc(100% - 1.5rem)", md: "46rem" }}
        maxH="calc(100dvh - 2rem)"
        borderRadius="14px"
        overflow="hidden"
        bg="workbench.canvas"
      >
        <ModalHeader
          py={5}
          bg="workbench.control"
          color="workbench.paper"
          borderBottom="1px solid"
          borderColor="workbench.controlRaised"
        >
          <Text fontSize="lg" fontWeight="700">
            提交反馈
          </Text>
          <Text mt={1} fontSize="xs" fontWeight="400" color="whiteAlpha.700">
            说明问题发生的位置和现象，截图可帮助更快定位
          </Text>
        </ModalHeader>
        <ModalCloseButton color="white" boxSize={12} />
        <ModalBody py={5}>
          <Flex direction="column" gap={4}>
            <Box>
              <Text
                fontSize="sm"
                color="workbench.text"
                mb={2}
                fontWeight="600"
              >
                反馈类型
              </Text>
              <Flex gap={3} wrap="wrap">
                {typeOptions.map((opt) => (
                  <Button
                    key={opt.value}
                    size="sm"
                    variant={type === opt.value ? "solid" : "outline"}
                    colorScheme={type === opt.value ? "primary" : "gray"}
                    minH="44px"
                    borderRadius="9px"
                    onClick={() => setType(opt.value)}
                  >
                    {opt.label}
                  </Button>
                ))}
              </Flex>
            </Box>

            <Box>
              <Text
                fontSize="sm"
                color="workbench.text"
                mb={2}
                fontWeight="600"
              >
                描述
              </Text>
              <Textarea
                placeholder="请描述您的反馈内容（支持粘贴图片）"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                onPaste={handlePaste}
                minH="140px"
                resize="vertical"
                bg="workbench.paper"
                borderColor="workbench.line"
                borderRadius="10px"
                _focusVisible={{
                  borderColor: "workbench.control",
                  boxShadow: "none",
                  outline: "2px solid",
                  outlineColor: "gold.400",
                  outlineOffset: "2px",
                }}
              />
            </Box>

            <Box>
              <Flex align="center" justify="space-between" mb={2}>
                <Text fontSize="sm" color="workbench.text" fontWeight="600">
                  图片上传
                </Text>
                <Button
                  size="sm"
                  minH="44px"
                  variant="outline"
                  colorScheme="gray"
                  onClick={() => inputRef.current?.click()}
                  isDisabled={files.length >= 5}
                >
                  选择图片
                </Button>
                <input
                  ref={inputRef}
                  type="file"
                  accept="image/png,image/jpeg,image/jpg"
                  multiple
                  onChange={handlePickFiles}
                  style={{ display: "none" }}
                />
              </Flex>
              <Text fontSize="xs" color="workbench.muted" mb={3}>
                最多 5 张，单张不超过 5MB，支持 PNG/JPG/JPEG
              </Text>
              {files.length > 0 && (
                <SimpleGrid columns={{ base: 2, md: 5 }} spacing={3}>
                  {files.map((f, idx) => {
                    return (
                      <Box
                        key={`${f.file.name}-${idx}`}
                        position="relative"
                        borderRadius="md"
                        overflow="hidden"
                        border="1px solid"
                        borderColor="gray.100"
                      >
                        <Image
                          src={f.url}
                          alt={f.file.name}
                          w="full"
                          h="90px"
                          objectFit="cover"
                        />
                        <IconButton
                          aria-label="remove"
                          icon={<FiTrash2 />}
                          size="xs"
                          variant="solid"
                          colorScheme="red"
                          position="absolute"
                          top={1}
                          right={1}
                          onClick={() => {
                            setFiles((prev) => {
                              const removed = prev[idx];
                              if (removed) URL.revokeObjectURL(removed.url);
                              return prev.filter((_, i) => i !== idx);
                            });
                          }}
                        />
                      </Box>
                    );
                  })}
                </SimpleGrid>
              )}
            </Box>
          </Flex>
        </ModalBody>
        <ModalFooter
          borderTop="1px solid"
          borderColor="workbench.line"
          bg="workbench.paper"
        >
          <Button mr={3} variant="outline" colorScheme="gray" onClick={onClose}>
            取消
          </Button>
          <Button
            minH="44px"
            bg="workbench.control"
            color="workbench.paper"
            _hover={{ bg: "workbench.controlRaised" }}
            onClick={handleSubmit}
            isLoading={addRecordLoading}
          >
            提交反馈
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
}

function FeedbackDetailModal({
  isOpen,
  onClose,
  recordId,
}: {
  isOpen: boolean;
  onClose: () => void;
  recordId: number | null;
}) {
  const showToast = useCustomToast();
  const { recordDetail, recordDetailLoading, fetchGetFeedbackRecord } =
    useGetFeedbackRecord();

  useEffect(() => {
    if (!isOpen || !recordId) return;
    const controller = new AbortController();
    fetchGetFeedbackRecord({
      url: `/feedback/record/${recordId}`,
      signal: controller.signal,
    })
      .then((res) => {
        if (res?.data?.code !== 0) {
          showToast({
            title: res?.data?.message || "获取详情失败",
            status: "error",
          });
        }
      })
      .catch((err) => {
        if (err?.code === "ERR_CANCELED") return;
        showToast({ title: "获取详情失败", status: "error" });
      });
    return () => {
      controller.abort();
    };
    // Service hooks expose request callbacks whose identity is not part of the fetch key.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen, recordId]);

  const keys: string[] = recordDetail?.photoKeys || [];
  const meta = resolveFeedbackStatusMeta(recordDetail?.status);

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      isCentered
      returnFocusOnClose={false}
    >
      <ModalOverlay bg="blackAlpha.600" />
      <ModalContent
        w={{ base: "calc(100% - 1.5rem)", md: "46rem" }}
        maxW={{ base: "calc(100% - 1.5rem)", md: "46rem" }}
        maxH="calc(100dvh - 2rem)"
        borderRadius="14px"
        overflow="hidden"
        bg="workbench.canvas"
      >
        <ModalHeader
          py={5}
          bg="workbench.control"
          color="workbench.paper"
          borderBottom="1px solid"
          borderColor="workbench.controlRaised"
        >
          反馈详情
        </ModalHeader>
        <ModalCloseButton color="white" boxSize={12} />
        <ModalBody py={5}>
          {recordDetailLoading ? (
            <Text color="workbench.muted">加载中…</Text>
          ) : (
            <Flex direction="column" gap={3}>
              <Flex gap={8} wrap="wrap">
                <Box minW="200px">
                  <Text fontSize="sm" color="gray.500">
                    姓名
                  </Text>
                  <Text color="gray.800">{recordDetail?.nickname || "-"}</Text>
                </Box>
                <Box minW="200px">
                  <Text fontSize="sm" color="gray.500">
                    手机号
                  </Text>
                  <Text color="gray.800">{recordDetail?.mobile || "-"}</Text>
                </Box>
                <Box minW="200px">
                  <Text fontSize="sm" color="gray.500">
                    反馈类型
                  </Text>
                  <Text color="gray.800">
                    {getTypeLabel(recordDetail?.type || "")}
                  </Text>
                </Box>
                <Box minW="200px">
                  <Text fontSize="sm" color="gray.500">
                    状态
                  </Text>
                  <Badge colorScheme={meta.scheme} variant="subtle" mt={1}>
                    {meta.text}
                  </Badge>
                </Box>
                <Box minW="200px">
                  <Text fontSize="sm" color="gray.500">
                    提交时间
                  </Text>
                  <Text color="gray.800">
                    {recordDetail?.createTime || "-"}
                  </Text>
                </Box>
              </Flex>
              <Box>
                <Text fontSize="sm" color="gray.500" mb={1}>
                  描述
                </Text>
                <Box
                  border="1px solid"
                  borderColor="workbench.line"
                  borderRadius="10px"
                  p={4}
                  bg="workbench.paper"
                >
                  <Text color="gray.800" whiteSpace="pre-wrap">
                    {recordDetail?.description || "-"}
                  </Text>
                </Box>
              </Box>
              <Box>
                <Text fontSize="sm" color="gray.500" mb={2}>
                  图片
                </Text>
                {keys.length ? (
                  <SimpleGrid columns={{ base: 2, md: 5 }} spacing={3}>
                    {keys.map((k) => (
                      <Box
                        key={k}
                        border="1px solid"
                        borderColor="gray.100"
                        borderRadius="md"
                        overflow="hidden"
                      >
                        <Image
                          src={buildImageUrl(k)}
                          alt={k}
                          w="full"
                          h="90px"
                          objectFit="cover"
                        />
                      </Box>
                    ))}
                  </SimpleGrid>
                ) : (
                  <Text color="gray.500">无</Text>
                )}
              </Box>
            </Flex>
          )}
        </ModalBody>
        <ModalFooter
          borderTop="1px solid"
          borderColor="workbench.line"
          bg="workbench.paper"
        >
          <Button variant="outline" colorScheme="gray" onClick={onClose}>
            关闭
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
}

export default function FileFeedbackPage() {
  const showToast = useCustomToast();
  const {
    userProfile: { isAdmin },
  } = useAppContext();

  const addModal = useDisclosure();
  const detailModal = useDisclosure();
  const deleteModal = useDisclosure();

  const [selectedRecord, setSelectedRecord] =
    useState<FeedbackRecordItem | null>(null);
  const [detailId, setDetailId] = useState<number | null>(null);

  const [deckOpen, setDeckOpen] = useState(false);
  const [searchForm, setSearchForm] = useState<any>({});
  const [typeOptions, setTypeOptions] = useState<
    Array<{ label: string; value: string }>
  >([]);

  const { fetchFeedbackOptions } = useFeedbackOptions();
  const { fetchFeedbackRecords } = useFeedbackRecords({});
  const { deleteRecordLoading, fetchDeleteFeedbackRecord } =
    useDeleteFeedbackRecord();
  const { updateStatusLoading, fetchUpdateFeedbackStatus } =
    useUpdateFeedbackStatus();

  const fetcher = useCallback(
    async (pageNum: number) => {
      const params: Record<string, unknown> = {
        pageNum,
        pageSize: FEEDBACK_PAGE_SIZE,
      };
      if (searchForm.keyword) params.keyword = searchForm.keyword;
      if (searchForm.type) params.type = searchForm.type;
      if (Array.isArray(searchForm.date) && searchForm.date.length === 2) {
        [params.startTime, params.endTime] = searchForm.date;
      }

      const response = await fetchFeedbackRecords({ params });
      if (response?.data?.code !== 0) {
        throw new Error(response?.data?.message || "获取列表失败");
      }
      const data = response?.data?.data || {};
      return {
        items: (data.items || []) as FeedbackRecordItem[],
        total: Number(data.total || 0),
      };
    },
    [fetchFeedbackRecords, searchForm],
  );

  const {
    items,
    initialLoading,
    loadingMore,
    hasMore,
    error: recordsError,
    loadFirstPage,
    loadMore,
  } = useInfiniteList<FeedbackRecordItem>({
    fetcher,
    pageSize: FEEDBACK_PAGE_SIZE,
    resetDeps: [searchForm],
  });

  useEffect(() => {
    fetchFeedbackOptions().then((res) => {
      if (res?.data?.code === 0) {
        const types = res?.data?.data?.types || [];
        setTypeOptions(buildTypeOptions(types));
      }
    });
    // Options are loaded once when the page mounts.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const formItems = useMemo(
    () => [
      {
        type: "input",
        name: "keyword",
        icon: true,
        placeholder: "请输入关键词",
        props: {
          "data-filter-kind": "input",
          className: "feedback-filter-control",
          w: "full",
          inputw: "full",
          h: "44px",
          m: 0,
          borderRadius: "10px",
        },
      },
      {
        type: "select",
        name: "type",
        placeholder: "反馈类型",
        props: {
          "data-filter-kind": "select",
          className: "feedback-filter-control",
          w: "full",
          h: "44px",
          m: 0,
          borderRadius: "10px",
        },
      },
      {
        type: "date_range",
        name: "date",
        props: {
          "data-filter-kind": "date",
          className: "feedback-filter-control",
          w: "full",
          h: "44px",
          m: 0,
          borderRadius: "10px",
        },
      },
    ],
    [],
  );

  const formOptions = useMemo(() => ({ type: typeOptions }), [typeOptions]);

  const cardViewState = resolveFeedbackCardViewState({
    loading: initialLoading,
    error: recordsError,
    itemCount: items.length,
  });

  const onFormChange = useCallback((formData) => {
    setSearchForm({ ...formData });
  }, []);
  const debouncedOnFormChange = useMemo(
    () => debounce(onFormChange, 400),
    [onFormChange],
  );
  useEffect(() => {
    return () => {
      debouncedOnFormChange.cancel();
    };
  }, [debouncedOnFormChange]);

  const handleDelete = async () => {
    if (!selectedRecord?.id) return;
    const res = await fetchDeleteFeedbackRecord({
      url: `/feedback/record/${selectedRecord.id}`,
    });
    if (res?.data?.code === 0) {
      deleteModal.onClose();
      setSelectedRecord(null);
      showToast({ title: "删除成功", status: "success" });
      await loadFirstPage();
      return;
    }
    showToast({ title: res?.data?.message || "删除失败", status: "error" });
  };

  return (
    <>
      <PageViewport
        id="feedback-management"
        scroll={false}
        overflowY={{ base: "auto", lg: "hidden" }}
        bg="workbench.canvas"
      >
        <WorkspaceShell
          h={{ base: "auto", lg: "full" }}
          minH="100%"
          contentProps={{ h: { base: "auto", lg: "full" }, minH: 0 }}
        >
          <Flex h={{ base: "auto", lg: "full" }} minH={0} direction="column">
            <ModuleWorkbenchHeader title="我要反馈" />

            <ModuleWorkbenchDeck
              title="把使用中的问题，直接留在处理队列"
              description="提交问题、建议或功能需求；处理状态和历史内容会保留在当前工作台。"
              expanded={deckOpen}
              onToggle={() => setDeckOpen((value) => !value)}
              action={
                <Button
                  leftIcon={<FiPlus />}
                  minH="44px"
                  px={6}
                  w={{ base: "full", md: "auto" }}
                  bg="workbench.paper"
                  color="workbench.control"
                  border="1px solid"
                  borderColor="whiteAlpha.600"
                  whiteSpace="nowrap"
                  _hover={{ bg: "gold.50", transform: "translateY(-1px)" }}
                  _active={{ transform: "translateY(0)" }}
                  _focusVisible={{
                    outline: "2px solid",
                    outlineColor: "gold.300",
                    outlineOffset: "3px",
                  }}
                  onClick={addModal.onOpen}
                >
                  提交反馈
                </Button>
              }
            >
              <Flex
                align="center"
                gap={1.5}
                px={2.5}
                py={1}
                borderRadius="full"
                bg="workbench.controlRaised"
              >
                <Box w="6px" h="6px" borderRadius="full" bg="gold.400" />
                <Text>支持文字说明与最多 5 张截图</Text>
              </Flex>
            </ModuleWorkbenchDeck>

            <DataSurface
              mt={6}
              flex={1}
              minH={{ base: "34rem", lg: 0 }}
              overflow="hidden"
              sx={{
                "& .feedback-filter-control": {
                  width: "100% !important",
                  minWidth: 0,
                  margin: "0 !important",
                },
                "& .feedback-filter-control > .chakra-input__group": {
                  width: "100%",
                  maxWidth: "none",
                },
                "& [data-filter-kind='input'] .chakra-input, & [data-filter-kind='select'] .chakra-button, & [data-filter-kind='date'] > div":
                  {
                    height: "44px",
                    minHeight: "44px",
                    width: "100%",
                    background: "var(--chakra-colors-workbench-paper)",
                    border: "1px solid var(--chakra-colors-workbench-line)",
                    borderRadius: "10px",
                    boxShadow: "none",
                  },
                "& [data-filter-kind='input'] .chakra-input:hover, & [data-filter-kind='select'] .chakra-button:hover, & [data-filter-kind='date'] > div:hover":
                  {
                    borderColor: "var(--chakra-colors-neutral-300)",
                  },
                "& [data-filter-kind='input'] .chakra-input:focus-visible, & [data-filter-kind='select'] .chakra-button:focus-visible, & [data-filter-kind='date'] > div:focus-within":
                  {
                    borderColor: "var(--chakra-colors-workbench-control)",
                    outline: "2px solid var(--chakra-colors-gold-400)",
                    outlineOffset: "2px",
                    boxShadow: "none",
                  },
                "& [data-filter-kind='input'] svg, & [data-filter-kind='select'] svg":
                  {
                    color: "var(--chakra-colors-workbench-muted)",
                  },
              }}
            >
              <Flex direction="column" h="full" minH={0}>
                <Box
                  p={{ base: 3, md: 4 }}
                  borderBottom="1px solid"
                  borderColor="workbench.line"
                  bg="workbench.paper"
                >
                  <Flex align="center" gap={2} mb={3}>
                    <FiFilter
                      size={15}
                      color="var(--chakra-colors-workbench-muted)"
                      aria-hidden
                    />
                    <Text fontSize="sm" fontWeight="700" color="workbench.text">
                      筛选反馈
                    </Text>
                    <Text fontSize="xs" color="workbench.muted">
                      关键词 · 类型 · 日期
                    </Text>
                  </Flex>
                  <CommonForm
                    isClearBtn={false}
                    formItems={formItems as any}
                    formOptions={formOptions as any}
                    onChange={debouncedOnFormChange}
                    isLoading={initialLoading || loadingMore}
                    props={{
                      display: "grid",
                      gridTemplateColumns:
                        "repeat(auto-fit, minmax(min(100%, 15rem), 1fr))",
                      gap: "var(--chakra-space-3)",
                      alignItems: "end",
                    }}
                  />
                </Box>

                <Box
                  id={FEEDBACK_SCROLL_ID}
                  flex={1}
                  minH={0}
                  overflowY="auto"
                  p={{ base: 3, md: 4 }}
                  className="thin-scrollbars"
                >
                  {cardViewState === "loading" && (
                    <FeedbackCardGrid aria-label="正在加载反馈记录">
                      {[1, 2, 3, 4, 5, 6].map((key) => (
                        <Skeleton key={key} h="140px" borderRadius="14px" />
                      ))}
                    </FeedbackCardGrid>
                  )}

                  {cardViewState === "error" && (
                    <Flex
                      minH="260px"
                      direction="column"
                      align="center"
                      justify="center"
                      gap={3}
                      px={5}
                      textAlign="center"
                      aria-live="polite"
                    >
                      <FiAlertTriangle
                        size={28}
                        color="var(--chakra-colors-error-500)"
                        aria-hidden
                      />
                      <Text fontWeight="700" color="workbench.text">
                        反馈记录加载失败
                      </Text>
                      <Text color="workbench.muted" fontSize="sm">
                        请检查网络连接后重新加载。
                      </Text>
                      <Button
                        minH="44px"
                        variant="outline"
                        borderColor="workbench.line"
                        onClick={loadFirstPage}
                      >
                        重新加载
                      </Button>
                    </Flex>
                  )}

                  {cardViewState === "empty" && (
                    <Flex
                      minH="260px"
                      direction="column"
                      align="center"
                      justify="center"
                      gap={3}
                      px={5}
                      textAlign="center"
                      aria-live="polite"
                    >
                      <FiInbox
                        size={30}
                        color="var(--chakra-colors-neutral-300)"
                        aria-hidden
                      />
                      <Text fontWeight="700" color="workbench.text">
                        暂无符合条件的反馈
                      </Text>
                      <Text color="workbench.muted" fontSize="sm">
                        调整筛选条件，或提交一条新反馈。
                      </Text>
                      <Button
                        minH="44px"
                        bg="workbench.control"
                        color="workbench.paper"
                        _hover={{ bg: "workbench.controlRaised" }}
                        onClick={addModal.onOpen}
                      >
                        提交反馈
                      </Button>
                    </Flex>
                  )}

                  {cardViewState === "ready" && (
                    <InfiniteScrollList
                      dataLength={items.length}
                      hasMore={hasMore}
                      loadMore={loadMore}
                      scrollableTarget={FEEDBACK_SCROLL_ID}
                    >
                      <FeedbackCardGrid>
                        {items.map((item) => {
                          const meta = resolveFeedbackStatusMeta(item.status);
                          const typeLabel = getTypeLabel(item.type);
                          return (
                            <CompactFeedbackCardFrame
                              key={item.id}
                              processed={meta.processed}
                            >
                              <Flex direction="column" gap={3}>
                                {/* 标题 + 处理状态 */}
                                <Flex
                                  justify="space-between"
                                  align="flex-start"
                                  gap={2}
                                  minW={0}
                                >
                                  <Tooltip
                                    label={item.description || "暂无描述"}
                                    hasArrow
                                  >
                                    <Text
                                      fontWeight="700"
                                      fontSize="sm"
                                      lineHeight="1.45"
                                      noOfLines={1}
                                      color="gray.800"
                                      flex={1}
                                      minW={0}
                                    >
                                      {item.description || "暂无描述"}
                                    </Text>
                                  </Tooltip>
                                  <Badge
                                    colorScheme={meta.scheme}
                                    variant="subtle"
                                    borderRadius="full"
                                    px={2}
                                    py={0.5}
                                    fontSize="10px"
                                    flexShrink={0}
                                  >
                                    {meta.text}
                                  </Badge>
                                </Flex>

                                {/* 类型 · 提交人 · 联系方式 · 时间：单行承载，过长截断并保留完整值提示 */}
                                <Flex
                                  align="center"
                                  gap={2}
                                  minW={0}
                                  fontSize="11px"
                                  color="workbench.muted"
                                >
                                  {typeLabel ? (
                                    <Badge
                                      variant="subtle"
                                      colorScheme="gray"
                                      borderRadius="full"
                                      px={2}
                                      py={0.5}
                                      fontSize="10px"
                                      flexShrink={0}
                                    >
                                      {typeLabel}
                                    </Badge>
                                  ) : null}
                                  <Tooltip
                                    label={`${item.userName || "-"} · ${item.mobile || "-"}`}
                                    hasArrow
                                  >
                                    <Text
                                      noOfLines={1}
                                      minW={0}
                                      flex={1}
                                      color="neutral.600"
                                    >
                                      {item.userName || "-"}
                                      <Text
                                        as="span"
                                        color="workbench.muted"
                                        ml={1.5}
                                      >
                                        {item.mobile || "-"}
                                      </Text>
                                    </Text>
                                  </Tooltip>
                                  <Text
                                    flexShrink={0}
                                    sx={{ fontVariantNumeric: "tabular-nums" }}
                                  >
                                    {item.createTime || "-"}
                                  </Text>
                                </Flex>

                                {/* 操作区：查看为主操作，管理操作右侧收拢，删除保持低干扰 */}
                                <Flex
                                  align="center"
                                  justify="space-between"
                                  gap={3}
                                  rowGap={2}
                                  wrap="wrap"
                                  pt={2.5}
                                  borderTop="1px solid"
                                  borderColor="neutral.100"
                                >
                                  <Button
                                    size="sm"
                                    h="32px"
                                    px={3}
                                    borderRadius="8px"
                                    position="relative"
                                    _before={CARD_ACTION_HIT_AREA}
                                    variant="outline"
                                    borderColor="workbench.line"
                                    bg="workbench.paper"
                                    color="workbench.control"
                                    fontSize="xs"
                                    fontWeight="700"
                                    leftIcon={<FiEye size={13} />}
                                    onClick={() => {
                                      setDetailId(Number(item.id));
                                      detailModal.onOpen();
                                    }}
                                    _hover={{
                                      bg: "workbench.canvas",
                                      borderColor: "neutral.300",
                                    }}
                                    _active={{
                                      bg: "neutral.100",
                                      transform: "translateY(1px)",
                                    }}
                                    _focusVisible={CARD_ACTION_FOCUS}
                                  >
                                    查看详情
                                  </Button>
                                  {isAdmin && (
                                    <Flex align="center" gap={3}>
                                      <StatusToggleButton
                                        isProcessed={meta.processed}
                                        onClick={async () => {
                                          const res =
                                            await fetchUpdateFeedbackStatus({
                                              url: `/feedback/record/${item.id}/status`,
                                              method: "PUT",
                                            });
                                          if (res?.data?.code === 0) {
                                            showToast({
                                              title: meta.processed
                                                ? "已恢复为待处理"
                                                : "已标记为已处理",
                                              status: "success",
                                            });
                                            await loadFirstPage();
                                            return;
                                          }
                                          showToast({
                                            title:
                                              res?.data?.message || "操作失败",
                                            status: "error",
                                          });
                                        }}
                                        isLoading={updateStatusLoading}
                                      />
                                      <Tooltip
                                        label="删除该反馈"
                                        hasArrow
                                        openDelay={200}
                                      >
                                        <IconButton
                                          aria-label="删除该反馈"
                                          icon={<FiTrash2 size={15} />}
                                          size="sm"
                                          h="32px"
                                          w="32px"
                                          minW="32px"
                                          borderRadius="8px"
                                          position="relative"
                                          _before={CARD_ACTION_HIT_AREA}
                                          variant="ghost"
                                          color="neutral.400"
                                          onClick={() => {
                                            setSelectedRecord(item);
                                            deleteModal.onOpen();
                                          }}
                                          _hover={{
                                            bg: "error.50",
                                            color: "error.600",
                                          }}
                                          _active={{
                                            bg: "error.100",
                                            transform: "translateY(1px)",
                                          }}
                                          _focusVisible={CARD_ACTION_FOCUS}
                                        />
                                      </Tooltip>
                                    </Flex>
                                  )}
                                </Flex>
                              </Flex>
                            </CompactFeedbackCardFrame>
                          );
                        })}
                      </FeedbackCardGrid>
                    </InfiniteScrollList>
                  )}
                </Box>
              </Flex>
            </DataSurface>
          </Flex>
        </WorkspaceShell>
      </PageViewport>

      <AddFeedbackModal
        isOpen={addModal.isOpen}
        onClose={addModal.onClose}
        typeOptions={typeOptions}
        onSuccess={loadFirstPage}
      />

      <FeedbackDetailModal
        isOpen={detailModal.isOpen}
        onClose={() => {
          detailModal.onClose();
          setDetailId(null);
        }}
        recordId={detailId}
      />

      <DeleteConfirmModal
        isOpen={deleteModal.isOpen}
        onClose={() => {
          deleteModal.onClose();
          setSelectedRecord(null);
        }}
        title="确认删除"
        description={
          <Box>
            <Text color="neutral.700" mb={2}>
              确认要删除该反馈记录吗？
            </Text>
            <Text color="red.600">
              注意：删除操作是不可逆的，确认后将无法恢复。
            </Text>
          </Box>
        }
        handleConfirm={handleDelete}
        isLoading={deleteRecordLoading}
      />
    </>
  );
}
