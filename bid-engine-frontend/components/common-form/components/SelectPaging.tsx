import { useEffect, useRef, useState } from "react";
import type { UIEvent } from "react";
import {
  Flex,
  Text,
  Button,
  Menu,
  MenuList,
  MenuItem,
  MenuButton,
  Spinner,
} from "@chakra-ui/react";
import { ChevronDownIcon } from "@chakra-ui/icons";
import useAxios from "axios-hooks";
import { get } from "lodash";
import { resolveResponsiveControlLayout } from "@/components/layout/responsive-layout.mjs";

type OptionItem = { label: string; value: number | string };

export type PaginateConfig = {
  url: string;
  method?: "GET" | "POST";
  params?: Record<string, any>;
  data?: Record<string, any>;
  pageParamKey?: string;
  pageSizeParamKey?: string;
  pageSize?: number;
  labelKey?: string;
  valueKey?: string;
  fieldKey?: string;
  responseListPath?: string;
  responseTotalPath?: string;
};

type SelectPagingProps = {
  name: string;
  label?: string;
  placeholder?: string;
  defaultValue?: OptionItem;
  props?: any;
  onChange: any;
  isClear: boolean;
  paginate: PaginateConfig;
};

export default function SelectPaging({
  name,
  label = "",
  placeholder,
  defaultValue,
  props,
  onChange,
  isClear,
  paginate,
}: SelectPagingProps) {
  const responsiveLayout = resolveResponsiveControlLayout(props, "11.5rem");
  const [curItem, setCurItem] = useState<OptionItem | undefined>(defaultValue);
  const [items, setItems] = useState<OptionItem[]>([]);
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(true);
  const [loading, setLoading] = useState(false);
  const listRef = useRef<HTMLDivElement | null>(null);

  const [, fetchPage] = useAxios(
    {
      url: paginate.url,
      method: paginate.method || "GET",
    },
    { manual: true, useCache: false },
  );

  const pageParamKey = paginate.pageParamKey || "page";
  const pageSizeParamKey = paginate.pageSizeParamKey || "page_size";
  const pageSize = paginate.pageSize || 10;
  const labelKey = paginate.labelKey || paginate.fieldKey || "label";
  const valueKey = paginate.valueKey || paginate.fieldKey || "value";
  const listPath = paginate.responseListPath || "data.data.list";
  const totalPath = paginate.responseTotalPath || "data.data.total";

  const getColor = (defaultColor: string, color: string) => {
    let c = defaultColor;
    if (!curItem?.value && curItem?.value !== 0) {
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
    if (!curItem?.value && curItem?.value !== 0) {
      c = color;
    } else if (curItem?.value === value) {
      c = defaultColor;
    } else {
      c = color;
    }
    return c;
  };

  const mapToOptions = (list: any[]): OptionItem[] => {
    return list.map((it) => ({
      label: it[labelKey],
      value: it[valueKey],
    }));
  };

  const loadPage = async (p: number) => {
    if (loading) return;
    setLoading(true);
    const req: any = {};
    if ((paginate.method || "GET") === "GET") {
      req.params = {
        ...(paginate.params || {}),
        [pageParamKey]: p,
        [pageSizeParamKey]: pageSize,
      };
    } else {
      req.data = {
        ...(paginate.data || paginate.params || {}),
        [pageParamKey]: p,
        [pageSizeParamKey]: pageSize,
      };
    }
    const res = await fetchPage(req);
    const rawList = get(res, listPath) || get(res, "data.data") || [];
    const total = get(res, totalPath);
    const mapped = mapToOptions(rawList);
    const next = p === 1 ? mapped : [...items, ...mapped];
    setItems(next);
    if (typeof total === "number") {
      const start = (p - 1) * pageSize;
      setHasMore(start + pageSize < total);
    } else {
      setHasMore(mapped.length === pageSize);
    }
    setPage(p);
    setLoading(false);
  };

  useEffect(() => {
    if (isClear) {
      setItems([]);
      setPage(1);
      setHasMore(true);
      setCurItem(defaultValue);
    }
  }, [isClear, defaultValue]);

  const handleClick = (item: OptionItem) => {
    setCurItem(item);
    onChange?.({ [name]: item.value });
  };

  const handleMenuButtonClick = () => {
    if (items.length === 0) {
      loadPage(1);
    }
  };

  const handleScroll = (e: UIEvent<HTMLDivElement>) => {
    const target = e.currentTarget;
    const nearBottom =
      target.scrollTop + target.clientHeight >= target.scrollHeight - 20;
    if (nearBottom && hasMore && !loading) {
      loadPage(page + 1);
    }
  };

  const getLabel = () => {
    if (curItem?.label) {
      return curItem.label;
    }
    if (placeholder) return placeholder;
    if (label) return `请选择${label}`;
    return "请选择";
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
      <Menu autoSelect={false} closeOnSelect>
        <MenuButton
          px={3.5}
          bg="workbench.paper"
          border="1px solid"
          borderColor="workbench.line"
          minH={props?.h || "11"}
          borderRadius={props?.borderRadius || "10px"}
          w={responsiveLayout.w}
          h={props?.h || "11"}
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
          rightIcon={<ChevronDownIcon boxSize={3.5} color="workbench.muted" />}
          onClick={handleMenuButtonClick}
          sx={{
            span: {
              pointerEvents: "unset",
            },
          }}
        >
          {getLabel()}
        </MenuButton>
        <MenuList
          ref={listRef as any}
          maxH="240px"
          overflowY="auto"
          zIndex={999}
          onScroll={handleScroll}
        >
          {items.map((item, index) => (
            <MenuItem
              bg={getbgColor(item.value, "neutral.100", "")}
              onClick={() => handleClick(item)}
              key={index}
              color="workbench.text"
            >
              {item.label}
            </MenuItem>
          ))}
          {loading && (
            <Flex align="center" justify="center" py="2">
              <Spinner size="sm" />
            </Flex>
          )}
        </MenuList>
      </Menu>
    </Flex>
  );
}
