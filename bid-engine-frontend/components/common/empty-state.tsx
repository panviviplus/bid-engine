"use client";

import { Box, Flex, Text } from "@chakra-ui/react";
import type { ReactNode } from "react";

interface EmptyStateProps {
  icon?: ReactNode;
  title: string;
  description?: string;
  action?: ReactNode;
}

/**
 * Hallmark · component: empty-state · genre: modern-minimal · theme: Cobalt (adapted)
 * 空态（招标解析 / 标书生成共用）：图标 + 标题 + 说明 + 行动按钮
 */
export default function EmptyState({
  icon,
  title,
  description,
  action,
}: EmptyStateProps) {
  return (
    <Flex
      direction="column"
      align="center"
      justify="center"
      gap={4}
      py={16}
      px={6}
      bg="white"
      borderRadius="xl"
      border="1px dashed"
      borderColor="neutral.300"
    >
      {icon && (
        <Flex
          w="16"
          h="16"
          borderRadius="full"
          align="center"
          justify="center"
          bg="primary.50"
          color="primary.500"
          fontSize="3xl"
        >
          {icon}
        </Flex>
      )}
      <Box textAlign="center">
        <Text fontWeight="700" color="neutral.700">
          {title}
        </Text>
        {description && (
          <Text fontSize="sm" color="neutral.400" mt={1}>
            {description}
          </Text>
        )}
      </Box>
      {action}
    </Flex>
  );
}
