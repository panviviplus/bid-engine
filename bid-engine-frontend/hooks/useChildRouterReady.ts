/* eslint-disable no-unused-vars */
import React, { useEffect } from "react";

type VueRouter = { push: (url: string) => void };

export default function useChildRouterReady(
  iframeRef: React.RefObject<HTMLIFrameElement>,
  onReady: (router: VueRouter) => void,
) {
  useEffect(() => {
    const iframe = iframeRef.current;
    if (!iframe) return;
    if ((iframe.contentWindow as any)?.skipPage) {
      onReady((iframe.contentWindow as any)?.skipPage);
      return;
    }

    let rafId: number;

    const poll = () => {
      const router = (iframe.contentWindow as any)?.skipPage;
      if (router) {
        onReady(router);
        return;
      }
      rafId = requestAnimationFrame(poll);
    };

    const start = () => {
      if (iframe.contentDocument?.readyState === "complete") {
        poll();
      } else {
        iframe.addEventListener("load", () => poll(), { once: true });
      }
    };

    start();
    return () => cancelAnimationFrame(rafId);
  }, [iframeRef, onReady]);
}
