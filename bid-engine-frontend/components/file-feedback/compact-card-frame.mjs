/* Hallmark · component: feedback-card-frame · genre: modern-minimal · theme: BidEngine deep-sea/gold
 * states: default · hover · focus-visible · processed · pending
 * sizing: content-driven — no artificial min-height, so short feedback keeps no dead space at the bottom
 */

import React from "react";
import { Box } from "@chakra-ui/react";

const h = React.createElement;

/**
 * @param {{ processed: boolean; children: import("react").ReactNode }} props
 */
export function CompactFeedbackCardFrame({ processed, children }) {
  return h(
    Box,
    {
      as: "article",
      bg: "workbench.paper",
      border: "1px solid",
      borderColor: processed ? "success.200" : "warning.200",
      borderRadius: "14px",
      boxShadow: "sm",
      transition: "border-color 0.2s cubic-bezier(0.16, 1, 0.3, 1)",
      _hover: {
        borderColor: processed ? "success.300" : "warning.300",
      },
    },
    h(Box, { p: 4 }, children),
  );
}
