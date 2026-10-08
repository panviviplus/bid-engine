"use client";

import React, { useEffect, useState } from "react";
import {
  Box,
  Flex,
  Select,
  NumberInput,
  NumberInputField,
  Icon,
} from "@chakra-ui/react";
import {
  Pagination,
  PaginationPage,
  PaginationPrevious,
  PaginationNext,
  PaginationContainer,
  PaginationPageGroup,
  PaginationSeparator,
} from "@ajna/pagination";
import {
  ChevronLeftIcon,
  ChevronRightIcon,
  TriangleDownIcon,
} from "@chakra-ui/icons";

function TableFooter({
  hiddenTotal,
  hiddenSelect,
  hiddenInput,
  total,
  checkedFilesNum,
  pages,
  pagesCount,
  currentPage,
  setCurrentPage,
  pageSize,
  setPageSize,
  handleCurrentPageChange,
  ...props
}) {
  const [inputError, setInputError] = useState(false);
  const [inputValue, setInputValue] = useState();

  const handlePageSizeChange = (event) => {
    const pageSize = Number(event.target.value);
    setPageSize(pageSize);
    setCurrentPage(1);
    setInputValue(1);
  };

  const handleInputKeyDown = (event) => {
    const curInputValue = Number(event.target.value);
    if (event.key === "Enter") {
      if (
        curInputValue > 0 &&
        curInputValue <= pagesCount &&
        Number.isInteger(curInputValue)
      ) {
        setInputError(false);
        setCurrentPage(curInputValue);
        if (handleCurrentPageChange) {
          handleCurrentPageChange();
        }
      } else {
        setInputError(true);
      }
    }
  };

  useEffect(() => {
    if (currentPage) {
      setInputValue(currentPage);
    }
  }, [currentPage]);

  return (
    <Flex py={4} px={6} align="center" justify="flex-end" {...props}>
      <Flex align="center" justify="end" gap={4} wrap="wrap">
        {checkedFilesNum > 0 && (
          <Flex fontSize="sm" color="gray.500" bg="gray.50" px={3} py={1} borderRadius="full">
            已选 <Text as="span" fontWeight="bold" color="primary.600" mx={1}>{checkedFilesNum}</Text> 个文档
          </Flex>
        )}
        {!hiddenTotal && <Box fontSize="sm" color="gray.500">共 {total} 条</Box>}
        {!hiddenSelect && (
          <Flex alignItems="center" fontSize="sm" color="gray.600">
            每页
            <Select
              mx={2}
              w="20"
              h={8}
              value={pageSize}
              onChange={handlePageSizeChange}
              fontSize="sm"
              borderRadius="md"
              borderColor="gray.200"
              _hover={{ borderColor: "gray.300" }}
              _focus={{ borderColor: "primary.500", boxShadow: "0 0 0 1px var(--chakra-colors-primary-500)" }}
              icon={<TriangleDownIcon fontSize={10} color="gray.400" />}
            >
              <option value={5}>5</option>
              <option value={10}>10</option>
              <option value={20}>20</option>
              <option value={50}>50</option>
            </Select>
            条
          </Flex>
        )}
        <Pagination
          pagesCount={pagesCount}
          currentPage={currentPage}
          onPageChange={(page) => {
            setCurrentPage(page);
            if (handleCurrentPageChange) {
              handleCurrentPageChange();
            }
          }}
        >
          <PaginationContainer align="center" justify="right" gap={1}>
            <PaginationPrevious
              boxSize="8"
              bg="white"
              border="1px solid"
              borderColor="gray.200"
              fontSize="sm"
              borderRadius="md"
              _hover={{ bg: "gray.50", borderColor: "gray.300" }}
              _disabled={{ opacity: 0.4, cursor: "not-allowed" }}
            >
              <Icon as={ChevronLeftIcon} boxSize="5" color="gray.500" />
            </PaginationPrevious>
            <PaginationPageGroup
              isInline
              align="center"
              separator={
                <PaginationSeparator
                  fontSize="sm"
                  boxSize="8"
                  jumpSize={1}
                  borderRadius="md"
                  color="gray.400"
                />
              }
            >
              {pages.map((page) => (
                <PaginationPage
                  key={`pagination_page_${page}`}
                  boxSize="8"
                  bg="white"
                  border="1px solid"
                  borderColor="gray.200"
                  fontSize="sm"
                  page={page}
                  _hover={{ bg: "gray.50", borderColor: "gray.300" }}
                  _current={{ 
                    bg: "primary.600", 
                    color: "white", 
                    borderColor: "primary.600",
                    _hover: { bg: "primary.700" }
                  }}
                  borderRadius="md"
                />
              ))}
            </PaginationPageGroup>
            <PaginationNext
              boxSize="8"
              bg="white"
              border="1px solid"
              borderColor="gray.200"
              fontSize="sm"
              borderRadius="md"
              _hover={{ bg: "gray.50", borderColor: "gray.300" }}
              _disabled={{ opacity: 0.4, cursor: "not-allowed" }}
            >
              <Icon as={ChevronRightIcon} boxSize="5" color="gray.500" />
            </PaginationNext>
          </PaginationContainer>
        </Pagination>
        <Flex align="center" h={8} fontSize="sm" color="gray.500">
          {/* 跳转到特定页 */}
          {!hiddenInput && (
            <Flex
              align="center"
              fontSize="sm"
              display={{ base: "none", md: "flex" }}
              ml={2}
            >
              <Box>跳至</Box>
              <NumberInput
                size="sm"
                w="14"
                h={8}
                mx={2}
                p={0}
                value={inputValue}
                onChange={(value) => setInputValue(value)}
                onKeyDown={handleInputKeyDown}
              >
                <NumberInputField
                  border="1px solid"
                  borderColor={inputError ? "red.500" : "gray.200"}
                  borderRadius="md"
                  h="full"
                  textAlign="center"
                  _focus={{ borderColor: "primary.500", boxShadow: "0 0 0 1px var(--chakra-colors-primary-500)" }}
                />
              </NumberInput>
              <Box>页</Box>
            </Flex>
          )}
        </Flex>
      </Flex>
    </Flex>
  );
}

export default TableFooter;
