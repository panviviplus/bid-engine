"use client";

/* Hallmark · pre-emit critique: P4 H5 E4 S4 R5 V4 */
/* Hallmark · component: bid-doc-preview · genre: modern-minimal · theme: smart-bid (existing tokens)
 * states: default · hover · focus-visible · active · disabled · loading · error · success
 * contrast: pass (workbench.text on workbench.paper, white on primary.600, workbench.muted on workbench.canvas)
 */
import {
  Box,
  Button,
  Flex,
  IconButton,
  Modal,
  ModalBody,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  Skeleton,
  Text,
  usePrefersReducedMotion,
} from "@chakra-ui/react";
import { FiX } from "react-icons/fi";
import { LuFileText } from "react-icons/lu";

import type { BidDocxCover, BidDocxTocEntry } from "./docx-export";

type Props = {
  isOpen: boolean;
  onClose: () => void;
  cover: BidDocxCover;
  toc: BidDocxTocEntry[];
  /** 最近一次导出的页码定位是否进行中 */
  loading?: boolean;
  /** 页码定位失败原因（目录仍会照常渲染，仅缺页码） */
  errorMessage?: string;
};

function fieldLine(label: string, value?: string): string | null {
  const text = String(value || "").trim();
  if (!text) return null;
  return `${label}：${text}`;
}

function indentation(level: number): string {
  const depth = Math.min(Math.max(level, 1), 4) - 1;
  return `${depth * 18}px`;
}

/**
 * 只读文档预览：按导出装配的样子渲染封面页、目录页与页眉页脚，
 * 让所见即所得在编辑阶段就能核对，而不必先导出再检查。
 */
export default function BidDocPreview({
  isOpen,
  onClose,
  cover,
  toc,
  loading = false,
  errorMessage,
}: Props) {
  const prefersReducedMotion = usePrefersReducedMotion();
  const coverFields = [
    fieldLine("项目编号", cover.projectNumber),
    fieldLine("标　　段", cover.lotLabel),
    fieldLine("招 标 人", cover.tendererName),
    fieldLine("投 标 人", cover.bidderName),
  ].filter((line): line is string => Boolean(line));
  const headerText = [cover.projectName, cover.projectNumber]
    .map((value) => String(value || "").trim())
    .filter(Boolean)
    .join("　");

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      size="4xl"
      motionPreset={prefersReducedMotion ? "none" : "scale"}
    >
      <ModalOverlay bg="blackAlpha.500" backdropFilter="blur(2px)" />
      <ModalContent
        maxW="900px"
        maxH="86vh"
        h="86vh"
        borderRadius="2xl"
        overflow="hidden"
        boxShadow="0 24px 64px rgba(30,58,95,0.22)"
        display="flex"
        flexDirection="column"
      >
        <ModalHeader px={5} pt={4} pb={3} flexShrink={0}>
          <Flex align="center" gap={2.5} pr={8}>
            <Flex
              w="8"
              h="8"
              borderRadius="lg"
              align="center"
              justify="center"
              bgGradient="linear(to-br, primary.600, gold.500)"
              color="white"
              flexShrink={0}
            >
              <LuFileText size={17} />
            </Flex>
            <Box minW={0}>
              <Text fontWeight="600" color="workbench.text" fontSize="md">
                文档预览
              </Text>
              <Text fontSize="xs" color="workbench.muted">
                封面页 · 目录页 · 页眉页脚按导出格式渲染
              </Text>
            </Box>
          </Flex>
          <IconButton
            aria-label="关闭预览"
            icon={<FiX />}
            size="sm"
            variant="ghost"
            color="workbench.muted"
            position="absolute"
            top={4}
            right={4}
            transition="background-color var(--dur-micro) var(--ease-out)"
            _hover={{ bg: "neutral.100", color: "workbench.text" }}
            _focusVisible={{
              outline: "2px solid",
              outlineColor: "gold.400",
              outlineOffset: "2px",
            }}
            _active={{ bg: "neutral.200" }}
          />
        </ModalHeader>

        <ModalBody
          px={{ base: 3, md: 5 }}
          pb={5}
          pt={1}
          overflowY="auto"
          minW={0}
          bg="workbench.canvas"
        >
          {loading && (
            <Flex direction="column" gap={3} py={2}>
              <Skeleton height="24px" borderRadius="md" />
              <Skeleton height="200px" borderRadius="lg" />
              <Skeleton height="24px" borderRadius="md" />
            </Flex>
          )}

          {!loading && errorMessage && (
            <Flex
              mb={3}
              px={3}
              py={2}
              borderRadius="md"
              border="1px solid"
              borderColor="warning.200"
              bg="warning.50"
            >
              <Text fontSize="xs" color="warning.700">
                页码定位未完成：{errorMessage}
              </Text>
            </Flex>
          )}

          {!loading && (
            <Flex direction="column" gap={4}>
              {/* 封面页 */}
              <Box
                bg="workbench.paper"
                border="1px solid"
                borderColor="workbench.line"
                borderRadius="6px"
                boxShadow="0 10px 26px rgba(24,38,54,0.08)"
                px={{ base: 4, md: 8 }}
                py={{ base: 8, md: 12 }}
                minH={{ base: "260px", md: "360px" }}
                display="flex"
                flexDirection="column"
                alignItems="center"
                justifyContent="space-between"
                textAlign="center"
              >
                <Text
                  fontSize={{ base: "md", md: "lg" }}
                  fontWeight="600"
                  color="workbench.text"
                  fontFamily="var(--font-display)"
                >
                  {String(cover.projectName || "").trim() || "（项目名称待补充）"}
                </Text>
                <Text
                  fontSize={{ base: "2xl", md: "4xl" }}
                  fontWeight="700"
                  color="workbench.text"
                  letterSpacing="0.28em"
                  lineHeight="1.3"
                >
                  {String(cover.docTitle || "投 标 文 件").trim()}
                </Text>
                <Box w="100%">
                  {coverFields.length === 0 ? (
                    <Text fontSize="sm" color="workbench.muted">
                      项目编号 / 招标人 / 投标人信息将在解析后自动填充
                    </Text>
                  ) : (
                    coverFields.map((line) => (
                      <Text
                        key={line}
                        fontSize={{ base: "sm", md: "md" }}
                        color="workbench.text"
                        lineHeight="2"
                      >
                        {line}
                      </Text>
                    ))
                  )}
                  <Text
                    mt={4}
                    fontSize={{ base: "sm", md: "md" }}
                    color="workbench.text"
                  >
                    {String(cover.date || "").trim() ||
                      new Date().toLocaleDateString("zh-CN")}
                  </Text>
                </Box>
              </Box>

              {/* 目录页 */}
              <Box
                bg="workbench.paper"
                border="1px solid"
                borderColor="workbench.line"
                borderRadius="6px"
                boxShadow="0 10px 26px rgba(24,38,54,0.08)"
                px={{ base: 4, md: 8 }}
                py={{ base: 6, md: 8 }}
                minW={0}
              >
                <Text
                  textAlign="center"
                  fontWeight="700"
                  fontSize={{ base: "lg", md: "xl" }}
                  letterSpacing="0.3em"
                  color="workbench.text"
                  mb={5}
                >
                  目　　录
                </Text>
                {toc.length === 0 ? (
                  <Text fontSize="sm" color="workbench.muted" textAlign="center">
                    确认大纲后，目录条目会自动出现在这里
                  </Text>
                ) : (
                  <Flex direction="column" gap={1.5}>
                    {toc.map((entry) => (
                      <Flex
                        key={`${entry.outlineId}-${entry.title}`}
                        align="baseline"
                        gap={2}
                        pl={indentation(entry.level)}
                        minW={0}
                      >
                        <Text
                          fontSize="sm"
                          color="workbench.text"
                          fontWeight={entry.level === 1 ? "600" : "400"}
                          minW={0}
                          overflowWrap="anywhere"
                        >
                          {entry.title}
                        </Text>
                        <Box
                          flex="1"
                          minW="24px"
                          borderBottom="1px dashed"
                          borderColor="workbench.line"
                          transform="translateY(-3px)"
                        />
                        <Text
                          fontSize="sm"
                          color={
                            entry.page ? "workbench.text" : "workbench.muted"
                          }
                          fontFamily="var(--font-outlier)"
                        >
                          {entry.page ?? "—"}
                        </Text>
                      </Flex>
                    ))}
                  </Flex>
                )}
              </Box>

              {/* 页眉页脚 */}
              <Box
                bg="workbench.paper"
                border="1px solid"
                borderColor="workbench.line"
                borderRadius="6px"
                px={{ base: 4, md: 6 }}
                py={5}
              >
                <Text
                  fontSize="xs"
                  color="workbench.muted"
                  mb={1}
                  letterSpacing="0.08em"
                >
                  正文页眉 / 页脚
                </Text>
                <Flex
                  justify="center"
                  borderBottom="1px solid"
                  borderColor="workbench.line"
                  pb={2}
                  mb={4}
                >
                  <Text fontSize="xs" color="workbench.muted">
                    {headerText || "项目名称 / 项目编号"}
                  </Text>
                </Flex>
                <Flex
                  justify="center"
                  borderTop="1px solid"
                  borderColor="workbench.line"
                  pt={2}
                >
                  <Text fontSize="xs" color="workbench.muted">
                    第 1 页　共 N 页（正文节页码从 1 起算，封面与目录不占正文页码）
                  </Text>
                </Flex>
              </Box>
            </Flex>
          )}
        </ModalBody>

        <ModalFooter
          px={5}
          py={3}
          gap={3}
          flexShrink={0}
          borderTop="1px solid"
          borderColor="workbench.line"
          bg="workbench.paper"
        >
          <Text fontSize="xs" color="workbench.muted" mr="auto" minW={0}>
            页码取自最近一次导出；用 Word 打开时目录会自动刷新。
          </Text>
          <Button
            size="sm"
            bg="primary.600"
            color="white"
            borderRadius="8px"
            transition="background-color var(--dur-micro) var(--ease-out)"
            _hover={{ bg: "primary.700" }}
            _focusVisible={{
              outline: "2px solid",
              outlineColor: "gold.400",
              outlineOffset: "2px",
            }}
            _active={{ bg: "primary.800" }}
            onClick={onClose}
          >
            关闭
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
}
