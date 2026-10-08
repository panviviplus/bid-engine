"use client";

/* Hallmark · component: detail-back-control · genre: modern-minimal · theme: chakra-smart-bid (preserved tokens)
 * states: default · hover · focus-visible · active; 动效 0.16s cubic-bezier(0.16,1,0.3,1)
 */
import React from "react";
import { Button, Tooltip } from "@chakra-ui/react";
import { FiArrowLeft } from "react-icons/fi";
import NextLink from "next/link";

/** 详情页常驻返回按钮。 */
export function BackButton({
  href,
  onClick,
  label = "返回列表",
  inverted = false,
}: {
  href?: string;
  onClick?: () => void;
  label?: string;
  inverted?: boolean;
}) {
  return (
    <Tooltip label={label} placement="bottom">
      <Button
        as={href ? NextLink : undefined}
        href={href}
        onClick={onClick}
        aria-label={label}
        h="44px"
        w="44px"
        minW={0}
        px={0}
        variant="ghost"
        color={inverted ? "whiteAlpha.800" : "neutral.500"}
        borderRadius="md"
        transition="background-color 0.16s cubic-bezier(0.16, 1, 0.3, 1), color 0.16s cubic-bezier(0.16, 1, 0.3, 1), transform 0.1s cubic-bezier(0.7, 0, 0.84, 0)"
        _hover={
          inverted
            ? { bg: "whiteAlpha.200", color: "white" }
            : { bg: "neutral.100", color: "primary.600" }
        }
        _active={{
          bg: inverted ? "whiteAlpha.300" : "primary.50",
          color: inverted ? "white" : "primary.700",
          transform: "scale(0.94)",
        }}
        _focusVisible={{
          boxShadow: inverted
            ? "0 0 0 2px var(--chakra-colors-gold-300)"
            : "0 0 0 2px var(--chakra-colors-primary-400)",
          outline: "none",
        }}
      >
        <FiArrowLeft size={15} aria-hidden />
      </Button>
    </Tooltip>
  );
}
