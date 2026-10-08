import { useRouter } from "next/navigation";
import { useEffect } from "react";

/**
 * 监听子页面（iframe）通过 postMessage 发来的路由跳转指令
 * 子页面只需要：
 *   window.parent.postMessage({ type: 'CHILD_NAVIGATE', payload: '/xxx' }, origin)
 */
export default function useChildRouterBridge() {
  const router = useRouter();

  useEffect(() => {
    const handler = (e: MessageEvent) => {
      // 安全校验：只接受同源消息
      if (e.origin !== window.location.origin) return;
      if (e.data?.type === "CHILD_NAVIGATE") {
        router.push(e.data.payload); // 支持 string 或对象 { pathname, query }
      }
    };

    window.addEventListener("message", handler);
    return () => window.removeEventListener("message", handler);
  }, [router]);
}
