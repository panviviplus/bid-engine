import React, { useEffect } from "react";
import {
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalFooter,
  ModalBody,
  ModalCloseButton,
  Button,
  Box,
  Text,
  useDisclosure,
} from "@chakra-ui/react";

interface SimpleListModalProps {
  data: any[];
  isShowKey: string[];
  title?: string;
}

export default function SimpleListModal({
  data,
  isShowKey,
  title = "错误信息",
}: SimpleListModalProps) {
  const { isOpen, onOpen, onClose } = useDisclosure();
  useEffect(() => {
    if (data?.length) {
      onOpen();
    }
  }, [data, onOpen]);
  return (
    <Modal isOpen={isOpen} onClose={onClose} size="lg" isCentered>
      <ModalOverlay />
      <ModalContent>
        <ModalHeader fontSize="sm">{title}</ModalHeader>
        <ModalCloseButton />
        <ModalBody>
          <Box
            as="pre"
            p={4}
            bg="gray.50"
            borderRadius="md"
            fontSize="md"
            whiteSpace="pre-wrap"
            wordBreak="break-all"
            maxHeight="300px"
            overflowY="auto"
          >
            {data?.map((item, index) => (
              <Text key={index}>
                {isShowKey?.map((key) => `${item[key]}   `)}
              </Text>
            ))}
          </Box>
        </ModalBody>
        <ModalFooter>
          <Button
            w="4.75rem"
            h="8"
            borderRadius="2px"
            fontSize="md"
            fontWeight={500}
            bg="primary.600"
            color="#FFFFFF"
            onClick={onClose}
          >
            关闭
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
}
