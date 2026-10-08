"use client";

import { useEffect, useMemo, useState } from "react";
import {
  Box,
  Button,
  Flex,
  Input,
  InputGroup,
  InputRightElement,
  Text,
} from "@chakra-ui/react";
import useAxios from "axios-hooks";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { useCustomToast } from "@/hooks/useCustomToast";
import AuthCanvas from "@/components/auth/AuthCanvas";
import { validateRegistrationPasswordPair } from "@/components/signin/experience/auth-card-state.mjs";

export default function SignupPage() {
  const router = useRouter();
  const showToast = useCustomToast();
  const [step, setStep] = useState(1);
  const [countdown, setCountdown] = useState(0);
  const [form, setForm] = useState({
    companyName: "",
    nickname: "",
    mobile: "",
    code: "",
    password: "",
    password2: "",
  });

  const [{ loading: sending }, sendSms] = useAxios(
    {
      method: "POST",
      url: "/sms/send",
      headers: { "Content-Type": "application/json" },
    },
    { manual: true },
  );

  const [{ loading: registering }, register] = useAxios(
    {
      method: "POST",
      url: "/register",
      headers: { "Content-Type": "application/json" },
      withCredentials: true,
    },
    { manual: true },
  );

  useEffect(() => {
    if (!countdown) return;
    const timer = setInterval(() => {
      setCountdown((prev) => (prev > 0 ? prev - 1 : 0));
    }, 1000);
    return () => clearInterval(timer);
  }, [countdown]);

  const canNext = useMemo(() => {
    return String(form.companyName || "").trim().length > 0;
  }, [form.companyName]);

  const handleSendCode = async () => {
    const mobile = String(form.mobile || "").trim();
    if (!mobile) {
      showToast({ title: "请先输入手机号" });
      return;
    }
    try {
      const res = await sendSms({ data: { mobile, scene: "register" } });
      if (res.data?.code !== 0) {
        showToast({ title: res.data?.message || "发送失败，请稍后再试。" });
        return;
      }
      setCountdown(60);
      const debugCode = res.data?.data?.debugCode;
      if (debugCode) {
        showToast({ title: `验证码：${debugCode}` });
      } else {
        showToast({ title: "验证码已发送" });
      }
    } catch (e) {
      showToast({ title: "发送失败，请稍后再试。" });
    }
  };

  const handleSubmit = async () => {
    const companyName = String(form.companyName || "").trim();
    const nickname = String(form.nickname || "").trim();
    const mobile = String(form.mobile || "").trim();
    const code = String(form.code || "").trim();
    const password = String(form.password || "").trim();
    const password2 = String(form.password2 || "").trim();

    if (!companyName) return showToast({ title: "请填写公司名称" });
    if (!nickname) return showToast({ title: "请填写姓名" });
    if (!mobile) return showToast({ title: "请填写手机号" });
    if (!code) return showToast({ title: "请填写验证码" });
    const passwordError = validateRegistrationPasswordPair(password, password2);
    if (passwordError) return showToast({ title: passwordError });

    try {
      const res = await register({
        data: {
          company_name: companyName,
          nickname,
          mobile,
          sms_code: code,
          password,
        },
      });
      if (res.data?.code !== 0) {
        showToast({ title: res.data?.message || "注册失败，请稍后再试。" });
        return;
      }
      router.replace("/");
    } catch (e) {
      showToast({ title: "注册失败，请稍后再试。" });
    }
  };

  return (
    <AuthCanvas formTitle="注册账号">
      <Flex direction="column" gap={1} mb={6}>
        <Text fontSize="sm" color="gray.600">
          使用手机号快速注册，先创建公司，再完善个人信息。
        </Text>
        <Text fontSize="sm" color="gray.600">
          已有账号？{" "}
          <Text
            as={Link}
            href="/signin"
            color="primary.600"
            fontWeight="600"
            display="inline"
          >
            去登录
          </Text>
        </Text>
      </Flex>

      {step === 1 ? (
        <Box>
          <Text fontSize="sm" color="gray.700" mb={2} fontWeight="700">
            公司信息
          </Text>
          <Input
            size="lg"
            placeholder="请输入公司名称"
            value={form.companyName}
            focusBorderColor="primary.600"
            onChange={(e) =>
              setForm((p) => ({ ...p, companyName: e.target.value }))
            }
          />
          <Button
            mt={6}
            w="full"
            h="54px"
            layerStyle="primarybutton"
            colorScheme="primary"
            isDisabled={!canNext}
            onClick={() => setStep(2)}
          >
            下一步
          </Button>
        </Box>
      ) : (
        <Box>
          <Text fontSize="sm" color="gray.700" mb={2} fontWeight="700">
            个人信息
          </Text>
          <Flex direction="column" gap={3}>
            <Input
              size="lg"
              placeholder="请输入姓名"
              value={form.nickname}
              focusBorderColor="primary.600"
              onChange={(e) =>
                setForm((p) => ({ ...p, nickname: e.target.value }))
              }
            />
            <Input
              size="lg"
              placeholder="请输入手机号"
              value={form.mobile}
              focusBorderColor="primary.600"
              onChange={(e) =>
                setForm((p) => ({ ...p, mobile: e.target.value }))
              }
            />
            <InputGroup size="lg">
              <Input
                placeholder="请输入验证码"
                value={form.code}
                focusBorderColor="primary.600"
                onChange={(e) =>
                  setForm((p) => ({ ...p, code: e.target.value }))
                }
              />
              <InputRightElement w="auto" pr={2}>
                <Button
                  size="sm"
                  variant="ghost"
                  colorScheme="primary"
                  isLoading={sending}
                  isDisabled={countdown > 0}
                  onClick={handleSendCode}
                >
                  {countdown > 0 ? `${countdown}s` : "发送验证码"}
                </Button>
              </InputRightElement>
            </InputGroup>
            <Input
              size="lg"
              type="password"
              placeholder="设置密码（必填）"
              value={form.password}
              focusBorderColor="primary.600"
              onChange={(e) =>
                setForm((p) => ({ ...p, password: e.target.value }))
              }
            />
            <Input
              size="lg"
              type="password"
              placeholder="确认密码"
              value={form.password2}
              focusBorderColor="primary.600"
              onChange={(e) =>
                setForm((p) => ({ ...p, password2: e.target.value }))
              }
            />
          </Flex>
          <Flex gap={3} mt={6}>
            <Button
              h="54px"
              w="full"
              variant="outline"
              onClick={() => setStep(1)}
            >
              上一步
            </Button>
            <Button
              h="54px"
              w="full"
              layerStyle="primarybutton"
              colorScheme="primary"
              isLoading={registering}
              onClick={handleSubmit}
            >
              注册并登录
            </Button>
          </Flex>
        </Box>
      )}

      <Text mt={8} textAlign="center" fontSize="xs" color="gray.500">
        © 标擎 · 让投标更智能
      </Text>
    </AuthCanvas>
  );
}
