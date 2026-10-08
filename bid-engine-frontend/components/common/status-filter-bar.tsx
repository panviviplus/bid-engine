"use client";

import { Box, Button, Flex, Text } from "@chakra-ui/react";

export interface StatusTabItem {
  key: string;
  label: string;
  color: string; // Chakra token，如 "primary.600"
}

interface StatusFilterBarProps {
  options: StatusTabItem[];
  value: string;
  counts: Record<string, number>;
  loading?: boolean;
  // eslint-disable-next-line no-unused-vars
  onChange: (_key: string) => void;
  ariaLabel?: string;
}

/**
 * Hallmark · component: status-filter-bar · genre: modern-minimal · theme: BidEngine deep-sea/gold
 * 状态筛选 + 统计合并组件：沿用招标解析工作台的白纸选中面与深海蓝边界。
 * states: default · hover · focus · active · selected · disabled
 */
export default function StatusFilterBar({
  options,
  value,
  counts,
  loading = false,
  onChange,
  ariaLabel = "状态筛选与统计",
}: StatusFilterBarProps) {
  return (
    <Flex
      role="tablist"
      aria-label={ariaLabel}
      gap={1}
      maxW="full"
      overflowX="auto"
      pb={1}
      opacity={loading ? 0.75 : 1}
      transition="opacity 0.2s cubic-bezier(0.16, 1, 0.3, 1)"
    >
      {options.map((opt) => {
        const active = value === opt.key;
        const count = counts[opt.key] ?? 0;
        return (
          <Button
            key={opt.key}
            role="tab"
            aria-selected={active}
            size="sm"
            minH="42px"
            flexShrink={0}
            variant="ghost"
            border="1px solid"
            borderColor={active ? "workbench.control" : "transparent"}
            bg={active ? "workbench.paper" : "transparent"}
            color={active ? "workbench.text" : "workbench.muted"}
            borderRadius="9px"
            px={3}
            gap={1.5}
            _hover={{ bg: "workbench.paper" }}
            _active={{
              transform: "translateY(1px)",
            }}
            _focusVisible={{
              outline: "2px solid",
              outlineColor: "gold.400",
              outlineOffset: "2px",
              boxShadow: "none",
            }}
            transition="background-color 0.18s cubic-bezier(0.16, 1, 0.3, 1), border-color 0.18s cubic-bezier(0.16, 1, 0.3, 1), transform 0.1s cubic-bezier(0.7, 0, 0.84, 0)"
            onClick={() => onChange(opt.key)}
          >
            <Box
              w="1.5"
              h="1.5"
              borderRadius="full"
              bg={opt.color}
              flex="none"
              aria-hidden
            />
            {opt.label}
            <Text
              as="span"
              textStyle="statValue"
              color={active ? "workbench.text" : "workbench.muted"}
            >
              {count}
            </Text>
          </Button>
        );
      })}
    </Flex>
  );
}
