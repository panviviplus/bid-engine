import React from "react";
import { Center, Spinner } from "@chakra-ui/react";

export default function SpinnerLoading() {
  return (
    <Center flex={1} borderRadius="md">
      <Spinner
        thickness="4px"
        speed="0.65s"
        emptyColor="white"
        color="primary.600"
        size="lg"
        my="auto"
        mx="auto"
      />
    </Center>
  );
}
