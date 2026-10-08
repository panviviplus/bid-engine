"use client";

/* eslint-disable no-use-before-define */
/* eslint-disable no-unused-vars */
/* Hallmark · component: create-bid-form · genre: modern-minimal · theme: BidEngine workbench (preserved tokens)
 * states: default · hover · focus-visible · active · disabled · loading · error · success
 * contrast: inherited from locked Chakra workbench tokens
 * pre-emit critique: P5 H4 E4 S5 R5 V4
 */
import { useCallback, useEffect, useRef, useState } from "react";
import {
  Box,
  Button,
  Flex,
  FormControl,
  FormErrorMessage,
  FormHelperText,
  FormLabel,
  Icon,
  Input,
  Modal,
  ModalBody,
  ModalCloseButton,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  Progress,
  Step,
  StepDescription,
  StepIcon,
  StepIndicator,
  StepNumber,
  StepSeparator,
  StepStatus,
  StepTitle,
  Stepper,
  Text,
  useSteps,
  useToast,
} from "@chakra-ui/react";
import {
  FiFile,
  FiPlus,
  FiUploadCloud,
  FiCheckCircle,
} from "react-icons/fi";
import { useRouter } from "next/navigation";

import {
  useBidGenCreateBlank,
  useBidGenCreateFromTemplate,
  useBidGenDetail,
} from "@/service/bid-gen";
import type { CreateMode } from "./types";

type Props = {
  isOpen: boolean;
  onClose: () => void;
  onCreated: (projectId: number) => void;
};

type AvailableCreateMode = Exclude<CreateMode, "tender">;

const MODES: {
  key: AvailableCreateMode;
  label: string;
  desc: string;
  icon: any;
}[] = [
  {
    key: "template",
    label: "从模板创建",
    desc: "上传投标书模板，提取大纲与正文初稿",
    icon: FiFile,
  },
  {
    key: "blank",
    label: "创建空白标书",
    desc: "从空白文档开始，自行编写大纲",
    icon: FiPlus,
  },
];

// 文件大小上限：100MB
const MAX_FILE_SIZE = 100 * 1024 * 1024;

const stageMeta: Record<string, { label: string; desc: string }> = {
  template_parse: { label: "模板文档解析", desc: "解析模板文档内容" },
  template_outline: { label: "模板大纲提取", desc: "提取标题大纲结构" },
};

export default function CreateBidModal({ isOpen, onClose, onCreated }: Props) {
  const router = useRouter();
  const toast = useToast();
  const [mode, setMode] = useState<AvailableCreateMode>("template");
  const [fileName, setFileName] = useState("");
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [blankName, setBlankName] = useState("");
  const [blankTouched, setBlankTouched] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);
  const blankNameRef = useRef<HTMLInputElement>(null);

  const [projectId, setProjectId] = useState<number>(0);
  const [submitting, setSubmitting] = useState(false);
  const [phase, setPhase] = useState<"form" | "parsing">("form");
  const [pollTick, setPollTick] = useState(0);

  const { fetchCreate: createBlank } = useBidGenCreateBlank();
  const { fetchCreate: createTemplate } = useBidGenCreateFromTemplate();
  const { detailData, fetchDetail } = useBidGenDetail(projectId);

  const steps = useSteps({
    index: 0,
    count: 3,
  });
  // setActiveStep 是稳定的 useState setter；不要把每次渲染都新建的 steps 对象塞进依赖数组，
  // 否则 reset 身份每次渲染都变，useEffect([isOpen, reset]) 会在每次渲染后都执行 reset，
  // 从而把用户刚选中的文件 / 输入的字段立刻清空。
  const { setActiveStep } = steps;

  // 解析阶段 → 轮询详情
  useEffect(() => {
    if (phase !== "parsing" || projectId <= 0) return;
    const t = window.setInterval(() => setPollTick((v) => v + 1), 2500);
    return () => window.clearInterval(t);
  }, [phase, projectId]);

  useEffect(() => {
    if (phase !== "parsing" || projectId <= 0) return;
    // 解析期轮询详情：显式丢弃返回值，避免产生悬空 promise
    void fetchDetail();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pollTick]);

  useEffect(() => {
    if (phase !== "parsing" || !detailData) return;
    if (detailData.status === "outline_review") {
      setPhase("form");
      toast({
        title: "模板解析完成，请确认大纲",
        status: "success",
        duration: 2500,
      });
      onCreated(projectId);
    } else if (detailData.status === "failed") {
      setPhase("form");
      toast({
        title: detailData.lastError || "解析失败",
        status: "error",
        duration: 4000,
      });
      onClose();
    }
  }, [detailData, phase, toast, onClose, onCreated, projectId]);

  const reset = useCallback(() => {
    setMode("template");
    setFileName("");
    setSelectedFile(null);
    setBlankName("");
    setBlankTouched(false);
    setProjectId(0);
    setPhase("form");
    setActiveStep(0);
  }, [setActiveStep]);

  useEffect(() => {
    if (isOpen) reset();
  }, [isOpen, reset]);

  const pickFile = () => {
    if (fileRef.current) fileRef.current.click();
  };

  // 选择文件：记录 File 对象 + 文件名（后续上传并解析直接使用，不再二次弹窗）
  // 大小限制：不超过 100MB，超限拒绝并提示
  const onFileChange = (e: any) => {
    const file = e?.target?.files?.[0] as File | undefined;
    if (file) {
      if (file.size > MAX_FILE_SIZE) {
        toast({
          title: "文件大小不能超过 100MB",
          description: `${file.name}（${formatFileSize(file.size)}）超出限制`,
          status: "warning",
          duration: 4000,
        });
        // 清空选择，避免残留
        if (fileRef.current) fileRef.current.value = "";
        setSelectedFile(null);
        setFileName("");
        return;
      }
      setSelectedFile(file);
      setFileName(file.name);
    }
    // 允许重复选择同一文件
    if (e?.target) e.target.value = "";
  };

  const removeFile = () => {
    setSelectedFile(null);
    setFileName("");
    if (fileRef.current) fileRef.current.value = "";
  };

  const submit = async () => {
    const file = selectedFile;
    if (!file) {
      toast({
        title: "请先选择模板文件",
        status: "warning",
        duration: 2500,
      });
      return;
    }
    setSubmitting(true);
    try {
      const fd = new FormData();
      fd.append("file", file);
      const res = await createTemplate({ data: fd });
      if (res?.data?.code === 200) {
        setProjectId(Number(res?.data?.data?.id));
        setPhase("parsing");
        setActiveStep(1);
        toast({
          title: "上传成功，正在解析",
          status: "success",
          duration: 3000,
        });
      } else {
        toast({
          title: res?.data?.msg || "创建失败",
          status: "error",
          duration: 4000,
        });
      }
    } catch (err: any) {
      toast({
        title: err?.response?.data?.msg || "创建失败",
        status: "error",
        duration: 4000,
      });
    } finally {
      setSubmitting(false);
    }
  };

  const createBlankProject = async () => {
    if (submitting) return;
    setBlankTouched(true);
    if (!blankName.trim()) {
      blankNameRef.current?.focus();
      return;
    }
    setSubmitting(true);
    try {
      const res = await createBlank({ data: { name: blankName.trim() } });
      if (res?.data?.code === 200) {
        toast({ title: "空白标书已创建", status: "success", duration: 2500 });
        onCreated(Number(res?.data?.data?.id));
      } else {
        toast({
          title: res?.data?.msg || "创建失败",
          status: "error",
          duration: 4000,
        });
      }
    } catch (err: any) {
      toast({
        title: err?.response?.data?.msg || "创建失败",
        status: "error",
        duration: 4000,
      });
    } finally {
      setSubmitting(false);
    }
  };

  // 解析阶段状态
  const stageStatus = detailData?.stageStatus || {};
  const parsingStages = ["template_parse", "template_outline"];

  return (
    <Modal isOpen={isOpen} onClose={onClose} size="xl" isCentered>
      <ModalOverlay bg="blackAlpha.600" />
      <ModalContent
        mx={3}
        maxH="calc(100dvh - 2rem)"
        borderRadius="14px"
        overflow="hidden"
        bg="workbench.canvas"
      >
        <ModalHeader
          bg="workbench.control"
          color="workbench.paper"
          py={5}
          borderBottom="1px solid"
          borderColor="workbench.controlRaised"
        >
          <Text fontSize="lg" fontWeight="700">
            创建标书
          </Text>
          <Text mt={1} fontSize="xs" fontWeight="400" color="whiteAlpha.700">
            选择起稿方式，创建后进入标书工作区
          </Text>
        </ModalHeader>
        <ModalCloseButton color="white" minW="44px" minH="44px" />
        <ModalBody py={5}>
          {phase === "form" && (
            <Box>
              {/* 模式选择 */}
              <Flex direction="column" gap={3}>
                {MODES.map((m) => {
                  const active = mode === m.key;
                  const Icon = m.icon;
                  return (
                    <Flex
                      key={m.key}
                      as="button"
                      onClick={() => {
                        setMode(m.key);
                        removeFile();
                        setBlankTouched(false);
                      }}
                      align="center"
                      gap={3}
                      p={3.5}
                      minH="68px"
                      borderRadius="10px"
                      border="1px solid"
                      borderColor={active ? "gold.400" : "workbench.line"}
                      bg={active ? "gold.50" : "workbench.paper"}
                      cursor="pointer"
                      w="full"
                      transition="border-color 0.2s, background-color 0.2s"
                      _hover={{
                        borderColor: active ? "gold.400" : "neutral.300",
                      }}
                      _active={{ transform: "translateY(1px)" }}
                      _focusVisible={{
                        outline: "2px solid",
                        outlineColor: "gold.400",
                        outlineOffset: "2px",
                      }}
                    >
                      <Flex
                        w="10"
                        h="10"
                        borderRadius="9px"
                        align="center"
                        justify="center"
                        bg={active ? "gold.500" : "workbench.control"}
                        color="white"
                        fontSize="lg"
                      >
                        <Icon />
                      </Flex>
                      <Box textAlign="left" flex="1">
                        <Text fontWeight="600" color="gray.800">
                          {m.label}
                        </Text>
                        <Text fontSize="xs" color="gray.500">
                          {m.desc}
                        </Text>
                      </Box>
                      {active && <Icon as={FiCheckCircle} color="gold.600" />}
                    </Flex>
                  );
                })}
              </Flex>

              <Flex
                mt={4}
                p={3}
                gap={3}
                align={{ base: "stretch", sm: "center" }}
                justify="space-between"
                direction={{ base: "column", sm: "row" }}
                border="1px solid"
                borderColor="workbench.line"
                borderRadius="10px"
                bg="workbench.paper"
              >
                <Box minW={0}>
                  <Text fontSize="sm" fontWeight="600" color="gray.800">
                    已有招标文件？
                  </Text>
                  <Text mt={0.5} fontSize="xs" color="gray.500">
                    请先完成招标解析，再从确认后的标书蓝图创建投标书。
                  </Text>
                </Box>
                <Button
                  flexShrink={0}
                  minH="44px"
                  variant="outline"
                  borderColor="primary.300"
                  whiteSpace="nowrap"
                  onClick={() => {
                    onClose();
                    router.push("/bid-analysis");
                  }}
                >
                  前往招标解析
                </Button>
              </Flex>

              {/* 表单 */}
              <Box mt={5}>
                {mode === "blank" ? (
                  <FormControl
                    isRequired
                    isInvalid={blankTouched && !blankName.trim()}
                  >
                    <FormLabel
                      htmlFor="blank-project-name"
                      mb={2}
                      fontSize="sm"
                      fontWeight="600"
                      color="gray.700"
                    >
                      项目名称
                    </FormLabel>
                    <Input
                      ref={blankNameRef}
                      id="blank-project-name"
                      placeholder="例如：XX 产业园智能化工程投标书"
                      value={blankName}
                      onChange={(e) => setBlankName(e.target.value)}
                      onBlur={() => setBlankTouched(true)}
                      size="lg"
                      borderRadius="lg"
                      aria-required="true"
                      aria-describedby="blank-project-name-message"
                      onKeyDown={(e) => {
                        if (e.key === "Enter") {
                          e.preventDefault();
                          createBlankProject();
                        }
                      }}
                    />
                    <Box id="blank-project-name-message" minH="20px" mt={1}>
                      <FormErrorMessage mt={0} fontSize="xs">
                        请输入项目名称
                      </FormErrorMessage>
                      {!(blankTouched && !blankName.trim()) && (
                        <FormHelperText mt={0} fontSize="xs" color="gray.500">
                          创建后先编排并确认大纲，再进入正文编辑
                        </FormHelperText>
                      )}
                    </Box>
                  </FormControl>
                ) : (
                  <Flex
                    direction="column"
                    align="center"
                    justify="center"
                    gap={3}
                    minH="190px"
                    border="1px dashed"
                    borderColor={fileName ? "success.400" : "primary.200"}
                    borderRadius="lg"
                    bg={fileName ? "success.50" : "workbench.paper"}
                    py={8}
                    cursor="pointer"
                    onClick={pickFile}
                    _hover={{ borderColor: "gold.400", bg: "gold.50" }}
                    transition="border-color 0.2s, background-color 0.2s"
                  >
                    <input
                      ref={fileRef}
                      type="file"
                      accept=".docx,.doc,.pdf"
                      style={{ display: "none" }}
                      onClick={(e) => e.stopPropagation()}
                      onChange={onFileChange}
                    />
                    <Flex
                      w="12"
                      h="12"
                      borderRadius="10px"
                      align="center"
                      justify="center"
                      bg={fileName ? "success.100" : "primary.100"}
                      color={fileName ? "success.600" : "primary.600"}
                      fontSize="2xl"
                    >
                      {fileName ? <FiCheckCircle /> : <FiUploadCloud />}
                    </Flex>
                    {fileName ? (
                      <>
                        <Text
                          fontSize="sm"
                          fontWeight="600"
                          color="gray.800"
                          px={4}
                          noOfLines={1}
                          wordBreak="break-all"
                        >
                          {fileName}
                          {selectedFile && (
                            <Text
                              as="span"
                              fontSize="xs"
                              color="gray.400"
                              fontWeight="400"
                            >
                              {" "}
                              · {formatFileSize(selectedFile.size)}
                            </Text>
                          )}
                        </Text>
                        <Flex align="center" gap={2}>
                          <Button
                            size="xs"
                            variant="outline"
                            borderColor="primary.200"
                            onClick={(e) => {
                              e.stopPropagation();
                              pickFile();
                            }}
                          >
                            更换文件
                          </Button>
                          <Button
                            size="xs"
                            variant="ghost"
                            color="error.500"
                            onClick={(e) => {
                              e.stopPropagation();
                              removeFile();
                            }}
                          >
                            移除
                          </Button>
                        </Flex>
                      </>
                    ) : (
                      <>
                        <Text fontSize="sm" fontWeight="600" color="gray.700">
                          点击上传投标书模板
                        </Text>
                        <Text fontSize="xs" color="gray.400">
                          支持 Word / PDF 格式，将提取大纲与正文 · 文件不超过
                          100MB
                        </Text>
                      </>
                    )}
                  </Flex>
                )}
              </Box>
            </Box>
          )}

          {phase === "parsing" && (
            <Box py={4}>
              <Stepper
                index={steps.activeStep}
                colorScheme="gold"
                size="sm"
                mb={6}
              >
                {parsingStages.map((key) => {
                  const st = stageStatus[key];
                  const meta = stageMeta[key] || { label: key, desc: "" };
                  return (
                    <Step key={key}>
                      <StepIndicator>
                        <StepStatus
                          complete={<StepIcon />}
                          incomplete={<StepNumber />}
                          active={<StepNumber />}
                        />
                      </StepIndicator>
                      <Box flexShrink="0">
                        <StepTitle>{meta.label}</StepTitle>
                        <StepDescription>
                          {st === "running" ? "执行中…" : meta.desc}
                        </StepDescription>
                      </Box>
                      <StepSeparator />
                    </Step>
                  );
                })}
              </Stepper>
              <Progress
                value={detailData?.progress || 15}
                size="sm"
                colorScheme="gold"
                borderRadius="full"
                hasStripe
                isAnimated
                mb={3}
              />
              <Flex align="center" gap={2} color="gray.500">
                <Box w="2" h="2" borderRadius="full" bg="gold.500" />
                <Text fontSize="sm">
                  AI 正在解析{fileName || ""}，请稍候…
                </Text>
              </Flex>
            </Box>
          )}

        </ModalBody>
        <ModalFooter
          borderTop="1px solid"
          borderColor="workbench.line"
          gap={3}
          bg="workbench.paper"
        >
          {phase === "form" &&
            (mode === "blank" ? (
              <Button
                flex="1"
                minH="44px"
                bg="workbench.control"
                color="workbench.paper"
                _hover={{ bg: "workbench.controlRaised" }}
                isDisabled={submitting}
                isLoading={submitting}
                loadingText="创建中…"
                onClick={createBlankProject}
              >
                创建空白标书
              </Button>
            ) : (
              <Button
                flex="1"
                minH="44px"
                bg="workbench.control"
                color="workbench.paper"
                _hover={{ bg: "workbench.controlRaised" }}
                isDisabled={!selectedFile}
                isLoading={submitting}
                loadingText="上传中…"
                onClick={submit}
              >
                上传并解析
              </Button>
            ))}
          {phase === "parsing" && (
            <Button flex="1" variant="ghost" onClick={onClose}>
              后台继续解析（可在列表查看进度）
            </Button>
          )}
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
}

function formatFileSize(bytes: number): string {
  if (!bytes || bytes <= 0) return "";
  const units = ["B", "KB", "MB", "GB"];
  let size = bytes;
  let idx = 0;
  while (size >= 1024 && idx < units.length - 1) {
    size /= 1024;
    idx += 1;
  }
  return `${size.toFixed(idx === 0 ? 0 : 1)}${units[idx]}`;
}
