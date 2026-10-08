"use client";

import {
  Heading,
  Modal,
  ModalBody,
  ModalCloseButton,
  ModalContent,
  ModalHeader,
  ModalOverlay,
} from "@chakra-ui/react";
import { useRef } from "react";

import SignupForm from "./SignupForm";

type SignupModalProps = {
  isOpen: boolean;
  onClose: () => void;
};

export default function SignupModal({ isOpen, onClose }: SignupModalProps) {
  const initialFocusRef = useRef<HTMLInputElement>(null);

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      initialFocusRef={initialFocusRef}
      returnFocusOnClose
      closeOnOverlayClick={false}
      scrollBehavior="inside"
      motionPreset="scale"
      isCentered
    >
      <ModalOverlay bg="blackAlpha.700" backdropFilter="blur(4px)" />
      <ModalContent
        w={{ base: "full", md: "34rem" }}
        maxW={{ base: "full", md: "34rem" }}
        h={{ base: "100dvh", md: "auto" }}
        maxH={{ base: "100dvh", md: "calc(100dvh - 4rem)" }}
        m={{ base: 0, md: 4 }}
        borderRadius={{ base: 0, md: "24px" }}
        overflow="hidden"
        bg="signin.paper"
        color="neutral.900"
        boxShadow="signinCard"
      >
        <ModalHeader
          px={{ base: 5, md: 8 }}
          pt={{
            base: "max(var(--space-sm), env(safe-area-inset-top))",
            md: 6,
          }}
          pb={4}
          borderBottom="1px solid"
          borderColor="neutral.200"
        >
          <Heading as="h2" fontSize="xl" letterSpacing="-0.02em">
            注册标擎
          </Heading>
        </ModalHeader>
        <ModalCloseButton
          top={{
            base: "max(var(--space-xs), env(safe-area-inset-top))",
            md: 4,
          }}
          right={{ base: 4, md: 6 }}
          boxSize="44px"
          borderRadius="lg"
          _focusVisible={{
            outline: "none",
            boxShadow: "0 0 0 2px var(--chakra-colors-gold-400)",
          }}
        />
        <ModalBody
          px={{ base: 5, md: 8 }}
          pt={5}
          pb={{
            base: "max(var(--space-md), env(safe-area-inset-bottom))",
            md: 7,
          }}
          overflowY="auto"
        >
          <SignupForm onClose={onClose} initialFocusRef={initialFocusRef} />
        </ModalBody>
      </ModalContent>
    </Modal>
  );
}
