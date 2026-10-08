import React from "react";
import {
  Popover,
  PopoverTrigger,
  PopoverContent,
  PopoverHeader,
  PopoverBody,
  PopoverArrow,
  PopoverCloseButton,
  IconButton,
  Checkbox,
  Stack,
  Text,
  Button,
} from "@chakra-ui/react";
import { SettingsIcon } from "@chakra-ui/icons";

interface ColumnConfigProps {
  allHeaders: { label: string }[];
  visibleLabels: string[];
  // eslint-disable-next-line no-unused-vars
  onChange: (newVisibleLabels: string[]) => void;
  mandatoryLabels?: string[];
  /** 可选：覆盖触发按钮属性（例如在需要 44px 触控目标的页面传 h/w/minW） */
  triggerProps?: Record<string, unknown>;
  /** 可选：传入后展示“恢复默认”按钮 */
  onReset?: () => void;
}

export default function ColumnConfig({
  allHeaders,
  visibleLabels,
  onChange,
  mandatoryLabels = [],
  triggerProps = {},
  onReset,
}: ColumnConfigProps) {
  const handleCheckboxChange = (label: string, isChecked: boolean) => {
    if (isChecked) {
      onChange([...visibleLabels, label]);
    } else {
      onChange(visibleLabels.filter((l) => l !== label));
    }
  };

  return (
    <Popover placement="bottom-end" closeOnBlur={false} closeOnEsc>
      <PopoverTrigger>
        <IconButton
          aria-label="配置列"
          icon={<SettingsIcon />}
          size="xs"
          variant="ghost"
          color="gray.500"
          {...triggerProps}
        />
      </PopoverTrigger>
      <PopoverContent width="200px">
        <PopoverArrow />
        <PopoverHeader fontWeight="bold" fontSize="sm">
          表格列配置
        </PopoverHeader>
        <PopoverCloseButton />
        <PopoverBody maxH="300px" overflowY="auto">
          <Stack spacing={2}>
            {allHeaders.map((head) => {
              // Skip rendering checkbox for columns that don't have a meaningful label or are "操作" if we handle it specially,
              // but usually we want to control everything except mandatory.
              if (!head.label) return null;
              const isMandatory = mandatoryLabels.includes(head.label);
              return (
                <Checkbox
                  key={head.label}
                  isChecked={visibleLabels.includes(head.label)}
                  isDisabled={isMandatory}
                  onChange={(e) =>
                    handleCheckboxChange(head.label, e.target.checked)
                  }
                  size="sm"
                >
                  <Text fontSize="sm" ml={1}>
                    {head.label}
                  </Text>
                </Checkbox>
              );
            })}
          </Stack>
          {onReset && (
            <Button
              mt={3}
              size="sm"
              variant="outline"
              width="full"
              onClick={onReset}
            >
              恢复默认列
            </Button>
          )}
        </PopoverBody>
      </PopoverContent>
    </Popover>
  );
}
