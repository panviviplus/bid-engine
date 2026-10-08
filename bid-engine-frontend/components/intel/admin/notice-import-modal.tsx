"use client";

/* Hallmark · component: intel-notice-import · genre: modern-minimal · theme: BidEngine deep-sea/gold (preserved tokens)
 * 交互：先给模板 → 说明每列怎么填 → 再上传；导入结果逐行反馈并给出批次号。
 * states: default · hover · focus-visible · active · disabled · loading(上传中) · error · success
 * Hallmark · pre-emit critique: P5 H5 E5 S5 R5 V4
 */
import React, { useRef, useState } from "react";
import {
  Alert,
  AlertDescription,
  AlertIcon,
  Box,
  Button,
  Code,
  Flex,
  HStack,
  Modal,
  ModalBody,
  ModalCloseButton,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  Stack,
  Switch,
  Table,
  TableContainer,
  Tbody,
  Td,
  Text,
  Th,
  Thead,
  Tr,
  useToast,
} from "@chakra-ui/react";
import { FiDownload, FiUpload } from "react-icons/fi";

import { FIELD_HEIGHT } from "@/components/intel/intel-select";
import {
  useIntelNoticeImport,
  type IntelNoticeImportResult,
} from "@/service/intel";

/** 表格列说明，与后端 noticeImportColumns 保持一致。 */
const COLUMN_GUIDE: Array<[string, string, string]> = [
  ["title", "必填", "公告标题，同时用于自动判定公告类型与行业"],
  ["publisher", "可选", "采购人（招标单位）"],
  ["agency", "可选", "代理机构"],
  ["project_code", "可选", "项目编号"],
  ["budget_amount_wan", "可选", "预算金额，单位万元，只填数字（例如 860）"],
  ["budget_text", "可选", "预算原文；留空时按万元自动生成"],
  ["region_province", "可选", "省级地区，例如 广东省"],
  ["region_city", "可选", "市级地区，例如 深圳市"],
  [
    "notice_type",
    "可选",
    "公告类型编码（open_tender / negotiation / inquiry 等），留空按标题自动判定",
  ],
  ["publish_date", "可选", "发布时间，yyyy-MM-dd"],
  ["deadline_at", "可选", "投标截止时间，yyyy-MM-dd 或 yyyy-MM-dd HH:mm:ss"],
  ["url", "可选", "来源链接；留空表示线下收集，系统会生成内部占位标识"],
  ["industries", "可选", "行业编码或名称，逗号分隔；留空按标题自动打标"],
  ["body_text", "可选", "公告正文或采购需求要点"],
  ["source_name", "可选", "来源名称；留空为“系统录入”"],
];

export default function NoticeImportModal({
  isOpen,
  onClose,
  onImported,
}: {
  isOpen: boolean;
  onClose: () => void;
  onImported: () => void;
}) {
  const toast = useToast();
  const fileRef = useRef<HTMLInputElement>(null);
  const [fileName, setFileName] = useState("");
  const [overwrite, setOverwrite] = useState(false);
  const [result, setResult] = useState<IntelNoticeImportResult | null>(null);
  const { noticeImportLoading, fetchImport } = useIntelNoticeImport();

  const reset = () => {
    setFileName("");
    setResult(null);
    setOverwrite(false);
    if (fileRef.current) fileRef.current.value = "";
  };

  const handleUpload = async () => {
    const file = fileRef.current?.files?.[0];
    if (!file) {
      toast({
        title: "请先选择表格文件",
        status: "warning",
        duration: 2500,
        isClosable: true,
      });
      return;
    }
    const body = new FormData();
    body.append("file", file);
    body.append("overwrite", overwrite ? "true" : "false");
    try {
      const res = await fetchImport({
        data: body,
        headers: { "Content-Type": "multipart/form-data" },
      });
      if (res?.data?.code && res.data.code !== 0) {
        throw new Error(res?.data?.message || "导入失败");
      }
      const data = res?.data?.data as IntelNoticeImportResult;
      setResult(data);
      toast({
        title: `导入完成：新增 ${data.created}，更新 ${data.updated}，跳过 ${data.skipped}，失败 ${data.failed?.length || 0}`,
        status: data.failed?.length ? "warning" : "success",
        duration: 5000,
        isClosable: true,
      });
      onImported();
    } catch (error: any) {
      toast({
        title: "导入失败",
        description:
          error?.response?.data?.message || error?.message || "请检查表格格式",
        status: "error",
        duration: 5000,
        isClosable: true,
      });
    }
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={() => {
        reset();
        onClose();
      }}
      size="4xl"
      isCentered
      scrollBehavior="inside"
    >
      <ModalOverlay />
      <ModalContent maxH="90dvh" borderRadius="16px">
        <ModalHeader pb={2}>
          批量导入情报
          <Text mt={1} fontSize="sm" fontWeight="400" color="workbench.muted">
            支持 CSV 与 XLSX，第一行为表头；导入的信息来源统一标记为系统录入。
          </Text>
        </ModalHeader>
        <ModalCloseButton />
        <ModalBody>
          <Stack spacing={5}>
            <Flex gap={3} wrap="wrap" align="center">
              <Button
                as="a"
                href="/api/zb/intel/admin/notices/template"
                download
                h={FIELD_HEIGHT}
                variant="outline"
                borderColor="primary.200"
                color="primary.600"
                leftIcon={<FiDownload aria-hidden />}
                _hover={{ bg: "primary.50" }}
                _active={{ transform: "scale(0.98)" }}
              >
                下载模板（含示例行）
              </Button>
              <Text fontSize="sm" color="workbench.muted">
                建议先下载模板，照着示例行填好再上传。
              </Text>
            </Flex>

            <Box
              bg="workbench.canvas"
              borderRadius="lg"
              borderWidth="1px"
              borderColor="neutral.100"
              p={4}
            >
              <Text fontSize="sm" fontWeight="600" mb={2}>
                表格怎么填
              </Text>
              <TableContainer>
                <Table size="sm">
                  <Thead>
                    <Tr>
                      <Th>列名</Th>
                      <Th w="90px">是否必填</Th>
                      <Th>说明</Th>
                    </Tr>
                  </Thead>
                  <Tbody>
                    {COLUMN_GUIDE.map(([name, required, desc]) => (
                      <Tr key={name}>
                        <Td>
                          <Code fontSize="xs">{name}</Code>
                        </Td>
                        <Td>{required}</Td>
                        <Td whiteSpace="normal" fontSize="sm">
                          {desc}
                        </Td>
                      </Tr>
                    ))}
                  </Tbody>
                </Table>
              </TableContainer>
            </Box>

            <Box>
              <input
                ref={fileRef}
                type="file"
                accept=".csv,.xlsx"
                hidden
                onChange={(event) => {
                  setResult(null);
                  setFileName(event.target.files?.[0]?.name || "");
                }}
              />
              <HStack spacing={3} wrap="wrap">
                <Button
                  h={FIELD_HEIGHT}
                  variant="outline"
                  borderColor="neutral.200"
                  leftIcon={<FiUpload aria-hidden />}
                  onClick={() => fileRef.current?.click()}
                  _hover={{ borderColor: "primary.300", color: "primary.600" }}
                  _active={{ transform: "scale(0.98)" }}
                >
                  选择文件
                </Button>
                <Text
                  fontSize="sm"
                  color={fileName ? "workbench.text" : "workbench.muted"}
                  noOfLines={1}
                >
                  {fileName || "尚未选择文件（支持 .csv / .xlsx，5MB 以内）"}
                </Text>
              </HStack>
              <HStack spacing={3} mt={3} align="flex-start">
                <Switch
                  colorScheme="primary"
                  isChecked={overwrite}
                  onChange={(event) => setOverwrite(event.target.checked)}
                  aria-label="覆盖已存在的情报"
                />
                <Text fontSize="sm" color="workbench.muted">
                  覆盖已存在情报（按来源链接判定；关闭时重复链接整行跳过，
                  只新增缺失的情报）
                </Text>
              </HStack>
            </Box>

            {result && (
              <Alert
                status={result.failed?.length ? "warning" : "success"}
                borderRadius="lg"
                alignItems="flex-start"
              >
                <AlertIcon />
                <AlertDescription fontSize="sm">
                  <Text fontWeight="600">
                    新增 {result.created} · 更新 {result.updated} · 跳过{" "}
                    {result.skipped} · 失败 {result.failed?.length || 0}
                  </Text>
                  {result.batch && (
                    <Text mt={1}>
                      本次批次号 <Code fontSize="xs">{result.batch}</Code>
                      ，可在情报管理列表按批次筛选后批量操作。
                    </Text>
                  )}
                  {result.failed?.length > 0 && (
                    <Box mt={2}>
                      {result.failed.slice(0, 10).map((item) => (
                        <Text key={`${item.row}-${item.message}`}>
                          第 {item.row} 行
                          {item.title ? `（${item.title}）` : ""}：
                          {item.message}
                        </Text>
                      ))}
                      {result.failed.length > 10 && (
                        <Text>
                          其余 {result.failed.length - 10} 条失败记录已省略
                        </Text>
                      )}
                    </Box>
                  )}
                  <Text mt={2} color="workbench.muted">
                    {result.notice}
                  </Text>
                </AlertDescription>
              </Alert>
            )}
          </Stack>
        </ModalBody>
        <ModalFooter gap={3}>
          <Button
            variant="ghost"
            h={FIELD_HEIGHT}
            onClick={() => {
              reset();
              onClose();
            }}
          >
            关闭
          </Button>
          <Button
            h={FIELD_HEIGHT}
            colorScheme="primary"
            leftIcon={<FiUpload aria-hidden />}
            isLoading={noticeImportLoading}
            loadingText="导入中"
            onClick={handleUpload}
            _active={{ transform: "scale(0.98)" }}
          >
            开始导入
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
}
