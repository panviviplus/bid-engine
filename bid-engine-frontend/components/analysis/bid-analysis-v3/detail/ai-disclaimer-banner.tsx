"use client";

/*
 * Hallmark · component: banner · genre: modern-minimal · theme: deep-sea workbench
 * states: default · hover · focus · active · disabled · dismissed
 * contrast: pass (gold.200 border on gold.50 surface, ink text)
 */

import {
  Box,
  Flex,
  HStack,
  Icon,
  IconButton,
  Text,
} from "@chakra-ui/react";
import { useState } from "react";
import { FiX, FiZap } from "react-icons/fi";

/**
 * AiDisclaimerBanner —— 详情页顶部的 AI 生成提示条，点击横幅或关闭按钮即可关闭（会话内状态）。
 * 沿用 V2 的提示语义，外观按 V3 workbench 设计令牌重绘：gold 为 AI/证据信号色。
 */
export function AiDisclaimerBanner() {
  const [dismissed, setDismissed] = useState(false);

  if (dismissed) return null;

  return (
    <Flex
      role="status"
      aria-label="AI 生成内容提示"
      flexShrink={0}
      mb={3}
      px={{ base: 3, md: 4 }}
      py={2}
      minH="44px"
      align="center"
      justify="space-between"
      gap={3}
      cursor="pointer"
      borderRadius="12px"
      bg="gold.50"
      border="1px solid"
      borderColor="gold.200"
      boxShadow="0 4px 14px rgba(11, 27, 43, 0.05)"
      transition="background-color 160ms cubic-bezier(0.16,1,0.3,1), border-color 160ms cubic-bezier(0.16,1,0.3,1), transform 100ms cubic-bezier(0.16,1,0.3,1)"
      onClick={() => setDismissed(true)}
      _hover={{ bg: "gold.100", borderColor: "gold.300" }}
      _active={{ transform: "translateY(1px)" }}
      _focusWithin={{
        boxShadow: "0 0 0 3px var(--chakra-colors-gold-300)",
      }}
      title="点击关闭"
    >
      <HStack minW={0} spacing={2.5}>
        <Flex
          w="28px"
          h="28px"
          flexShrink={0}
          borderRadius="8px"
          align="center"
          justify="center"
          bg="gold.200"
          color="gold.700"
        >
          <Icon as={FiZap} boxSize={3.5} />
        </Flex>
        <Text
          fontSize="xs"
          color="neutral.700"
          fontWeight="560"
          letterSpacing="0.02em"
          lineHeight="1.6"
        >
          内容均由 AI 提炼生成，仅供参考，请注意甄别
        </Text>
      </HStack>
      <Box flexShrink={0}>
        <IconButton
          aria-label="关闭 AI 提示"
          icon={<FiX />}
          size="sm"
          minW="36px"
          minH="36px"
          variant="ghost"
          color="gold.700"
          onClick={(event) => {
            event.stopPropagation();
            setDismissed(true);
          }}
          _hover={{ bg: "gold.100", color: "gold.800" }}
          _active={{ transform: "translateY(1px)" }}
          _focusVisible={{
            boxShadow: "0 0 0 3px var(--chakra-colors-gold-300)",
          }}
        />
      </Box>
    </Flex>
  );
}

export default AiDisclaimerBanner;
