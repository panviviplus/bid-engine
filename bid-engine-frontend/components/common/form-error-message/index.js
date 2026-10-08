import React from "react";
import { Box, FormErrorMessage } from "@chakra-ui/react";

export default function ErrorMessage({ children, ...props }) {
  return (
    <Box display="flex" fontSize="xs" mt={1} {...props}>
      &nbsp;
      <FormErrorMessage mt={0}>{children}</FormErrorMessage>
    </Box>
  );
}
