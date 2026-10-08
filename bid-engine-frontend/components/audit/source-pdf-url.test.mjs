import assert from "node:assert/strict";
import test from "node:test";

import { buildReviewSourcePdfUrl } from "./source-pdf-url.mjs";

test("builds the review PDF URL through the same-origin API proxy", () => {
  assert.equal(
    buildReviewSourcePdfUrl("12", 34),
    "/api/zb/review/source-pdf?project_id=12&file_id=34",
  );
});
