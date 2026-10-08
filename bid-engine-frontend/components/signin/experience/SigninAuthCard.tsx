"use client";

import {
  Box,
  Button,
  Flex,
  Heading,
  Tab,
  TabList,
  TabPanel,
  TabPanels,
  Tabs,
  Text,
  useDisclosure,
} from "@chakra-ui/react";

import PasswordSignin from "@/components/signin/password-signin";
import SmsSignin from "@/components/signin/sms-signin";

import SignupModal from "./SignupModal";
import FeishuSignin from "./FeishuSignin";
import { getAuthAssistActions } from "./auth-card-state.mjs";

type SigninAuthCardProps = {
  tabIndex: number;
  // eslint-disable-next-line no-unused-vars
  onTabChange: (...args: [number]) => void;
  unifiedSmsAuthEnabled: boolean;
};

export default function SigninAuthCard({
  tabIndex,
  onTabChange,
  unifiedSmsAuthEnabled,
}: SigninAuthCardProps) {
  const signupModal = useDisclosure();
  const assistActions = getAuthAssistActions(tabIndex);

  const renderTabAction = (action: {
    kind: string;
    label: string;
    targetIndex?: number;
  }) => (
    <Button
      minH="44px"
      h="44px"
      px={0}
      variant="link"
      color="primary.600"
      fontSize="sm"
      fontWeight="700"
      whiteSpace="nowrap"
      onClick={() => onTabChange(action.targetIndex ?? 0)}
      _hover={{ color: "primary.700", textDecoration: "none" }}
      _active={{ color: "primary.800" }}
      _focusVisible={{
        outline: "none",
        boxShadow: "0 0 0 2px var(--chakra-colors-gold-400)",
        borderRadius: "md",
      }}
    >
      {action.label}
    </Button>
  );

  return (
    <Box
      w="full"
      maxW={{ base: "34rem", xl: "32rem" }}
      justifySelf={{ base: "stretch", md: "center", xl: "end" }}
      mr={{ base: 0, xl: 2 }}
      bg="signin.paper"
      color="neutral.900"
      borderRadius="24px"
      border="1px solid"
      borderColor="whiteAlpha.700"
      boxShadow="signinCard"
      p={{ base: 5, sm: 7 }}
    >
      <Flex align="center" gap={1} minW={0}>
        <Heading fontSize="xl" letterSpacing="-0.02em">
          登录标擎
        </Heading>
        <Box
          className="signin-auth-pill"
          position="relative"
          overflow="hidden"
          px={2.5}
          py={1}
          borderRadius="full"
          bg="primary.800"
          color="gold.200"
          border="1px solid"
          borderColor="gold.400"
          fontSize="12px"
          fontWeight="800"
          whiteSpace="nowrap"
          lineHeight="1.2"
        >
          <Text as="span" position="relative" zIndex={1}>
            智能投标工作台
          </Text>
        </Box>
      </Flex>

      <Box mt={5} minW={0}>
        <Tabs
          index={tabIndex}
          onChange={onTabChange}
          variant="unstyled"
          isFitted
        >
          <TabList bg="neutral.100" p={1} borderRadius="xl">
            {["验证码登录", "密码登录"].map((label) => (
              <Tab
                key={label}
                minH="44px"
                borderRadius="lg"
                fontSize="sm"
                fontWeight="700"
                color="neutral.500"
                whiteSpace="nowrap"
                _selected={{
                  bg: "white",
                  color: "primary.700",
                  boxShadow: "sm",
                }}
                _focusVisible={{
                  boxShadow: "0 0 0 2px var(--chakra-colors-gold-400)",
                }}
              >
                {label}
              </Tab>
            ))}
          </TabList>

          <TabPanels>
            <TabPanel p={0}>
              <SmsSignin unifiedAuthEnabled={unifiedSmsAuthEnabled} />
            </TabPanel>
            <TabPanel p={0}>
              <PasswordSignin />
            </TabPanel>
          </TabPanels>

          <Flex
            minH="44px"
            align="center"
            justify="space-between"
            gap={3}
            mt={2}
          >
            <Box flexShrink={0}>
              {assistActions.left && renderTabAction(assistActions.left)}
            </Box>
            {assistActions.right.kind === "signup" ? (
              <Flex ml="auto" align="center" gap={1} whiteSpace="nowrap">
                <Text color="neutral.500" fontSize="sm">
                  {assistActions.right.prompt}
                </Text>
                <Button
                  minH="44px"
                  h="44px"
                  px={0}
                  variant="link"
                  color="primary.600"
                  fontSize="sm"
                  fontWeight="700"
                  whiteSpace="nowrap"
                  onClick={signupModal.onOpen}
                  _hover={{ color: "primary.700", textDecoration: "none" }}
                  _active={{ color: "primary.800" }}
                  _focusVisible={{
                    outline: "none",
                    boxShadow: "0 0 0 2px var(--chakra-colors-gold-400)",
                    borderRadius: "md",
                  }}
                >
                  {assistActions.right.label}
                </Button>
              </Flex>
            ) : (
              <Box ml="auto">{renderTabAction(assistActions.right)}</Box>
            )}
          </Flex>
        </Tabs>
      </Box>

      <FeishuSignin />

      <Text mt={3} textAlign="center" color="neutral.400" fontSize="xs">
        登录或注册即表示同意服务条款与隐私政策
      </Text>

      <SignupModal isOpen={signupModal.isOpen} onClose={signupModal.onClose} />
    </Box>
  );
}
