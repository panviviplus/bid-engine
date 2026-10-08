"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import {
  Flex,
  Center,
  Spinner,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  TableContainer,
  Checkbox,
  Divider,
} from "@chakra-ui/react";
import TableFooter from "./table-footer";
import Empty from "./empty";

export default function TableWithCheckbox({
  wordsLibrary,
  headers,
  total,
  bodyData,
  pages,
  pageSize,
  pagesCount,
  setPageSize,
  currentPage,
  setCurrentPage,
  useCheckBox,
  isLoading,
  isError,
  errorMessage,
  checkedItems,
  setCheckedItems,
  renderHeaderButton,
  onChangeChecked,
  minBodyRows,
  footerProps = {},
}) {
  // 当前页所有文件ID
  const currentPageFileIds = bodyData
    ?.filter((item) => !item.isDisabled)
    ?.map((item) => item.id);
  const getIsChecked = (item) => {
    return checkedItems?.includes(item);
  };
  // 全选
  const allChecked =
    currentPageFileIds?.length > 0 && currentPageFileIds?.every(getIsChecked);
  // 半选
  const isIndeterminate = currentPageFileIds?.some(getIsChecked) && !allChecked;

  // 检查该项目是否已经被勾选
  const isCheckedFile = (id) => {
    return checkedItems?.find((fileId) => fileId === id) !== undefined;
  };
  // 全选按钮禁用
  const isAllDisabled =
    !bodyData ||
    bodyData?.length <= 0 ||
    bodyData?.every((item) => item.isDisabled);
  // 全选
  const handleAllCheck = (e) => {
    if (e.target.checked) {
      setCheckedItems((pre) => {
        return [
          ...pre.filter((item) => !currentPageFileIds.includes(item)),
          ...currentPageFileIds,
        ];
      });
    } else {
      setCheckedItems((pre) => {
        return pre.filter((item) => !currentPageFileIds.includes(item));
      });
    }
    onChangeChecked?.(e.target.checked, "all");
  };
  // 单选
  const handleSingleCheck = (e, curFile, index) => {
    if (e.target.checked) {
      setCheckedItems((pre) => [...pre, curFile.id]);
    } else setCheckedItems((pre) => pre.filter((item) => item !== curFile.id));
    onChangeChecked?.(e.target.checked, curFile.id, index);
  };

  // Sticky Logic
  const CHECKBOX_WIDTH = 50;
  const getWidthVal = (w) => {
    if (typeof w === "number") return w;
    if (typeof w === "string" && w.endsWith("px")) return parseInt(w, 10);
    return 150; // default assumption for calculation if missing
  };

  const { stickyLeftMap, stickyRightMap, hasFixedLeft } = useMemo(() => {
    const leftMap = new Map();
    const rightMap = new Map();

    // Check if any column is fixed left
    const hasFixedLeft = headers?.some(
      (h) => h.fixed === "left" || h.fixed === true,
    );

    let currentLeft = 0;

    if (useCheckBox && hasFixedLeft) {
      currentLeft += CHECKBOX_WIDTH;
    }

    headers?.forEach((head, index) => {
      if (head.fixed === "left" || head.fixed === true) {
        leftMap.set(index, currentLeft);
        currentLeft += getWidthVal(head.width);
      }
    });

    let currentRight = 0;
    for (let i = (headers?.length || 0) - 1; i >= 0; i -= 1) {
      const head = headers[i];
      if (head.fixed === "right") {
        rightMap.set(i, currentRight);
        currentRight += getWidthVal(head.width);
      }
    }

    return { stickyLeftMap: leftMap, stickyRightMap: rightMap, hasFixedLeft };
  }, [headers, useCheckBox]);

  const containerRef = useRef(null);
  const [shadowState, setShadowState] = useState({
    showLeftShadow: false,
    showRightShadow: false,
  });

  const updateShadowState = () => {
    const el = containerRef.current;
    if (!el) return;
    const { scrollLeft, scrollWidth, clientWidth } = el;
    const showLeftShadow = scrollLeft > 0;
    const showRightShadow = scrollLeft + clientWidth < scrollWidth;
    setShadowState((prev) =>
      prev.showLeftShadow === showLeftShadow &&
      prev.showRightShadow === showRightShadow
        ? prev
        : { showLeftShadow, showRightShadow },
    );
  };

  const normalizedTotal = Number(total) || 0;
  const normalizedPageSize = Number(pageSize) || 0;
  const normalizedCurrentPage = Number(currentPage) || 1;
  const computedPagesCount =
    normalizedPageSize > 0
      ? Math.ceil(normalizedTotal / normalizedPageSize)
      : 0;
  const effectivePagesCount = Number(pagesCount) || computedPagesCount || 0;
  const effectivePages = useMemo(() => {
    if (Array.isArray(pages) && pages.length) return pages;
    if (!effectivePagesCount) return [];
    if (effectivePagesCount <= 7) {
      return Array.from({ length: effectivePagesCount }, (_, i) => i + 1);
    }
    const set = new Set([
      1,
      effectivePagesCount,
      normalizedCurrentPage - 1,
      normalizedCurrentPage,
      normalizedCurrentPage + 1,
    ]);
    return Array.from(set)
      .filter((p) => p >= 1 && p <= effectivePagesCount)
      .sort((a, b) => a - b);
  }, [pages, effectivePagesCount, normalizedCurrentPage]);

  useEffect(() => {
    const frameId = window.requestAnimationFrame(() => {
      updateShadowState();
    });
    const handleResize = () => updateShadowState();
    window.addEventListener("resize", handleResize);
    return () => {
      window.removeEventListener("resize", handleResize);
      window.cancelAnimationFrame(frameId);
    };
  }, [headers, bodyData, useCheckBox]);

  const leftShadowStyle = {
    content: '""',
    pos: "absolute",
    top: 0,
    bottom: 0,
    right: "-7px",
    width: "7px",
    bg: "linear-gradient(to right, rgba(0,0,0,0.08) 0%, transparent 100%)",
    pointerEvents: "none",
  };

  const rightShadowStyle = {
    content: '""',
    pos: "absolute",
    top: 0,
    bottom: 0,
    left: "-3px",
    width: "7px",
    bg: "linear-gradient(to left, rgba(0,0,0,0.08) 0%, transparent 100%)",
    pointerEvents: "none",
  };

  return (
    <Flex
      direction="column"
      flex={1}
      w="full"
      h="full"
      bg="white"
      overflow="hidden"
      borderRadius="base"
      minH={0}
    >
      <Divider />
      {renderHeaderButton && <Flex my="4">{renderHeaderButton()}</Flex>}
      <TableContainer
        ref={containerRef}
        flex={1}
        overflowY="auto"
        overflowX="auto"
        minH={0}
        onScroll={updateShadowState}
      >
        <Table variant="simple" sx={{borderCollapse: "separate", borderSpacing: "0"}}>
          <Thead bg="gray.50" h="3.5rem" position="sticky" top={0} zIndex={20}>
            <Tr pos="relative">
              {useCheckBox && (
                <Th
                  pos="sticky"
                  top={0}
                  left={0}
                  zIndex={hasFixedLeft ? 20 : 10}
                  bg="gray.50"
                  borderBottom="1px solid"
                  borderColor="gray.100"
                  w={`${CHECKBOX_WIDTH}px`}
                  minW={`${CHECKBOX_WIDTH}px`}
                  maxW={`${CHECKBOX_WIDTH}px`}
                >
                  <Checkbox
                    colorScheme="primary"
                    isChecked={allChecked}
                    isDisabled={isAllDisabled}
                    isIndeterminate={isIndeterminate}
                    onChange={handleAllCheck}
                  />
                </Th>
              )}

              {headers.map((head, index) => {
                const isLeft = stickyLeftMap.has(index);
                const isRight = stickyRightMap.has(index);
                const isFixed = isLeft || isRight;

                const left = isLeft ? stickyLeftMap.get(index) : undefined;
                const right = isRight ? stickyRightMap.get(index) : undefined;

                return (
                  <Th
                    key={head.label ?? index}
                    fontSize="xs"
                    fontWeight="600"
                    color="gray.500"
                    textTransform="uppercase"
                    letterSpacing="wider"
                    pos="sticky"
                    top={0}
                    left={left}
                    right={right}
                    zIndex={isFixed ? 20 : 10}
                    bg="gray.50"
                    borderBottom="1px solid"
                    borderColor="gray.100"
                    py={4}
                    w={
                      head.headerRender
                        ? `calc(${head.width} + 50px)`
                        : head.width
                    }
                    minW={
                      head.headerRender
                        ? `calc(${head.width} + 50px)`
                        : head.width
                    }
                    maxW={
                      head.headerRender
                        ? `calc(${head.width} + 50px)`
                        : head.width
                    }
                    _after={
                      isLeft && shadowState.showLeftShadow
                        ? leftShadowStyle
                        : undefined
                    }
                    _before={
                      isRight && shadowState.showRightShadow
                        ? rightShadowStyle
                        : undefined
                    }
                  >
                    {head.headerRender ? head.headerRender(head) : head?.label}
                  </Th>
                );
              })}
            </Tr>
          </Thead>
          <Tbody bg="white">
            {!isLoading &&
              !isError &&
              bodyData?.length > 0 &&
              bodyData?.map((item, index) => (
                <Tr
                  key={item.id ?? index}
                  _hover={{ bg: "gray.50" }}
                  transition="background 0.2s"
                >
                  {useCheckBox && (
                    <Td
                      pos={hasFixedLeft ? "sticky" : undefined}
                      left={0}
                      zIndex={hasFixedLeft ? 5 : undefined}
                      bg={hasFixedLeft ? "white" : undefined}
                      w={`${CHECKBOX_WIDTH}px`}
                      minW={`${CHECKBOX_WIDTH}px`}
                      maxW={`${CHECKBOX_WIDTH}px`}
                      borderBottom="1px solid"
                      borderColor="gray.50"
                    >
                      <Checkbox
                        colorScheme="primary"
                        isChecked={isCheckedFile(item.id)}
                        onChange={(e) => handleSingleCheck(e, item, index)}
                        isDisabled={item.isDisabled}
                      />
                    </Td>
                  )}
                  {headers?.map((header, i) => {
                    const isLeft = stickyLeftMap.has(i);
                    const isRight = stickyRightMap.has(i);
                    const isFixed = isLeft || isRight;
                    const left = isLeft ? stickyLeftMap.get(i) : undefined;
                    const right = isRight ? stickyRightMap.get(i) : undefined;

                    return (
                      <Td
                        key={header.label ?? i}
                        pl={6}
                        py={4}
                        fontSize="sm"
                        color="gray.700"
                        pos={isFixed ? "sticky" : undefined}
                        left={left}
                        right={right}
                        zIndex={isFixed ? 5 : undefined}
                        bg={isFixed ? "white" : undefined}
                        w={header.width}
                        minW={header.width}
                        maxW={header.width}
                        borderBottom="1px solid"
                        borderColor="gray.50"
                        _after={
                          isLeft && shadowState.showLeftShadow
                            ? leftShadowStyle
                            : undefined
                        }
                        _before={
                          isRight && shadowState.showRightShadow
                            ? rightShadowStyle
                            : undefined
                        }
                      >
                        {item[header.label]}
                      </Td>
                    );
                  })}
                </Tr>
              ))}
          </Tbody>
        </Table>
        {/* loading */}
        {isLoading && (
          <Center flex={1} h="full" maxH="500px">
            <Spinner
              thickness="4px"
              speed="0.65s"
              emptyColor="white"
              color="primary.600"
              size="lg"
              my="auto"
              mx="auto"
            />
          </Center>
        )}
        {/* empty */}
        {!isLoading && !isError && (!bodyData || bodyData.length === 0) && (
          <Center flex={1} h="full" maxH="500px">
            <Empty type="bid-file-empty" message="暂无数据～" />
          </Center>
        )}
        {/* error */}
        {!isLoading && isError && (
          <Center flex={1} h="full" transform="scale(1.1)" maxH="500px">
            <Empty
              type="bid-error"
              message={errorMessage || "发生未知错误，请检查网络或者稍后再试。"}
            />
          </Center>
        )}
      </TableContainer>
      {effectivePagesCount ? (
        <TableFooter
          total={normalizedTotal}
          pages={effectivePages}
          pagesCount={effectivePagesCount}
          currentPage={normalizedCurrentPage}
          setCurrentPage={setCurrentPage}
          pageSize={normalizedPageSize}
          setPageSize={setPageSize}
          handleCurrentPageChange={() => {}}
          {...footerProps}
        />
      ) : null}
    </Flex>
  );
}
