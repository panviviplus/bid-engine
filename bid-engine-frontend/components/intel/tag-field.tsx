"use client";

/* Hallmark · component: intel-tag-field · genre: modern-minimal · theme: BidEngine deep-sea/gold (preserved tokens)
 * states: default · hover · focus-visible · active · disabled · loading(不适用) · empty · error
 * 触控目标 ≥44px；窄屏单列；行业名称无法全量枚举，因此支持自由输入 + 建议项。
 */
import React, { useState } from "react";
import {
  Button,
  Flex,
  FormControl,
  FormErrorMessage,
  FormHelperText,
  FormLabel,
  IconButton,
  Input,
  InputGroup,
  InputRightElement,
  Text,
  Wrap,
  WrapItem,
} from "@chakra-ui/react";
import { FiPlus, FiX } from "react-icons/fi";

const FIELD_H = "44px";

const FOCUS_PROPS = {
  _focusVisible: {
    borderColor: "primary.500",
    boxShadow: "0 0 0 2px var(--chakra-colors-primary-200)",
    outline: "none",
  },
};

/** 已选标签（可移除）。 */
function Chip({
  label,
  onRemove,
  tone = "selected",
}: {
  label: string;
  onRemove: () => void;
  tone?: "selected" | "suggestion";
}) {
  if (tone === "suggestion") {
    return null;
  }
  return (
    <Flex
      align="center"
      gap={1}
      pl={3}
      pr={1}
      minH="32px"
      borderRadius="full"
      bg="primary.500"
      color="white"
      fontSize="sm"
      maxW="100%"
    >
      <Text as="span" noOfLines={1} overflowWrap="anywhere">
        {label}
      </Text>
      <IconButton
        aria-label={`移除 ${label}`}
        icon={<FiX aria-hidden />}
        size="xs"
        minW="24px"
        h="24px"
        variant="ghost"
        color="white"
        borderRadius="full"
        _hover={{ bg: "whiteAlpha.300" }}
        _focusVisible={{
          boxShadow: "0 0 0 2px var(--chakra-colors-gold-300)",
          outline: "none",
        }}
        onClick={onRemove}
      />
    </Flex>
  );
}

/**
 * 枚举多选（公告类型等固定取值）。
 */
export function OptionTagField({
  label,
  helperText,
  options,
  value,
  onChange,
}: {
  label: string;
  helperText?: string;
  options: Array<{ code: string; name: string }>;
  value: string[];
  // eslint-disable-next-line no-unused-vars
  onChange: (next: string[]) => void;
}) {
  const toggle = (code: string) =>
    onChange(
      value.includes(code)
        ? value.filter((item) => item !== code)
        : [...value, code],
    );

  return (
    <FormControl>
      <FormLabel fontSize="sm" fontWeight="600">
        {label}
      </FormLabel>
      <Wrap spacing={2}>
        {options.map((item) => {
          const active = value.includes(item.code);
          return (
            <WrapItem key={item.code}>
              <Button
                type="button"
                size="sm"
                minH="32px"
                px={3}
                borderRadius="full"
                variant={active ? "solid" : "outline"}
                colorScheme={active ? "primary" : undefined}
                borderColor="neutral.200"
                color={active ? "white" : "neutral.600"}
                fontWeight={active ? "600" : "400"}
                aria-pressed={active}
                _hover={{
                  bg: active ? "primary.600" : "primary.50",
                  borderColor: active ? "primary.600" : "primary.300",
                  color: active ? "white" : "primary.700",
                }}
                _active={{ transform: "scale(0.97)" }}
                _focusVisible={{
                  boxShadow: "0 0 0 2px var(--chakra-colors-primary-300)",
                  outline: "none",
                }}
                onClick={() => toggle(item.code)}
              >
                {item.name}
              </Button>
            </WrapItem>
          );
        })}
      </Wrap>
      {helperText && (
        <FormHelperText fontSize="xs">{helperText}</FormHelperText>
      )}
    </FormControl>
  );
}

/**
 * 可自由输入 + 建议项的多选字段（行业、地区）。
 *
 * 行业名称无法全量枚举：用户既能点建议项，也能直接输入自定义文本后回车/点按钮添加。
 */
export function SuggestTagField({
  label,
  helperText,
  placeholder,
  value,
  onChange,
  suggestions,
  suggestionLimit = 12,
}: {
  label: string;
  helperText?: string;
  placeholder?: string;
  value: string[];
  // eslint-disable-next-line no-unused-vars
  onChange: (next: string[]) => void;
  suggestions: string[];
  suggestionLimit?: number;
}) {
  const [draft, setDraft] = useState("");
  const [error, setError] = useState("");

  const add = (raw: string) => {
    const next = raw.trim();
    if (!next) return;
    if (next.length > 32) {
      setError("最多 32 个字符");
      return;
    }
    if (value.some((item) => item.toLowerCase() === next.toLowerCase())) {
      setError("该条件已添加");
      return;
    }
    setError("");
    onChange([...value, next]);
    setDraft("");
  };

  const remove = (item: string) =>
    onChange(value.filter((value0) => value0 !== item));

  const remaining = suggestions
    .filter(
      (item) =>
        !value.some(
          (selected) => selected.toLowerCase() === item.toLowerCase(),
        ),
    )
    .slice(0, suggestionLimit);

  return (
    <FormControl isInvalid={Boolean(error)}>
      <FormLabel fontSize="sm" fontWeight="600">
        {label}
      </FormLabel>

      {value.length > 0 && (
        <Wrap spacing={2} mb={2}>
          {value.map((item) => (
            <WrapItem key={item} maxW="100%">
              <Chip label={item} onRemove={() => remove(item)} />
            </WrapItem>
          ))}
        </Wrap>
      )}

      <InputGroup>
        <Input
          h={FIELD_H}
          pr="4.5rem"
          value={draft}
          placeholder={placeholder}
          onChange={(event) => {
            setDraft(event.target.value);
            if (error) setError("");
          }}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              event.preventDefault();
              add(draft);
            }
          }}
          borderColor="neutral.200"
          {...FOCUS_PROPS}
        />
        <InputRightElement h={FIELD_H} w="4.5rem">
          <Button
            h="32px"
            size="sm"
            variant="ghost"
            leftIcon={<FiPlus aria-hidden />}
            color="primary.600"
            isDisabled={!draft.trim()}
            _hover={{ bg: "primary.50" }}
            onClick={() => add(draft)}
          >
            添加
          </Button>
        </InputRightElement>
      </InputGroup>

      {error ? (
        <FormErrorMessage fontSize="xs">{error}</FormErrorMessage>
      ) : (
        helperText && (
          <FormHelperText fontSize="xs">{helperText}</FormHelperText>
        )
      )}

      {remaining.length > 0 && (
        <Flex mt={2} gap={2} wrap="wrap" align="center">
          <Text fontSize="xs" color="workbench.muted">
            常用：
          </Text>
          {remaining.map((item) => (
            <Button
              key={item}
              type="button"
              size="xs"
              minH="28px"
              px={2.5}
              borderRadius="full"
              variant="outline"
              borderColor="neutral.200"
              color="neutral.600"
              fontWeight="400"
              _hover={{
                bg: "primary.50",
                borderColor: "primary.300",
                color: "primary.700",
              }}
              _active={{ transform: "scale(0.97)" }}
              _focusVisible={{
                boxShadow: "0 0 0 2px var(--chakra-colors-primary-300)",
                outline: "none",
              }}
              onClick={() => add(item)}
            >
              {item}
            </Button>
          ))}
        </Flex>
      )}
    </FormControl>
  );
}
