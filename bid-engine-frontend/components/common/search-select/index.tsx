import React, { useState, useCallback, useRef, useEffect } from "react";
import {
  Input,
  Popover,
  PopoverTrigger,
  PopoverContent,
  PopoverBody,
  VStack,
  Box,
  Text,
  Spinner,
  useDisclosure,
} from "@chakra-ui/react";

export default function SearchSelect({
  showName,
  onChange,
  fetchApi,
  isDisabled,
  defaultValue,
}: {
  showName: string;
  // eslint-disable-next-line no-unused-vars
  onChange: (v: any) => void;
  // eslint-disable-next-line no-unused-vars
  fetchApi: (value: any, page: number) => { list: any[]; hasMore: boolean };
  isDisabled?: boolean;
  defaultValue?: string;
}) {
  const [kw, setKw] = useState("");
  const [list, setList] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(true);
  const { isOpen, onOpen, onClose } = useDisclosure();
  const scrollRef = useRef<HTMLDivElement>(null);
  const timerRef = useRef<number>();

  useEffect(() => {
    if (defaultValue) {
      setKw(defaultValue);
    }
  }, [defaultValue]);

  const loadPage = useCallback(
    async (key: string, p: number, reset = false) => {
      if (loading) return;
      setLoading(true);
      const { list: newList, hasMore: more } = await fetchApi(key, p);
      setList((prev) => (reset ? newList : [...prev, ...newList]));
      setHasMore(more);
      setPage(p);
      setLoading(false);
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [loading],
  );

  const handleSearch = useCallback(
    (val: string) => {
      setKw(val);
      if (timerRef.current) clearTimeout(timerRef.current);
      if (!val) {
        setList([]);
        onClose();
        return;
      }
      timerRef.current = window.setTimeout(() => {
        loadPage(val, 1, true);
        onOpen();
      }, 800);
    },
    [loadPage, onOpen, onClose],
  );

  const handleScroll = useCallback(() => {
    const el = scrollRef.current;
    if (
      !el ||
      loading ||
      !hasMore ||
      el.scrollTop + el.clientHeight < el.scrollHeight - 10
    )
      return;
    loadPage(kw, page + 1);
  }, [loading, hasMore, kw, page, loadPage]);

  const handleSelect = (item: any) => {
    setKw(item[showName]);
    onChange(item);
    onClose();
  };

  return (
    <Box minH="8" position="relative">
      <Popover isOpen={isOpen} onClose={onClose} placement="bottom-start">
        <PopoverTrigger>
          <Box w="full" data-focus-lock-passthrough>
            <Input
              isDisabled={isDisabled}
              borderRadius="2px"
              border="1px solid #c2c2c2"
              h={8}
              size="md"
              pl={2}
              fontSize="sm"
              _placeholder={{ color: "#c8c9cc" }}
              placeholder="请输入"
              value={kw}
              onChange={(e) => handleSearch(e.target.value)}
            />
          </Box>
        </PopoverTrigger>

        <PopoverContent
          minW={280}
          maxH={300}
          overflowY="auto"
          ref={scrollRef}
          onScroll={handleScroll}
        >
          <PopoverBody p={2}>
            {!list.length && !loading && (
              <Text fontSize="sm" color="neutral.500" textAlign="center">
                暂无数据
              </Text>
            )}
            <VStack align="stretch" spacing={1}>
              {list.map((item) => (
                <Box
                  key={item.id}
                  px={1}
                  py={1}
                  cursor="pointer"
                  _hover={{ bg: "neutral.100" }}
                  onClick={() => handleSelect(item)}
                  fontSize="sm"
                >
                  {item[showName]}
                </Box>
              ))}
              {loading && (
                <Spinner
                  thickness="3px"
                  speed="0.65s"
                  emptyColor="neutral.200"
                  color="brand.500"
                  size="sm"
                  alignSelf="center"
                  my={2}
                />
              )}
              {!hasMore && !!list.length && (
                <Text
                  fontSize="sm"
                  color="neutral.400"
                  textAlign="center"
                  py={2}
                >
                  没有更多了
                </Text>
              )}
            </VStack>
          </PopoverBody>
        </PopoverContent>
      </Popover>
    </Box>
  );
}
