import React, { useEffect, useState } from "react";
import {
  FormControl,
  Input,
  InputGroup,
  FormLabel,
  InputRightElement,
  InputLeftElement,
  IconButton,
  Icon,
} from "@chakra-ui/react";
import { FiSearch } from "react-icons/fi";
import { CloseIcon } from "@chakra-ui/icons";
import { resolveResponsiveControlLayout } from "@/components/layout/responsive-layout.mjs";

export type InputType = {
  name: string;
  label?: string;
  icon?: boolean;
  placeholder?: string;
  props?: any;
  // eslint-disable-next-line no-unused-vars
  onChange: (value: any) => void;
  isClear: boolean;
};

export default function InputCom({
  name,
  label,
  // handleSearch,
  placeholder = "请输入",
  props,
  icon,
  onChange,
  isClear,
}: InputType) {
  const [isInvalid, setIsInvalid] = useState(false);
  const [value, setValue] = useState<any>("");
  const controlH = props?.h || "11";
  const responsiveLayout = resolveResponsiveControlLayout(
    props,
    props?.inputw || "17.5rem",
  );
  // 定义防抖函数
  // const debounce = (func, delay) => {
  //   let timeoutId;
  //   return (...args) => {
  //     clearTimeout(timeoutId);
  //     timeoutId = setTimeout(() => func(...args), delay);
  //   };
  // };
  // const debouncedSearch = debounce(handleSearch, 1000);
  useEffect(() => {
    if (isClear) {
      setValue("");
    }
  }, [isClear]);

  const handleClear = () => {
    setValue("");
    onChange({ [name]: "" });
    setIsInvalid(false);
  };

  return (
    <FormControl
      {...props}
      w={responsiveLayout.w}
      ml={responsiveLayout.ml}
      mr={responsiveLayout.mr}
      display="flex"
      flexDirection={{ base: "column", sm: "row" }}
      alignItems={{ base: "stretch", sm: "center" }}
      gap={{ base: 1, sm: 0 }}
    >
      {label && (
        <FormLabel
          m="0"
          pr={{ base: 0, sm: 2 }}
          fontSize="md"
          fontWeight="normal"
          minW={{ base: 0, sm: "80px" }}
          textAlign={{ base: "left", sm: "right" }}
        >
          {`${label}:`}
        </FormLabel>
      )}
      <InputGroup
        maxW={{ base: "full", md: "20rem" }}
        w={responsiveLayout.w}
        onKeyDown={(e) => {
          const value = (e.target as HTMLInputElement).value;
          if (e.key === "Enter") {
            if (value.trim().length <= 0) {
              setIsInvalid(true);
            } else {
              setIsInvalid(false);
              // handleSearch(value);
            }
          }
        }}
        onBlur={() => {
          setIsInvalid(false);
        }}
      >
        {icon && (
          <InputLeftElement h={controlH}>
            <Icon as={FiSearch} boxSize="5" color="workbench.muted" />
          </InputLeftElement>
        )}
        <Input
          border="1px solid"
          borderColor="workbench.line"
          color="workbench.text"
          fontSize="sm"
          borderRadius={props?.borderRadius || "10px"}
          pl={icon ? "9" : "3"}
          name={name}
          placeholder={placeholder}
          bg="workbench.paper"
          _hover={{ borderColor: "neutral.300" }}
          _focus={{
            borderColor: isInvalid ? "error.400" : "primary.400",
            boxShadow: "0 0 0 3px rgba(30, 58, 95, 0.1)",
          }}
          _placeholder={{ color: "workbench.muted" }}
          h={controlH}
          value={value}
          onChange={(e) => {
            const value = (e.target as HTMLInputElement).value;
            if (value.trim().length > 0) {
              setIsInvalid(false);
            }
            onChange({ [name]: value });
            setValue(value);
          }}
        />
        {value && (
          <InputRightElement h={controlH}>
            <Icon
              as={CloseIcon}
              w={3}
              h={3}
              color="gray.400"
              cursor="pointer"
              onClick={handleClear}
              _hover={{ color: "gray.600" }}
            />
          </InputRightElement>
        )}
      </InputGroup>
    </FormControl>
  );
}
