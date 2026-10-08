import { redirect } from "next/navigation";

/**
 * “提醒中心”已并入“订阅与提醒”。
 *
 * 提醒本来就是针对订阅项的提醒，合并后统一在订阅卡片的抽屉里查看；
 * 这里保留旧路径并重定向，避免历史链接、导航角标与浏览器书签 404。
 */
export default function IntelAlertsPage() {
  redirect("/intel/subscriptions");
}
