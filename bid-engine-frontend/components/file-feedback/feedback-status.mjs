/**
 * The backend's only processed state is numeric 1. Every other value is an
 * unprocessed/new feedback state and must remain labelled “待处理”.
 *
 * @param {unknown} status
 * @returns {{ text: "待处理" | "已处理"; scheme: "orange" | "green"; processed: boolean }}
 */
export function resolveFeedbackStatusMeta(status) {
  const processed = Number(status) === 1;
  return processed
    ? { text: "已处理", scheme: "green", processed: true }
    : { text: "待处理", scheme: "orange", processed: false };
}
