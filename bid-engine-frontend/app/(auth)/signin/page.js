"use client";

import { Center, Spinner } from "@chakra-ui/react";
import { useEffect } from "react";
import { useCustomToast } from "@/hooks/useCustomToast";
import { useRouter, useSearchParams } from "next/navigation";
import useAxios from "axios-hooks";
import SigninExperience from "@/components/signin/experience/SigninExperience";

const notLogin = true;

function Signin() {
  const searchParams = useSearchParams();
  const code = searchParams.get("agentAuthCode");
  const showToast = useCustomToast();
  const router = useRouter();
  // 登录
  const [, login] = useAxios(
    {
      method: "GET",
      url: "/authLogin",
    },
    { manual: true },
  );
  const handleLogin = async () => {
    if (!code) return;
    try {
      const res = await login({
        params: {
          code,
        },
      });
      if (res.data.code !== 0) {
        showToast({
          id: "loginId",
          title: res.data.message || "服务异常，请稍后再试。",
        });
        return;
      }
      if (res.data.code === "403") {
        router.push("/403");
        return;
      }
      router.replace("/");
    } catch (error) {
      showToast({ title: "服务异常，请稍后再试。" });
    }
  };
  useEffect(() => {
    if (code) {
      handleLogin();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [code]);
  if (notLogin && code) {
    return (
      <Center flex={1} minH="100dvh" bg="signin.canvas">
        <Spinner color="gold.400" size="lg" />
      </Center>
    );
  }

  return <SigninExperience />;
}

export default Signin;
