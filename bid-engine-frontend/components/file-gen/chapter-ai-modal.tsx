"use client";

/* eslint-disable no-unused-vars */
/* Hallmark · component: chapter-ai-modal · genre: modern-minimal · theme: smart-bid (existing tokens)
 * states: default · hover · focus-visible · active · disabled · loading · error · success
 * contrast: pass (primary.600 on white, gray.600 on neutral.50, white on primary.600)
 */
import { useCallback, useEffect, useState } from "react";
import {
  Box,
  Button,
  Flex,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  Text,
  usePrefersReducedMotion,
} from "@chakra-ui/react";
import { FiRotateCcw, FiZoomIn, FiZoomOut, FiX } from "react-icons/fi";
import { LuSparkles } from "react-icons/lu";
import type { GenMode } from "@/service/bid-gen";

type Props = {
  isOpen: boolean;
  outlineId: number;
  title: string;
  /** 当前功能面板篇幅档位（重写默认目标） */
  defaultLength: string;
  onClose: () => void;
  onConfirm: (mode: GenMode, tier: string, instruction: string) => void;
};

// 篇幅档位：口径为全篇字数下限，与后端 concise/standard/detailed 三档一一对应
const TIERS = [
  { key: "concise", label: "精简", desc: "3000 字+" },
  { key: "standard", label: "标准", desc: "5000 字+" },
  { key: "detailed", label: "详细", desc: "8000 字+" },
];

const OPERATIONS: {
  key: GenMode;
  label: string;
  desc: string;
  icon: React.ElementType;
  defaultTier: (current: string) => string;
}[] = [
  {
    key: "rewrite",
    label: "重写本章",
    desc: "保持原意重写，语言更专业精炼",
    icon: FiRotateCcw,
    defaultTier: (current) => current || "standard",
  },
  {
    key: "expand",
    label: "扩写",
    desc: "在保留原内容基础上补充论证与细节",
    icon: FiZoomIn,
    defaultTier: () => "detailed",
  },
  {
    key: "condense",
    label: "缩写",
    desc: "精简压缩，保留全部核心要点",
    icon: FiZoomOut,
    defaultTier: () => "concise",
  },
];

function tierLabel(key: string): string {
  return TIERS.find((t) => t.key === key)?.label || "标准";
}

function tierDesc(key: string): string {
  const t = TIERS.find((x) => x.key === key);
  return t ? `全篇 ${t.desc}` : "全篇 5000 字+";
}

export default function ChapterAiModal({
  isOpen,
  outlineId,
  title,
  defaultLength,
  onClose,
  onConfirm,
}: Props) {
  const prefersReducedMotion = usePrefersReducedMotion();
  const [mode, setMode] = useState<GenMode>("rewrite");
  const [tier, setTier] = useState<string>("standard");
  const [instruction, setInstruction] = useState("");

  // 打开时按弹层目标重置：操作默认重写本章，篇幅默认=当前面板档位
  useEffect(() => {
    if (!isOpen) return;
    setMode("rewrite");
    setTier(defaultLength || "standard");
    setInstruction("");
  }, [isOpen, defaultLength]);

  const chooseMode = useCallback(
    (m: GenMode) => {
      setMode(m);
      const op = OPERATIONS.find((o) => o.key === m);
      setTier(op ? op.defaultTier(defaultLength) : "standard");
    },
    [defaultLength],
  );

  const op = OPERATIONS.find((o) => o.key === mode) || OPERATIONS[0];
  const ModeIcon = op.icon;

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      isCentered
      motionPreset={prefersReducedMotion ? "none" : "scale"}
      size="md"
    >
      <ModalOverlay bg="blackAlpha.500" backdropFilter="blur(2px)" />
      <ModalContent
        maxW="440px"
        borderRadius="2xl"
        boxShadow="0 24px 64px rgba(30,58,95,0.22)"
        overflow="hidden"
      >
        <ModalHeader px={5} pt={4} pb={0}>
          <Flex align="center" gap={2.5}>
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
              <LuSparkles size="16" />
            </Flex>
            <Box minW="0">
              <Text fontSize="sm" fontWeight="700" color="gray.800" noOfLines={1}>
                {title || "未命名章节"}
              </Text>
              <Text fontSize="xs" color="gray.400">
                章节 AI 改写
              </Text>
            </Box>
            <Box flex="1" />
            <Button
              size="xs"
              variant="ghost"
              color="gray.400"
              aria-label="关闭"
              _hover={{ color: "gray.600", bg: "gray.50" }}
              _focusVisible={{
                boxShadow: "0 0 0 2px var(--chakra-colors-primary-400)",
                outline: "none",
              }}
              onClick={onClose}
            >
              <FiX size="14" />
            </Button>
          </Flex>
        </ModalHeader>

        <ModalBody px={5} py={4}>
          {/* 操作选择 */}
          <Flex direction="column" gap={2} role="radiogroup" aria-label="改写操作">
            {OPERATIONS.map((o) => {
              const Icon = o.icon;
              const selected = mode === o.key;
              return (
                <Box
                  key={o.key}
                  role="radio"
                  aria-checked={selected}
                  tabIndex={0}
                  cursor="pointer"
                  px={3}
                  py={2.5}
                  borderRadius="lg"
                  border="1.5px solid"
                  borderColor={selected ? "primary.400" : "gray.200"}
                  bg={selected ? "primary.50" : "white"}
                  boxShadow={
                    selected
                      ? "inset 0 0 0 1px var(--chakra-colors-primary-400)"
                      : undefined
                  }
                  _hover={{
                    borderColor: selected ? "primary.400" : "primary.300",
                    bg: selected ? "primary.50" : "neutral.50",
                  }}
                  _focusVisible={{
                    boxShadow: "0 0 0 3px rgba(212,168,83,0.45)",
                    outline: "none",
                  }}
                  _active={{ transform: prefersReducedMotion ? undefined : "scale(0.99)" }}
                  transition="border-color 0.15s ease, background-color 0.15s ease"
                  onClick={() => chooseMode(o.key)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      chooseMode(o.key);
                    }
                  }}
                >
                  <Flex align="center" gap={2.5}>
                    <Flex
                      w="7"
                      h="7"
                      borderRadius="md"
                      align="center"
                      justify="center"
                      bg={selected ? "primary.600" : "neutral.100"}
                      color={selected ? "white" : "gray.500"}
                      flexShrink={0}
                      transition="background-color 0.15s ease, color 0.15s ease"
                    >
                      <Icon size="14" />
                    </Flex>
                    <Box minW="0">
                      <Text fontSize="sm" fontWeight="600" color="gray.800">
                        {o.label}
                      </Text>
                      <Text fontSize="xs" color="gray.500" noOfLines={1}>
                        {o.desc}
                      </Text>
                    </Box>
                    <Box flex="1" />
                    <Box
                      w="3.5"
                      h="3.5"
                      borderRadius="full"
                      border="1.5px solid"
                      borderColor={selected ? "primary.500" : "gray.300"}
                      bg={selected ? "primary.500" : "white"}
                      boxShadow={
                        selected ? "inset 0 0 0 2px white" : undefined
                      }
                      flexShrink={0}
                    />
                  </Flex>
                </Box>
              );
            })}
          </Flex>

          {/* 目标篇幅 */}
          <Box mt={4}>
            <Text fontSize="xs" fontWeight="600" color="gray.600" mb={2}>
              目标篇幅
            </Text>
            <Flex gap={1.5} role="radiogroup" aria-label="目标篇幅">
              {TIERS.map((t) => {
                const selected = tier === t.key;
                return (
                  <Button
                    key={t.key}
                    size="sm"
                    flex="1"
                    role="radio"
                    aria-checked={selected}
                    variant={selected ? "solid" : "outline"}
                    bg={selected ? "primary.600" : "white"}
                    color={selected ? "white" : "gray.600"}
                    borderColor="primary.200"
                    borderRadius="lg"
                    _hover={{ opacity: 0.9 }}
                    _focusVisible={{
                      boxShadow: "0 0 0 3px rgba(212,168,83,0.45)",
                      outline: "none",
                    }}
                    onClick={() => setTier(t.key)}
                    title={`篇幅：${t.desc}`}
                  >
                    {t.label}
                  </Button>
                );
              })}
            </Flex>
            <Text fontSize="xs" color="gray.500" mt={2}>
              {op.label} · 目标篇幅：{tierLabel(tier)}（{tierDesc(tier)}）
            </Text>
          </Box>

          {/* 补充要求（可选） */}
          <Box mt={4}>
            <Input
              size="sm"
              placeholder="补充要求（可选），如：强调履约能力"
              value={instruction}
              onChange={(e) => setInstruction(e.target.value)}
              bg="gray.50"
              borderColor="gray.200"
              borderRadius="lg"
              _hover={{ borderColor: "gray.300", bg: "white" }}
              _focusVisible={{
                boxShadow: "0 0 0 2px var(--chakra-colors-primary-400)",
                borderColor: "primary.400",
                outline: "none",
                bg: "white",
              }}
            />
          </Box>
        </ModalBody>

        <ModalFooter px={5} py={4} borderTop="1px solid" borderColor="gray.100" gap={3}>
          <Button
            size="sm"
            variant="ghost"
            onClick={onClose}
            _focusVisible={{
              boxShadow: "0 0 0 2px var(--chakra-colors-primary-400)",
              outline: "none",
            }}
          >
            取消
          </Button>
          <Button
            size="sm"
            bg="primary.600"
            color="white"
            leftIcon={<LuSparkles size="14" />}
            _hover={{ bg: "primary.700", transform: "translateY(-1px)" }}
            _active={{ bg: "primary.800" }}
            _focusVisible={{
              boxShadow: "0 0 0 3px rgba(212,168,83,0.45)",
              outline: "none",
            }}
            isDisabled={!outlineId}
            onClick={() => onConfirm(mode, tier, instruction.trim())}
          >
            开始{op.label}
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
}
