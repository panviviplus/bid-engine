import React from "react";
import { Image, useDisclosure, ImageProps } from "@chakra-ui/react";
import ImagePreview from "./preview";

type Props = {
  src: string;
  alt?: string;
} & Omit<ImageProps, "onClick">;

export default function ChakraImagePreview(props: Props) {
  const { src, alt = "preview", ...rest } = props;
  const { isOpen, onOpen, onClose } = useDisclosure();

  return (
    <>
      {/* 小图 */}
      <Image src={src} alt={alt} cursor="zoom-in" onClick={onOpen} {...rest} />

      {/* 全屏蒙层 */}
      <ImagePreview src={src} alt={alt} isOpen={isOpen} onClose={onClose} />
    </>
  );
}
