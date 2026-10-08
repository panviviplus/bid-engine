"use client";

import { SWRConfig } from "swr";
import { AppContextProvider } from "@/contexts/app-context";
import axios from "axios";
import useAxios from "axios-hooks";
import { env } from "next-runtime-env";
import { useEffect } from "react";
import ChakraProvider from "./chakra";

export default function Providers({ children }) {
  (useAxios as any).configure({
    axios: axios.create({
      withCredentials: true,
      baseURL: "/api",
    }),
    // 关闭 SSR 端请求：axios-hooks 默认 ssr:true 会在服务端渲染时同步发起相对 URL 请求，
    // Node 端 new URL('/api/...') 抛 ERR_INVALID_URL 导致全量加载 500。改为客户端 useEffect 发起。
    defaultOptions: { ssr: false },
  });
  useEffect(() => {
    const handler = (e: any) => {
      const msg = e?.message || e?.error?.message || "";
      const name = e?.error?.name || "";
      const reason = e?.reason?.message || e?.reason || "";
      const text = `${msg} ${name} ${reason}`;
      if (
        !sessionStorage.getItem("_ck_fixed") &&
        /chunk.*failed|script.*failed|ChunkLoadError|Failed to fetch dynamically imported module|Importing a module script failed/i.test(
          text,
        )
      ) {
        sessionStorage.setItem("_ck_fixed", "1");
        window.location.reload();
      }
    };
    const onUnhandled = (e: any) => handler(e);
    window.addEventListener("error", handler);
    window.addEventListener("unhandledrejection", onUnhandled);
    return () => {
      window.removeEventListener("error", handler);
      window.removeEventListener("unhandledrejection", onUnhandled);
    };
  }, []);
  const swrConfigValue = {
    revalidateOnFocus: false,
    fetcher: (resource) => {
      return fetch(resource, { credentials: "include" }).then((res) =>
        res.json(),
      );
    },
  };
  return (
    <SWRConfig value={swrConfigValue}>
      <AppContextProvider>
        <ChakraProvider>{children}</ChakraProvider>
      </AppContextProvider>
    </SWRConfig>
  );
}
