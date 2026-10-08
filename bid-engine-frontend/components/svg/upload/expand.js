import { chakra } from "@chakra-ui/react";
import React from "react";

function ExpandSVG(props) {
  return (
    <chakra.svg
      width="16px"
      height="16px"
      viewBox="0 0 16 16"
      xmlns="http://www.w3.org/2000/svg"
      {...props}
    >
      <path
        d="M12 0L6 4 0 0m12 4L6 8 0 4"
        transform="matrix(-1 0 0 -1 14 12)"
        stroke="#999"
        fill="none"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.2"
      />
    </chakra.svg>
  );
}

export default ExpandSVG;
