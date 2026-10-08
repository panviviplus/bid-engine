"use client";

import {
  Box,
  Button,
  Container,
  Flex,
  Grid,
  Heading,
  SimpleGrid,
  Text,
} from "@chakra-ui/react";
import { motion } from "framer-motion";
import { AUTH_PAGE_GUTTERS } from "@/components/layout/responsive-layout.mjs";

import {
  ENTRY_SCENARIOS,
  LEGAL_LABELS,
  SCREEN_THREE_COPY,
  SIGNIN_STAGES,
  VALUE_CARDS,
} from "./signin-content.mjs";
import { getDownstreamIndexes } from "./rotation.mjs";
import { useAutoRotation } from "./use-auto-rotation";

type EntrySectionProps = {
  onLoginClick: () => void;
  onPasswordLoginClick: () => void;
  unifiedSmsAuthEnabled: boolean;
};

export default function EntrySection({
  onLoginClick,
  onPasswordLoginClick,
  unifiedSmsAuthEnabled,
}: EntrySectionProps) {
  const { index, selectIndex, reducedMotion, interactionProps } =
    useAutoRotation({
      count: ENTRY_SCENARIOS.length,
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
            maxW="43rem"
            fontSize={{ base: "3xl", md: "4xl" }}
            lineHeight="1.1"
            letterSpacing="-0.04em"
          >
            您的进度，
            <br />
            就是标擎接手的起点
          </Heading>
          <Text
            maxW="38rem"
            color="signin.textMuted"
            fontSize="md"
            lineHeight="1.85"
            whiteSpace="pre-line"
          >
            {SCREEN_THREE_COPY.body}
          </Text>
        </Flex>

        <Grid
          mt={8}
          templateColumns={{
            base: "minmax(0, 1fr)",
            lg: "clamp(15rem, 18vw, 22rem) minmax(0, 1fr)",
          }}
          gap={3}
          {...interactionProps}
        >
          <Flex direction="column" gap={2}>
            {ENTRY_SCENARIOS.map((item, itemIndex) => {
              const active = itemIndex === index;
              return (
                <Button
                  key={item.title}
                  h="auto"
                  minH="68px"
                  px={4}
                  py={3}
                  borderRadius="xl"
                  bg={active ? "signin.surfaceStrong" : "signin.surface"}
                  color="white"
                  border="1px solid"
                  borderColor={active ? "primary.400" : "signin.line"}
                  justifyContent="flex-start"
                  textAlign="left"
                  whiteSpace="normal"
                  transform={
                    active && !reducedMotion ? "translateX(3px)" : undefined
                  }
                  transition="background-color 220ms var(--ease-out), border-color 220ms var(--ease-out), transform 220ms var(--ease-out)"
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
                    <Text fontSize="sm" fontWeight="800">
                      {item.title}
                    </Text>
                    <Text mt={1} color="signin.textMuted" fontSize="xs">
                      {item.action}
                    </Text>
                  </Box>
                </Button>
              );
            })}
          </Flex>

          <Box
            minH="22rem"
            borderRadius="16px"
            bg="signin.paper"
            color="neutral.900"
            overflow="hidden"
          >
            <motion.div
              style={{
                display: "flex",
                alignItems: "stretch",
                width: `${ENTRY_SCENARIOS.length * 100}%`,
              }}
              animate={{
                x: `-${(index * 100) / ENTRY_SCENARIOS.length}%`,
              }}
              transition={{
                duration: reducedMotion ? 0 : 0.48,
                ease: [0.65, 0, 0.35, 1],
              }}
            >
              {ENTRY_SCENARIOS.map((item, itemIndex) => {
                const downstream = getDownstreamIndexes(
                  item.startIndex,
                  SIGNIN_STAGES.length,
                );
                return (
                  <Box
                    key={item.title}
                    flex={`0 0 ${100 / ENTRY_SCENARIOS.length}%`}
                    minW={0}
                    p={{ base: 5, md: 7 }}
                    aria-hidden={itemIndex !== index}
                  >
                    <Text color="primary.700" fontSize="sm" fontWeight="800">
                      {item.eyebrow}
                    </Text>
                    <Heading mt={3} fontSize={{ base: "xl", md: "2xl" }}>
                      {item.title}
                    </Heading>
                    <Text
                      mt={3}
                      color="neutral.600"
                      fontSize="sm"
                      lineHeight="1.8"
                    >
                      {item.detail}
                    </Text>

                    <SimpleGrid
                      mt={9}
                      columns={{ base: 2, sm: 3, md: 6 }}
                      spacing={3}
                    >
                      {SIGNIN_STAGES.map((stage, stageIndex) => {
                        const current = stageIndex === item.startIndex;
                        const enabled = downstream.includes(stageIndex);
                        let background = "neutral.100";
                        let color = "neutral.400";
                        let borderColor = "neutral.200";
                        let dotColor = "neutral.300";
                        if (enabled) {
                          background = "primary.50";
                          color = "primary.700";
                          borderColor = "primary.200";
                          dotColor = "primary.500";
                        }
                        if (current) {
                          background = "gold.50";
                          color = "gold.800";
                          borderColor = "gold.300";
                          dotColor = "gold.400";
                        }
                        return (
                          <Flex
                            key={stage.key}
                            minH="74px"
                            direction="column"
                            align="center"
                            justify="center"
                            textAlign="center"
                            borderRadius="xl"
                            bg={background}
                            color={color}
                            border="1px solid"
                            borderColor={borderColor}
                          >
                            <Box
                              boxSize="10px"
                              borderRadius="full"
                              bg={dotColor}
                              boxShadow={current ? "signinGlowSoft" : "none"}
                            />
                            <Text mt={2} fontSize="xs" fontWeight="800">
                              {stage.shortTitle}
                            </Text>
                          </Flex>
                        );
                      })}
                    </SimpleGrid>
                  </Box>
                );
              })}
            </motion.div>
          </Box>
        </Grid>

        <SimpleGrid mt={3} columns={{ base: 1, md: 3 }} spacing={3}>
          {VALUE_CARDS.map((card) => (
            <Box
              key={card.title}
              p={5}
              borderRadius="14px"
              bg="signin.surface"
              border="1px solid"
              borderColor="signin.line"
            >
              <Text color="white" fontSize="md" fontWeight="800">
                {card.title}
              </Text>
              <Text
                mt={2}
                color="signin.textMuted"
                fontSize="sm"
                lineHeight="1.75"
              >
                {card.body}
              </Text>
            </Box>
          ))}
        </SimpleGrid>

        <Flex
          mt={3}
          direction={{ base: "column", md: "row" }}
          align={{ base: "stretch", md: "center" }}
          justify="space-between"
          gap={5}
          p={{ base: 5, md: 6 }}
          borderRadius="14px"
          bg="signin.paper"
          color="neutral.900"
        >
          <Box>
            <Text fontSize="lg" fontWeight="800">
              从您当前所处的环节开始
            </Text>
            <Text mt={1} color="neutral.600" fontSize="sm">
              {unifiedSmsAuthEnabled
                ? "手机号验证后即可进入；未注册手机号将自动创建账号并登录。"
                : "已有账号可直接登录；新用户可先完成注册后进入工作台。"}
            </Text>
          </Box>
          <Flex gap={3} wrap="wrap">
            <Button minH="44px" colorScheme="primary" onClick={onLoginClick}>
              登录标擎
            </Button>
            <Button
              minH="44px"
              variant="outline"
              onClick={onPasswordLoginClick}
            >
              密码登录
            </Button>
          </Flex>
        </Flex>

        <Flex
          mt={8}
          pt={5}
          align={{ base: "flex-start", md: "center" }}
          justify="space-between"
          direction={{ base: "column", md: "row" }}
          gap={4}
          borderTop="1px solid"
          borderColor="signin.line"
          color="signin.textMuted"
          fontSize="sm"
        >
          <Text fontWeight="700">标擎 · 智慧投标</Text>
          <Flex gap={4} wrap="wrap">
            {LEGAL_LABELS.map((label) => (
              <Text key={label}>{label}</Text>
            ))}
          </Flex>
          <Text>© {new Date().getFullYear()} 标擎</Text>
        </Flex>
      </Container>
    </Box>
  );
}
