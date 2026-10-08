"use client";

/* Hallmark · component: intel-source-card · genre: modern-minimal · theme: BidEngine deep-sea/gold (preserved tokens)
 * 设计语言对齐“招标解析”项目卡片：DataSurface 纸面卡片 + 健康度描边 + 等宽数字
 * 信息层级：身份 → 健康度 → 地址 → 采集情况 → 主操作；低频/破坏性操作收进 ⋯ 菜单
 * states: default · hover · focus-visible · active · disabled · loading(探测/采集/删除) · error(连续失败) · success(运行正常)
 * responsive: 320px 单列（主操作等分整行）→ 宽屏多列；标签不折行；触控目标 ≥44px
 * Hallmark · pre-emit critique: P5 H5 E5 S5 R5 V4
 */
import React from "react";
import {
  Badge,
  Box,
  Button,
  ButtonGroup,
  Divider,
  Flex,
  HStack,
  IconButton,
  Menu,
  MenuButton,
  MenuItem,
  MenuList,
  Portal,
  Switch,
  Text,
  Tooltip,
} from "@chakra-ui/react";
import {
  FiEdit3,
  FiExternalLink,
  FiMoreHorizontal,
  FiTrash2,
  FiWifi,
  FiZap,
} from "react-icons/fi";

import { DataSurface } from "@/components/analysis/bid-analysis-v3/workspace";
import type { IntelSource } from "@/service/intel";

const CARD_MIN_H = { base: "auto", lg: "272px" };
const LABEL_W = { base: "4.75rem", sm: "5.25rem" };

/** 采集源健康度 → 卡片描边与状态徽标。 */
function healthOf(source: IntelSource) {
  if (!source.enabled) {
    return {
      borderColor: "workbench.line",
      colorScheme: "neutral",
      label: "已停用",
      dot: "neutral.400",
    };
  }
  if (source.consecutive_failures >= 3) {
    return {
      borderColor: "error.200",
      colorScheme: "error",
      label: "连续失败",
      dot: "error.400",
    };
  }
  if (source.consecutive_failures > 0) {
    return {
      borderColor: "warning.200",
      colorScheme: "warning",
      label: "近期失败",
      dot: "warning.400",
    };
  }
  if (source.last_success_at) {
    return {
      borderColor: "success.200",
      colorScheme: "success",
      label: "运行正常",
      dot: "success.400",
    };
  }
  return {
    borderColor: "info.200",
    colorScheme: "info",
    label: "待采集",
    dot: "info.400",
  };
}

/** 一行「标签 + 值」，标签定宽、值可换行，窄屏也不会挤成一团。 */
function InfoRow({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <Flex align="baseline" gap={2} minW={0}>
      <Text
        flexShrink={0}
        w={LABEL_W}
        fontSize="xs"
        color="workbench.muted"
        whiteSpace="nowrap"
      >
        {label}
      </Text>
      <Box minW={0} flex={1} fontSize="sm" color="workbench.text">
        {children}
      </Box>
    </Flex>
  );
}

export default function SourceCard({
  source,
  probing,
  collecting,
  deleting,
  toggling,
  probeText,
  onProbe,
  onCollect,
  onEdit,
  onDelete,
  onToggle,
}: {
  source: IntelSource;
  probing: boolean;
  collecting: boolean;
  deleting: boolean;
  toggling: boolean;
  probeText?: string;
  onProbe: () => void;
  onCollect: () => void;
  onEdit: () => void;
  onDelete: () => void;
  // eslint-disable-next-line no-unused-vars
  onToggle: (next: boolean) => void;
}) {
  const health = healthOf(source);
  const busy = probing || collecting || deleting || toggling;
  const address = source.list_url || source.homepage_url || "";
  const failed = source.consecutive_failures > 0;

  return (
    <DataSurface
      minH={CARD_MIN_H}
      p={{ base: 4, md: 5 }}
      display="flex"
      flexDirection="column"
      borderColor={health.borderColor}
      transition="transform .18s cubic-bezier(0.16,1,0.3,1), box-shadow .18s cubic-bezier(0.16,1,0.3,1), border-color .18s ease-out"
      _hover={{
        transform: "translateY(-2px)",
        boxShadow: "0 14px 34px rgba(11, 27, 43, 0.10)",
      }}
      _focusWithin={{
        boxShadow: "0 0 0 2px var(--chakra-colors-primary-200)",
      }}
    >
      {/* ① 身份行：名称 + 健康度 + 停用开关 + 低频操作菜单 */}
      <Flex align="flex-start" gap={3} minW={0}>
        <Box minW={0} flex={1}>
          <HStack spacing={2} align="center" minW={0}>
            <Box
              aria-hidden
              w="8px"
              h="8px"
              flexShrink={0}
              borderRadius="full"
              bg={health.dot}
            />
            <Badge
              colorScheme={health.colorScheme}
              variant="subtle"
              borderRadius="md"
              whiteSpace="nowrap"
            >
              {health.label}
            </Badge>
            {failed && (
              <Text
                fontSize="xs"
                color={
                  source.consecutive_failures >= 3 ? "error.600" : "orange.600"
                }
                sx={{ fontVariantNumeric: "tabular-nums" }}
                whiteSpace="nowrap"
              >
                连续 {source.consecutive_failures} 次
              </Text>
            )}
          </HStack>

          <Text
            as="h3"
            mt={2}
            minW={0}
            fontSize={{ base: "md", md: "lg" }}
            lineHeight="1.3"
            fontWeight="750"
            color="workbench.control"
            letterSpacing="-0.02em"
            overflowWrap="anywhere"
            noOfLines={2}
          >
            {source.name || source.source_key}
          </Text>
          <Text
            mt={1}
            fontSize="xs"
            color="workbench.muted"
            fontFamily="mono"
            noOfLines={1}
          >
            {source.source_key}
          </Text>
        </Box>

        <HStack spacing={1} flexShrink={0}>
          <Tooltip
            label={
              source.enabled ? "停用后不再参与自动采集" : "启用后参与自动采集"
            }
            openDelay={600}
          >
            <Box minH="44px" display="flex" alignItems="center" px={1}>
              <Switch
                colorScheme="primary"
                size="md"
                isChecked={Boolean(source.enabled)}
                isDisabled={toggling}
                onChange={(event) => onToggle(event.target.checked)}
                aria-label={
                  source.enabled
                    ? `停用采集源 ${source.name}`
                    : `启用采集源 ${source.name}`
                }
              />
            </Box>
          </Tooltip>

          <Menu placement="bottom-end" isLazy>
            <MenuButton
              as={IconButton}
              aria-label={`${source.name || source.source_key} 的更多操作`}
              icon={<FiMoreHorizontal aria-hidden />}
              h="44px"
              w="44px"
              minW="44px"
              variant="ghost"
              color="neutral.500"
              borderRadius="md"
              isDisabled={busy}
              _hover={{ bg: "neutral.100", color: "workbench.text" }}
              _focusVisible={{
                boxShadow: "0 0 0 2px var(--chakra-colors-primary-300)",
                outline: "none",
              }}
            />
            <Portal>
              <MenuList minW="180px" borderRadius="12px" boxShadow="lg">
                <MenuItem
                  icon={<FiEdit3 size={15} />}
                  minH="44px"
                  fontSize="sm"
                  onClick={onEdit}
                  _hover={{ bg: "primary.50" }}
                  _focus={{ bg: "primary.50" }}
                >
                  编辑采集源
                </MenuItem>
                <MenuItem
                  icon={<FiTrash2 size={15} />}
                  minH="44px"
                  fontSize="sm"
                  color="error.600"
                  onClick={onDelete}
                  _hover={{ bg: "error.50" }}
                  _focus={{ bg: "error.50" }}
                >
                  删除采集源
                </MenuItem>
              </MenuList>
            </Portal>
          </Menu>
        </HStack>
      </Flex>

      <Divider my={4} borderColor="neutral.100" />

      {/* ② 采集配置与状态：一行一条，窄屏自然换行 */}
      <Flex direction="column" gap={2.5} minW={0}>
        <InfoRow label="列表地址">
          {address ? (
            <Text
              as="a"
              href={address}
              target="_blank"
              rel="noreferrer noopener"
              fontFamily="mono"
              fontSize="xs"
              color="primary.600"
              overflowWrap="anywhere"
              noOfLines={2}
              _hover={{ color: "primary.700", textDecoration: "underline" }}
              _focusVisible={{
                outline: "none",
                boxShadow: "0 0 0 2px var(--chakra-colors-primary-300)",
                borderRadius: "sm",
              }}
            >
              {address} <FiExternalLink aria-hidden size={11} />
            </Text>
          ) : (
            <Text fontSize="sm" color="workbench.muted">
              未配置
            </Text>
          )}
        </InfoRow>

        <InfoRow label="采集方式">
          <HStack spacing={2} wrap="wrap">
            <Badge
              variant="subtle"
              colorScheme="primary"
              bg="primary.50"
              color="primary.700"
              borderRadius="md"
              px={2.5}
              py={1}
              fontSize="xs"
              fontWeight="600"
              lineHeight="short"
            >
              {source.discovery_mode}
            </Badge>
            <Text fontSize="sm" color="workbench.muted">
              {source.needs_browser ? "需浏览器渲染" : "静态请求"}
            </Text>
          </HStack>
        </InfoRow>

        <InfoRow label="归属">
          <Text fontSize="sm" noOfLines={1}>
            {[source.category, source.region, source.industry_hint]
              .filter(Boolean)
              .join(" · ") || "未标注"}
          </Text>
        </InfoRow>

        <InfoRow label="最近成功">
          <Flex gap={3} wrap="wrap" sx={{ fontVariantNumeric: "tabular-nums" }}>
            <Text fontSize="sm">{source.last_success_at || "从未成功"}</Text>
            <Text fontSize="sm" color="workbench.muted">
              优先级 {source.priority}
            </Text>
          </Flex>
        </InfoRow>
      </Flex>

      {/* ③ 异常与探测结果：有内容才出现，避免卡片常态臃肿 */}
      {source.last_error && (
        <Box
          mt={3}
          p={3}
          borderRadius="10px"
          bg="workbench.control"
          color="white"
        >
          <Text fontSize="xs" color="whiteAlpha.700">
            最近一次错误
          </Text>
          <Text mt={1} fontSize="sm" overflowWrap="anywhere" noOfLines={3}>
            {source.last_error}
          </Text>
        </Box>
      )}
      {probeText && (
        <Box
          mt={3}
          px={3}
          py={2}
          borderRadius="10px"
          bg="info.50"
          border="1px solid"
          borderColor="info.200"
        >
          <Text
            fontSize="xs"
            color="info.700"
            overflowWrap="anywhere"
            noOfLines={3}
          >
            探测结果：{probeText}
          </Text>
        </Box>
      )}

      {/* ④ 主操作：整行等分，窄屏也只有一个操作行 */}
      <ButtonGroup
        mt="auto"
        pt={4}
        isAttached
        variant="outline"
        width="full"
        display="flex"
      >
        <Button
          flex={1}
          h="44px"
          minW={0}
          leftIcon={<FiWifi aria-hidden />}
          whiteSpace="nowrap"
          borderColor="neutral.200"
          color="workbench.text"
          fontWeight="600"
          isLoading={probing}
          loadingText="探测中"
          isDisabled={busy && !probing}
          onClick={onProbe}
          _hover={{
            bg: "primary.50",
            borderColor: "primary.300",
            color: "primary.700",
          }}
          _active={{ transform: "scale(0.99)" }}
          _focusVisible={{
            boxShadow: "0 0 0 2px var(--chakra-colors-primary-300)",
            outline: "none",
            zIndex: 1,
          }}
        >
          探测
        </Button>
        <Button
          flex={1}
          h="44px"
          minW={0}
          leftIcon={<FiZap aria-hidden />}
          whiteSpace="nowrap"
          borderColor="primary.500"
          bg="primary.500"
          color="white"
          fontWeight="600"
          isLoading={collecting}
          loadingText="触发中"
          isDisabled={busy && !collecting}
          onClick={onCollect}
          _hover={{
            bg: "primary.600",
            borderColor: "primary.600",
            color: "white",
          }}
          _active={{ transform: "scale(0.99)" }}
          _focusVisible={{
            boxShadow: "0 0 0 2px var(--chakra-colors-primary-200)",
            outline: "none",
            zIndex: 1,
          }}
        >
          立即采集
        </Button>
      </ButtonGroup>
    </DataSurface>
  );
}
