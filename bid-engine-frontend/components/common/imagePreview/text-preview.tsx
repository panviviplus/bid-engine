import React from "react";
import { useDisclosure, ImageProps, Button } from "@chakra-ui/react";
import ImagePreview from "./preview";

type Props = {
  src: string;
  alt?: string;
  btnText?: string;
} & Omit<ImageProps, "onClick">;

export default function ChakraImagePreview(props: Props) {
  const { src, alt = "preview", btnText = "Text" } = props;
  const { isOpen, onOpen, onClose } = useDisclosure();

  return (
    <>
      {/* 小图 */}
      <Button
        color="primaryButton.600"
        variant="link"
        fontSize="sm"
        fontWeight="normal"
        onClick={onOpen}
      >
        {btnText}
      </Button>
      {/* 全屏蒙层 */}
      <ImagePreview src={src} alt={alt} isOpen={isOpen} onClose={onClose} />
    </>
  );
}
