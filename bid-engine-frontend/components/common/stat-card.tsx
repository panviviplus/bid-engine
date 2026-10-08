"use client";

import {
  Box, Text, Flex, Stat, StatNumber, StatHelpText,
  Tooltip, IconButton, Menu, MenuButton, MenuList, MenuItem,
} from "@chakra-ui/react";
import { ChevronDownIcon } from "@chakra-ui/icons";
import type { ReactNode } from "react";

export interface StatCardProps {
  label: string;
  value: string | number;
  helpText?: string;
  tooltip?: string;
  icon?: ReactNode;
  accentColor?: string;
  /** 显示周期选择器 */
  showPeriod?: boolean;
  /** 当前周期值 */
  period?: string;
  /** 周期切换回调 */
  onPeriodChange?: (period: string) => void;
}

const PERIOD_OPTIONS = [
  { value: "today", label: "今日" },
  { value: "week", label: "近一周" },
  { value: "month", label: "本月" },
  { value: "year", label: "近一年" },
  { value: "custom", label: "自定义" },
];

/** 根据 period 值获取动态标题 */
function getDynamicLabel(baseLabel: string, period: string): string {
  const opt = PERIOD_OPTIONS.find((o) => o.value === period);
  if (!opt || period === "custom") return baseLabel;
  const suffix = baseLabel.replace(/^(今日|本月|近一周|近一年)/, "");
  return opt.label + suffix;
}

/**
 * 统计指标卡片 — Dashboard 及模块概览页使用
 * 周期切换采用 "⋯" Menu 交互，与网站整体风格统一
 */
export default function StatCard({
  label,
  value,
  helpText,
  tooltip,
  icon,
  accentColor = "primary.600",
  showPeriod = false,
  period = "today",
  onPeriodChange,
}: StatCardProps) {
  const displayLabel = showPeriod ? getDynamicLabel(label, period) : label;
  const currentPeriodLabel = PERIOD_OPTIONS.find((o) => o.value === period)?.label || "";

  const cardContent = (
    <Box
      bg="white"
      borderRadius="xl"
      border="1px solid"
      borderColor="neutral.100"
      boxShadow="0 1px 3px rgba(0,0,0,0.06)"
      p={5}
      transition="all 0.2s"
      _hover={{
        boxShadow: "0 4px 12px rgba(0,0,0,0.08)",
        borderColor: "primary.200",
      }}
    >
      <Flex align="flex-start" justify="space-between" mb={2}>
        <Text fontSize="sm" fontWeight="500" color="neutral.500" lineHeight="1.5">
          {displayLabel}
        </Text>
        <Flex align="center" gap={2}>
          {showPeriod && onPeriodChange && (
            <Menu placement="bottom-end" autoSelect={false}>
              <MenuButton
                as={IconButton}
                aria-label="切换周期"
                icon={<Text fontSize="lg" lineHeight="1">⋯</Text>}
                variant="ghost"
                size="xs"
                minW="24px"
                h="24px"
                borderRadius="full"
                color="neutral.400"
                _hover={{ bg: "neutral.100", color: "primary.500" }}
                _active={{ bg: "primary.50", color: "primary.600" }}
              />
              <MenuList
                minW="110px"
                py={1}
                borderRadius="lg"
                border="1px solid"
                borderColor="neutral.100"
                boxShadow="0 4px 16px rgba(0,0,0,0.1)"
                fontSize="sm"
              >
                {PERIOD_OPTIONS.map((opt) => (
                  <MenuItem
                    key={opt.value}
                    onClick={() => onPeriodChange(opt.value)}
                    fontSize="sm"
                    color={opt.value === period ? "primary.600" : "neutral.700"}
                    fontWeight={opt.value === period ? "600" : "400"}
                    bg={opt.value === period ? "primary.50" : "transparent"}
                    borderRadius="md"
                    mx={1}
                    _hover={{ bg: "neutral.50" }}
                  >
                    {opt.label}
                  </MenuItem>
                ))}
              </MenuList>
            </Menu>
          )}
          {icon && (
            <Box color={accentColor} ml={showPeriod ? 0 : undefined}>
              {icon}
            </Box>
          )}
        </Flex>
      </Flex>
      <Stat>
        <StatNumber fontSize="2xl" fontWeight="700" color="neutral.900" lineHeight="1.2">
          {value}
        </StatNumber>
        {helpText && (
          <StatHelpText fontSize="xs" color="neutral.400" mt={1} mb={0}>
            {helpText}
          </StatHelpText>
        )}
      </Stat>
    </Box>
  );

  if (tooltip) {
    return (
      <Tooltip
        label={tooltip}
        placement="bottom"
        hasArrow
        openDelay={1000}
        bg="neutral.800"
        color="white"
        px={4}
        py={2}
        borderRadius="lg"
        fontSize="sm"
        maxW="280px"
      >
        {cardContent}
      </Tooltip>
    );
  }

  return cardContent;
}

export { PERIOD_OPTIONS };
