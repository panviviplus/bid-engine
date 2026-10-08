/* eslint-disable no-else-return */
import { saveAs } from "file-saver";

export const formatOptions = (
  data: any[],
  key?: string,
  value?: string,
  saveData?: boolean,
  labelRight?: string,
) => {
  if (!data) return [];
  const list: { label: string; value: string | number; item?: any }[] = [];
  if (key && value) {
    data.forEach((item) => {
      let newLabel = labelRight
        ? `${item[key]} (${item[labelRight]})`
        : item[key];
      if (!newLabel) {
        newLabel = item.label;
      }
      const itemData: { label: string; value: string | number; item?: any } = {
        label: newLabel,
        value: item[value] ? item[value] : item.value,
      };
      if (saveData) {
        itemData.item = item;
      }
      list.push(itemData);
    });
  } else {
    data.forEach((item) => {
      const itemData: { label: string; value: string | number; item?: any } = {
        label: item.label ? item.label : item,
        value: item.value ? item.value : item,
      };
      if (saveData) {
        itemData.item = item;
      }
      list.push(itemData);
    });
  }

  return list;
};

export const getOptionsName = (
  list: { label: string; value: string | number }[],
  value: string | number,
) => {
  if (!list?.length) return "";
  return list.filter((item) => item.value === value)[0]?.label || "";
};
export const getOptionsValue = (
  list: { label: string; value: string | number }[],
  label: string,
) => {
  if (!list.length) return "";
  return list.filter((item) => item.label === label)[0]?.value || "";
};

export async function exportAndSave(res: any, name?: string) {
  const disposition =
    res.headers?.["content-disposition"] ||
    res.headers?.["Content-Disposition"];
  let fileName = name || "download"; // 兜底
  if (disposition) {
    // eslint-disable-next-line no-useless-escape
    const utf8Match = disposition.match(/filename\*=UTF-8''([^;]+)/i);
    const asciiMatch = disposition.match(/filename=["']?([^"';]+)["']?/i);
    if (utf8Match) {
      const v = String(utf8Match[1]).trim().replace(/^["']|["']$/g, "");
      fileName = decodeURIComponent(v.replace(/\+/g, "%20"));
    } else if (asciiMatch) {
      fileName = asciiMatch[1];
    }
  }
  saveAs(res.data, fileName);
}

export function isMarkdownTable(str: string): boolean {
  const lines = str
    .replace(/\r\n/g, "\n")
    .replace(/\r/g, "\n")
    .split("\n")
    .filter((l) => l.trim().length > 0);

  if (lines.length < 2) return false;

  const head = lines[0];
  if (!head.includes("|") || (head.match(/\|/g)?.length ?? 0) < 2) return false;

  const sep = lines[1];
  if (!sep.includes("|") || !sep.includes("-")) return false;

  const lastIdx = lines.length - 1;
  let ok = lines.slice(2).some((l) => l.includes("|"));
  if (!ok && lastIdx >= 2) {
    ok = lines[lastIdx].includes("|");
  }
  return ok;
}

const isPureNumber = (s: string) =>
  /^[+-]?(?:\d{1,3}(?:,\d{3})*|\d+)(?:\.\d+)?$/.test(s);
const hasContent = (s: string) => /[A-Za-z\u4E00-\u9FFF]/.test(s);
const isSeparatorLine = (s: string) => /^\s*\|?[\s\-:|]*\|?\s*$/.test(s);
/** 单元格内部再分割：分号 / 句号 / <br>  不区分大小写 */
const innerSplit = (s: string): string[] =>
  s
    .split(/[；。 ]|&lt;br&gt;|<br>/i)
    .map((t) => t.trim())
    .filter(Boolean);

/** 去掉行首行尾 | 并按 | 拆成单元格 */
const splitCells = (line: string): string[] =>
  line
    .replace(/^\||\|$/g, "")
    .split("|")
    .map((c) => c.trim());
/** 把相邻且完全相同的字符串压成一个 */
const dedupeConsecutive = (arr: string[]): string[] =>
  arr.filter((s, idx) => s !== arr[idx - 1]);
/** 长为 2 */
const isTwoChars = (s: string) => s.length === 2;
export function extractMdTableCells(md: string): string[] {
  // 去掉表头（第一行）
  const body = md.slice(md.indexOf("\n") + 1);
  const lineRules = {
    skipSeparator: isSeparatorLine,
    skipEmptyCell: (c: string) => !c,
    skipNumber: isPureNumber,
    skipNoContent: (c: string) => !hasContent(c),
    skipTwoChars: isTwoChars,
  };

  return dedupeConsecutive(
    body
      .split(/\r?\n/)
      .filter((l) => !lineRules.skipSeparator(l))
      .flatMap(splitCells)
      .filter((c) => !lineRules.skipEmptyCell(c))
      .flatMap(innerSplit)
      .filter(
        (c) =>
          !lineRules.skipNumber(c) &&
          !lineRules.skipNoContent(c) &&
          !lineRules.skipTwoChars(c),
      ),
  );
}

export function fixBrokenTableLines(str: string): string {
  const raw = str.replace(/\r\n/g, "\n").replace(/\r/g, "\n");
  const lines = raw.split("\n");
  const fixed: string[] = [];
  let buffer = "";

  for (const line of lines) {
    const trimmed = line.trim();
    if (trimmed === "") continue;
    buffer += buffer === "" ? line : ` ${line.trim()}`;
    const bufTrim = buffer.trim();
    if (
      bufTrim.startsWith("|") &&
      bufTrim.endsWith("|") &&
      (bufTrim.match(/\|/g)?.length || 0) >= 2
    ) {
      fixed.push(buffer);
      buffer = "";
    }
  }

  if (buffer.trim() !== "") {
    fixed.push(buffer);
  }
  return fixed.join("\n");
}

export function copyToClipboard(text: string): Promise<void> {
  return new Promise((resolve, reject) => {
    if (navigator.clipboard && window.isSecureContext) {
      // 现代方式（安全上下文）
      navigator.clipboard
        .writeText(text)
        .then(() => resolve())
        .catch((err) => reject(err));
    } else {
      // 降级方案
      const textarea = document.createElement("textarea");
      textarea.value = text;
      textarea.style.position = "fixed";
      document.body.appendChild(textarea);
      textarea.select();
      try {
        const success = document.execCommand("copy");
        if (success) {
          resolve();
        } else {
          reject(new Error("失败"));
        }
      } catch (err) {
        reject(err);
      } finally {
        document.body.removeChild(textarea);
      }
    }
  });
}

export function toFormData(payload) {
  const fd = new FormData();
  Object.entries(payload).forEach(([k, v]) => {
    if (Array.isArray(v)) {
      v.forEach((item) => fd.append(k, item));
    } else if (v instanceof File || v instanceof Blob) {
      fd.append(k, v);
    } else if (typeof v === "object" && v !== null) {
      fd.append(k, JSON.stringify(v));
    } else if (v !== null && v !== undefined) {
      fd.append(k, String(v));
    }
  });
  return fd;
}

export function formatRepeatParam(key: string, arr: string[]) {
  if (!Array.isArray(arr)) return "";
  return arr.map((val) => `${key}=${encodeURIComponent(val)}`).join("&");
}

export function flattenTree(nodes: any[]) {
  const res: any[] = [];

  function walk(list) {
    for (const node of list) {
      const { children, ...item } = node;
      res.push(item);
      if (Array.isArray(children)) walk(children);
    }
  }

  walk(nodes);
  return res;
}

export async function urlToBase64(url: string, init?: RequestInit) {
  const res = await fetch(url, init);
  const blob = await res.blob();
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onloadend = () => resolve(String(reader.result));
    reader.onerror = reject;
    reader.readAsDataURL(blob);
  });
}

export async function urlsToBase64(urls: string[], init?: RequestInit) {
  const arr = await Promise.all(
    urls.map(async (u) => {
      try {
        return await urlToBase64(u, init);
      } catch (e) {
        return "";
      }
    }),
  );
  return arr.filter(Boolean);
}

function normalizeImgSrc(src: string) {
  if (!src) return src;
  let s = src.trim();
  s = s.replace(/^`+/, "").replace(/`+$/, "");
  return s.trim();
}

async function fetchImageAsBase64(url: string, init?: RequestInit) {
  const res = await fetch(url, init);
  const buffer = await res.arrayBuffer();
  const bytes = new Uint8Array(buffer);
  let binary = "";
  for (let i = 0; i < bytes.byteLength; i += 1) {
    binary += String.fromCharCode(bytes[i]);
  }
  const base64 = btoa(binary);
  return `data:image/png;base64,${base64}`;
}

export async function convertHtmlImgSrcToBase64(
  html: string,
  init?: RequestInit,
) {
  if (!html) return html;
  const imgSrcRegex = /<img\b[^>]*\bsrc=["']([^"']+)["'][^>]*>/gi;
  const urlSet = new Set<string>();
  const urls: string[] = [];
  let match;
  // eslint-disable-next-line no-cond-assign
  while ((match = imgSrcRegex.exec(html)) !== null) {
    const src = normalizeImgSrc(match[1]);
    if (!src) continue;
    if (!/^https?:\/\//i.test(src)) continue;
    if (!urlSet.has(src)) {
      urlSet.add(src);
      urls.push(src);
    }
  }
  if (!urls.length) return html;
  const urlToBase64Map = new Map<string, string>();
  await Promise.all(
    urls.map(async (u) => {
      try {
        const base64 = await fetchImageAsBase64(u, init);
        if (base64) urlToBase64Map.set(u, base64);
      } catch (e) {
        return undefined;
      }
    }),
  );
  if (!urlToBase64Map.size) return html;
  const replaceRegex = /<img\b[^>]*\bsrc=["']([^"']+)["'][^>]*>/gi;
  return html.replace(replaceRegex, (m, rawSrc) => {
    const src = normalizeImgSrc(rawSrc);
    if (!src) return m;
    const base64 = urlToBase64Map.get(src);
    if (!base64) return m;
    return m.replace(rawSrc, base64);
  });
}

export function downloadBase64(base64, fileName = "image.png") {
  const byte = atob(base64.replace(/^.*,/, ""));
  const ab = new ArrayBuffer(byte.length);
  const ia = new Uint8Array(ab);
  for (let i = 0; i < byte.length; i += 1) ia[i] = byte.charCodeAt(i);
  const blob = new Blob([ab], { type: "image/png" });

  saveAs(blob, fileName);
}

export const camelCase = (str) =>
  str.replace(/_([a-z0-9])/gi, (_, c) => c.toUpperCase());
