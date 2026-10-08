import { useToast } from "@chakra-ui/react";
import type { UseToastOptions } from "@chakra-ui/react";
import { useCallback } from "react";

export const useCustomToast = () => {
  const toast = useToast();

  const showToast = useCallback(
    ({ id, status = "error", title, ...options }: UseToastOptions) => {
      const opt: UseToastOptions = {
        title,
        position: "top",
        status,
        duration: 2000,
        isClosable: true,
        ...options,
      };
      if (id && !toast.isActive(id)) {
        toast({
          id,
          ...opt,
        });
      } else if (!id) {
        toast(opt);
      }
      return toast;
    },
    [toast],
  );

  return showToast;
};
