"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Flex, Box, Spinner, Text } from "@chakra-ui/react";
import { env } from "next-runtime-env";

type Props = {
  src?: string;
  title?: string;
  attachmentPath?: string;
  postUrl?: string;
};

export default function PdfPreview({
  src,
  title,
  attachmentPath,
  postUrl,
}: Props) {
  const [blobUrl, setBlobUrl] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string>("");
  const containerRef = useRef<HTMLDivElement>(null);
  const [containerWidth, setContainerWidth] = useState<number>(800);
  const [containerHeight, setContainerHeight] = useState<number>(600);

  useEffect(() => {
    let mounted = true;
    const fetchBlob = async () => {
      setLoading(true);
      setError("");
      try {
        if (attachmentPath) {
          const res = await fetch(
            postUrl || `${env("NEXT_PUBLIC_SYSTEM_SERVER")}/material/preview`,
            {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              credentials: "include",
              body: JSON.stringify({ attachmentPath }),
            },
          );
          if (!res.ok) throw new Error("预览接口返回异常");
          const blob = await res.blob();
          const url = URL.createObjectURL(blob);
          if (mounted) setBlobUrl(url);
        } else if (src) {
          const res = await fetch(src, { credentials: "include" });
          if (!res.ok) throw new Error("预览接口返回异常");
          const blob = await res.blob();
          const url = URL.createObjectURL(blob);
          if (mounted) setBlobUrl(url);
        } else {
          throw new Error("缺少预览参数");
        }
      } catch (e: any) {
        if (mounted) setError(e?.message || "预览失败");
      } finally {
        if (mounted) setLoading(false);
      }
    };
    fetchBlob();
    return () => {
      mounted = false;
      if (blobUrl) URL.revokeObjectURL(blobUrl);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [src, attachmentPath, postUrl]);

  const headerTitle = useMemo(() => {
    if (title) return title;
    if (attachmentPath) return attachmentPath.split("/").pop() || "PDF预览";
    try {
      const u = new URL(src || "");
      return u.pathname.split("/").pop() || "PDF预览";
    } catch {
      return "PDF预览";
    }
  }, [src, title, attachmentPath]);

  useEffect(() => {
    const measure = () => {
      const w = containerRef.current?.clientWidth || window.innerWidth;
      const h = containerRef.current?.clientHeight || window.innerHeight;
      setContainerWidth(Math.max(320, Math.floor(w - 48)));
      setContainerHeight(Math.max(240, Math.floor(h - 24)));
    };
    measure();
    window.addEventListener("resize", measure);
    return () => window.removeEventListener("resize", measure);
  }, []);

  return (
    <Flex direction="column" h="100dvh" w="full" minW={0} overflow="hidden">
      <Flex align="center" px={4} py={2} borderBottom="1px solid #eee">
        <Text fontSize="sm" color="#4c4c4cff">
          {headerTitle}
        </Text>
      </Flex>

      <Box ref={containerRef} flex="1" overflow="auto" bg="#f7f7f7">
        {loading && (
          <Flex h="full" align="center" justify="center">
            <Spinner
              thickness="4px"
              speed="0.65s"
              emptyColor="gray.200"
              color="blue.500"
              size="xl"
            />
          </Flex>
        )}
        {!loading && error && (
          <Flex h="full" align="center" justify="center">
            <Text color="red.500" fontSize="sm">
              {error}
            </Text>
          </Flex>
        )}
        {!loading && !error && blobUrl && (
          <Flex align="flex-start" justify="center" py={4}>
            <Box
              bg="#fff"
              border="1px solid #eaeaea"
              pos="relative"
              w={containerWidth}
              h={`${Math.max(320, containerHeight - 32)}px`}
            >
              <object
                data={blobUrl}
                type="application/pdf"
                width="100%"
                height="100%"
              >
                <iframe
                  src={blobUrl}
                  width="100%"
                  height="100%"
                  title="pdf-preview-frame"
                  sandbox="allow-scripts allow-downloads"
                  referrerPolicy="no-referrer"
                  style={{ border: "none" }}
                />
              </object>
            </Box>
          </Flex>
        )}
      </Box>
    </Flex>
  );
}
