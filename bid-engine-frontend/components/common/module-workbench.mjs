/* Hallmark · genre: modern-minimal · macrostructure: Workbench · theme: BidEngine deep-sea/gold · enrichment: none · nav: inherited side rail · footer: none
 * audience: enterprise bidding teams · use: create, find, and continue project work · tone: technical, restrained, professional
 * Hallmark · pre-emit critique: P5 H5 E4 S5 R5 V4
 */

import React from "react";
import { Box, Collapse, Flex, HStack, Icon, Text } from "@chakra-ui/react";
import { FiChevronDown } from "react-icons/fi";

const h = React.createElement;

/**
 * 模块页标题栏。
 *
 * - `titleSuffix` 默认“工作台”，用于“招标解析 工作台”这类组合标题；
 *   纯功能页（我的订阅 / 提醒中心 / 系统配置）传 null 即可去掉无意义的后缀。
 * - `activity` 兼容两种传法：**字符串**渲染为带状态点的胶囊；**React 节点**（如按钮）
 *   直接作为兄弟节点渲染。节点绝不能塞进 `Text`（会渲染成 `<p>` 里套 `<div>`，
 *   触发 hydration 报错）。
 *
 * @param {{
 *   title: string;
 *   titleSuffix?: import("react").ReactNode;
 *   activity?: import("react").ReactNode;
 * }} props
 */
export function ModuleWorkbenchHeader({
  title,
  titleSuffix = "工作台",
  activity = null,
}) {
  const isNodeActivity = React.isValidElement(activity);

  // 字符串活动摘要渲染为带状态点的胶囊；React 节点（按钮等）直接作为兄弟节点渲染，
  // 避免被塞进 Text（<p>）里形成 <p><div/></p>。
  const renderActivity = () => {
    if (!activity) return null;
    if (isNodeActivity) {
      return h(
        HStack,
        {
          role: "status",
          "aria-live": "polite",
          align: "center",
          flexShrink: 0,
        },
        activity,
      );
    }
    return h(
      HStack,
      {
        role: "status",
        "aria-live": "polite",
        minH: "44px",
        px: 4,
        bg: "workbench.paper",
        border: "1px solid",
        borderColor: "workbench.line",
        borderRadius: "full",
        fontSize: "sm",
        color: "workbench.muted",
        flexShrink: 0,
      },
      h(Box, {
        "aria-hidden": true,
        w: "8px",
        h: "8px",
        flexShrink: 0,
        borderRadius: "full",
        bg: "gold.400",
      }),
      h(Text, null, activity),
    );
  };

  return h(
    Flex,
    {
      align: { base: "flex-start", sm: "flex-end" },
      justify: "space-between",
      gap: 4,
      mb: 6,
      direction: { base: "column", sm: "row" },
    },
    h(
      HStack,
      { align: "baseline", spacing: 2, minW: 0 },
      h(
        Text,
        {
          as: "h1",
          minW: 0,
          fontSize: "28px",
          lineHeight: "1.15",
          fontWeight: "800",
          color: "workbench.control",
          letterSpacing: "-0.035em",
          overflowWrap: "anywhere",
        },
        title,
      ),
      titleSuffix
        ? h(
            Text,
            {
              flexShrink: 0,
              fontSize: "md",
              fontWeight: "600",
              color: "workbench.muted",
              letterSpacing: "-0.01em",
            },
            titleSuffix,
          )
        : null,
    ),
    renderActivity(),
  );
}

/**
 * @param {{
 *   title: string;
 *   description?: import("react").ReactNode;
 *   action?: import("react").ReactNode;
 *   children?: import("react").ReactNode;
 *   expanded?: boolean;
 *   onToggle?: () => void;
 * }} props
 */
export function ModuleWorkbenchDeck({
  title,
  description,
  action,
  children,
  expanded = false,
  onToggle = () => {},
}) {
  return h(
    Box,
    {
      role: "region",
      "aria-label": title,
      position: "relative",
      overflow: "hidden",
      bg: "workbench.control",
      color: "workbench.paper",
      border: "1px solid",
      borderColor: "workbench.controlRaised",
      borderRadius: "14px",
      boxShadow: "lg",
      _before: {
        content: '""',
        position: "absolute",
        inset: 0,
        pointerEvents: "none",
        opacity: 0.18,
        backgroundImage:
          "linear-gradient(var(--chakra-colors-workbench-controlRaised) 1px, transparent 1px), linear-gradient(90deg, var(--chakra-colors-workbench-controlRaised) 1px, transparent 1px)",
        backgroundSize: "28px 28px",
      },
    },
    h(
      Flex,
      {
        position: "relative",
        zIndex: 1,
        align: { base: "stretch", md: "center" },
        justify: "space-between",
        gap: 3,
        px: { base: 4, md: 6 },
        py: expanded ? { base: 4, md: 6 } : 3,
        direction: { base: "column", md: "row" },
      },
      h(
        Flex,
        {
          as: "button",
          type: "button",
          flex: 1,
          minW: 0,
          minH: expanded ? "52px" : "36px",
          p: 0,
          align: "center",
          justify: "space-between",
          gap: 4,
          color: "workbench.paper",
          textAlign: "left",
          "aria-expanded": expanded,
          "aria-controls": "module-workbench-deck-details",
          onClick: onToggle,
          _focusVisible: {
            outline: "2px solid",
            outlineColor: "gold.300",
            outlineOffset: "3px",
            borderRadius: "md",
          },
        },
        h(
          Box,
          { minW: 0 },
          h(
            Text,
            {
              as: "h2",
              fontSize: { base: "xl", md: "27px" },
              lineHeight: "1.2",
              fontWeight: "800",
              letterSpacing: "-0.025em",
              overflowWrap: "anywhere",
            },
            title,
          ),
        ),
        h(Icon, {
          "aria-hidden": true,
          as: FiChevronDown,
          boxSize: 5,
          flexShrink: 0,
          transform: expanded ? "rotate(180deg)" : "rotate(0deg)",
          transition: "transform 0.22s cubic-bezier(0.65, 0, 0.35, 1)",
        }),
      ),
      action ? h(Box, { flexShrink: 0 }, action) : null,
    ),
    h(
      Collapse,
      { in: expanded, animateOpacity: true },
      h(
        Box,
        {
          id: "module-workbench-deck-details",
          position: "relative",
          zIndex: 1,
          px: { base: 4, md: 6 },
          pb: { base: 5, md: 6 },
        },
        description
          ? h(
              Text,
              {
                maxW: "65ch",
                color: "whiteAlpha.700",
                fontSize: "sm",
                lineHeight: "1.65",
              },
              description,
            )
          : null,
        children
          ? h(
              Flex,
              {
                mt: description ? 4 : 0,
                gap: 2,
                wrap: "wrap",
                color: "whiteAlpha.800",
                fontSize: "xs",
              },
              children,
            )
          : null,
      ),
    ),
  );
}
