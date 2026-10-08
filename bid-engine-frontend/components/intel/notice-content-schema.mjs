import rehypeRaw from "rehype-raw";
import rehypeSanitize, { defaultSchema } from "rehype-sanitize";

const DOCUMENT_TAGS = [
  "h1",
  "h2",
  "h3",
  "h4",
  "h5",
  "h6",
  "p",
  "br",
  "ol",
  "ul",
  "li",
  "table",
  "caption",
  "colgroup",
  "col",
  "thead",
  "tbody",
  "tfoot",
  "tr",
  "th",
  "td",
  "blockquote",
  "pre",
  "code",
  "strong",
  "b",
  "em",
  "i",
  "u",
  "s",
  "del",
  "a",
];

/** @type {import("hast-util-sanitize").Schema} */
export const noticeSanitizeSchema = {
  ...defaultSchema,
  tagNames: DOCUMENT_TAGS,
  attributes: {
    a: ["href", "title"],
    ol: ["start", "type"],
    th: ["colSpan", "rowSpan", "scope"],
    td: ["colSpan", "rowSpan"],
    col: ["span"],
  },
  protocols: {
    href: ["http", "https", "mailto"],
  },
};

/** @type {import("unified").PluggableList} */
export const noticeHTMLRehypePlugins = [
  rehypeRaw,
  [rehypeSanitize, noticeSanitizeSchema],
];

/** @type {import("unified").PluggableList} */
export const noticeMarkdownRehypePlugins = [
  [rehypeSanitize, noticeSanitizeSchema],
];
