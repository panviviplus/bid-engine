/* Hallmark · component: feedback-card-grid · genre: modern-minimal · theme: BidEngine deep-sea/gold
 * sparse lists keep bounded columns; dense lists fill available rows from the left
 */

import React from "react";
import { Grid } from "@chakra-ui/react";

const h = React.createElement;

/**
 * @param {{ children: import("react").ReactNode; [key: string]: unknown }} props
 */
export function FeedbackCardGrid({ children, ...props }) {
  return h(
    Grid,
    {
      w: "full",
      templateColumns: "repeat(auto-fill, minmax(min(100%, 280px), 340px))",
      gap: 4,
      alignItems: "start",
      justifyContent: "start",
      ...props,
    },
    children,
  );
}
