"use client";

/* Hallmark · genre: modern-minimal · macrostructure: Workbench · design-system: design.md · designed-as-app */
/* Hallmark · pre-emit critique: P5 H5 E4 S5 R5 V4 */

import { Box, type BoxProps } from "@chakra-ui/react";
import type { ReactNode } from "react";
import { PAGE_GUTTERS, getResponsivePageLayout } from "./responsive-layout.mjs";

type ResponsivePageProps = BoxProps & {
  children: ReactNode;
  scroll?: boolean;
};

export function PageViewport({
  children,
  scroll = true,
  ...props
}: ResponsivePageProps) {
  const { viewport } = getResponsivePageLayout({ viewportWidth: 0, scroll });

  return (
    <Box
      w={viewport.width}
      h={viewport.height}
      minH={viewport.minHeight}
      minW={viewport.minWidth}
      overflowX={viewport.overflowX}
      overflowY={viewport.overflowY}
      {...props}
    >
      {children}
    </Box>
  );
}

export function PageContent({ children, ...props }: BoxProps) {
  return (
    <Box w="full" minW={0} maxW="none" mx={0} px={PAGE_GUTTERS} {...props}>
      {children}
    </Box>
  );
}
