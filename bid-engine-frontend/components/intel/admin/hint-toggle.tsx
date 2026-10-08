"use client";

/* Hallmark · component: intel-hint-toggle · genre: modern-minimal · theme: BidEngine workbench (preserved tokens)
 * 职责：各子 tab「业务说明」的折叠开关。抽成共享组件是为了保证四个 tab 的
 * 位置与样式完全一致——位置不统一会让管理员每次都要重新找一遍。
 * 位置约定：固定放在顶部操作条的最右侧（动作按钮在它左边）。
 * states: default · hover · focus-visible · active · expanded
 */
import React from "react";
import { IconButton, Tooltip } from "@chakra-ui/react";
import { FiChevronDown, FiChevronUp } from "react-icons/fi";

export default function HintToggle({
  label,
  expanded,
  onToggle,
}: {
  /** 无障碍名称，同时作为 Tooltip 文案（例如“采集说明”） */
  label: string;
  expanded: boolean;
  onToggle: () => void;
}) {
  return (
    <Tooltip label={label} openDelay={500}>
      <IconButton
        aria-label={label}
        aria-expanded={expanded}
        icon={
          expanded ? <FiChevronUp aria-hidden /> : <FiChevronDown aria-hidden />
        }
        h="44px"
        w="44px"
        minW="44px"
        variant="ghost"
        color="neutral.500"
        borderRadius="md"
        onClick={onToggle}
        _hover={{ bg: "primary.50", color: "primary.700" }}
        _active={{ transform: "scale(0.96)" }}
        _focusVisible={{
          boxShadow: "0 0 0 2px var(--chakra-colors-primary-300)",
          outline: "none",
        }}
      />
    </Tooltip>
  );
}
