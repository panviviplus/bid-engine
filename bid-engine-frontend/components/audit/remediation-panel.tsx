"use client";

/* Hallmark · component: remediation-panel (bid-audit V2) · genre: modern-minimal · theme: BidEngine deep-sea/gold
 * states: default · hover · focus-visible · active · loading · empty
 * Hallmark · pre-emit critique: P5 H5 E4 S5 R5 V4
 */

import React, { useMemo, useState } from "react";
import {
  Badge,
  Box,
  Button,
  Drawer,
  DrawerBody,
  DrawerCloseButton,
  DrawerContent,
  DrawerHeader,
  DrawerOverlay,
  Flex,
  Select,
  Text,
  useToast,
} from "@chakra-ui/react";
import {
  REMEDIATION_META,
  Remediation,
  DIMENSION_META,
  SEVERITY_META,
} from "./types";

interface Props {
  isOpen: boolean;
  onClose: () => void;
  remediations: Remediation[];
  onUpdate: (
    id: number,
    payload: { status?: string; note?: string },
  ) => Promise<void>;
  onJumpToItem: (itemId: number) => void;
}

const STATUS_FILTERS = [
  { key: "all", label: "全部" },
  { key: "todo", label: "待整改" },
  { key: "doing", label: "整改中" },
  { key: "done", label: "已完成" },
  { key: "ignored", label: "已忽略" },
];

export default function RemediationPanel({
  isOpen,
  onClose,
  remediations,
  onUpdate,
  onJumpToItem,
}: Props) {
  const toast = useToast();
  const [filter, setFilter] = useState("all");
  const [pendingId, setPendingId] = useState<number | null>(null);

  const counts = useMemo(() => {
    const c: Record<string, number> = { all: remediations.length };
    for (const r of remediations) c[r.status] = (c[r.status] || 0) + 1;
    return c;
  }, [remediations]);

  const list = useMemo(
    () =>
      filter === "all"
        ? remediations
        : remediations.filter((r) => r.status === filter),
    [remediations, filter],
  );

  const update = async (id: number, payload: { status?: string }) => {
    setPendingId(id);
    try {
      await onUpdate(id, payload);
    } catch (e: any) {
      toast({ title: e?.message || "更新失败", status: "error" });
    } finally {
      setPendingId(null);
    }
  };

  return (
    <Drawer isOpen={isOpen} onClose={onClose} placement="right" size="md">
      <DrawerOverlay bg="blackAlpha.500" />
      <DrawerContent>
        <DrawerCloseButton />
        <DrawerHeader
          fontSize="md"
          borderBottom="1px solid"
          borderColor="neutral.100"
        >
          整改清单
          <Text mt={1} fontSize="xs" fontWeight={400} color="neutral.500">
            共 {remediations.length} 项 · 未闭环 {counts.todo || 0} 项
          </Text>
        </DrawerHeader>
        <DrawerBody px={4} py={3}>
          <Flex gap={2} mb={3} flexWrap="wrap">
            {STATUS_FILTERS.map((f) => (
              <Badge
                key={f.key}
                as="button"
                onClick={() => setFilter(f.key)}
                variant={filter === f.key ? "solid" : "subtle"}
                bg={filter === f.key ? "primary.600" : "neutral.100"}
                color={filter === f.key ? "white" : "neutral.600"}
                fontSize="11px"
                borderRadius="full"
                px={2.5}
                py={1}
                cursor="pointer"
                transition="all 0.15s"
                _hover={{ opacity: 0.85 }}
                _focusVisible={{
                  outline: "2px solid",
                  outlineColor: "gold.400",
                  outlineOffset: "2px",
                }}
              >
                {f.label} {counts[f.key] || 0}
              </Badge>
            ))}
          </Flex>

          {list.length === 0 ? (
            <Flex direction="column" align="center" gap={2} py={12}>
              <Text fontSize="sm" color="neutral.400">
                当前筛选下没有整改项
              </Text>
            </Flex>
          ) : (
            <Flex direction="column" gap={2.5}>
              {list.map((r) => {
                const rm = REMEDIATION_META[r.status] || REMEDIATION_META.todo;
                const dim = DIMENSION_META[r.dimension];
                return (
                  <Box
                    key={r.id}
                    border="1px solid"
                    borderColor="neutral.200"
                    borderRadius="12px"
                    p={3}
                    bg="white"
                  >
                    <Flex align="center" gap={2} flexWrap="wrap" mb={1.5}>
                      <Badge
                        variant="subtle"
                        bg={rm.bg}
                        color={rm.color}
                        fontSize="10px"
                        borderRadius="full"
                        px={2}
                      >
                        {rm.label}
                      </Badge>
                      {dim && (
                        <Badge
                          variant="subtle"
                          bg={dim.bg}
                          color={dim.color}
                          fontSize="10px"
                          borderRadius="full"
                          px={2}
                        >
                          {dim.label}
                        </Badge>
                      )}
                      <Text
                        fontSize="10px"
                        fontWeight={700}
                        color={
                          SEVERITY_META[r.severity]?.color || "neutral.400"
                        }
                      >
                        {SEVERITY_META[r.severity]?.label || ""}
                      </Text>
                    </Flex>
                    <Text
                      fontSize="13px"
                      fontWeight={600}
                      color="neutral.800"
                      cursor="pointer"
                      _hover={{ color: "primary.600" }}
                      onClick={() => onJumpToItem(r.checklist_item_id)}
                    >
                      {r.title}
                    </Text>
                    {r.suggestion && (
                      <Text
                        mt={1}
                        fontSize="11px"
                        color="neutral.600"
                        whiteSpace="pre-wrap"
                      >
                        {r.suggestion}
                      </Text>
                    )}
                    <Flex mt={2.5} align="center" gap={2} flexWrap="wrap">
                      <Select
                        size="sm"
                        w="130px"
                        value={r.status}
                        isDisabled={pendingId === r.id}
                        onChange={(e) =>
                          update(r.id, { status: e.target.value })
                        }
                      >
                        {Object.entries(REMEDIATION_META).map(([key, meta]) => (
                          <option key={key} value={key}>
                            {meta.label}
                          </option>
                        ))}
                      </Select>
                      <Button
                        size="sm"
                        variant="outline"
                        isLoading={pendingId === r.id && r.status !== "done"}
                        onClick={() => update(r.id, { status: "done" })}
                      >
                        标记完成
                      </Button>
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => onJumpToItem(r.checklist_item_id)}
                      >
                        查看检查项
                      </Button>
                    </Flex>
                  </Box>
                );
              })}
            </Flex>
          )}
        </DrawerBody>
      </DrawerContent>
    </Drawer>
  );
}
