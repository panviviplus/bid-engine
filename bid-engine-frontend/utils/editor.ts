function stripHeadingPrefix(text: string) {
  if (!text) return "";
  const han = "一二三四五六七八九十";
  const numClass = `0-9${han}`;
  const re1 = new RegExp(`^[（(][${numClass}]+([.．、][${numClass}]+)*[)）]\\s*`);
  const re2 = new RegExp(`^[${numClass}]+([.．、][${numClass}]+)*[.．、]?\\s*`);
  const re3 = new RegExp(`^第[${numClass}]+(章|节|部分|篇|条|目)[、，.．、]?\\s*`);
  return text.replace(/^\s+/, "").replace(re1, "").replace(re2, "").replace(re3, "");
}

function extractHeadings(html: string) {
  const parser = new DOMParser();
  const doc = parser.parseFromString(html, "text/html");
  const classLevelMap: Record<string, number> = {};
  const styleNodes = doc.querySelectorAll("style");
  for (let i = 0; i < styleNodes.length; i += 1) {
    const css = styleNodes[i].textContent || "";
    if (!css) continue;
    const blocks = css.split("}");
    for (let bi = 0; bi < blocks.length; bi += 1) {
      const block = blocks[bi];
      const idxOpen = block.lastIndexOf("{");
      if (idxOpen < 0) continue;
      const selectors = block.slice(0, idxOpen);
      const body = block.slice(idxOpen + 1);
      const mm = body.match(/mso-outline-level\s*:\s*(\d+)/i);
      if (!mm || !mm[1]) continue;
      const lvl = parseInt(mm[1], 10);
      if (Number.isNaN(lvl)) continue;
      const parts = selectors.split(",");
      for (let k = 0; k < parts.length; k += 1) {
        const sel = parts[k].trim();
        const ms1 = sel.match(/^\s*(p|h[1-6]|\*)?\s*\.([a-zA-Z0-9_-]+)/i);
        const ms2 = sel.match(/^\s*(p|h[1-6]|\*)\s*\[class~=['\"]?([a-zA-Z0-9_-]+)['\"]?\]/i);
        const ms = ms1 || ms2;
        if (ms) {
          const t = (ms[1] || "*").toUpperCase();
          const c = ms[2].toLowerCase();
          classLevelMap[`${t}.${c}`] = lvl;
        }
      }
    }
  }
  const nodes = doc.querySelectorAll("h1,h2,h3,h4,h5,h6,p");
  const res: { level: number; text: string; raw: string }[] = [];
  for (let i = 0; i < nodes.length; i += 1) {
    const el = nodes[i] as HTMLElement;
    const tag = el.tagName.toUpperCase();
    let level = 0;
    let rawText = "";
    if (tag.startsWith("H") && tag.length === 2) {
      level = parseInt(tag.charAt(1), 10);
      rawText = (el.textContent || "").trim();
    } else if (tag === "P") {
      const styleAttr = (el.getAttribute("style") || "").toLowerCase();
      const m1 = styleAttr.match(/mso-outline-level\s*:\s*(\d+)/i);
      if (m1 && m1[1]) {
        level = parseInt(m1[1], 10);
      } else {
        const cls = (el.getAttribute("class") || "").toLowerCase();
        const m2 = cls.match(/msoheading\s*(\d)/i) || cls.match(/msoheading(\d)/i);
        if (m2 && m2[1]) level = parseInt(m2[1], 10);
        if (!level && cls) {
          const classes = cls.split(/\s+/).filter(Boolean);
          for (let ci = 0; ci < classes.length; ci += 1) {
            const key1 = `${tag}.${classes[ci]}`;
            const key2 = `*.${classes[ci]}`;
            if (classLevelMap[key1]) {
              level = classLevelMap[key1];
              break;
            }
            if (classLevelMap[key2]) {
              level = classLevelMap[key2];
              break;
            }
          }
        }
      }
      if (level > 0) {
        const bolds = el.querySelectorAll("b");
        if (bolds && bolds.length) {
          let buf = "";
          for (let j = 0; j < bolds.length; j += 1) {
            const t = (bolds[j].textContent || "").trim();
            if (t) buf += (buf ? " " : "") + t;
          }
          rawText = buf || (el.textContent || "").trim();
        } else {
          rawText = (el.textContent || "").trim();
        }
      }
    }
    if (level > 0 && rawText) {
      const lvl = Math.min(Math.max(level, 1), 6);
      res.push({ level: lvl, text: rawText, raw: rawText });
    }
  }
  return res;
}

function convertToHTML(arr: any[]) {
  if (!arr?.length) return;
  return arr
    .map((item) => {
      const { level, type, content } = item;
      if (type === "title") {
        const lvl = Number(level);
        const base = Number.isNaN(lvl) ? 0 : lvl;
        const hLevel = Math.min(Math.max(base + 1, 1), 6);
        return `<h${hLevel}>${content}</h${hLevel}>`;
      }
      if (type === "paragraph") {
        const text = String(content || "").replace(/\r\n|\r|\n|\n\n/g, "<br/>");
        return `<p>${text}</p>`;
      }
      return "";
    })
    .join("\n");
}

function escapeHtml(s: string) {
  return String(s || "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/\"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function outlineTreeToHTML(nodes: any[]) {
  const walk = (list: any[]): string => {
    if (!Array.isArray(list) || list.length === 0) return "";
    return list
      .map((n) => {
        const lvlRaw = Number(n?.level || 1);
        const lvl = Math.min(Math.max(Number.isNaN(lvlRaw) ? 1 : lvlRaw, 1), 6);
        const title = escapeHtml(n?.title || "");
        const titleHtml = title ? `<h${lvl}>${title}</h${lvl}>` : "";
        const contents = Array.isArray(n?.contents) ? n.contents : [];
        const contentHtml = contents
          .map((c: any) => {
            const t = escapeHtml(String(c || "")).replace(/\r\n|\r|\n/g, "<br/>");
            return t ? `<p>${t}</p>` : "";
          })
          .join("\n");
        const childrenHtml = walk(n?.children || []);
        return [titleHtml, contentHtml, childrenHtml].filter(Boolean).join("\n");
      })
      .filter(Boolean)
      .join("\n");
  };
  return walk(nodes);
}

export { stripHeadingPrefix, extractHeadings, convertToHTML, outlineTreeToHTML };
