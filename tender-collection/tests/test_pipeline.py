"""采集服务单元测试：用固定 fixture 做抽取与发现的快照式校验。

运行：`python -m pytest tests -q`（或 `python -m unittest discover -s tests`）。
测试不访问网络：所有用例都基于本地 fixture。
"""

from __future__ import annotations

import unittest
from pathlib import Path

from app.discover import _build_cursor, _extract_links, _parse_cursor
from app.extract import extract_article, extract_attachments, extract_body_html, extract_fields
from app.recipes import Recipe, RecipeRegistry
from app.recipes import parse_params, resolve_recipe

FIXTURES = Path(__file__).resolve().parent / "fixtures"


class ExtractTests(unittest.TestCase):
    def setUp(self) -> None:
        self.html = (FIXTURES / "detail.html").read_text(encoding="utf-8")

    def test_extract_article_keeps_title_and_body(self) -> None:
        title, text, markdown, warnings = extract_article(self.html)
        self.assertIn("智慧园区", title)
        self.assertIn("预算金额", text)
        self.assertTrue(markdown)
        # 正文可抽取时不应出现「正文过短」告警
        self.assertNotIn("正文过短，可能是空壳页面或需要登录", warnings)

    def test_extract_fields_from_rules(self) -> None:
        _, text, _, _ = extract_article(self.html)
        fields = extract_fields(text)
        self.assertEqual(fields.get("project_code"), "ZB-2026-0912")
        self.assertEqual(fields.get("publisher_name"), "某某市大数据管理局")
        self.assertEqual(fields.get("agency_name"), "某某招标代理有限公司")
        self.assertIn("1,234.5", fields.get("budget_text", ""))
        self.assertEqual(fields.get("region_text"), "广东省")
        self.assertTrue(fields.get("deadline_text", "").startswith("2026-10-12"))

    def test_extract_attachments_records_links_only(self) -> None:
        attachments = extract_attachments(self.html, "https://example.com/notice/1.html")
        urls = [item.url for item in attachments]
        self.assertIn("https://example.com/files/zbwj.pdf", urls)

    def test_extract_fields_rejects_noisy_captures(self) -> None:
        """规则容易误伤的正文片段必须被拒收，交给后端 LLM 兜底。"""
        noisy = (
            "采购人需求说明：本项目  采购代理机构通过“信用中国”网站查询"
            "开标时间后，资格审查时采购代理机构通过信用中国网站查询相关主体"
        )
        fields = extract_fields(noisy)
        self.assertNotIn("publisher_name", fields)
        self.assertNotIn("agency_name", fields)
        self.assertNotIn("deadline_text", fields)

    def test_publish_date_prefers_page_header_label(self) -> None:
        """页头「公告时间」在正文容器之外，必须能从整页文本里取到。"""
        body = "项目概况 某某项目招标项目的潜在投标人应获取招标文件，并于2026年10月12日 09点30分前递交投标文件。"
        page = "中国政府采购网 公告时间 2026年09月21日 18:54 " + body
        fields = extract_fields(body, page_text=page)
        self.assertEqual(fields.get("publish_date"), "2026-09-21")

    def test_publish_date_fallback_skips_deadline_context(self) -> None:
        """正文只出现投标截止时间时，宁可不写发布时间，也不能写成截止时间。"""
        body = "项目概况 投标人应获取招标文件，并于2026年10月12日 09点30分（北京时间）前递交投标文件。"
        fields = extract_fields(body)
        self.assertNotIn("publish_date", fields)

    def test_publish_date_fallback_accepts_plain_leading_date(self) -> None:
        fields = extract_fields("2026-09-21 某某单位智慧园区平台建设项目公开招标公告 一、项目基本情况")
        self.assertEqual(fields.get("publish_date"), "2026-09-21")

    def test_notice_type_text_only_from_title(self) -> None:
        from app.extract import _looks_like_notice_type

        self.assertTrue(_looks_like_notice_type("某某项目公开招标公告"))
        self.assertFalse(_looks_like_notice_type("河南省气象探测数据中心"))

    def test_extract_body_html_preserves_semantic_structure_and_safe_attributes(self) -> None:
        """删除清洗逻辑或误删文档属性时，本测试必须失败。"""
        html = (FIXTURES / "detail_structured.html").read_text(encoding="utf-8")
        body_html = extract_body_html(
            html,
            "https://example.com/notices/42/index.html",
        )
        self.assertIn("<h2>一、项目概况</h2>", body_html)
        self.assertIn("<h2>二、申请人的资格要求</h2>", body_html)
        self.assertNotIn("<p></p>", body_html)
        self.assertIn('<ol start="3" type="1">', body_html)
        self.assertIn("<ul>", body_html)
        self.assertIn('colspan="2"', body_html)
        self.assertIn('rowspan="2"', body_html)
        self.assertIn('href="https://example.com/files/spec.pdf"', body_html)

    def test_extract_body_html_removes_executable_and_noise_content(self) -> None:
        """放宽标签、属性或噪声白名单时，本测试必须失败。"""
        html = (FIXTURES / "detail_structured.html").read_text(encoding="utf-8")
        body_html = extract_body_html(
            html,
            "https://example.com/notices/42/index.html",
        )
        for forbidden in (
            "<script",
            "<iframe",
            "onclick=",
            "style=",
            "javascript:",
            "购买广告",
            "分享本文",
            "站点导航",
            "站点页脚",
        ):
            self.assertNotIn(forbidden, body_html)

    def test_extract_body_html_returns_empty_without_trusted_container(self) -> None:
        """正文容器不可信时必须降级，不能把整页外壳当正文保存。"""
        html = "<html><body><nav>导航</nav><div>短内容</div><footer>页脚</footer></body></html>"
        self.assertEqual(extract_body_html(html, "https://example.com"), "")


class DiscoverTests(unittest.TestCase):
    def setUp(self) -> None:
        self.html = (FIXTURES / "list.html").read_text(encoding="utf-8")

    def test_generic_link_discovery_filters_noise(self) -> None:
        recipe = Recipe(source_key="demo", link_pattern=r"/\d{6}/t\d{8}_\d+\.htm")
        items, stop = _extract_links(recipe, "https://example.com/list/", self.html, "", "", set())
        self.assertFalse(stop)
        self.assertEqual(len(items), 3)
        self.assertTrue(all("t20260921" in item.url or "t20260920" in item.url or "t20260919" in item.url for item in items))

    def test_cursor_url_stops_at_seen_item(self) -> None:
        recipe = Recipe(source_key="demo", link_pattern=r"/\d{6}/t\d{8}_\d+\.htm")
        seen_url = "https://example.com/list/gkzb/202609/t20260920_10000002.htm"
        items, stop = _extract_links(
            recipe,
            "https://example.com/list/",
            self.html,
            "url",
            seen_url,
            set(),
        )
        self.assertTrue(stop)
        self.assertEqual(len(items), 1)

    def test_cursor_round_trip(self) -> None:
        self.assertEqual(_parse_cursor("date:2026-09-21"), ("date", "2026-09-21"))
        self.assertEqual(_parse_cursor("url:https://a.b/c"), ("url", "https://a.b/c"))
        self.assertEqual(_parse_cursor(""), ("", ""))
        from app.schemas import DiscoveredItem

        cursor = _build_cursor([DiscoveredItem(url="https://a.b/c", publish_date="2026-09-21")], "")
        self.assertEqual(cursor, "date:2026-09-21")
        cursor = _build_cursor([DiscoveredItem(url="https://a.b/c")], "")
        self.assertEqual(cursor, "url:https://a.b/c")

    def test_links_outside_list_directory_are_dropped(self) -> None:
        """列表页里的导航/其它栏目链接不能当成公告详情链接。"""
        html = """
        <html><body>
          <nav><a href="/zcdt/202609/t20260915_27326320.htm">政策动态新闻</a></nav>
          <ul>
            <li><a href="./gkzb/202609/t20260921_10000001.htm">某某项目公开招标公告</a></li>
            <li><a href="./fblbgg/202609/t20260921_10000002.htm">某某项目废标公告</a></li>
          </ul>
        </body></html>
        """
        recipe = Recipe(source_key="demo", link_pattern=r"/\d{6}/t\d{8}_\d+\.htm")
        items, _ = _extract_links(recipe, "https://example.com/cggg/zygg/", html, "", "", set())
        self.assertEqual(len(items), 2)
        self.assertTrue(all("/cggg/zygg/" in item.url for item in items))


class RecipeTests(unittest.TestCase):
    def test_repo_recipes_load_and_cover_first_batch(self) -> None:
        registry = RecipeRegistry(Path(__file__).resolve().parent.parent / "recipes")
        self.assertGreaterEqual(len(registry), 12)
        expected = {
            "ccgp_central",
            "bj_ggzy",
            "sh_ggzy",
            "zj_ggzy",
            "national_ggzy",
            "sc_ggzy",
            "csg_bidding",
            "sgcc_ecp",
            "cmcc_b2b",
            "cdb_cg",
            "psbc_cg",
            "jiangnan_bidding",
        }
        self.assertTrue(expected.issubset({recipe.source_key for recipe in registry.all()}))

    def test_recipe_camel_case_keys_are_normalized(self) -> None:
        registry = RecipeRegistry(Path(__file__).resolve().parent.parent / "recipes")
        ccgp = registry.get("ccgp_central")
        self.assertIsNotNone(ccgp)
        assert ccgp is not None
        self.assertEqual(ccgp.discovery_mode, "list")
        self.assertFalse(ccgp.needs_browser)
        self.assertTrue(ccgp.link_pattern)

    def test_imported_source_falls_back_to_generic_recipe(self) -> None:
        """数据库导入的源没有代码 recipe 时，也要能跑通用规则。"""
        recipe = resolve_recipe(
            "demo_city_ggzy",
            "https://ggzy.example.gov.cn/jyxx/zbgg.html",
            needs_browser=True,
            discovery_mode="list",
            params='{"linkPattern": "/zbgg/\\\\d+", "maxItems": 20}',
        )
        self.assertEqual(recipe.source_key, "demo_city_ggzy")
        self.assertTrue(recipe.needs_browser)
        self.assertEqual(recipe.list_urls, ["https://ggzy.example.gov.cn/jyxx/zbgg.html"])
        # params 覆盖生效（camelCase 键会转成 snake_case）
        self.assertEqual(recipe.link_pattern, "/zbgg/\\d+")
        self.assertEqual(recipe.max_items, 20)

    def test_code_recipe_wins_over_generic_but_params_can_override(self) -> None:
        recipe = resolve_recipe(
            "ccgp_central",
            "http://www.ccgp.gov.cn/cggg/zygg/",
            params='{"maxItems": 5}',
        )
        # 代码 recipe 提供精确规则
        self.assertTrue(recipe.link_pattern)
        # params 可覆盖单个字段
        self.assertEqual(recipe.max_items, 5)

    def test_parse_params_ignores_invalid_json(self) -> None:
        self.assertEqual(parse_params(""), {})
        self.assertEqual(parse_params("not-json"), {})
        self.assertEqual(parse_params("[1,2]"), {})
        self.assertEqual(parse_params('{"maxItems": 3}'), {"maxItems": 3})


if __name__ == "__main__":
    unittest.main()
