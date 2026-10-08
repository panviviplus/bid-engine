"use client";

import { Badge, Box, Flex, Text } from "@chakra-ui/react";

type StatusType = "success" | "warning" | "error" | "info" | "processing" | "default";

const statusConfig: Record<StatusType, { color: string; bg: string; dot: string; label: string }> = {
  success: { color: "success.600", bg: "success.50", dot: "success.500", label: "成功" },
  warning: { color: "warning.600", bg: "warning.50", dot: "warning.500", label: "警告" },
  error: { color: "error.600", bg: "error.50", dot: "error.500", label: "失败" },
  info: { color: "info.600", bg: "info.50", dot: "info.500", label: "信息" },
  processing: { color: "primary.600", bg: "primary.50", dot: "primary.500", label: "处理中" },
  default: { color: "neutral.500", bg: "neutral.100", dot: "neutral.400", label: "默认" },
};

interface StatusBadgeProps {
  status: StatusType;
  label?: string;
  showDot?: boolean;
}

/**
 * 统一状态标签 — 所有列表/卡片的状态展示
 * 带颜色圆点 + 文字 + 浅色背景
 */
export default function StatusBadge({ status, label, showDot = true }: StatusBadgeProps) {
  const config = statusConfig[status] || statusConfig.default;
  return (
    <Badge
      variant="subtle"
      bg={config.bg}
      color={config.color}
      borderRadius="full"
      px={3}
      py={1}
      textTransform="none"
      fontSize="xs"
      fontWeight="500"
    >
      <Flex align="center" gap={1.5}>
        {showDot && (
          <Box
            w={2}
            h={2}
            borderRadius="full"
            bg={config.dot}
          />
        )}
        <Text>{label || config.label}</Text>
      </Flex>
    </Badge>
  );
}

export { type StatusType };
