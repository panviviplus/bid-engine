import React from "react";
import {
  Menu,
  MenuButton,
  MenuList,
  MenuItem,
  Button,
  Portal,
  Icon,
} from "@chakra-ui/react";
import { DownloadIcon } from "@chakra-ui/icons";

interface ExportButtonProps {
  /** 导出回调，type 为 pdf 或 docx */
  // eslint-disable-next-line no-unused-vars
  exportDoc: (type: "pdf" | "docx") => void | Promise<void>;
  /** 按钮文案，默认“导出” */
  label?: string;
  /** 是否禁用 */
  isDisabled?: boolean;
  w: string;
  h: number | string;
  bg: string;
  /** 按钮颜色方案 */
  colorScheme?: string;
  color: string;
  /** 是否显示左侧图标 */
  showIcon?: boolean;
  loading: boolean;
}

/**
 * 导出按钮：点击后弹出下拉菜单，支持选择 pdf 或 docx 格式，
 * 选择后把类型回传给父组件的 exportDoc 方法。
 */
function ExportButton({
  exportDoc,
  label = "导出",
  isDisabled = false,
  colorScheme = "blue",
  color,
  showIcon = true,
  w,
  h,
  bg,
  loading,
}: ExportButtonProps) {
  const handleSelect = async (type: "pdf" | "docx") => {
    await exportDoc(type);
  };

  return (
    <Menu>
      <MenuButton
        as={Button}
        w={w}
        h={h}
        bg={bg}
        color={color}
        colorScheme={colorScheme}
        isDisabled={isDisabled}
        leftIcon={showIcon ? <DownloadIcon /> : undefined}
        borderRadius="2px"
        isLoading={loading}
      >
        {label}
      </MenuButton>
      <Portal>
        <MenuList zIndex={1500}>
          <MenuItem
            icon={<Icon as={DownloadIcon} />}
            onClick={() => handleSelect("pdf")}
          >
            导出为 PDF
          </MenuItem>
          <MenuItem
            icon={<Icon as={DownloadIcon} />}
            onClick={() => handleSelect("docx")}
          >
            导出为 DOCX
          </MenuItem>
        </MenuList>
      </Portal>
    </Menu>
  );
}

export default ExportButton;
