"use client";

import { Box, Button, Flex, FormControl, FormLabel, Input, Switch, Text } from "@chakra-ui/react";
import useAxios from "axios-hooks";
import { QRCodeSVG } from "qrcode.react";
import { useEffect, useRef, useState } from "react";

type State = {
  bound: boolean;
  notify_enabled: boolean;
  contact_mobile: string;
  company_display_name: string;
};
type Challenge = { command: string; bot_url: string; expires_at: number };

export default function FeishuBindingPanel({ isOpen, onStateChange }: {
  isOpen: boolean;
  onStateChange?: (bound: boolean) => void;
}) {
  const [account, setAccount] = useState<State | null>(null);
  const [challenge, setChallenge] = useState<Challenge | null>(null);
  const [status, setStatus] = useState("idle");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const completing = useRef(false);
  const [, getAccount] = useAxios({ url: "/user/feishu", method: "GET", withCredentials: true }, { manual: true });
  const [, create] = useAxios({ url: "/user/feishu/bind/challenge", method: "POST", withCredentials: true }, { manual: true });
  const [, getStatus] = useAxios({ url: "/user/feishu/bind/challenge", method: "GET", withCredentials: true }, { manual: true });
  const [, complete] = useAxios({ url: "/user/feishu/bind/complete", method: "POST", withCredentials: true }, { manual: true });
  const [, updateNotify] = useAxios({ url: "/user/feishu/notify", method: "PUT", withCredentials: true }, { manual: true });
  const [, updateProfile] = useAxios({ url: "/user/feishu/profile", method: "PUT", withCredentials: true }, { manual: true });
  const [, unbind] = useAxios({ url: "/user/feishu", method: "DELETE", withCredentials: true }, { manual: true });

  const refresh = async () => {
    const res = await getAccount();
    if (res.data?.code !== 0) throw new Error(res.data?.message || "读取飞书关联信息失败");
    const next = res.data.data as State;
    setAccount(next);
    onStateChange?.(next.bound);
  };

  useEffect(() => {
    if (!isOpen) return;
    void refresh().catch((err) => setMessage(err?.message || "读取飞书关联信息失败"));
    // Only refresh on modal opening; edits in this panel update local state.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen]);

  const begin = async () => {
    setMessage(""); setStatus("loading"); completing.current = false;
    try {
      const res = await create();
      if (res.data?.code !== 0) throw new Error(res.data?.message || "获取绑定码失败");
      setChallenge(res.data.data);
      setStatus("waiting");
    } catch (err: any) { setStatus("error"); setMessage(err?.message || "获取绑定码失败"); }
  };

  useEffect(() => {
    if (!isOpen || status !== "waiting" || !challenge) return;
    let active = true;
    let inFlight = false;
    const poll = async () => {
      if (!active || inFlight || completing.current) return;
      if (Date.now() >= challenge.expires_at * 1000) { setStatus("expired"); return; }
      inFlight = true;
      try {
        const res = await getStatus();
        if (!active) return;
        if (res.data?.data?.status === "expired") { setStatus("expired"); return; }
        if (res.data?.data?.status !== "confirmed") return;
        completing.current = true;
        setStatus("confirming");
        const done = await complete();
        if (done.data?.code !== 0) throw new Error(done.data?.message || "绑定失败");
        await refresh();
        setChallenge(null);
        setStatus("idle");
        setMessage("飞书账号已关联");
      } catch (err: any) {
        if (active && completing.current) { setStatus("error"); setMessage(err?.message || "绑定失败，请重新获取绑定码"); }
      } finally { inFlight = false; }
    };
    const timer = window.setInterval(poll, 2000);
    void poll();
    return () => { active = false; window.clearInterval(timer); };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen, challenge, status]);

  const saveContact = async () => {
    if (!account) return;
    setBusy(true); setMessage("");
    try {
      const res = await updateProfile({ data: {
        contact_mobile: account.contact_mobile,
        company_display_name: account.company_display_name,
      } });
      if (res.data?.code !== 0) throw new Error(res.data?.message || "保存失败");
      setMessage("联系资料已保存；联系电话尚未验证，不能用于登录");
    } catch (err: any) { setMessage(err?.message || "保存失败"); }
    finally { setBusy(false); }
  };

  const setNotifications = async (enabled: boolean) => {
    if (!account) return;
    setBusy(true); setMessage("");
    try {
      const res = await updateNotify({ data: { enabled } });
      if (res.data?.code !== 0) throw new Error(res.data?.message || "设置失败");
      setAccount({ ...account, notify_enabled: enabled });
    } catch (err: any) { setMessage(err?.message || "设置失败"); }
    finally { setBusy(false); }
  };

  const remove = async () => {
    setBusy(true); setMessage("");
    try {
      const res = await unbind();
      if (res.data?.code !== 0) throw new Error(res.data?.message || "解除关联失败");
      await refresh();
      setMessage("已解除飞书关联");
    } catch (err: any) { setMessage(err?.message || "解除关联失败"); }
    finally { setBusy(false); }
  };

  return (
    <Box borderTop="1px solid" borderColor="neutral.100" pt={5}>
      <Flex justify="space-between" align="center" gap={3}>
        <Box>
          <Text fontWeight="700" color="neutral.800">飞书账号</Text>
          <Text fontSize="xs" color="neutral.500">{account?.bound ? "已关联，可扫码登录并接收提醒" : "关联后可扫码登录并接收提醒"}</Text>
        </Box>
        {!account?.bound && <Button size="sm" colorScheme="primary" isLoading={status === "loading"} isDisabled={status === "waiting" || status === "confirming"} onClick={begin}>获取绑定码</Button>}
      </Flex>
      {!account?.bound && challenge && (status === "waiting" || status === "confirming") && (
        <Flex mt={4} gap={4} direction={{ base: "column", sm: "row" }} align="center">
          <Box p={2} bg="white" border="1px solid" borderColor="neutral.200" borderRadius="lg" aria-label="飞书机器人二维码">
            <QRCodeSVG value={challenge.bot_url} size={112} />
          </Box>
          <Box flex={1} minW={0}>
            <Text fontSize="xs" color="neutral.500">发送给机器人</Text>
            <Text fontSize="lg" fontWeight="800" letterSpacing="0.08em" color="primary.700" userSelect="all">{challenge.command}</Text>
            <Text fontSize="xs" color="neutral.500">{status === "confirming" ? "正在关联…" : "5 分钟内有效；首次使用需先通过机器人所有者审核。"}</Text>
          </Box>
        </Flex>
      )}
      {!account?.bound && (status === "expired" || status === "error") && <Button mt={3} size="sm" variant="outline" onClick={begin}>重新获取绑定码</Button>}
      {account?.bound && (
        <Box mt={4}>
          <Flex align="center" justify="space-between" gap={3} mb={4}>
            <Text fontSize="sm" color="neutral.700">接收招标情报提醒</Text>
            <Switch aria-label="接收招标情报提醒" colorScheme="primary" isChecked={account.notify_enabled} isDisabled={busy} onChange={(e) => void setNotifications(e.target.checked)} />
          </Flex>
          <FormControl mb={3}>
            <FormLabel fontSize="sm">公司名称（仅作资料展示）</FormLabel>
            <Input value={account.company_display_name} maxLength={255} onChange={(e) => setAccount({ ...account, company_display_name: e.target.value })} />
          </FormControl>
          <FormControl>
            <FormLabel fontSize="sm">联系电话（未验证，不能用于登录）</FormLabel>
            <Input value={account.contact_mobile} maxLength={32} onChange={(e) => setAccount({ ...account, contact_mobile: e.target.value })} />
          </FormControl>
          <Flex mt={3} justify="space-between" gap={3}>
            <Button size="sm" colorScheme="primary" isLoading={busy} onClick={saveContact}>保存联系资料</Button>
            <Button size="sm" variant="ghost" color="neutral.500" isDisabled={busy} onClick={remove}>解除关联</Button>
          </Flex>
        </Box>
      )}
      {message && <Text mt={3} fontSize="xs" color={status === "error" ? "red.600" : "neutral.600"} role="status">{message}</Text>}
    </Box>
  );
}
