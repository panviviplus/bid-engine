"use client";

/* eslint-disable no-nested-ternary */

/* Hallmark · component: intel-run-detail · genre: modern-minimal · theme: BidEngine deep-sea/gold (preserved tokens)
 * states: default · hover · focus-visible · active · disabled · loading · empty · error
 */
import React, { useCallback, useEffect, useState } from "react";
import {
  Badge,
  Box,
  Button,
  Modal,
  ModalBody,
  ModalCloseButton,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  Skeleton,
  Stack,
  Table,
  TableContainer,
  Tbody,
  Td,
  Text,
  Th,
  Thead,
  Tr,
} from "@chakra-ui/react";

import {
  useIntelRunDetail,
  type IntelRun,
  type IntelRunSourceDetail,
} from "@/service/intel";

const STATUS_SCHEME: Record<string, string> = {
  success: "success",
  failed: "error",
  running: "info",
};

export default function RunDetailModal({
  isOpen,
  onClose,
  run,
}: {
  isOpen: boolean;
  onClose: () => void;
  run: IntelRun | null;
}) {
  const { fetchDetail } = useIntelRunDetail();
  const [sources, setSources] = useState<IntelRunSourceDetail[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    if (!run) return;
    setLoading(true);
    setError("");
    try {
      const res = await fetchDetail({ url: `/zb/intel/runs/${run.run_id}` });
      setSources((res?.data?.data?.sources || []) as IntelRunSourceDetail[]);
    } catch (err: any) {
      setSources([]);
      setError(
        err?.response?.data?.message || err?.message || "批次明细加载失败",
      );
    } finally {
      setLoading(false);
    }
  }, [fetchDetail, run]);

  useEffect(() => {
    if (isOpen && run) load();
  }, [isOpen, run, load]);

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      size="4xl"
      isCentered
      scrollBehavior="inside"
    >
      <ModalOverlay />
      <ModalContent maxH="88dvh">
        <ModalHeader pb={2}>
          采集批次明细
          <Text mt={1} fontSize="sm" fontWeight="400" color="workbench.muted">
            {run?.run_id} ·{" "}
            {run?.trigger_type === "manual" ? "手动触发" : "定时触发"} ·{" "}
            {run?.started_at}
          </Text>
        </ModalHeader>
        <ModalCloseButton />
        <ModalBody>
          {loading ? (
            <Stack spacing={3}>
              {[0, 1, 2].map((key) => (
                <Skeleton key={key} height="48px" borderRadius="md" />
              ))}
            </Stack>
          ) : error ? (
            <Box>
              <Text color="error.600" fontSize="sm" mb={3}>
                {error}
              </Text>
              <Button size="sm" minH="44px" onClick={load}>
                重新加载
              </Button>
            </Box>
          ) : sources.length === 0 ? (
            <Text fontSize="sm" color="workbench.muted">
              该批次没有源明细记录（可能是批次刚开始或源任务投递失败）。
            </Text>
          ) : (
            <TableContainer>
              <Table size="sm">
                <Thead>
                  <Tr>
                    <Th>采集源</Th>
                    <Th w="90px">状态</Th>
                    <Th isNumeric>发现</Th>
                    <Th isNumeric>入库</Th>
                    <Th isNumeric>跳过</Th>
                    <Th isNumeric>耗时</Th>
                    <Th>错误</Th>
                  </Tr>
                </Thead>
                <Tbody>
                  {sources.map((item) => (
                    <Tr key={item.source_key}>
                      <Td>
                        <Text fontSize="sm">
                          {item.source_name || item.source_key}
                        </Text>
                        <Text fontSize="xs" color="workbench.muted">
                          {item.source_key}
                        </Text>
                      </Td>
                      <Td>
                        <Badge
                          colorScheme={STATUS_SCHEME[item.status] || "neutral"}
                          variant="subtle"
                        >
                          {item.status}
                        </Badge>
                      </Td>
                      <Td isNumeric>{item.discovered}</Td>
                      <Td isNumeric>{item.inserted}</Td>
                      <Td isNumeric>{item.skipped}</Td>
                      <Td isNumeric>{item.duration_ms} ms</Td>
                      <Td whiteSpace="normal" fontSize="xs">
                        {item.error || "-"}
                      </Td>
                    </Tr>
                  ))}
                </Tbody>
              </Table>
            </TableContainer>
          )}
        </ModalBody>
        <ModalFooter>
          <Button h="44px" variant="ghost" onClick={onClose}>
            关闭
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
}
