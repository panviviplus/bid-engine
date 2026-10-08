"use client";

/* Hallmark · component: edge-ear (bid-audit preview summon) · genre: modern-minimal · theme: chakra-smart-bid (preserved tokens)
 * states: default · hover · focus-visible · active; reduced-motion 降级为 ≤150ms 透明度
 */
import React from "react";
import { Box, Tooltip, keyframes } from "@chakra-ui/react";
import { useReducedMotion } from "framer-motion";

// 面板折叠后，召唤耳从右侧滑入贴边
const earIn = keyframes`
  from { opacity: 0; transform: translateX(18px); }
  to { opacity: 1; transform: translateX(0); }
`;

export default function EdgeEar({
  char,
  label,
  onClick,
  top = "50%",
}: {
  char: string;
  label: string;
  onClick: () => void;
  top?: string;
}) {
  const reduceMotion = useReducedMotion();
  return (
    <Tooltip label={label} placement="left" hasArrow>
      <Box
        as="button"
        type="button"
        aria-label={label}
        onClick={onClick}
        position="absolute"
        right={0}
        top={top}
        transform="translateY(-50%)"
        w="28px"
        h="44px"
        display="flex"
        alignItems="center"
        justifyContent="center"
        bg="white"
        color="primary.600"
        border="1px solid"
        borderColor="neutral.200"
        borderRight="none"
        borderRadius="10px 0 0 10px"
        boxShadow="0 4px 12px rgba(15, 23, 42, 0.10)"
        fontSize="12px"
        fontWeight="700"
        lineHeight={1}
        cursor="pointer"
        zIndex={5}
        transition="background-color 0.16s cubic-bezier(0.16, 1, 0.3, 1), color 0.16s cubic-bezier(0.16, 1, 0.3, 1), transform 0.16s cubic-bezier(0.16, 1, 0.3, 1), box-shadow 0.16s cubic-bezier(0.16, 1, 0.3, 1)"
        _hover={{
          bg: "primary.50",
          color: "primary.700",
          transform: "translateY(-50%) translateX(-2px)",
          boxShadow: "0 6px 14px rgba(15, 23, 42, 0.16)",
        }}
        _active={{
          transform: "translateY(-50%) translateX(-2px) scale(0.96)",
        }}
        _focusVisible={{
          boxShadow: "0 0 0 2px var(--chakra-colors-primary-400)",
          outline: "none",
        }}
        animation={
          reduceMotion ? undefined : `${earIn} 0.3s cubic-bezier(0.16, 1, 0.3, 1)`
        }
      >
        {char}
      </Box>
    </Tooltip>
  );
}
