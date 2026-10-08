"use client";

/* Hallmark · component: audit-create-form · genre: modern-minimal · theme: BidEngine deep-sea/gold
 * states: default · hover · focus-visible · active · disabled · loading · error · success
 */

import React, { useCallback, useRef, useState } from "react";
import {
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  ModalCloseButton,
  Button,
  Text,
  Input,
  Flex,
  Box,
  IconButton,
  Switch,
  FormControl,
  FormLabel,
  useToast,
} from "@chakra-ui/react";
import { FiUploadCloud, FiFile, FiX } from "react-icons/fi";
import { useReviewCreate } from "@/service/audit";

interface Props {
  isOpen: boolean;
  onClose: () => void;
  onCreated: () => void;
}

const ACCEPT = ".pdf,.doc,.docx";

function FileDropzone({
  title,
  hint,
  files,
  error,
  disabled,
  onAdd,
  onRemove,
}: {
  title: string;
  hint: string;
  files: File[];
  error?: string;
  disabled?: boolean;
  // eslint-disable-next-line no-unused-vars
  onAdd: (files: File[]) => void;
  // eslint-disable-next-line no-unused-vars
  onRemove: (index: number) => void;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  let dropBorderColor = "neutral.300";
  if (files.length) dropBorderColor = "primary.300";
  if (error) dropBorderColor = "error.400";
  return (
    <Box>
      <input
        ref={inputRef}
        type="file"
        accept={ACCEPT}
        multiple
        disabled={disabled}
        style={{ display: "none" }}
        onChange={(e) => {
          const picked = Array.from(e.target.files || []);
          if (picked.length) onAdd(picked);
          e.target.value = "";
        }}
      />
      <Box
        role="button"
        tabIndex={disabled ? -1 : 0}
        w="full"
        minH="150px"
        border="1px dashed"
        borderColor={dropBorderColor}
        borderRadius="10px"
        bg={files.length ? "primary.50" : "workbench.paper"}
        p={4}
        textAlign="center"
        cursor={disabled ? "not-allowed" : "pointer"}
        opacity={disabled ? 0.6 : 1}
        aria-disabled={Boolean(disabled)}
        transition="background-color 0.2s cubic-bezier(0.16, 1, 0.3, 1), border-color 0.2s cubic-bezier(0.16, 1, 0.3, 1), transform 0.1s cubic-bezier(0.7, 0, 0.84, 0)"
        _hover={{
          borderColor: error ? "error.500" : "primary.400",
          bg: "primary.50",
        }}
        _active={{ transform: "translateY(1px)" }}
        _focusVisible={{
          outline: "2px solid",
          outlineColor: "gold.400",
          outlineOffset: "2px",
        }}
        onClick={() => {
          if (!disabled) inputRef.current?.click();
        }}
        onKeyDown={(event) => {
          if (!disabled && (event.key === "Enter" || event.key === " ")) {
            event.preventDefault();
            inputRef.current?.click();
          }
        }}
      >
        <Flex direction="column" align="center" gap={1}>
          <FiUploadCloud size={22} color="var(--chakra-colors-primary-500)" />
          <Text fontWeight="600" color="primary.700" fontSize="sm">
            {title}
          </Text>
          <Text fontSize="xs" color="neutral.400">
            {hint}
          </Text>
        </Flex>
      </Box>
      {files.length > 0 && (
        <Flex direction="column" gap={1.5} mt={2.5}>
          {files.map((f, i) => (
            <Flex
              key={`${f.name}-${i}`}
              align="center"
              gap={2}
              bg="white"
              border="1px solid"
              borderColor="neutral.200"
              borderRadius="md"
              px={2.5}
              py={1.5}
            >
              <FiFile size={13} color="var(--chakra-colors-primary-500)" />
              <Text fontSize="xs" flex={1} noOfLines={1} title={f.name}>
                {f.name}
              </Text>
              <Text fontSize="10px" color="neutral.400">
                {(f.size / 1024 / 1024).toFixed(2)}MB
              </Text>
              <IconButton
                aria-label="移除"
                icon={<FiX />}
                isDisabled={disabled}
                size="xs"
                variant="ghost"
                color="neutral.400"
                onClick={(e) => {
                  e.stopPropagation();
                  onRemove(i);
                }}
              />
            </Flex>
          ))}
        </Flex>
      )}
      {error && (
        <Text mt={1.5} color="error.600" fontSize="xs">
          {error}
        </Text>
      )}
    </Box>
  );
}

export default function CreateAuditModal({
  isOpen,
  onClose,
  onCreated,
}: Props) {
  const toast = useToast();
  const { createLoading, fetchCreate } = useReviewCreate();

  const [name, setName] = useState("");
  const [tenderFiles, setTenderFiles] = useState<File[]>([]);
  const [bidFiles, setBidFiles] = useState<File[]>([]);
  const [isAnonymous, setIsAnonymous] = useState(false);
  const [submitted, setSubmitted] = useState(false);

  const reset = useCallback(() => {
    setName("");
    setTenderFiles([]);
    setBidFiles([]);
    setIsAnonymous(false);
    setSubmitted(false);
  }, []);

  const handleClose = useCallback(() => {
    reset();
    onClose();
  }, [onClose, reset]);

  const submit = useCallback(async () => {
    setSubmitted(true);
    if (!tenderFiles.length || !bidFiles.length) {
      return;
    }
    const formData = new FormData();
    if (name.trim()) formData.append("name", name.trim());
    formData.append("is_anonymous", isAnonymous ? "true" : "false");
    tenderFiles.forEach((f) => formData.append("tender_files", f));
    bidFiles.forEach((f) => formData.append("bid_files", f));
    try {
      const res = await fetchCreate({ data: formData });
      if (res?.data?.code === 200) {
        toast({ title: "审核项目已创建，正在解析", status: "success" });
        handleClose();
        onCreated();
      } else {
        toast({ title: res?.data?.msg || "创建失败", status: "error" });
      }
    } catch (e: any) {
      toast({ title: e?.response?.data?.msg || "创建失败", status: "error" });
    }
  }, [
    tenderFiles,
    bidFiles,
    name,
    isAnonymous,
    fetchCreate,
    toast,
    handleClose,
    onCreated,
  ]);

  return (
    <Modal
      isOpen={isOpen}
      onClose={handleClose}
      isCentered
      size="xl"
      scrollBehavior="inside"
    >
      <ModalOverlay bg="blackAlpha.600" />
      <ModalContent
        mx={3}
        borderRadius="14px"
        maxH="calc(100dvh - 2rem)"
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
            创建审核项目
          </Text>
          <Text mt={1} fontSize="xs" fontWeight="400" color="whiteAlpha.700">
            上传招标文件与投标文件，系统将逐项核对并定位原文
          </Text>
        </ModalHeader>
        <ModalCloseButton color="white" minW="44px" minH="44px" />
        <ModalBody pt={5}>
          <FormControl>
            <FormLabel
              htmlFor="review-project-name"
              fontSize="sm"
              fontWeight="600"
              color="workbench.text"
              mb={1.5}
            >
              项目名称{" "}
              <Text as="span" fontWeight="400" color="neutral.500">
                （可选）
              </Text>
            </FormLabel>
            <Input
              id="review-project-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="默认使用投标文件名"
              h="44px"
              borderRadius="10px"
              bg="workbench.paper"
              borderColor="workbench.line"
              isDisabled={createLoading}
              _focusVisible={{
                borderColor: "workbench.control",
                boxShadow: "none",
                outline: "2px solid",
                outlineColor: "gold.400",
                outlineOffset: "2px",
              }}
            />
          </FormControl>
          <Text
            mt={5}
            mb={2.5}
            fontSize="sm"
            fontWeight="600"
            color="workbench.text"
          >
            上传审核文件
          </Text>
          <Flex
            gap={4}
            direction={{ base: "column", md: "row" }}
            align="stretch"
          >
            <Box flex={1} minW={0}>
              <FileDropzone
                title="招标文件"
                hint="点击选择 PDF、DOC 或 DOCX，可添加补遗文件"
                files={tenderFiles}
                error={
                  submitted && !tenderFiles.length
                    ? "请上传招标文件"
                    : undefined
                }
                disabled={createLoading}
                onAdd={(fs) => setTenderFiles((prev) => [...prev, ...fs])}
                onRemove={(i) =>
                  setTenderFiles((prev) => prev.filter((_, idx) => idx !== i))
                }
              />
            </Box>
            <Box flex={1} minW={0}>
              <FileDropzone
                title="投标文件"
                hint="点击选择 PDF、DOC 或 DOCX，可添加多卷文件"
                files={bidFiles}
                error={
                  submitted && !bidFiles.length ? "请上传投标文件" : undefined
                }
                disabled={createLoading}
                onAdd={(fs) => setBidFiles((prev) => [...prev, ...fs])}
                onRemove={(i) =>
                  setBidFiles((prev) => prev.filter((_, idx) => idx !== i))
                }
              />
            </Box>
          </Flex>
          <FormControl mt={5}>
            <Flex
              align="center"
              justify="space-between"
              gap={4}
              minH="76px"
              px={4}
              py={3}
              bg="workbench.paper"
              border="1px solid"
              borderColor="workbench.line"
              borderRadius="10px"
            >
              <Box minW={0} flex={1}>
                <FormLabel
                  htmlFor="review-force-anonymous"
                  mb={1}
                  fontSize="sm"
                  fontWeight="600"
                  color="workbench.text"
                  cursor="pointer"
                >
                  强制暗标检查
                </FormLabel>
                <Text fontSize="xs" color="neutral.600">
                  关闭时按招标文件要求自动识别，开启时始终检查匿名性与版式。
                </Text>
              </Box>
              <Flex
                align="center"
                justify="flex-end"
                gap={2.5}
                minW="100px"
                minH="44px"
                flexShrink={0}
              >
                <Text
                  fontSize="xs"
                  fontWeight="600"
                  color={isAnonymous ? "gold.700" : "neutral.600"}
                  whiteSpace="nowrap"
                >
                  {isAnonymous ? "强制开启" : "自动识别"}
                </Text>
                <Switch
                  id="review-force-anonymous"
                  isChecked={isAnonymous}
                  onChange={(e) => setIsAnonymous(e.target.checked)}
                  colorScheme="gold"
                  isDisabled={createLoading}
                />
              </Flex>
            </Flex>
          </FormControl>
        </ModalBody>
        <ModalFooter
          borderTop="1px solid"
          borderColor="workbench.line"
          mt={4}
          bg="workbench.paper"
        >
          <Button
            variant="ghost"
            mr={3}
            onClick={handleClose}
            isDisabled={createLoading}
          >
            取消
          </Button>
          <Button
            minH="44px"
            isLoading={createLoading}
            loadingText="创建中…"
            onClick={submit}
            bg="workbench.control"
            color="workbench.paper"
            _hover={{ bg: "workbench.controlRaised" }}
          >
            开始审核
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
}
