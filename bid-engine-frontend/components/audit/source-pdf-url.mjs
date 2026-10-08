export function buildReviewSourcePdfUrl(projectId, fileId) {
  return `/api/zb/review/source-pdf?project_id=${projectId}&file_id=${fileId}`;
}
