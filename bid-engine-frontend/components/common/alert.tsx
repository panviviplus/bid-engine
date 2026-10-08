import { useRef } from "react";
import {
  Button,
  AlertDialog,
  AlertDialogOverlay,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogBody,
  AlertDialogFooter,
} from "@chakra-ui/react";

export default function Alert({
  title,
  content,
  isOpen,
  onClose,
  handleConfirm,
}) {
  const cancelRef = useRef<HTMLButtonElement>(null);

  return (
    <AlertDialog
      isOpen={isOpen}
      leastDestructiveRef={cancelRef}
      onClose={onClose}
    >
      <AlertDialogOverlay>
        <AlertDialogContent>
          <AlertDialogHeader fontSize="md" fontWeight="bold">
            {title}
          </AlertDialogHeader>

          <AlertDialogBody fontSize="sm">{content}</AlertDialogBody>

          <AlertDialogFooter>
            <Button
              w="4.75rem"
              h="8"
              borderRadius="2px"
              fontSize="md"
              fontWeight={500}
              color="neutral.600"
              bg="white"
              border="1px solid #D9D9D9"
              ref={cancelRef}
              onClick={onClose}
            >
              取消
            </Button>
            <Button
              w="4.75rem"
              h="8"
              type="submit"
              bg="primary.600"
              borderRadius="2px"
              color="#FFFFFF"
              fontWeight={500}
              fontSize="md"
              onClick={() => {
                handleConfirm();
                onClose();
              }}
              ml={3}
            >
              确认
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialogOverlay>
    </AlertDialog>
  );
}
