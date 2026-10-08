import React from "react";
import { Icon } from "@chakra-ui/react";

export default function GridIcon(props: any) {
  return (
    <Icon viewBox="0 0 24 24" {...props}>
      <path
        fill="currentColor"
        d="M3 3h8v8H3V3zm10 0h8v8h-8V3zM3 13h8v8H3v-8zm10 0h8v8h-8v-8z"
      />
    </Icon>
  );
}

