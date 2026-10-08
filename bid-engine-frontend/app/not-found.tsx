"use client";

import { Button, Flex } from "@chakra-ui/react";
import NextLink from "next/link";
import { usePathname } from "next/navigation";
import Empty from "../components/common/empty";

const ERROR_MAP = [
  {
    key: "/403",
    type: "bid-forbidden",
    msg: "抱歉，您没有权限访问该页面 ╮(╯_╰)╭",
    isBack: false,
  },
  // { key: "/500", type: "error-500", msg: "服务器出小差了，请稍后再试 (╯﹏╰)" },
  // { key: "/502", type: "error-502", msg: "网关超时，请稍后再试 ⌛" },
] as const;

/* ---------- 默认 404 兜底 ---------- */
const DEFAULT_404 = {
  type: "error-404",
  msg: "对不起，这个页面好像找不到了 o(╥﹏╥)o",
  isBack: true,
};

export default function Custom404() {
  const pathname = usePathname();
  const hit = ERROR_MAP.find((e) => pathname.startsWith(e.key));
  const { type, msg, isBack } = hit ?? DEFAULT_404;
  return (
    <Flex flex={1} align="center" justify="center">
      <Empty type={type} message={msg}>
        {isBack && (
          <NextLink href="/" passHref>
            <Button
              colorScheme="white"
              w="112px"
              mt={5}
              fontSize="16px"
              color="primary.600"
              backgroundColor="neutral.800"
              borderWidth="1px"
              borderColor="primary.600"
              _hover={{
                backgroundColor: "neutral.50",
              }}
            >
              返回主页
            </Button>
          </NextLink>
        )}
      </Empty>
    </Flex>
  );
}
