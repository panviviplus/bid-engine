import { chakra } from "@chakra-ui/react";
import React from "react";

function DeleteSVG(props) {
  return (
    <chakra.svg
      width="12px"
      height="12px"
      viewBox="0 0 12 12"
      version="1.1"
      xmlns="http://www.w3.org/2000/svg"
      {...props}
    >
      <g
        id="系统管理"
        stroke="none"
        strokeWidth="1"
        fill="none"
        fillRule="evenodd"
      >
        <g
          id="系统管理-知识库普通用户备份"
          transform="translate(-264, -170)"
          fill="currentColor"
          fillRule="nonzero"
        >
          <g id="新对话" transform="translate(60, 110)">
            <g id="知识库" transform="translate(29, 31)">
              <g id="删减" transform="translate(175, 29)">
                <rect
                  id="矩形"
                  opacity="0"
                  x="0"
                  y="0"
                  width="12"
                  height="12"
                />
                <path
                  d="M2.496,6.21214062 L2.596,6.21214062 L2.496,6.21214062 Z M0.816,0.936 L0.816,11.088 L10.992,11.088 L10.992,0.936 L0.816,0.936 Z M1.67314286,1.79314286 L10.2445714,1.79314286 L10.2445714,10.3645714 L1.67314286,10.3645714 Z M3.36,6.576 C3.024,6.576 2.76,6.312 2.76,5.976 C2.76,5.64 3.024,5.4 3.36,5.4 L8.448,5.4 C8.784,5.4 9.048,5.64 9.048,5.976 C9.024,6.336 8.76,6.576 8.448,6.576 L3.36,6.576 Z"
                  id="形状"
                />
              </g>
            </g>
          </g>
        </g>
      </g>
    </chakra.svg>
  );
}

export default DeleteSVG;
