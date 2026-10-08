"""请求模型容错测试：调用方把空值序列化成 null 时不应 422。"""

from __future__ import annotations

import unittest

from pydantic import ValidationError

from app.schemas import CollectRequest, DiscoverRequest, ExtractRequest, NoticeDocument


class RequestToleranceTests(unittest.TestCase):
    def test_discover_accepts_null_keywords(self) -> None:
        req = DiscoverRequest.model_validate(
            {
                "sourceKey": "demo",
                "listUrl": "https://example.com/zbgg",
                "keywords": None,
            }
        )
        self.assertEqual(req.keywords, [])

    def test_collect_accepts_null_keywords_and_null_params(self) -> None:
        req = CollectRequest.model_validate(
            {
                "sourceKey": "demo",
                "listUrl": "https://example.com/zbgg",
                "keywords": None,
                "params": None,
                "cursor": None,
            }
        )
        self.assertEqual(req.keywords, [])
        self.assertEqual(req.params, "")
        self.assertEqual(req.cursor, "")

    def test_keywords_still_validated_when_provided(self) -> None:
        req = CollectRequest.model_validate(
            {"sourceKey": "demo", "keywords": ["信息化", "平台"]}
        )
        self.assertEqual(req.keywords, ["信息化", "平台"])
        with self.assertRaises(ValidationError):
            CollectRequest.model_validate({"sourceKey": "demo", "keywords": "信息化"})

    def test_extract_accepts_null_source_key(self) -> None:
        req = ExtractRequest.model_validate(
            {"url": "https://example.com/a.html", "sourceKey": None}
        )
        self.assertEqual(req.source_key, "")

    def test_notice_document_serializes_body_html_as_camel_case(self) -> None:
        """删除跨服务字段或破坏 camelCase 别名时，本测试必须失败。"""
        payload = NoticeDocument(body_html="<p>正文</p>").model_dump(by_alias=True)
        self.assertEqual(payload["bodyHtml"], "<p>正文</p>")


if __name__ == "__main__":
    unittest.main()
