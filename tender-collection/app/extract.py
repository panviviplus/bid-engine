"""抽取阶段：详情页 → 规范化公告文档。

取数策略：不再逐站写详情页正则。
- 正文：通用正文抽取（trafilatura 主、readability 兜底、Beansoup 纯文本兜底）；
- 字段：规则优先（正则/标签），置信度与告警如实写进 extract 元信息，
  缺失字段由后端用 LLM 补全（本服务不调用 LLM，保持职责单一）。
"""

from __future__ import annotations

import logging
import re
from datetime import datetime, timezone
from urllib.parse import urljoin, urlparse

import trafilatura
from bs4 import BeautifulSoup, Comment, Tag

from .discover import ATTACHMENT_SUFFIX, DATE_IN_TEXT
from .fetcher import Fetcher
from .recipes import Recipe
from .schemas import Attachment, ExtractMeta, NoticeDocument

logger = logging.getLogger(__name__)

# 字段抽取规则：标签 → 正则。规则优先，命中即停；未命中交由后端 LLM 兜底。
FIELD_PATTERNS: dict[str, list[re.Pattern[str]]] = {
    "publisher_name": [
        re.compile(r"(?:采购人|采购单位|招标人|招标单位|采购主体)(?:名称)?[：:\s]+([^\n，,；;、]{4,60})"),
        re.compile(r"(?:受)[^\n]{0,10}?([^\n，,；;]{2,40})(?:委托)[^\n]{0,20}?进行"),
    ],
    "agency_name": [
        re.compile(r"(?:采购代理机构|代理机构|招标代理机构|代理单位)(?:名称)?[：:\s]+([^\n，,；;、]{4,60})"),
    ],
    "project_code": [
        re.compile(r"(?:项目编号|招标编号|采购编号|项目代码)[：:\s]*([A-Za-z0-9\-_/（）()]{4,60})"),
    ],
    "budget_text": [
        re.compile(r"(?:预算金额|项目预算|预算总额|最高限价|招标控制价|控制价)[：:\s]*([^\n；;]{1,60})"),
    ],
    "deadline_text": [
        re.compile(r"(?:投标文件递交截止时间|响应文件递交截止时间|投标截止时间|递交截止时间|开标时间)[：:\s]*([^\n；;]{4,60})"),
    ],
    "notice_type_text": [
        re.compile(r"([\u4e00-\u9fa5]{2,12}(?:公告|公示|通知))"),
    ],
}

# 规则很容易误伤的通用词开头：命中说明捕获到的是正文片段而不是字段值。
VALUE_STOP_WORDS = (
    "需求", "名称", "地址", "信息", "要求", "方式", "时间", "条件", "内容",
    "范围", "标准", "详见", "见附件", "无", "通过", "根据", "按照", "本", "该",
)

# 需要「捕获内容包含日期时间」才认可的字段（避免把整段说明当成截止时间）。
NEED_DATETIME_FIELDS = {"deadline_text"}

DATETIME_HINT = re.compile(r"\d{4}\s*[-/年.]\s*\d{1,2}|\d{1,2}\s*[:：]\s*\d{2}")

# 发布日期优先按「发布时间/发布日期」等标签定位，避免误取正文中第一个日期。
PUBLISH_DATE_PATTERNS = [
    re.compile(r"(?:发布时间|发布日期|公告时间|公告日期|发布公告日期)[^\d]{0,6}(20\d{2})[-/年.](\d{1,2})[-/月.](\d{1,2})"),
]

# 无标签兜底时用于排除「截止/开标」语境，避免把投标截止时间当发布时间。
DEADLINE_CONTEXT = re.compile(r"(递交|截止|开标|投标|响应文件|获取招标文件|领取招标文件)")

TITLE_SELECTORS = ["h1", ".title", ".article-title", "#title", ".detail-title"]
CONTENT_SELECTORS = [
    ".vF_detail_content",
    "#content",
    ".article-content",
    ".detail-content",
    ".content",
    "article",
]

ALLOWED_BODY_TAGS = {
    "h1", "h2", "h3", "h4", "h5", "h6",
    "p", "br", "ol", "ul", "li",
    "table", "caption", "colgroup", "col", "thead", "tbody", "tfoot", "tr", "th", "td",
    "blockquote", "pre", "code", "strong", "b", "em", "i", "u", "s", "del", "a",
}
ALLOWED_BODY_ATTRS: dict[str, set[str]] = {
    "a": {"href", "title"},
    "ol": {"start", "type"},
    "th": {"colspan", "rowspan", "scope"},
    "td": {"colspan", "rowspan"},
    "col": {"span"},
}
DANGEROUS_BODY_TAGS = {
    "script", "style", "iframe", "object", "embed", "form", "input", "button",
    "template", "svg", "canvas", "noscript", "video", "audio", "source", "link", "meta",
}
NOISE_BODY_TAGS = {"nav", "footer", "aside"}
NOISE_ATTR_RE = re.compile(
    r"(?:^|[-_])(?:ad|ads|advert|advertisement|share|sharing|recommend|related|"
    r"qrcode|qr-code|toolbar|tools|navigation|footer|sidebar)(?:[-_]|$)",
    re.IGNORECASE,
)
SAFE_LINK_SCHEMES = {"http", "https", "mailto"}
SECTION_HEADING_RE = re.compile(
    r"^(?:[一二三四五六七八九十百]+[、.]|第[一二三四五六七八九十百0-9]+[章节部分])"
)


async def extract_document(
    recipe: Recipe | None,
    fetcher: Fetcher,
    url: str,
    source_key: str = "",
    source_name: str = "",
    needs_browser: bool = False,
    include_raw_html: bool = False,
) -> NoticeDocument:
    """抓取并抽取单条公告。"""
    result = await fetcher.fetch(url, force_browser=needs_browser or bool(recipe and recipe.needs_browser))
    warnings = list(result.warnings)
    if not result.content.strip():
        return NoticeDocument(
            source_key=source_key,
            source_name=source_name,
            url=url,
            canonical_url=url,
            extract=ExtractMeta(strategy=result.strategy, confidence=0.0, warnings=warnings or ["页面为空"]),
            fetched_at=_now(),
        )

    resolved_content_selectors = (recipe.content_selectors if recipe else []) or CONTENT_SELECTORS
    title, body_text, body_markdown, title_warnings = extract_article(
        result.content,
        title_selectors=(recipe.title_selectors if recipe else []) or TITLE_SELECTORS,
        content_selectors=resolved_content_selectors,
    )
    body_html = extract_body_html(result.content, url, resolved_content_selectors)
    warnings.extend(title_warnings)

    # 页头元信息（如「公告时间 2026年09月21日」）常位于正文容器之外，
    # 因此字段抽取额外带上整页纯文本，正文容器仍只用于正文与其余字段。
    page_text = " ".join(BeautifulSoup(result.content, "lxml").get_text(" ").split())
    fields = extract_fields(body_text, page_text=page_text)
    attachments = extract_attachments(result.content, url)

    # 公告类型只认「标题里出现的那一个」：正文里常提到变更/澄清等字样，
    # 用它做类型判定会把公开招标公告误判成变更公告。
    notice_type_text = title if _looks_like_notice_type(title) else ""

    confidence = _score(title, body_text, fields, warnings)
    document = NoticeDocument(
        source_key=source_key,
        source_name=source_name,
        url=url,
        canonical_url=url,
        title=title,
        publisher_name=fields.get("publisher_name", ""),
        agency_name=fields.get("agency_name", ""),
        project_code=fields.get("project_code", ""),
        budget_text=fields.get("budget_text", ""),
        publish_date=fields.get("publish_date", ""),
        deadline_text=fields.get("deadline_text", ""),
        region_text=fields.get("region_text", ""),
        notice_type_text=notice_type_text,
        body_html=body_html,
        body_markdown=body_markdown,
        body_text=body_text,
        attachments=attachments,
        fetched_at=_now(),
        extract=ExtractMeta(strategy=result.strategy, confidence=confidence, warnings=warnings),
        raw_html=result.content if include_raw_html else None,
    )
    return document


def extract_article(
    html: str,
    title_selectors: list[str] | None = None,
    content_selectors: list[str] | None = None,
) -> tuple[str, str, str, list[str]]:
    """抽取正文，返回（标题, 纯文本, Markdown, 告警）。"""
    warnings: list[str] = []
    soup = BeautifulSoup(html, "lxml")

    title = ""
    for selector in (title_selectors or TITLE_SELECTORS):
        node = soup.select_one(selector)
        if node:
            title = " ".join(node.get_text().split())
            if title:
                break
    if not title and soup.title:
        title = " ".join((soup.title.get_text() or "").split())

    # 优先在站点声明的正文容器内抽取，缩小噪声范围
    fragment = html
    for selector in (content_selectors or CONTENT_SELECTORS):
        node = soup.select_one(selector)
        if node and len(node.get_text(strip=True)) > 80:
            fragment = str(node)
            break

    markdown = trafilatura.extract(
        fragment,
        output_format="markdown",
        include_links=True,
        include_formatting=True,
        favor_precision=True,
    )
    text = trafilatura.extract(fragment, include_links=False, include_formatting=False)

    if not text:
        warnings.append("trafilatura 抽取为空，回退 readability")
        text = _readability_text(fragment)
        markdown = markdown or text

    if not text:
        warnings.append("readability 抽取为空，回退纯文本")
        text = " ".join(BeautifulSoup(fragment, "lxml").get_text(" ").split())
        markdown = markdown or text

    if not title:
        warnings.append("未识别到标题")
    if len(text) < 50:
        warnings.append("正文过短，可能是空壳页面或需要登录")

    return title, text or "", markdown or text or "", warnings


def extract_body_html(
    html: str,
    base_url: str,
    content_selectors: list[str] | None = None,
) -> str:
    """从可信正文容器生成白名单语义 HTML；无法定位正文时返回空串。"""
    soup = BeautifulSoup(html, "lxml")
    selected: Tag | None = None
    for selector in (content_selectors or CONTENT_SELECTORS):
        node = soup.select_one(selector)
        if isinstance(node, Tag) and len(node.get_text(strip=True)) > 80:
            selected = node
            break
    if selected is None:
        return ""

    fragment = BeautifulSoup(str(selected), "lxml")
    root = fragment.body.find() if fragment.body else fragment.find()
    if not isinstance(root, Tag):
        return ""

    for comment in list(root.find_all(string=lambda value: isinstance(value, Comment))):
        comment.extract()
    for tag in list(root.find_all(DANGEROUS_BODY_TAGS)):
        tag.decompose()
    for tag in list(root.find_all(True)):
        if tag.name in NOISE_BODY_TAGS or _is_noise_body_node(tag):
            tag.decompose()

    _promote_section_headings(root)

    for tag in list(root.find_all(True)):
        if tag.name not in ALLOWED_BODY_TAGS:
            tag.unwrap()
            continue
        allowed_attrs = ALLOWED_BODY_ATTRS.get(tag.name, set())
        for attr in list(tag.attrs):
            if attr not in allowed_attrs:
                del tag.attrs[attr]
        _sanitize_body_attributes(tag, base_url)

    for paragraph in list(root.find_all("p")):
        if not paragraph.get_text(" ", strip=True) and paragraph.find("br") is None:
            paragraph.decompose()

    return root.decode_contents(formatter="minimal").strip()


def _is_noise_body_node(tag: Tag) -> bool:
    values: list[str] = []
    for attr in ("id", "class", "role"):
        raw = tag.get(attr)
        if isinstance(raw, list):
            values.extend(str(item) for item in raw)
        elif raw:
            values.append(str(raw))
    return any(NOISE_ATTR_RE.search(value) for value in values)


def _promote_section_headings(root: Tag) -> None:
    """把源站常见的纯粗体编号段落规范成章节标题。"""
    for paragraph in root.find_all("p"):
        meaningful_children = [
            child
            for child in paragraph.contents
            if not (isinstance(child, str) and not child.strip())
        ]
        if len(meaningful_children) != 1:
            continue
        strong = meaningful_children[0]
        if not isinstance(strong, Tag) or strong.name not in {"strong", "b"}:
            continue
        title = strong.get_text(" ", strip=True)
        if not SECTION_HEADING_RE.match(title):
            continue
        paragraph.name = "h2"
        strong.unwrap()


def _sanitize_body_attributes(tag: Tag, base_url: str) -> None:
    if tag.name == "a":
        raw_href = str(tag.get("href") or "").strip()
        href = urljoin(base_url, raw_href) if raw_href else ""
        if not href or urlparse(href).scheme.lower() not in SAFE_LINK_SCHEMES:
            tag.attrs.pop("href", None)
        else:
            tag["href"] = href
        if "href" not in tag.attrs:
            tag.unwrap()
        return

    numeric_attrs = {"start", "colspan", "rowspan", "span"}
    for attr in numeric_attrs.intersection(tag.attrs):
        value = str(tag.get(attr) or "").strip()
        if not re.fullmatch(r"-?\d+", value):
            del tag.attrs[attr]
            continue
        number = int(value)
        if attr != "start" and number <= 0:
            del tag.attrs[attr]
        else:
            tag[attr] = str(max(-100000, min(100000, number)))

    if tag.name == "ol" and "type" in tag.attrs:
        list_type = str(tag.get("type") or "")
        if list_type not in {"1", "a", "A", "i", "I"}:
            del tag.attrs["type"]
    if tag.name == "th" and "scope" in tag.attrs:
        scope = str(tag.get("scope") or "").lower()
        if scope not in {"row", "col", "rowgroup", "colgroup"}:
            del tag.attrs["scope"]
        else:
            tag["scope"] = scope


def _readability_text(html: str) -> str:
    try:
        from readability import Document

        doc = Document(html)
        return " ".join(BeautifulSoup(doc.summary(), "lxml").get_text(" ").split())
    except Exception as exc:  # noqa: BLE001 - 兜底失败按空处理
        logger.debug("readability 抽取失败: %s", exc)
        return ""


def extract_fields(text: str, page_text: str = "") -> dict[str, str]:
    """按规则抽取结构化字段（缺省为空串，由后端 LLM 兜底）。

    text 为正文容器纯文本；page_text 为整页纯文本，仅用于补充正文容器之外
    的页头元信息（典型是「公告时间」）。
    """
    fields: dict[str, str] = {}
    if not text and not page_text:
        return fields
    head = text[:4000]
    for name, patterns in FIELD_PATTERNS.items():
        for pattern in patterns:
            match = pattern.search(head)
            if not match:
                continue
            value = _clean_value(match.group(1), name)
            if not value:
                continue
            fields[name] = value[:120]
            break

    publish_date = _find_publish_date(page_text, head)
    if publish_date:
        fields["publish_date"] = publish_date

    for province in _PROVINCES:
        if province in head:
            fields.setdefault("region_text", province)
            break
    return fields


def _find_publish_date(page_text: str, body_head: str) -> str:
    """定位发布日期：优先带标签的页头/正文元信息，最后才用无标签兜底。"""
    for source in (page_text, body_head):
        if not source:
            continue
        for pattern in PUBLISH_DATE_PATTERNS:
            match = pattern.search(source)
            if match:
                year, month, day = match.groups()
                return f"{int(year):04d}-{int(month):02d}-{int(day):02d}"
    return _fallback_publish_date(body_head)


def _fallback_publish_date(body_head: str) -> str:
    """无标签兜底：只看正文开头，且排除投标截止/开标等语境。

    宁可留空交给后端 LLM，也不要把「前递交投标文件的截止时间」写成发布时间。
    """
    for match in DATE_IN_TEXT.finditer(body_head[:600]):
        window = body_head[max(0, match.start() - 25) : match.end() + 25]
        if DEADLINE_CONTEXT.search(window):
            continue
        year, month, day = match.groups()
        return f"{int(year):04d}-{int(month):02d}-{int(day):02d}"
    return ""


def _clean_value(raw: str, field_name: str) -> str:
    """清洗并校验规则捕获值；明显跑偏的值返回空串，交给后端 LLM 兜底。"""
    value = " ".join((raw or "").split()).strip("：: 　-—·")
    if len(value) < 2:
        return ""
    if value.startswith(VALUE_STOP_WORDS):
        return ""
    if value.endswith(("如下", "详见", "另行通知", "为准")):
        return ""
    if "“" in value or "”" in value or "《" in value:
        # 引号/书名号通常是正文引用，不是字段值
        return ""
    if field_name in NEED_DATETIME_FIELDS and not DATETIME_HINT.search(value):
        return ""
    return value


NOTICE_TYPE_HINTS = (
    "公开招标", "邀请招标", "竞争性磋商", "竞争性谈判", "磋商", "谈判",
    "询价", "资格预审", "单一来源", "变更", "澄清", "更正", "终止", "废标",
    "招标公告", "采购公告", "公示", "通知",
)


def _looks_like_notice_type(text: str) -> bool:
    """标题是否本身就是一条公告类型表述。"""
    return any(hint in (text or "") for hint in NOTICE_TYPE_HINTS)


def extract_attachments(html: str, base_url: str) -> list[Attachment]:
    """抽取附件链接（只记录链接，不下载文件）。"""
    soup = BeautifulSoup(html, "lxml")
    out: list[Attachment] = []
    seen: set[str] = set()
    for anchor in soup.find_all("a", href=True):
        href = anchor["href"].strip()
        name = " ".join((anchor.get_text() or "").split())
        if not href or href.startswith("#"):
            continue
        is_attachment = bool(ATTACHMENT_SUFFIX.search(href))
        looks_like_attachment = any(keyword in name for keyword in ("附件", "下载", "文件"))
        if not (is_attachment or (looks_like_attachment and href.startswith(("http", "/", "./", "../")))):
            continue
        url = urljoin(base_url, href)
        if url in seen:
            continue
        seen.add(url)
        out.append(Attachment(name=name or url.rsplit("/", 1)[-1], url=url))
        if len(out) >= 20:
            break
    return out


def _score(title: str, body: str, fields: dict[str, str], warnings: list[str]) -> float:
    """粗略置信度：标题、正文长度、字段命中数共同决定，供后端排序与排障。"""
    score = 0.0
    if title:
        score += 0.25
    if len(body) >= 800:
        score += 0.4
    elif len(body) >= 200:
        score += 0.2
    score += min(0.3, 0.06 * len(fields))
    score -= 0.05 * len(warnings)
    return max(0.0, min(1.0, round(score, 4)))


def _now() -> str:
    return datetime.now(timezone.utc).astimezone().isoformat(timespec="seconds")


_PROVINCES = [
    "北京市", "天津市", "上海市", "重庆市",
    "河北省", "山西省", "辽宁省", "吉林省", "黑龙江省",
    "江苏省", "浙江省", "安徽省", "福建省", "江西省", "山东省",
    "河南省", "湖北省", "湖南省", "广东省", "海南省",
    "四川省", "贵州省", "云南省", "陕西省", "甘肃省", "青海省",
    "内蒙古自治区", "广西壮族自治区", "西藏自治区", "宁夏回族自治区", "新疆维吾尔自治区",
]
