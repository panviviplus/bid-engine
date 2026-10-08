"use client";

import {
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  ModalCloseButton,
  Flex,
  Image,
  Text,
  Button,
  Box,
} from "@chakra-ui/react";

type LoadingTipModalProps = {
  isOpen: boolean;
  onClose: () => void;
  title?: string;
  primaryText?: string;
  secondaryText?: string;
  imageSrc?: string;
  imageAlt?: string;
  imageWidth?: string;
  imageHeight?: string;
  buttonText?: string;
  onButtonClick?: () => void;
  isCentered?: boolean;
  closeOnOverlayClick?: boolean;
};

export default function LoadingTipModal({
  isOpen,
  onClose,
  title = "提示",
  primaryText = "正在重新解析，请稍候…",
  secondaryText = "解析需要一些时间，您可离开当前页面，稍后在列表查看结果。",
  imageSrc = "/images/empty/loading.gif",
  imageAlt = "loading",
  imageWidth = "220px",
  imageHeight = "200px",
  buttonText,
  onButtonClick,
  isCentered = true,
  closeOnOverlayClick = false,
}: LoadingTipModalProps) {
  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      isCentered={isCentered}
      closeOnOverlayClick={closeOnOverlayClick}
    >
      <ModalOverlay />
      <ModalContent bg="#f9fcf9" maxW="400px">
        <ModalHeader py={2}>
          <Box as="span" fontSize="md">
            {title}
          </Box>
        </ModalHeader>
        <ModalCloseButton top={3} />
        <ModalBody>
          <Flex
            direction="column"
            align="center"
            justify="center"
            gap={4}
            py={2}
          >
            <Image
              src={imageSrc}
              alt={imageAlt}
              w={imageWidth}
              h={imageHeight}
            />
            <Text fontSize="md" color="neutral.r85">
              {primaryText}
            </Text>
            <Text fontSize="sm" color="neutral.r65" textAlign="center">
              {secondaryText}
            </Text>
          </Flex>
        </ModalBody>
        {buttonText && (
          <ModalFooter>
            <Button
              w="4.75rem"
              h="8"
              borderRadius="2px"
              fontSize="sm"
              fontWeight={500}
              color="neutral.600"
              bg="white"
              mr="3"
              border="1px solid #D9D9D9"
              onClick={() => {
                if (onButtonClick) {
                  onButtonClick();
                } else {
                  onClose();
                }
              }}
            >
              {buttonText}
            </Button>
          </ModalFooter>
        )}
      </ModalContent>
    </Modal>
  );
}
