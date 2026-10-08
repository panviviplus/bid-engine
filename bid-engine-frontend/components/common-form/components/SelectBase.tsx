import { useState, useEffect } from "react";
import {
  Flex,
  Text,
  Button,
  Menu,
  MenuList,
  MenuItem,
  MenuButton,
  MenuItemOption,
  MenuOptionGroup,
  Icon,
} from "@chakra-ui/react";
import { ChevronDownIcon, CloseIcon } from "@chakra-ui/icons";
import { cloneDeep } from "lodash";
import { resolveResponsiveControlLayout } from "@/components/layout/responsive-layout.mjs";

type OptionItem = { label: string; value: number | string };
type OptionsType = OptionItem[];
export type SelectBaseProps = {
  options: OptionsType;
  name: string;
  label?: string;
  placeholder?: string;
  defaultValue?: OptionItem | OptionItem[];
  type?: "single" | "select_multi";
  props?: any;
  onChange: any;
  isClear: boolean;
};

let list: OptionsType = [];
export default function SelectBase({
  name,
  options,
  label = "",
  placeholder,
  defaultValue,
  type,
  props,
  onChange,
  isClear,
}: SelectBaseProps) {
  const responsiveLayout = resolveResponsiveControlLayout(props, "11.5rem");
  const [curItem, setCurItem] = useState<OptionItem | OptionItem[] | undefined>(
    defaultValue,
  );
  const [curMultiItem, setCurMultiItem] = useState<string[]>(
    Array.isArray(curItem) ? curItem?.map((item) => `${item.value}`) : [],
  );
  useEffect(() => {
    if (isClear) {
      list = [];
      setCurItem(defaultValue);
      setCurMultiItem([]);
    }
  }, [isClear, defaultValue]);
  useEffect(() => {
    if (defaultValue && Array.isArray(defaultValue)) {
      list = [...defaultValue];
    }
  }, [defaultValue]);

  const updateCurItem = (cur: OptionItem) => {
    const tempList = cloneDeep(list);
    for (let i = 0; i < list.length; i += 1) {
      if (list[i].value === cur.value) {
        tempList.splice(i, 1);
        list.splice(i, 1);
        setCurItem(tempList);
        setCurMultiItem(tempList.map((item) => `${item.value}`));
        onChange?.({ [name]: tempList.map((item) => item.value) });
        return;
      }
    }
    list.push(cur);
    tempList.push(cur);
    onChange?.({ [name]: tempList.map((item) => item.value) });
    setCurItem(tempList);
  };
  const handleClick = (item: OptionItem) => {
    if (type === "select_multi") {
      updateCurItem(item);
      return;
    }
    onChange?.({ [name]: item.value });
    setCurItem(item);
  };
  const handleClear = (e: React.MouseEvent) => {
    e.stopPropagation();
    onChange?.({ [name]: undefined });
    setCurItem(undefined);
  };

  const getColor = (defaultColor: string, color: string) => {
    let c = defaultColor;
    if (Array.isArray(curItem) && !curItem.length) {
      c = color;
    } else if (
      !Array.isArray(curItem) &&
      (curItem?.value === undefined || curItem?.value === null)
    ) {
      c = color;
    }
    return c;
  };
  const getbgColor = (
    value: number | string,
    defaultColor: string,
    color: string,
  ) => {
    let c = defaultColor;
    if (Array.isArray(curItem)) {
      if (!curItem.length) {
        c = color;
      } else if (curMultiItem.indexOf(`${value}`) > -1) {
        c = defaultColor;
      } else {
        c = color;
      }
    } else if (!Array.isArray(curItem)) {
      if (!curItem?.value) {
        c = color;
      } else if ((curItem as OptionItem).value === value) {
        c = defaultColor;
      } else {
        c = color;
      }
    }
    return c;
  };

  const getLabel = () => {
    if (Array.isArray(curItem) && curItem.length) {
      return (
        <Flex wrap="wrap">
          {curItem.map((item, index) => (
            <Flex
              key={index}
              alignItems="center"
              gap={1}
              px={2}
              bg="workbench.canvas"
              border="1px solid"
              borderColor="workbench.line"
              borderRadius="full"
              minH="6"
              mr={1.5}
              mb={1}
              position="relative"
              zIndex={999}
            >
              <Text fontSize="xs" color="workbench.text">
                {item.label}
              </Text>
              <Icon
                zIndex={999}
                as={CloseIcon}
                boxSize={2.5}
                color="workbench.muted"
                cursor="pointer"
                _hover={{ color: "error.600" }}
                onClick={() => updateCurItem(item)}
              />
            </Flex>
          ))}
        </Flex>
      );
    }
    if (!Array.isArray(curItem) && (curItem as OptionItem)?.label) {
      return (curItem as OptionItem).label;
    }
    if (placeholder) return placeholder;
    if (label) return `请选择${label}`;
    return "请选择";
  };

  const handleMultiChange = (value: (string | number)[]) => {
    setCurMultiItem(value.map((v) => `${v}`));
  };

  return (
    <Flex
      {...props}
      w={responsiveLayout.w}
      ml={responsiveLayout.ml}
      mr={responsiveLayout.mr}
      direction={{ base: "column", sm: "row" }}
      align={{ base: "stretch", sm: "center" }}
      gap={{ base: 1, sm: 0 }}
    >
      {label && (
        <Text
          pr={{ base: 0, sm: 2 }}
          fontSize="sm"
          fontWeight="600"
          color="workbench.muted"
          minW={{ base: 0, sm: "80px" }}
          textAlign={{ base: "left", sm: "right" }}
        >
          {label}:
        </Text>
      )}
      <Menu autoSelect={false} closeOnSelect={type !== "select_multi"}>
        <MenuButton
          px={3.5}
          bg="workbench.paper"
          border="1px solid"
          borderColor="workbench.line"
          minH={props?.h || "11"}
          borderRadius={props?.borderRadius || "10px"}
          w={responsiveLayout.w}
          h={curMultiItem.length > 2 ? "auto" : props?.h || "11"}
          textAlign="left"
          as={Button}
          fontWeight="500"
          fontSize="sm"
          color={getColor("workbench.text", "workbench.muted")}
          _hover={{ borderColor: "neutral.300" }}
          _focusVisible={{
            borderColor: "primary.400",
            boxShadow: "0 0 0 3px rgba(30, 58, 95, 0.1)",
            outline: "none",
          }}
          rightIcon={
            <Flex align="center">
              {!Array.isArray(curItem) && curItem?.value && (
                <Icon
                  as={CloseIcon}
                  w={3}
                  h={3}
                  color="workbench.muted"
                  mr={1.5}
                  onClick={handleClear}
                  _hover={{ color: "neutral.600" }}
                />
              )}
              <ChevronDownIcon boxSize={3.5} color="workbench.muted" />
            </Flex>
          }
          sx={{
            span: {
              pointerEvents: "unset",
            },
          }}
        >
          {getLabel()}
        </MenuButton>
        <MenuList zIndex={99} maxH="22rem" overflowY="auto">
          {type === "select_multi" ? (
            <MenuOptionGroup
              type="checkbox"
              onChange={handleMultiChange as any}
              value={curMultiItem}
            >
              {options.map((item, index) => (
                <MenuItemOption
                  onClick={() => handleClick(item)}
                  key={index}
                  value={`${item.value}`}
                  icon={<span />}
                  bg={getbgColor(item.value, "neutral.100", "")}
                >
                  {item.label}
                </MenuItemOption>
              ))}
            </MenuOptionGroup>
          ) : (
            options.map((item, index) => (
              <MenuItem
                bg={getbgColor(item.value, "neutral.100", "")}
                onClick={() => handleClick(item)}
                key={index}
                color="workbench.text"
              >
                {item.label}
              </MenuItem>
            ))
          )}
        </MenuList>
      </Menu>
    </Flex>
  );
}
