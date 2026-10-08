"use client";

/* Hallmark · component: intel-source-import · genre: modern-minimal · theme: BidEngine deep-sea/gold (preserved tokens)
 * states: default · hover · focus-visible · active · disabled · loading(上传中) · error · success
 * 交互：先给模板，再告诉管理员每列怎么填，最后才让上传；导入结果逐行反馈。
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

import {
  useIntelSourceImport,
  type IntelSourceImportResult,
} from "@/service/intel";

const FIELD_H = "44px";

/** 表格列说明，与后端 sourceImportColumns 保持一致。 */
const COLUMN_GUIDE: Array<[string, string, string]> = [
  ["source_key", "必填", "小写字母/数字/下划线，2-64 位，全平台唯一"],
  ["name", "必填", "采集源展示名称"],
  ["homepage_url", "可选", "门户地址，以 http(s):// 开头"],
  ["list_url", "必填", "招标公告列表页地址，以 http(s):// 开头"],
  ["category", "可选", "国家级 / 地方级 / 国央企 / 银行 / 高校 / 民营 / 其他"],
  ["region", "可选", "地区，例如 广东省、深圳市"],
  ["industry_hint", "可选", "网站行业属性，例如 通用、电力、金融"],
  [
    "discovery_mode",
    "可选",
    "api=站内接口，list=列表页（默认），browser=需要渲染",
  ],
  ["needs_browser", "可选", "1/是 表示必须用无头浏览器渲染；留空按 0 处理"],
  ["enabled", "可选", "1/是 启用（默认），0/否 停用"],
  ["priority", "可选", "0-9999，数字越小越先执行，默认 100"],
  [
    "params",
    "可选",
    '采集规则覆盖（JSON），例如 {"linkPattern":"/zbgg/\\\\d+","contentSelectors":[".content"]}',
  ],
];

export default function SourceImportModal({
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
  const [result, setResult] = useState<IntelSourceImportResult | null>(null);
  const { importLoading, fetchImport } = useIntelSourceImport();

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
      const data = res?.data?.data as IntelSourceImportResult;
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
      size="3xl"
      isCentered
      scrollBehavior="inside"
    >
      <ModalOverlay />
      <ModalContent maxH="88dvh">
        <ModalHeader pb={2}>
          导入采集源
          <Text mt={1} fontSize="sm" fontWeight="400" color="workbench.muted">
            支持 CSV 与 XLSX，第一行为表头；同一份表格可反复导入，重复的
            source_key 默认跳过。
          </Text>
        </ModalHeader>
        <ModalCloseButton />
        <ModalBody>
          <Stack spacing={5}>
            <Flex gap={3} wrap="wrap" align="center">
              <Button
                as="a"
                href="/api/zb/intel/sources/template"
                download
                h={FIELD_H}
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
                建议先下载模板，照着示例填好再上传。
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
                  h={FIELD_H}
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
              <HStack spacing={3} mt={3}>
                <Switch
                  colorScheme="primary"
                  isChecked={overwrite}
                  onChange={(event) => setOverwrite(event.target.checked)}
                  aria-label="覆盖已存在的采集源"
                />
                <Text fontSize="sm" color="workbench.muted">
                  覆盖同名采集源（关闭时重复 source_key
                  会被跳过，只新增缺失的源）
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
                  {result.failed?.length > 0 && (
                    <Box mt={2}>
                      {result.failed.slice(0, 10).map((item) => (
                        <Text key={`${item.row}-${item.message}`}>
                          第 {item.row} 行
                          {item.source_key ? `（${item.source_key}）` : ""}：
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
            h={FIELD_H}
            onClick={() => {
              reset();
              onClose();
            }}
          >
            关闭
          </Button>
          <Button
            h={FIELD_H}
            colorScheme="primary"
            leftIcon={<FiUpload aria-hidden />}
            isLoading={importLoading}
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
