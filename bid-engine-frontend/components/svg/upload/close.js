import { Icon } from "@chakra-ui/react";
import React from "react";

function CloseSVG(props) {
  return (
    <Icon
      width="16px"
      height="16px"
      viewBox="0 0 16 16"
      version="1.1"
      xmlns="http://www.w3.org/2000/svg"
      {...props}
    >
      <path
        d="M2.784 2.089l.069.058L8 7.293l5.146-5.147.069-.058a.5.5 0 0 1 .638.765h0L8.707 8l5.147 5.146a.5.5 0 0 1-.638.765l-.069-.058L8 8.707l-5.146 5.147-.069.058a.5.5 0 0 1-.638-.765h0L7.293 8 2.146 2.854a.5.5 0 0 1 .638-.765z"
        fill="currentColor"
        fillRule="evenodd"
      />
    </Icon>
  );
}

export default CloseSVG;
