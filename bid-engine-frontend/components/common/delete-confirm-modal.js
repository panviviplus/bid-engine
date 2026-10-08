/* Hallmark · component: delete-confirm-modal · genre: modern-minimal · theme: BidEngine deep-sea/gold (preserved tokens)
 * states: default · hover · focus-visible · active · disabled · loading
 * geometry: fluid width (≤30rem, viewport-clamped) · auto height (≤100dvh-2rem) · scrollable body
 * Hallmark · pre-emit critique: P5 H5 E5 S5 R5 V4
 */

import React from "react";
import {
  Button,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalFooter,
  ModalBody,
  ModalCloseButton,
} from "@chakra-ui/react";

export default function DeleteConfirmModal({
  onClose,
  isOpen,
  description,
  title,
  isLoading,
  handleConfirm,
}) {
  // 兼容传空标题的调用方（例如删除文件确认）：无标题时不渲染空表头，
  // 改为给正文让出右侧空间，避免关闭按钮压住文案。
  const hasTitle =
    typeof title === "string" ? title.trim().length > 0 : Boolean(title);

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      isCentered
      size="md"
      scrollBehavior="inside"
      returnFocusOnClose={false}
    >
      <ModalOverlay bg="blackAlpha.600" />
      <ModalContent
        mx={3}
        maxW={{ base: "calc(100vw - 1.5rem)", sm: "30rem" }}
        maxH="calc(100dvh - 2rem)"
        borderRadius="14px"
        overflow="hidden"
      >
        {hasTitle && (
          <ModalHeader
            px={5}
            py={4}
            pr={14}
            fontSize="md"
            fontWeight="700"
            color="workbench.text"
            borderBottom="1px solid"
            borderColor="neutral.100"
          >
            {title}
          </ModalHeader>
        )}
        <ModalCloseButton size="md" top={2} insetEnd={3} />
        <ModalBody
          px={5}
          py={hasTitle ? 5 : 6}
          pr={hasTitle ? 5 : 12}
          fontSize="md"
          lineHeight="tall"
          color="neutral.700"
          overflowWrap="anywhere"
        >
          {description}
        </ModalBody>
        <ModalFooter px={5} pt={0} pb={5} gap={2}>
          <Button variant="outline" size="sm" minW="5.25rem" onClick={onClose}>
            取消
          </Button>
          <Button
            colorScheme="primary"
            size="sm"
            minW="5.25rem"
            onClick={handleConfirm}
            isLoading={isLoading}
          >
            确定
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
}
