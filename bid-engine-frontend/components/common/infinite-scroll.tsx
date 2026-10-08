"use client";

import { Box, Flex, Spinner, Text } from "@chakra-ui/react";
import InfiniteScroll from "react-infinite-scroll-component";
import { useEffect, useRef } from "react";
import type { ReactNode } from "react";

interface InfiniteScrollListProps {
  dataLength: number;
  hasMore: boolean;
  loadMore: () => void;
  scrollableTarget?: string;
  loader?: ReactNode;
  endMessage?: ReactNode;
  children: ReactNode;
}

function DefaultLoader() {
  return (
    <Flex justify="center" align="center" gap={2} py={6}>
      <Spinner size="sm" color="primary.500" speed="0.8s" />
      <Text fontSize="sm" color="neutral.400">
        加载中…
      </Text>
    </Flex>
  );
}

function DefaultEndMessage() {
  return (
    <Flex direction="column" align="center" gap={3} py={8}>
      <Box
        w="full"
        maxW="200px"
        h="1px"
        bgGradient="linear(to-r, transparent, primary.300, transparent)"
      />
      <Text textStyle="monoLabel">已加载全部</Text>
    </Flex>
  );
}

/**
 * Hallmark · component: infinite-scroll · genre: modern-minimal · theme: Cobalt (adapted)
 * 长画布无限滚动封装（react-infinite-scroll-component）：
 * - scrollableTarget 指向页面自有的滚动容器 id；内部包装层禁用自身滚动
 * - 首屏内容不足以撑满容器时（scrollHeight <= clientHeight）自动继续加载，直到铺满或到底
 */
export default function InfiniteScrollList({
  dataLength,
  hasMore,
  loadMore,
  scrollableTarget,
  loader,
  endMessage,
  children,
}: InfiniteScrollListProps) {
  const containerRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (!scrollableTarget) return;
    containerRef.current = document.getElementById(scrollableTarget);
  }, [scrollableTarget]);

  // 首屏/追加后未铺满容器则自动加载下一页（避免"内容不够高、无法滚动"导致卡在第一页）
  useEffect(() => {
    const el = containerRef.current;
    if (!el || !hasMore) return;
    if (el.scrollHeight <= el.clientHeight) {
      loadMore();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dataLength, hasMore]);

  return (
    <InfiniteScroll
      dataLength={dataLength}
      next={loadMore}
      hasMore={hasMore}
      loader={loader ?? <DefaultLoader />}
      endMessage={endMessage ?? <DefaultEndMessage />}
      scrollableTarget={scrollableTarget}
      style={{ overflow: "visible" }}
    >
      {children}
    </InfiniteScroll>
  );
}
