"use client";

import { Badge, Flex, Box, Text } from "@chakra-ui/react";

interface AIBadgeProps {
  label?: string;
  size?: "sm" | "md";
}

/**
 * AI 功能标识 — 用于 AI 驱动的按钮、功能标签等
 * 深蓝→金色渐变，体现 "智能" 品牌感知
 */
export default function AIBadge({ label = "AI", size = "sm" }: AIBadgeProps) {
  return (
    <Badge
      variant="solid"
      bgGradient="linear(to-r, primary.600, gold.500)"
      color="white"
      borderRadius="full"
      px={size === "sm" ? 2.5 : 3}
      py={size === "sm" ? 0.5 : 1}
      textTransform="none"
      fontSize={size === "sm" ? "xs" : "sm"}
      fontWeight="600"
      boxShadow="0 1px 4px rgba(30, 58, 95, 0.25)"
      letterSpacing="0.02em"
    >
      <Flex align="center" gap={1}>
        <Box as="span" fontSize="xs" opacity={0.9}>
          ✦
        </Box>
        <Text>{label}</Text>
      </Flex>
    </Badge>
  );
}
