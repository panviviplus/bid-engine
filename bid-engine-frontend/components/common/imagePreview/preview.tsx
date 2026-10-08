import React from "react";
import {
  Image,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalCloseButton,
  ImageProps,
} from "@chakra-ui/react";

type Props = {
  src: string;
  alt?: string;
  isOpen: boolean;
  onClose: () => void;
} & Omit<ImageProps, "onClick">;

export default function ChakraImagePreview(props: Props) {
  const { src, alt = "preview", isOpen, onClose } = props;

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      size="full"
      motionPreset="scale"
      isCentered
      blockScrollOnMount
    >
      <ModalOverlay bg="blackAlpha.800" />
      <ModalContent bg="transparent" shadow="none" maxW="100vw" maxH="100vh">
        <ModalCloseButton color="white" zIndex={1} />
        <Image
          src={src}
          alt={alt}
          maxW="90vw"
          maxH="90vh"
          m="auto"
          objectFit="contain"
          cursor="zoom-out"
          onClick={onClose} // 点击图片本身也关闭
        />
      </ModalContent>
    </Modal>
  );
}
