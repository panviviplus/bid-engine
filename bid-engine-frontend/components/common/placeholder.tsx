"use client";

import { Box, Flex, Heading, Text, VStack } from "@chakra-ui/react";
import { TimeIcon } from "@chakra-ui/icons";

interface PlaceholderProps {
  title: string;
  description?: string;
  icon?: "coming-soon" | "development" | "empty";
}

const icons = {
  "coming-soon": TimeIcon,
  "development": TimeIcon,
  "empty": TimeIcon,
};

/**
 * 功能占位组件 — 预留给尚未实现的功能模块
 * 显示 "功能开发中" 或自定义提示
 */
export default function Placeholder({
  title,
  description = "该功能正在开发中，敬请期待。",
  icon = "coming-soon",
}: PlaceholderProps) {
  const IconComponent = icons[icon] || TimeIcon;

  return (
    <Flex
      align="center"
      justify="center"
      minH="400px"
      w="full"
      bg="white"
      borderRadius="xl"
      border="1px dashed"
      borderColor="neutral.300"
    >
      <VStack spacing={4} maxW="360px" textAlign="center" px={6}>
        <Box
          bg="primary.50"
          p={4}
          borderRadius="full"
          color="primary.500"
        >
          <IconComponent boxSize={10} />
        </Box>
        <Heading fontSize="lg" fontWeight="600" color="neutral.800">
          {title}
        </Heading>
        <Text fontSize="sm" color="neutral.400" lineHeight="1.7">
          {description}
        </Text>
      </VStack>
    </Flex>
  );
}
