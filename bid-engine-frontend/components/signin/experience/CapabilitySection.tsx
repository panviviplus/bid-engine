"use client";

import { ArrowBackIcon, ArrowForwardIcon, CheckIcon } from "@chakra-ui/icons";
import {
  Badge,
  Box,
  Button,
  Container,
  Flex,
  Grid,
  Heading,
  HStack,
  IconButton,
  SimpleGrid,
  Text,
  VStack,
} from "@chakra-ui/react";
import { motion } from "framer-motion";
import { AUTH_PAGE_GUTTERS } from "@/components/layout/responsive-layout.mjs";

import {
  ASSET_TAGS,
  SCREEN_TWO_COPY,
  SIGNIN_STAGES,
} from "./signin-content.mjs";
import { getNextIndex } from "./rotation.mjs";
import { useAutoRotation } from "./use-auto-rotation";

const demoPanelStyle = {
  p: { base: 4, md: 6 },
  borderRadius: "14px",
  bg: "neutral.50",
  border: "1px solid",
  borderColor: "neutral.200",
};

function IntelligenceDemo() {
  return (
    <VStack align="stretch" spacing={3}>
      {["轨道交通设备采购项目", "智慧园区建设项目", "信息化平台升级项目"].map(
        (name, index) => {
          const status = ["匹配", "待评估", "关注"][index];
          return (
            <Flex key={name} {...demoPanelStyle} py={4} align="center" gap={3}>
              <Box boxSize="10px" borderRadius="full" bg="primary.500" />
              <Text flex={1} fontSize="sm" fontWeight="700">
                {name}
              </Text>
              <Badge colorScheme={index === 0 ? "blue" : "gray"}>
                {status}
              </Badge>
            </Flex>
          );
        },
      )}
    </VStack>
  );
}

function AnalysisDemo() {
  return (
    <Grid templateColumns={{ base: "1fr", md: "minmax(0, 1fr) 13rem" }} gap={4}>
      <Box {...demoPanelStyle}>
        {[74, 94, 82, 90, 68, 88].map((width, index) => (
          <Box
            key={`${width}-${index}`}
            h="8px"
            w={`${width}%`}
            mt={index ? 3 : 0}
            borderRadius="full"
            bg={index === 1 || index === 4 ? "gold.200" : "neutral.200"}
          />
        ))}
      </Box>
      <Box {...demoPanelStyle}>
        <Text fontSize="sm" fontWeight="800">
          已识别
        </Text>
        <VStack align="stretch" mt={3} spacing={2}>
          {["资格条件", "评分办法", "响应要求", "否决风险"].map((item) => (
            <Text
              key={item}
              p={2.5}
              borderRadius="md"
              bg="primary.50"
              fontSize="sm"
            >
              {item}
            </Text>
          ))}
        </VStack>
      </Box>
    </Grid>
  );
}

function BidReviewDemo() {
  return (
    <SimpleGrid columns={{ base: 1, sm: 2 }} spacing={3}>
      {[
        ["主体资格", "已满足", "green"],
        ["同类业绩", "待补充", "yellow"],
        ["核心人员", "可覆盖", "green"],
        ["交付风险", "需确认", "gray"],
      ].map(([title, status, scheme]) => (
        <Box key={title} {...demoPanelStyle}>
          <Text fontSize="sm" fontWeight="800">
            {title}
          </Text>
          <Badge mt={3} colorScheme={scheme}>
            {status}
          </Badge>
        </Box>
      ))}
    </SimpleGrid>
  );
}

function GenerationDemo() {
  return (
    <Grid templateColumns={{ base: "1fr", md: "13rem minmax(0, 1fr)" }} gap={4}>
      <Box {...demoPanelStyle}>
        <VStack align="stretch" spacing={3}>
          {["一、投标函", "二、资格证明", "三、技术方案", "四、项目业绩"].map(
            (item) => (
              <Text key={item} fontSize="sm">
                {item}
              </Text>
            ),
          )}
        </VStack>
      </Box>
      <Box {...demoPanelStyle}>
        <Text fontSize="sm" fontWeight="800">
          技术方案
        </Text>
        {[92, 86, 96, 72, 88].map((width, index) => (
          <Box
            key={`${width}-${index}`}
            mt={3}
            h="8px"
            w={`${width}%`}
            bg="neutral.200"
            borderRadius="full"
          />
        ))}
      </Box>
    </Grid>
  );
}

function DocumentReviewDemo() {
  return (
    <Grid templateColumns={{ base: "1fr", md: "minmax(0, 1fr) 16rem" }} gap={4}>
      <Box {...demoPanelStyle}>
        <HStack spacing={2} mb={4}>
          {["销", "技", "法"].map((role) => (
            <Flex
              key={role}
              boxSize="32px"
              align="center"
              justify="center"
              borderRadius="full"
              bg="primary.600"
              color="white"
              fontSize="xs"
              fontWeight="800"
            >
              {role}
            </Flex>
          ))}
        </HStack>
        <VStack align="stretch" spacing={2}>
          {["响应项已逐条覆盖", "证明材料需要补充", "技术亮点建议前置"].map(
            (item) => (
              <Text key={item} p={3} borderRadius="md" bg="white" fontSize="sm">
                {item}
              </Text>
            ),
          )}
        </VStack>
      </Box>
      <Box {...demoPanelStyle}>
        <Text fontSize="sm" fontWeight="800">
          审核清单
        </Text>
        <VStack align="stretch" mt={3} spacing={3}>
          {["合规性 · 已核对", "完整性 · 待完善", "竞争力 · 建议优化"].map(
            (item) => (
              <Text key={item} fontSize="sm" color="neutral.600">
                {item}
              </Text>
            ),
          )}
        </VStack>
      </Box>
    </Grid>
  );
}

function DeliveryDemo() {
  return (
    <Flex
      minH="15rem"
      align="center"
      justify="center"
      borderRadius="14px"
      bg="success.50"
      textAlign="center"
    >
      <Box>
        <Flex
          mx="auto"
          boxSize="48px"
          align="center"
          justify="center"
          borderRadius="full"
          bg="success.100"
          color="success.700"
        >
          <CheckIcon boxSize={5} />
        </Flex>
        <Text mt={4} color="success.800" fontSize="lg" fontWeight="800">
          投标文件可导出
        </Text>
        <Text mt={2} color="success.700" fontSize="sm">
          内容、附件与审核结果已确认
        </Text>
      </Box>
    </Flex>
  );
}

function StageDemo({ stageKey }: { stageKey: string }) {
  switch (stageKey) {
    case "intelligence":
      return <IntelligenceDemo />;
    case "analysis":
      return <AnalysisDemo />;
    case "bid-review":
      return <BidReviewDemo />;
    case "generation":
      return <GenerationDemo />;
    case "document-review":
      return <DocumentReviewDemo />;
    default:
      return <DeliveryDemo />;
  }
}

export default function CapabilitySection() {
  const { index, selectIndex, reducedMotion, interactionProps } =
    useAutoRotation({
      count: SIGNIN_STAGES.length,
      intervalMs: 6_000,
      requireDesktop: true,
    });

  return (
    <Box
      as="section"
      minH={{ base: "auto", lg: "100svh" }}
      py={{ base: 14, md: 18 }}
    >
      <Container w="full" maxW="none" px={AUTH_PAGE_GUTTERS}>
        <Flex
          align={{ base: "flex-start", lg: "flex-end" }}
          justify="space-between"
          direction={{ base: "column", lg: "row" }}
          gap={7}
        >
          <Heading
            color="white"
            maxW="46rem"
            fontSize={{ base: "3xl", md: "4xl" }}
            lineHeight="1.1"
            letterSpacing="-0.04em"
          >
            标擎，让 AI 融入
            <br />
            投标的每一个环节！
          </Heading>
          <Text
            maxW="38rem"
            color="signin.textMuted"
            fontSize="md"
            lineHeight="1.85"
            whiteSpace="pre-line"
          >
            {SCREEN_TWO_COPY.body}
          </Text>
        </Flex>

        <Box
          mt={8}
          p={{ base: 4, md: 5 }}
          borderRadius="18px"
          bg="signin.panel"
          border="1px solid"
          borderColor="signin.line"
          {...interactionProps}
        >
          <SimpleGrid columns={{ base: 1, sm: 2, lg: 6 }} spacing={2}>
            {SIGNIN_STAGES.map((item, itemIndex) => {
              const active = itemIndex === index;
              return (
                <Button
                  key={item.key}
                  h="auto"
                  minH="74px"
                  py={3}
                  px={3}
                  borderRadius="xl"
                  bg={active ? "signin.surfaceStrong" : "signin.surface"}
                  color="white"
                  border="1px solid"
                  borderColor={active ? "primary.400" : "signin.line"}
                  whiteSpace="normal"
                  textAlign="left"
                  justifyContent="flex-start"
                  opacity={active ? 1 : 0.76}
                  transform={
                    active && !reducedMotion ? "translateY(-2px)" : undefined
                  }
                  transition="background-color 220ms var(--ease-out), border-color 220ms var(--ease-out), opacity 220ms var(--ease-out), transform 220ms var(--ease-out)"
                  onClick={() => selectIndex(itemIndex)}
                  aria-pressed={active}
                  _hover={{ bg: "signin.surfaceStrong" }}
                  _active={{ transform: "translateY(1px)" }}
                  _focusVisible={{
                    outline: "none",
                    boxShadow: "0 0 0 2px var(--chakra-colors-gold-400)",
                  }}
                >
                  <Box>
                    {item.planned && (
                      <Text color="gold.300" fontSize="xs" fontWeight="800">
                        规划中
                      </Text>
                    )}
                    <Text fontSize="sm" fontWeight="800">
                      {item.title}
                    </Text>
                    <Text
                      mt={1}
                      color="signin.textMuted"
                      fontSize="xs"
                      fontWeight="500"
                    >
                      {item.action}
                    </Text>
                  </Box>
                </Button>
              );
            })}
          </SimpleGrid>

          <Box
            mt={3}
            overflow="hidden"
            borderRadius="14px"
            bg="signin.paper"
            color="neutral.900"
          >
            <Flex
              minH="48px"
              align="center"
              justify="space-between"
              gap={4}
              px={{ base: 4, md: 5 }}
              borderBottom="1px solid"
              borderColor="neutral.200"
            >
              <Text fontSize="sm" fontWeight="800">
                产品能力微演示
              </Text>
              <HStack spacing={2}>
                <IconButton
                  aria-label="上一项能力"
                  icon={<ArrowBackIcon />}
                  size="sm"
                  variant="ghost"
                  onClick={() =>
                    selectIndex(
                      index === 0 ? SIGNIN_STAGES.length - 1 : index - 1,
                    )
                  }
                />
                <IconButton
                  aria-label="下一项能力"
                  icon={<ArrowForwardIcon />}
                  size="sm"
                  variant="ghost"
                  onClick={() =>
                    selectIndex(getNextIndex(index, SIGNIN_STAGES.length))
                  }
                />
              </HStack>
            </Flex>

            <Box minH={{ base: "23rem", md: "21rem" }} overflow="hidden">
              <motion.div
                style={{
                  display: "flex",
                  alignItems: "stretch",
                  width: `${SIGNIN_STAGES.length * 100}%`,
                }}
                animate={{
                  x: `-${(index * 100) / SIGNIN_STAGES.length}%`,
                }}
                transition={{
                  duration: reducedMotion ? 0 : 0.48,
                  ease: [0.65, 0, 0.35, 1],
                }}
              >
                {SIGNIN_STAGES.map((item, itemIndex) => (
                  <Box
                    key={item.key}
                    flex={`0 0 ${100 / SIGNIN_STAGES.length}%`}
                    minW={0}
                    p={{ base: 5, md: 7 }}
                    aria-hidden={itemIndex !== index}
                  >
                    <Heading fontSize="xl">{item.title}</Heading>
                    <Text mt={2} mb={5} color="neutral.600" fontSize="sm">
                      {item.description}
                    </Text>
                    <StageDemo stageKey={item.key} />
                  </Box>
                ))}
              </motion.div>
            </Box>
          </Box>

          <Grid
            mt={3}
            templateColumns={{ base: "1fr", lg: "minmax(0, 1fr) 13rem" }}
            gap={3}
          >
            <Flex
              direction={{ base: "column", md: "row" }}
              align={{ base: "flex-start", md: "center" }}
              justify="space-between"
              gap={4}
              p={4}
              borderRadius="xl"
              bg="signin.paper"
              color="neutral.900"
            >
              <Box>
                <Text fontSize="md" fontWeight="800">
                  素材库 · 投标资产底座
                </Text>
                <Text mt={1} color="neutral.600" fontSize="sm">
                  为情报匹配、投标审核、标书生成和标书审核提供可复用素材。
                </Text>
              </Box>
              <Flex gap={2} wrap="wrap">
                {ASSET_TAGS.map((tag) => (
                  <Text
                    key={tag}
                    px={2.5}
                    py={1.5}
                    borderRadius="md"
                    bg="neutral.100"
                    fontSize="xs"
                  >
                    {tag}
                  </Text>
                ))}
              </Flex>
            </Flex>
            <Box p={4} borderRadius="xl" bg="signin.paper" color="neutral.900">
              <Text fontSize="md" fontWeight="800">
                更多功能
              </Text>
              <Text mt={1} color="neutral.600" fontSize="sm">
                持续扩展，敬请期待。
              </Text>
            </Box>
          </Grid>
        </Box>
      </Container>
    </Box>
  );
}
