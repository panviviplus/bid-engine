"use client";

import { Box, Flex, Heading, Text, HStack } from "@chakra-ui/react";
import type { ReactNode } from "react";

interface PageHeaderProps {
  title: string;
  description?: string;
  children?: ReactNode;
}

/**
 * 统一页面标题栏 — 标题 + 描述 + 操作按钮
 * 所有功能模块页面的标准头部组件
 */
export default function PageHeader({ title, description, children }: PageHeaderProps) {
  return (
    <Flex
      align="flex-start"
      justify="space-between"
      wrap="wrap"
      gap={4}
      mb={6}
    >
      <Box minW={0} flex="1 1 24rem">
        <Heading
          fontSize={{ base: "1.5rem", md: "1.75rem" }}
          fontWeight="700"
          color="neutral.900"
          letterSpacing="-0.02em"
          overflowWrap="anywhere"
        >
          {title}
        </Heading>
        {description && (
          <Text mt={1} color="neutral.500" fontSize="sm">
            {description}
          </Text>
        )}
      </Box>
      {children && (
        <HStack
          spacing={3}
          flexWrap="wrap"
          w={{ base: "full", sm: "auto" }}
          justify={{ base: "flex-start", sm: "flex-end" }}
        >
          {children}
        </HStack>
      )}
    </Flex>
  );
}
