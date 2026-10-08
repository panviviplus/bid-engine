"use client";

import { useState } from "react";
import {
  FormControl,
  FormLabel,
  Input,
  InputGroup,
  InputRightElement,
  Select,
  IconButton,
  Accordion,
  AccordionButton,
  AccordionIcon,
  AccordionItem,
  AccordionPanel,
  Box,
  SimpleGrid,
  Text,
} from "@chakra-ui/react";
import { ViewIcon, ViewOffIcon } from "@chakra-ui/icons";

const ENDPOINT_OPTIONS = [
  { value: "/chat/completions", label: "Chat Completions" },
  { value: "/responses", label: "Responses API" },
];

const ALL_FIELDS = [
  "base_url",
  "api_key",
  "model",
  "endpoint_path",
  "context_window_tokens",
  "max_output_tokens",
];

export default function ModelConfigForm({
  values,
  onChange,
  showLabels = true,
  required = false,
}) {
  const [showKey, setShowKey] = useState(false);

  // required: true → all fields required; false/undefined → none; string[] → only listed fields
  const requiredFields = Array.isArray(required)
    ? required
    : required === true
      ? ALL_FIELDS
      : [];

  const isRequired = (field) => requiredFields.includes(field);

  const handleChange = (field) => (e) => {
    const value =
      field === "context_window_tokens" || field === "max_output_tokens"
        ? Number(e.target.value)
        : e.target.value;
    onChange({ ...values, [field]: value });
  };

  return (
    <>
      {showLabels && (
        <FormControl isRequired={isRequired("base_url")}>
          <FormLabel fontSize="sm" color="neutral.600">
            Base URL
          </FormLabel>
          <Input
            size="md"
            placeholder="https://api.deepseek.com"
            value={values.base_url || ""}
            onChange={handleChange("base_url")}
            borderRadius="lg"
          />
        </FormControl>
      )}

      <FormControl isRequired={isRequired("api_key")}>
        {showLabels && (
          <FormLabel fontSize="sm" color="neutral.600">
            API Key
          </FormLabel>
        )}
        <InputGroup size="md">
          <Input
            type={showKey ? "text" : "password"}
            placeholder="sk-xxxxxxxx"
            value={values.api_key || ""}
            onChange={handleChange("api_key")}
            borderRadius="lg"
          />
          <InputRightElement>
            <IconButton
              aria-label="toggle-key"
              icon={showKey ? <ViewOffIcon /> : <ViewIcon />}
              variant="ghost"
              size="sm"
              onClick={() => setShowKey(!showKey)}
            />
          </InputRightElement>
        </InputGroup>
      </FormControl>

      <FormControl isRequired={isRequired("model")}>
        {showLabels && (
          <FormLabel fontSize="sm" color="neutral.600">
            模型 ID
          </FormLabel>
        )}
        <Input
          size="md"
          placeholder="deepseek-v4-flash"
          value={values.model || ""}
          onChange={handleChange("model")}
          borderRadius="lg"
        />
      </FormControl>

      <FormControl isRequired={isRequired("endpoint_path")}>
        {showLabels && (
          <FormLabel fontSize="sm" color="neutral.600">
            响应类型
          </FormLabel>
        )}
        <Select
          size="md"
          value={values.endpoint_path || "/chat/completions"}
          onChange={handleChange("endpoint_path")}
        >
          {ENDPOINT_OPTIONS.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </Select>
      </FormControl>

      <Accordion allowToggle w="full">
        <AccordionItem
          border="1px solid"
          borderColor="neutral.100"
          borderRadius="lg"
          overflow="hidden"
        >
          <AccordionButton
            minH="44px"
            _expanded={{ bg: "primary.50", color: "primary.700" }}
          >
            <Box
              as="span"
              flex="1"
              textAlign="left"
              fontSize="sm"
              fontWeight="600"
            >
              高级上下文预算
            </Box>
            <AccordionIcon />
          </AccordionButton>
          <AccordionPanel pb={4} bg="neutral.50">
            <SimpleGrid columns={{ base: 1, md: 2 }} spacing={3}>
              <FormControl isRequired={isRequired("context_window_tokens")}>
                <FormLabel fontSize="xs" color="neutral.600">
                  上下文窗口 Tokens
                </FormLabel>
                <Input
                  type="number"
                  min={8192}
                  step={1024}
                  value={values.context_window_tokens || 32768}
                  onChange={handleChange("context_window_tokens")}
                  bg="white"
                />
              </FormControl>
              <FormControl isRequired={isRequired("max_output_tokens")}>
                <FormLabel fontSize="xs" color="neutral.600">
                  最大输出 Tokens
                </FormLabel>
                <Input
                  type="number"
                  min={512}
                  step={512}
                  value={values.max_output_tokens || 8192}
                  onChange={handleChange("max_output_tokens")}
                  bg="white"
                />
              </FormControl>
            </SimpleGrid>
            <Text mt={2} fontSize="xs" color="neutral.600">
              系统会为 Prompt、Schema 和安全余量预留至少 4096
              tokens，再按章节打包原文。
            </Text>
          </AccordionPanel>
        </AccordionItem>
      </Accordion>
    </>
  );
}
