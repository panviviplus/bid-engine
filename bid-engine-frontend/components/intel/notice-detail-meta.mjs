export function formatInsightMeta(createdAt) {
  const timestamp = String(createdAt || "").trim();
  return timestamp ? `${timestamp} · AI 结论仅供参考` : "AI 结论仅供参考";
}
