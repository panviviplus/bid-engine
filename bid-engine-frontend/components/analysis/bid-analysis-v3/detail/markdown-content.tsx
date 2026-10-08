"use client";

import {
  Box,
  Table,
  TableContainer,
  Tbody,
  Td,
  Text,
  Th,
  Thead,
  Tr,
} from "@chakra-ui/react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

export function MarkdownContent({ content }: { content: string }) {
  return (
    <Box
      className="markdown-content"
      minW={0}
      fontSize="sm"
      lineHeight="1.8"
      color="workbench.text"
      sx={{
        "& > *:first-of-type": { marginTop: 0 },
        "& p": { whiteSpace: "pre-wrap", overflowWrap: "anywhere" },
        "& ul, & ol": { paddingLeft: "1.25rem", marginBlock: "0.5rem" },
        "& li": { marginBlock: "0.15rem" },
      }}
    >
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          p: ({ children }) => <Text>{children}</Text>,
          table: ({ children }) => (
            <TableContainer
              mt={3}
              mb={3}
              overflowX="auto"
              border="1px solid"
              borderColor="workbench.line"
              borderRadius="9px"
            >
              <Table size="sm" variant="simple">
                {children}
              </Table>
            </TableContainer>
          ),
          thead: ({ children }) => <Thead bg="neutral.50">{children}</Thead>,
          tbody: ({ children }) => <Tbody>{children}</Tbody>,
          tr: ({ children }) => <Tr>{children}</Tr>,
          th: ({ children }) => (
            <Th whiteSpace="normal" minW="120px" color="workbench.text">
              {children}
            </Th>
          ),
          td: ({ children }) => (
            <Td whiteSpace="normal" minW="120px" verticalAlign="top">
              {children}
            </Td>
          ),
        }}
      >
        {content}
      </ReactMarkdown>
    </Box>
  );
}
