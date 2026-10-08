import { ChakraProvider } from "@chakra-ui/react";
import defaultTheme from "@/theme/default";

export default function Chakra({ children }) {
  return <ChakraProvider theme={defaultTheme}>{children}</ChakraProvider>;
}
