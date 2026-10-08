"use client";

/* Hallmark · component: intel-select · genre: modern-minimal · theme: BidEngine deep-sea/gold (preserved tokens)
 * 为什么不用原生 Select：操作系统渲染的 options 列表无法继承设计令牌，
 * 在深色/紧凑的筛选栏里视觉割裂。这里用 Chakra Menu 自绘下拉面板。
 * states: default · hover · focus-visible · active(展开) · selected · disabled · empty
 * a11y: role=menuitemradio + aria-checked · Esc 关闭 · 44px 触控目标
 */
import React, { useEffect, useRef, useState } from "react";
import {
  Box,
  Button,
  HStack,
  Icon,
  IconButton,
  Menu,
  MenuButton,
  MenuItem,
  MenuList,
  Portal,
  Text,
} from "@chakra-ui/react";
import { FiCheck, FiChevronDown, FiX } from "react-icons/fi";

export type IntelSelectOption = {
  value: string;
  label: string;
  /** 可选的次要说明（例如来源的站点性质），仅在选项内展示 */
  hint?: string;
};

export const FIELD_HEIGHT = "44px";

export default function IntelSelect({
  value,
  options,
  onChange,
  placeholder = "全部",
  ariaLabel,
  isDisabled = false,
  isClearable = false,
  emptyText = "暂无可选项",
  maxW,
}: {
  value: string;
  options: IntelSelectOption[];
  // eslint-disable-next-line no-unused-vars
  onChange: (next: string) => void;
  placeholder?: string;
  ariaLabel?: string;
  isDisabled?: boolean;
  isClearable?: boolean;
  emptyText?: string;
  maxW?: React.ComponentProps<typeof Box>["maxW"];
}) {
  const [isOpen, setIsOpen] = useState(false);
  const [menuWidth, setMenuWidth] = useState(0);
  const triggerRef = useRef<HTMLButtonElement>(null);

  // 展开时把面板宽度对齐触发器：下拉面板与输入框等宽，视觉上是一体的
  useEffect(() => {
    if (!isOpen || !triggerRef.current) return;
    setMenuWidth(triggerRef.current.offsetWidth);
  }, [isOpen]);

  const selected = options.find((item) => item.value === value);
  const hasValue = Boolean(selected);

  return (
    <Menu
      placement="bottom-start"
      isLazy
      closeOnSelect
      isOpen={isOpen}
      onOpen={() => setIsOpen(true)}
      onClose={() => setIsOpen(false)}
    >
      <Box position="relative" w="100%" maxW={maxW}>
        <MenuButton
          ref={triggerRef}
          as={Button}
          aria-label={ariaLabel}
          isDisabled={isDisabled}
          h={FIELD_HEIGHT}
          w="100%"
          pl={3}
          pr={hasValue && isClearable && !isDisabled ? 20 : 10}
          variant="outline"
          bg="white"
          borderWidth="1px"
          borderColor={isOpen ? "primary.400" : "neutral.200"}
          borderRadius="lg"
          fontWeight="500"
          fontSize="sm"
          justifyContent="flex-start"
          color={hasValue ? "workbench.text" : "workbench.muted"}
          _hover={{
            borderColor: "primary.300",
            bg: "primary.50",
          }}
          _active={{ bg: "primary.50" }}
          _focusVisible={{
            borderColor: "primary.500",
            boxShadow: "0 0 0 2px var(--chakra-colors-primary-200)",
            outline: "none",
          }}
          _disabled={{
            bg: "neutral.50",
            color: "neutral.400",
            borderColor: "neutral.100",
            opacity: 1,
            cursor: "not-allowed",
          }}
        >
          <Text
            as="span"
            noOfLines={1}
            textAlign="left"
            display="block"
            w="100%"
            minW={0}
          >
            {selected?.label || placeholder}
          </Text>
        </MenuButton>

        {isClearable && hasValue && !isDisabled ? (
          <IconButton
            aria-label="清空选择"
            icon={<FiX aria-hidden size={13} />}
            position="absolute"
            top={0}
            right={7}
            zIndex={1}
            h={FIELD_HEIGHT}
            minW={FIELD_HEIGHT}
            w={FIELD_HEIGHT}
            variant="ghost"
            color="workbench.muted"
            borderRadius="full"
            _hover={{ color: "primary.600", bg: "primary.100" }}
            _active={{ bg: "primary.100" }}
            _focusVisible={{
              boxShadow: "0 0 0 2px var(--chakra-colors-primary-300)",
              outline: "none",
            }}
            onClick={() => onChange("")}
          />
        ) : null}

        <Icon
          as={FiChevronDown}
          aria-hidden
          pointerEvents="none"
          position="absolute"
          top="50%"
          right={3}
          boxSize={4}
          color="workbench.muted"
          transition="transform .18s cubic-bezier(0.16,1,0.3,1)"
          transform={
            isOpen ? "translateY(-50%) rotate(180deg)" : "translateY(-50%)"
          }
          sx={{
            "@media (prefers-reduced-motion: reduce)": { transition: "none" },
          }}
        />
      </Box>

      <Portal>
        <MenuList
          minW={`${Math.max(menuWidth, 200)}px`}
          maxW={{ base: "92vw", md: "26rem" }}
          maxH="18rem"
          overflowY="auto"
          py={1.5}
          borderRadius="12px"
          border="1px solid"
          borderColor="neutral.100"
          boxShadow="0 16px 40px rgba(11, 27, 43, 0.14)"
          /* Chakra 菜单定位层默认 z-index 是 1（来自主题 Menu.list），在弹窗里会被
           * z-index 1400 的 Modal 遮罩盖住——面板确实渲染了，但用户看不到，表现为
           * “下拉框没有数据”。这里显式抬到 popover(1500) 层，同时兼容弹窗内外的用法。 */
          zIndex="popover"
        >
          {options.length === 0 ? (
            <Box px={3} py={2}>
              <Text fontSize="sm" color="workbench.muted">
                {emptyText}
              </Text>
            </Box>
          ) : (
            options.map((item) => {
              const isSelected = item.value === value;
              return (
                <MenuItem
                  key={item.value}
                  role="menuitemradio"
                  aria-checked={isSelected}
                  minH="40px"
                  px={3}
                  fontSize="sm"
                  color={isSelected ? "primary.700" : "workbench.text"}
                  fontWeight={isSelected ? "650" : "500"}
                  bg={isSelected ? "primary.50" : "transparent"}
                  _hover={{ bg: "primary.50" }}
                  _focus={{ bg: "primary.50" }}
                  onClick={() => onChange(item.value)}
                >
                  <HStack spacing={2} w="100%" justify="space-between">
                    <HStack spacing={2} minW={0}>
                      <Icon
                        as={FiCheck}
                        boxSize={4}
                        color="primary.600"
                        opacity={isSelected ? 1 : 0}
                        aria-hidden
                      />
                      <Text as="span" noOfLines={1}>
                        {item.label}
                      </Text>
                    </HStack>
                    {item.hint ? (
                      <Text
                        as="span"
                        fontSize="xs"
                        color="workbench.muted"
                        noOfLines={1}
                        flexShrink={0}
                      >
                        {item.hint}
                      </Text>
                    ) : null}
                  </HStack>
                </MenuItem>
              );
            })
          )}
        </MenuList>
      </Portal>
    </Menu>
  );
}
