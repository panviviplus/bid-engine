import React from "react";
import { Flex, Button } from "@chakra-ui/react";
import useAxios from "axios-hooks";
import { useCustomToast } from "@/hooks/useCustomToast";
import { useRouter, useSearchParams } from "next/navigation";

export default function SmsLogin() {
  const searchParams = useSearchParams();
  const code = searchParams.get("agentAuthCode");
  const router = useRouter();
  const showToast = useCustomToast();

  // 登录
  const [{ loading: isLoginLoading }, login] = useAxios(
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

  return (
    <Flex m="auto" align="center" direction="column" py={{ base: 6, md: 9 }}>
      <Button
        type="submit"
        w="full"
        fontSize="16px"
        fontWeight="normal"
        mt={5}
        h="54px"
        layerStyle="primarybutton"
        colorScheme="primary"
        isLoading={isLoginLoading}
        onClick={handleLogin}
      >
        登录
      </Button>
    </Flex>
  );
}
