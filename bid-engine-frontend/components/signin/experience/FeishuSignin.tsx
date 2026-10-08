"use client";

import { Box, Button, Flex, Text } from "@chakra-ui/react";
import useAxios from "axios-hooks";
import { QRCodeSVG } from "qrcode.react";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";

type Challenge = { code: string; command: string; bot_url: string; expires_at: number };

export default function FeishuSignin() {
  const router = useRouter();
  const [challenge, setChallenge] = useState<Challenge | null>(null);
  const [status, setStatus] = useState("idle");
  const [error, setError] = useState("");
  const completing = useRef(false);
  const [, create] = useAxios({ url: "/feishu/login/challenge", method: "POST", withCredentials: true }, { manual: true });
  const [, readStatus] = useAxios({ url: "/feishu/login/challenge", method: "GET", withCredentials: true }, { manual: true });
  const [, complete] = useAxios({ url: "/feishu/login/complete", method: "POST", withCredentials: true }, { manual: true });

  const begin = async () => {
    setError("");
    setStatus("loading");
    completing.current = false;
    try {
      const res = await create();
      if (res.data?.code !== 0 || !res.data?.data) throw new Error(res.data?.message || "获取一次性码失败");
      setChallenge(res.data.data);
      setStatus("waiting");
    } catch (err: any) {
      setStatus("error");
      setError(err?.message || "飞书登录暂不可用");
    }
  };

  useEffect(() => {
    if (status !== "waiting" || !challenge) return;
    let active = true;
    let inFlight = false;
    const poll = async () => {
      if (!active || inFlight || completing.current) return;
      if (Date.now() >= challenge.expires_at * 1000) { setStatus("expired"); return; }
      inFlight = true;
      try {
        const res = await readStatus();
        if (!active) return;
        const next = res.data?.data?.status;
        if (next === "expired") { setStatus("expired"); return; }
        if (next !== "confirmed") return;
        completing.current = true;
        setStatus("confirming");
        const done = await complete();
        if (done.data?.code !== 0) throw new Error(done.data?.message || "登录失败");
        router.replace("/");
      } catch (err: any) {
        if (active && completing.current) { setError(err?.message || "登录失败，请重新获取一次性码"); setStatus("error"); }
      } finally { inFlight = false; }
    };
    const timer = window.setInterval(poll, 2000);
    void poll();
    return () => { active = false; window.clearInterval(timer); };
  }, [challenge, complete, readStatus, router, status]);

  return (
    <Box mt={3} pt={4} borderTop="1px solid" borderColor="neutral.200">
      <Flex align="center" justify="space-between" gap={3}>
        <Box>
          <Text fontSize="sm" fontWeight="700" color="neutral.700">飞书扫码登录</Text>
          <Text fontSize="xs" color="neutral.500">扫码打开机器人，向它发送页面上的一次性码</Text>
        </Box>
        {(!challenge || status === "expired" || status === "error") && (
          <Button size="sm" colorScheme="primary" isLoading={status === "loading"} onClick={begin}>
            {challenge ? "重新获取" : "开始扫码"}
          </Button>
        )}
      </Flex>
      {challenge && (status === "waiting" || status === "confirming") && (
        <Flex mt={4} gap={4} align="center" direction={{ base: "column", sm: "row" }}>
          <Box p={2} bg="white" border="1px solid" borderColor="neutral.200" borderRadius="lg" aria-label="飞书机器人二维码">
            <QRCodeSVG value={challenge.bot_url} size={124} />
          </Box>
          <Box flex={1} minW={0}>
            <Text fontSize="xs" color="neutral.500">发送给机器人</Text>
            <Text fontSize="xl" fontWeight="800" letterSpacing="0.08em" color="primary.700" userSelect="all">
              {challenge.command}
            </Text>
            <Text mt={2} fontSize="xs" color="neutral.500">
              {status === "confirming" ? "已收到飞书确认，正在登录…" : "5 分钟内有效。首次使用需等待机器人所有者审核；审核后请重新获取码。"}
            </Text>
          </Box>
        </Flex>
      )}
      {status === "expired" && <Text mt={3} fontSize="sm" color="orange.600">一次性码已过期，请重新获取。</Text>}
      {error && <Text mt={3} fontSize="sm" color="red.600" role="alert">{error}</Text>}
    </Box>
  );
}
