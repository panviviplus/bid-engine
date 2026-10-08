"use client";

/* Hallmark · component: rule-library (bid-audit V2) · genre: modern-minimal · theme: BidEngine deep-sea/gold
 * states: default · hover · focus-visible · active · loading · empty · disabled
 * Hallmark · pre-emit critique: P5 H5 E4 S5 R5 V4
 */

import React, { useCallback, useEffect, useMemo, useState } from "react";
import {
  Badge,
  Box,
  Button,
  Flex,
  FormControl,
  FormLabel,
  Input,
  Modal,
  ModalBody,
  ModalCloseButton,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  Select,
  Switch,
  Text,
  Textarea,
  useDisclosure,
  useToast,
} from "@chakra-ui/react";
import { FiPlus, FiTrash2, FiEdit3 } from "react-icons/fi";
import { BackButton } from "@/components/common/header-controls";
import EmptyState from "@/components/common/empty-state";
import DeleteConfirmModal from "@/components/common/delete-confirm-modal";
import { useReviewRules, useRuleSave, useRuleDelete } from "@/service/audit";
import {
  AuditRule,
  DIMENSION_META,
  DIMENSION_ORDER,
  SEVERITY_META,
} from "./types";

const EMPTY_FORM = {
  id: 0,
  dimension: "compliance",
  category: "",
  title: "",
  requirement: "",
  expected_evidence: "",
  severity: "medium",
  enabled: true,
};

export default function RuleLibrary() {
  const toast = useToast();
  const { rulesLoading, fetchRules } = useReviewRules({});
  const { saveLoading, fetchSave } = useRuleSave();
  const { fetchDelete } = useRuleDelete();

  const [dimensionFilter, setDimensionFilter] = useState("");
  const [rules, setRules] = useState<AuditRule[]>([]);
  const [form, setForm] = useState({ ...EMPTY_FORM });
  const [deleteTarget, setDeleteTarget] = useState<AuditRule | null>(null);
  const [deleting, setDeleting] = useState(false);
  const { isOpen, onOpen, onClose } = useDisclosure();
  const {
    isOpen: deleteIsOpen,
    onOpen: deleteOnOpen,
    onClose: deleteOnClose,
  } = useDisclosure();

  const load = useCallback(async () => {
    const res = await fetchRules({
      params: { dimension: dimensionFilter || undefined },
    });
    setRules(res?.data?.data?.list || []);
  }, [fetchRules, dimensionFilter]);

  useEffect(() => {
    load();
  }, [load]);

  const counts = useMemo(() => {
    const c: Record<string, number> = { "": rules.length };
    for (const r of rules) c[r.dimension] = (c[r.dimension] || 0) + 1;
    return c;
  }, [rules]);

  const openCreate = () => {
    setForm({ ...EMPTY_FORM, dimension: dimensionFilter || "compliance" });
    onOpen();
  };

  const openEdit = (rule: AuditRule) => {
    setForm({
      id: rule.id,
      dimension: rule.dimension,
      category: rule.category,
      title: rule.title,
      requirement: rule.requirement,
      expected_evidence: rule.expected_evidence,
      severity: rule.severity,
      enabled: rule.enabled,
    });
    onOpen();
  };

  const submit = useCallback(async () => {
    if (!form.title.trim()) {
      toast({ title: "请填写规则名称", status: "warning" });
      return;
    }
    try {
      const res = await fetchSave({
        data: {
          id: form.id || undefined,
          dimension: form.dimension,
          category: form.category.trim(),
          title: form.title.trim(),
          requirement: form.requirement.trim(),
          expected_evidence: form.expected_evidence.trim(),
          severity: form.severity,
          enabled: form.enabled,
        },
      });
      if (res?.data?.code === 200) {
        toast({
          title: form.id ? "规则已更新" : "规则已创建",
          status: "success",
        });
        onClose();
        load();
      } else {
        toast({ title: res?.data?.msg || "保存失败", status: "error" });
      }
    } catch (e: any) {
      toast({ title: e?.response?.data?.msg || "保存失败", status: "error" });
    }
  }, [form, fetchSave, toast, onClose, load]);

  const confirmDelete = useCallback(async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      const res = await fetchDelete({ data: { id: deleteTarget.id } });
      if (res?.data?.code === 200) {
        toast({ title: "已删除", status: "success" });
        setDeleteTarget(null);
        deleteOnClose();
        load();
      } else {
        toast({ title: res?.data?.msg || "删除失败", status: "error" });
      }
    } catch (e: any) {
      toast({ title: e?.response?.data?.msg || "删除失败", status: "error" });
    } finally {
      setDeleting(false);
    }
  }, [deleteTarget, fetchDelete, toast, deleteOnClose, load]);

  const toggleEnabled = useCallback(
    async (rule: AuditRule) => {
      try {
        await fetchSave({
          data: {
            id: rule.id,
            dimension: rule.dimension,
            category: rule.category,
            title: rule.title,
            requirement: rule.requirement,
            expected_evidence: rule.expected_evidence,
            severity: rule.severity,
            enabled: !rule.enabled,
          },
        });
        load();
      } catch {
        toast({ title: "更新失败", status: "error" });
      }
    },
    [fetchSave, load, toast],
  );

  return (
    <Flex
      direction="column"
      h="full"
      minH={0}
      bg="workbench.canvas"
      p={{ base: 3, md: 6 }}
    >
      <Flex align="center" gap={3} mb={4} flexWrap="wrap">
        <BackButton href="/bid-audit" />
        <Text fontSize="xl" fontWeight={800} color="workbench.control">
          审核规则库
        </Text>
        <Text fontSize="xs" color="neutral.500">
          企业自有检查点，创建审核时自动并入审核清单
        </Text>
        <Box flex={1} />
        <Button
          leftIcon={<FiPlus />}
          colorScheme="primary"
          onClick={openCreate}
        >
          新建规则
        </Button>
      </Flex>

      <Flex gap={2} mb={4} flexWrap="wrap">
        <Badge
          as="button"
          onClick={() => setDimensionFilter("")}
          variant={dimensionFilter === "" ? "solid" : "subtle"}
          bg={dimensionFilter === "" ? "primary.600" : "neutral.100"}
          color={dimensionFilter === "" ? "white" : "neutral.600"}
          fontSize="11px"
          borderRadius="full"
          px={2.5}
          py={1}
          cursor="pointer"
        >
          全部 {counts[""] || 0}
        </Badge>
        {DIMENSION_ORDER.map((dim) => (
          <Badge
            key={dim}
            as="button"
            onClick={() => setDimensionFilter(dim)}
            variant={dimensionFilter === dim ? "solid" : "subtle"}
            bg={
              dimensionFilter === dim ? "primary.600" : DIMENSION_META[dim].bg
            }
            color={
              dimensionFilter === dim ? "white" : DIMENSION_META[dim].color
            }
            fontSize="11px"
            borderRadius="full"
            px={2.5}
            py={1}
            cursor="pointer"
          >
            {DIMENSION_META[dim].label} {counts[dim] || 0}
          </Badge>
        ))}
      </Flex>

      <Box flex={1} minH={0} overflowY="auto" className="thin-scrollbars">
        {rulesLoading ? (
          <Text fontSize="sm" color="neutral.400" py={10} textAlign="center">
            加载中…
          </Text>
        ) : rules.length === 0 ? (
          <EmptyState
            title="还没有企业规则"
            description="把常用的检查口径沉淀成规则，创建审核时会自动并入清单"
            action={
              <Button size="sm" colorScheme="primary" onClick={openCreate}>
                新建第一条规则
              </Button>
            }
          />
        ) : (
          <Flex direction="column" gap={2.5}>
            {rules.map((rule) => {
              const dim = DIMENSION_META[rule.dimension];
              return (
                <Box
                  key={rule.id}
                  bg="white"
                  border="1px solid"
                  borderColor={rule.enabled ? "neutral.200" : "neutral.100"}
                  borderRadius="12px"
                  p={3.5}
                  opacity={rule.enabled ? 1 : 0.65}
                >
                  <Flex align="center" gap={2} flexWrap="wrap" mb={1.5}>
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
                    {rule.category && (
                      <Badge
                        variant="outline"
                        colorScheme="gray"
                        fontSize="10px"
                        borderRadius="full"
                      >
                        {rule.category}
                      </Badge>
                    )}
                    <Text
                      fontSize="10px"
                      fontWeight={700}
                      color={
                        SEVERITY_META[rule.severity]?.color || "neutral.400"
                      }
                    >
                      {SEVERITY_META[rule.severity]?.label || ""}
                    </Text>
                    <Box flex={1} />
                    <Text fontSize="10px" color="neutral.400">
                      命中 {rule.hit_count} 次 · v{rule.version}
                    </Text>
                    <Switch
                      size="sm"
                      isChecked={rule.enabled}
                      onChange={() => toggleEnabled(rule)}
                    />
                  </Flex>
                  <Text fontSize="14px" fontWeight={700} color="neutral.800">
                    {rule.title}
                  </Text>
                  {rule.requirement && (
                    <Text
                      mt={1}
                      fontSize="12px"
                      color="neutral.600"
                      whiteSpace="pre-wrap"
                    >
                      {rule.requirement}
                    </Text>
                  )}
                  {rule.expected_evidence && (
                    <Text mt={1} fontSize="11px" color="neutral.500">
                      期望证据：{rule.expected_evidence}
                    </Text>
                  )}
                  <Flex mt={2.5} gap={2}>
                    <Button
                      size="xs"
                      variant="outline"
                      leftIcon={<FiEdit3 size={11} />}
                      onClick={() => openEdit(rule)}
                    >
                      编辑
                    </Button>
                    <Button
                      size="xs"
                      variant="ghost"
                      colorScheme="error"
                      leftIcon={<FiTrash2 size={11} />}
                      onClick={() => {
                        setDeleteTarget(rule);
                        deleteOnOpen();
                      }}
                    >
                      删除
                    </Button>
                  </Flex>
                </Box>
              );
            })}
          </Flex>
        )}
      </Box>

      <Modal
        isOpen={isOpen}
        onClose={onClose}
        isCentered
        size="lg"
        scrollBehavior="inside"
      >
        <ModalOverlay bg="blackAlpha.600" />
        <ModalContent mx={3} borderRadius="14px" maxH="calc(100dvh - 2rem)">
          <ModalHeader
            fontSize="md"
            borderBottom="1px solid"
            borderColor="neutral.100"
          >
            {form.id ? "编辑审核规则" : "新建审核规则"}
          </ModalHeader>
          <ModalCloseButton />
          <ModalBody py={4}>
            <Flex direction="column" gap={3}>
              <Flex gap={3}>
                <FormControl flex={1}>
                  <FormLabel fontSize="xs" color="neutral.500" mb={1}>
                    审核维度
                  </FormLabel>
                  <Select
                    size="sm"
                    value={form.dimension}
                    onChange={(e) =>
                      setForm((f) => ({ ...f, dimension: e.target.value }))
                    }
                  >
                    {DIMENSION_ORDER.map((dim) => (
                      <option key={dim} value={dim}>
                        {DIMENSION_META[dim].label}
                      </option>
                    ))}
                  </Select>
                </FormControl>
                <FormControl flex={1}>
                  <FormLabel fontSize="xs" color="neutral.500" mb={1}>
                    风险级别
                  </FormLabel>
                  <Select
                    size="sm"
                    value={form.severity}
                    onChange={(e) =>
                      setForm((f) => ({ ...f, severity: e.target.value }))
                    }
                  >
                    {Object.entries(SEVERITY_META).map(([key, meta]) => (
                      <option key={key} value={key}>
                        {meta.label}
                      </option>
                    ))}
                  </Select>
                </FormControl>
              </Flex>
              <Flex gap={3}>
                <FormControl flex={1}>
                  <FormLabel fontSize="xs" color="neutral.500" mb={1}>
                    规则名称 *
                  </FormLabel>
                  <Input
                    size="sm"
                    value={form.title}
                    onChange={(e) =>
                      setForm((f) => ({ ...f, title: e.target.value }))
                    }
                    placeholder="如：承装类许可证是否在有效期内"
                  />
                </FormControl>
                <FormControl w="160px" flexShrink={0}>
                  <FormLabel fontSize="xs" color="neutral.500" mb={1}>
                    分类
                  </FormLabel>
                  <Input
                    size="sm"
                    value={form.category}
                    onChange={(e) =>
                      setForm((f) => ({ ...f, category: e.target.value }))
                    }
                    placeholder="如：资格材料"
                  />
                </FormControl>
              </Flex>
              <FormControl>
                <FormLabel fontSize="xs" color="neutral.500" mb={1}>
                  判定口径
                </FormLabel>
                <Textarea
                  size="sm"
                  rows={3}
                  value={form.requirement}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, requirement: e.target.value }))
                  }
                  placeholder="写清楚企业内部的检查口径与判定标准"
                />
              </FormControl>
              <FormControl>
                <FormLabel fontSize="xs" color="neutral.500" mb={1}>
                  期望证据
                </FormLabel>
                <Input
                  size="sm"
                  value={form.expected_evidence}
                  onChange={(e) =>
                    setForm((f) => ({
                      ...f,
                      expected_evidence: e.target.value,
                    }))
                  }
                  placeholder="如：加盖公章的许可证扫描件"
                />
              </FormControl>
              <FormControl>
                <Flex align="center" gap={2}>
                  <Switch
                    isChecked={form.enabled}
                    onChange={(e) =>
                      setForm((f) => ({ ...f, enabled: e.target.checked }))
                    }
                  />
                  <Text fontSize="xs" color="neutral.600">
                    启用该规则（创建审核时自动并入清单）
                  </Text>
                </Flex>
              </FormControl>
            </Flex>
          </ModalBody>
          <ModalFooter borderTop="1px solid" borderColor="neutral.100">
            <Button variant="ghost" mr={3} onClick={onClose}>
              取消
            </Button>
            <Button
              colorScheme="primary"
              isLoading={saveLoading}
              onClick={submit}
            >
              保存
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      <DeleteConfirmModal
        isOpen={deleteIsOpen}
        onClose={() => {
          setDeleteTarget(null);
          deleteOnClose();
        }}
        title="删除审核规则"
        description={
          <>
            确定删除规则
            <Text as="span" fontWeight="700" color="workbench.text">
              {deleteTarget?.title || ""}
            </Text>
            吗？已创建的审核项目不受影响。
          </>
        }
        isLoading={deleting}
        handleConfirm={confirmDelete}
      />
    </Flex>
  );
}
