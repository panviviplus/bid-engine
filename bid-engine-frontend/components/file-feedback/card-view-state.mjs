/**
 * Keep already-rendered cards visible during background refreshes while still
 * distinguishing the initial loading, error, and empty states.
 *
 * @param {{ loading: boolean; error: unknown; itemCount: number }} input
 * @returns {"loading" | "error" | "empty" | "ready"}
 */
export function resolveFeedbackCardViewState({ loading, error, itemCount }) {
  if (Math.max(0, Number(itemCount) || 0) > 0) return "ready";
  if (loading) return "loading";
  if (error) return "error";
  return "empty";
}
