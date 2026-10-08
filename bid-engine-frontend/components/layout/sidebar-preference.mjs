const STORAGE_KEY = "bid-engine.sidebar.collapsed";

function storageKeyFor(userId) {
  return userId ? `${STORAGE_KEY}.${userId}` : STORAGE_KEY;
}

/**
 * 读取用户侧栏折叠偏好。
 * 无记录或值不合法时返回 null（调用方按默认折叠处理）；SSR 下返回 null。
 */
export function readSidebarPreference(userId) {
  if (typeof window === "undefined") return null;
  const raw = window.localStorage.getItem(storageKeyFor(userId));
  if (raw === "true") return true;
  if (raw === "false") return false;
  return null;
}

/** 写入用户侧栏折叠偏好；SSR 下为 no-op。 */
export function writeSidebarPreference(userId, collapsed) {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(storageKeyFor(userId), String(collapsed));
}
