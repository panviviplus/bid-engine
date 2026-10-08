/* Hallmark · component: notice-document-content · genre: modern-minimal workbench · theme: chakra-smart-bid (preserved)
 * pre-emit critique: P5 H5 E4 S5 R5 V3 · contrast: pass (40–41) · mobile: pass by bounded table scroll
 */

"use client";

import {
  Box,
  Code,
  Heading,
  Link,
  ListItem,
  OrderedList,
  Table,
  TableCaption,
  TableContainer,
  Tbody,
  Td,
  Text,
  Tfoot,
  Th,
  Thead,
  Tr,
  UnorderedList,
} from "@chakra-ui/react";
import ReactMarkdown, { Components } from "react-markdown";
import { ReactNode } from "react";
import remarkGfm from "remark-gfm";

import {
  noticeHTMLRehypePlugins,
  noticeMarkdownRehypePlugins,
} from "./notice-content-schema.mjs";

const headingProps = {
  color: "workbench.text",
  fontStyle: "normal",
  letterSpacing: "-0.02em",
  lineHeight: "1.3",
  overflowWrap: "anywhere" as const,
};

const documentComponents: Components = {
  h1: ({ children }) => (
    <Heading
      as="h1"
      mt={8}
      mb={4}
      fontSize={{ base: "xl", md: "2xl" }}
      {...headingProps}
    >
      {children}
    </Heading>
  ),
  h2: ({ children }) => (
    <Heading
      as="h2"
      mt={8}
      mb={3}
      fontSize={{ base: "lg", md: "xl" }}
      {...headingProps}
    >
      {children}
    </Heading>
  ),
  h3: ({ children }) => (
    <Heading
      as="h3"
      mt={6}
      mb={3}
      fontSize={{ base: "md", md: "lg" }}
      {...headingProps}
    >
      {children}
    </Heading>
  ),
  h4: ({ children }) => (
    <Heading as="h4" mt={5} mb={2} fontSize="md" {...headingProps}>
      {children}
    </Heading>
  ),
  h5: ({ children }) => (
    <Heading as="h5" mt={4} mb={2} fontSize="sm" {...headingProps}>
      {children}
    </Heading>
  ),
  h6: ({ children }) => (
    <Heading as="h6" mt={4} mb={2} fontSize="sm" {...headingProps}>
      {children}
    </Heading>
  ),
  p: ({ children }) => (
    <Text
      as="p"
      mb={4}
      maxW="75ch"
      fontSize="md"
      lineHeight="1.8"
      overflowWrap="anywhere"
    >
      {children}
    </Text>
  ),
  ul: ({ children }) => (
    <UnorderedList mb={4} ml={6} spacing={2} styleType="disc">
      {children}
    </UnorderedList>
  ),
  ol: ({ children, start }) => (
    <OrderedList mb={4} ml={6} spacing={2} start={start} styleType="decimal">
      {children}
    </OrderedList>
  ),
  li: ({ children }) => (
    <ListItem pl={1} fontSize="md" lineHeight="1.8" overflowWrap="anywhere">
      {children}
    </ListItem>
  ),
  table: ({ children }) => (
    <TableContainer
      my={5}
      maxW="100%"
      overflowX="auto"
      border="1px solid"
      borderColor="workbench.line"
      borderRadius="lg"
    >
      <Table size="sm" variant="simple" w="max-content" minW="100%">
        {children}
      </Table>
    </TableContainer>
  ),
  caption: ({ children }) => (
    <TableCaption color="workbench.muted" fontSize="sm">
      {children}
    </TableCaption>
  ),
  thead: ({ children }) => <Thead bg="neutral.50">{children}</Thead>,
  tbody: ({ children }) => <Tbody>{children}</Tbody>,
  tfoot: ({ children }) => <Tfoot bg="neutral.50">{children}</Tfoot>,
  tr: ({ children }) => <Tr _even={{ bg: "neutral.50" }}>{children}</Tr>,
  th: ({ children, colSpan, rowSpan, scope }) => (
    <Th
      colSpan={colSpan}
      rowSpan={rowSpan}
      scope={scope}
      minW="120px"
      px={4}
      py={3}
      color="workbench.text"
      fontSize="sm"
      lineHeight="1.5"
      textTransform="none"
      letterSpacing="normal"
      whiteSpace="normal"
      verticalAlign="middle"
    >
      {children}
    </Th>
  ),
  td: ({ children, colSpan, rowSpan }) => (
    <Td
      colSpan={colSpan}
      rowSpan={rowSpan}
      minW="120px"
      px={4}
      py={3}
      fontSize="sm"
      lineHeight="1.7"
      whiteSpace="normal"
      verticalAlign="top"
      overflowWrap="anywhere"
    >
      {children}
    </Td>
  ),
  blockquote: ({ children }) => (
    <Box
      as="blockquote"
      my={5}
      pl={4}
      borderInlineStart="3px solid"
      borderColor="gold.300"
      color="neutral.600"
    >
      {children}
    </Box>
  ),
  pre: ({ children }) => (
    <Box
      as="pre"
      my={5}
      p={4}
      overflowX="auto"
      border="1px solid"
      borderColor="workbench.line"
      borderRadius="md"
      bg="neutral.50"
      fontSize="sm"
      lineHeight="1.7"
    >
      {children}
    </Box>
  ),
  code: ({ children }) => (
    <Code px={1.5} py={0.5} colorScheme="gray" fontSize="0.9em">
      {children}
    </Code>
  ),
  a: ({ children, href, title }) => (
    <Link
      href={href}
      title={title}
      target="_blank"
      rel="noopener noreferrer"
      color="primary.600"
      textDecoration="underline"
      textUnderlineOffset="3px"
      transition="color 150ms cubic-bezier(0.16, 1, 0.3, 1)"
      _hover={{ color: "primary.700", textDecorationThickness: "2px" }}
      _active={{ color: "primary.800" }}
      _focusVisible={{
        outline: "2px solid var(--chakra-colors-gold-400)",
        outlineOffset: "2px",
        borderRadius: "sm",
      }}
    >
      {children}
    </Link>
  ),
};

function DocumentFrame({ children }: { children: ReactNode }) {
  return (
    <Box
      minW={0}
      color="workbench.text"
      fontSize="md"
      lineHeight="1.8"
      sx={{
        "& > *:first-of-type": { marginTop: 0 },
        "& > *:last-child": { marginBottom: 0 },
        "& li > p": { marginBottom: 0 },
        "& li > ul, & li > ol": {
          marginTop: "var(--chakra-space-2)",
          marginBottom: 0,
        },
      }}
    >
      {children}
    </Box>
  );
}

export function NoticeDocumentContent({
  html,
  markdown,
  text,
}: {
  html?: string;
  markdown?: string;
  text?: string;
}) {
  const htmlContent = html?.trim() || "";
  if (htmlContent) {
    return (
      <DocumentFrame>
        <ReactMarkdown
          rehypePlugins={noticeHTMLRehypePlugins}
          components={documentComponents}
        >
          {htmlContent}
        </ReactMarkdown>
      </DocumentFrame>
    );
  }

  const markdownContent = markdown?.trim() || "";
  if (markdownContent) {
    return (
      <DocumentFrame>
        <ReactMarkdown
          remarkPlugins={[remarkGfm]}
          rehypePlugins={noticeMarkdownRehypePlugins}
          components={documentComponents}
        >
          {markdownContent}
        </ReactMarkdown>
      </DocumentFrame>
    );
  }

  const plainText = text?.trim() || "";
  return plainText ? (
    <Text
      maxW="75ch"
      fontSize="md"
      lineHeight="1.8"
      whiteSpace="pre-wrap"
      overflowWrap="anywhere"
    >
      {plainText}
    </Text>
  ) : null;
}
