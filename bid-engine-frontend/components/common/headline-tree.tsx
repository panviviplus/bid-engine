"use client";

/* eslint-disable no-unused-vars */
import React, { useState } from "react";
import { VStack, Text, Box } from "@chakra-ui/react";

export interface CatalogueItem {
  level: number;
  text: string;
  [key: string]: any;
}

interface Props {
  data?: CatalogueItem[];
  onItemClick?: (
    item: CatalogueItem,
    nextItem: CatalogueItem,
    index: number,
  ) => void;
  height: number | string;
  tabLevel?: number;
}

function CatalogueTree({ data, onItemClick, height, tabLevel = 2 }: Props) {
  const [activeIdx, setActiveIdx] = useState<number | null>(null);
  const handleClick = (item: CatalogueItem, idx: number) => {
    console.log("点击目录项", item, idx);
    if (!onItemClick) return;
    setActiveIdx(idx);
    if (data?.length && data.length - 1 === idx) {
      onItemClick?.(item, data[idx], idx);
      return;
    }
    onItemClick?.(item, data![idx + 1], idx);
  };
  return (
    <Box height={height} overflowY="auto" pt={1}>
      <VStack align="stretch" spacing={1}>
        {data?.map((item, idx) => (
          <Box
            key={idx}
            pl={item.level * tabLevel}
            _hover={{ bg: "#EAF1FD", cursor: "pointer" }}
            bg={activeIdx === idx ? "#EAF1FD" : undefined}
            onClick={() => handleClick(item, idx)}
          >
            <Text noOfLines={1} fontSize="sm" color="gray.700">
              {item.raw || item.title}
            </Text>
          </Box>
        ))}
      </VStack>
    </Box>
  );
}

export default CatalogueTree;
