import React, { useEffect, useMemo, useState } from "react";
import {
  Box,
  Flex,
  Button,
  Input,
  InputGroup,
  InputRightElement,
  FormControl,
  FormErrorMessage,
  Text,
} from "@chakra-ui/react";
import { Field, Form, Formik } from "formik";
import * as Yup from "yup";
import useAxios from "axios-hooks";
import { useCustomToast } from "@/hooks/useCustomToast";
import { useRouter } from "next/navigation";
import { getSmsAuthConfig } from "@/components/signin/experience/feature-flags.mjs";

export default function SmsSignin({ unifiedAuthEnabled = false }) {
  const router = useRouter();
  const showToast = useCustomToast();
  const [countdown, setCountdown] = useState(0);
  const authConfig = useMemo(
    () => getSmsAuthConfig(unifiedAuthEnabled),
    [unifiedAuthEnabled],
  );

  const ValidationSchema = useMemo(
    () =>
      Yup.object().shape({
        code: Yup.string().trim().required("必填"),
        mobile: Yup.string().trim().required("必填"),
      }),
    [],
  );

  const [{ loading: sending }, sendSms] = useAxios(
    {
      method: "POST",
      url: "/sms/send",
      headers: { "Content-Type": "application/json" },
    },
    { manual: true },
  );

  const [{ loading: isLoginLoading }, login] = useAxios(
    {
      method: "POST",
      url: authConfig.endpoint,
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

  const handleSendCode = async (mobile) => {
    const tel = String(mobile || "").trim();
    if (!tel) {
      showToast({ title: "请先输入手机号" });
      return;
    }
    try {
      const res = await sendSms({
        data: { mobile: tel, scene: authConfig.scene },
      });
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

  const handleLogin = async (values) => {
    try {
      const data = {
        mobile: values.mobile,
        sms_code: values.code,
      };
      if (authConfig.includeLoginType) {
        data.login_type = "sms";
      }
      const res = await login({
        data,
      });
      if (res.data?.code !== 0) {
        showToast({
          id: "loginId",
          title: res.data?.message || "服务异常，请稍后再试。",
        });
        return;
      }
      router.replace("/");
    } catch (error) {
      showToast({ title: "服务异常，请稍后再试。" });
    }
  };

  return (
    <Flex m="auto" align="center" direction="column" py={5}>
      {authConfig.helper && (
        <Text
          w="full"
          mb={4}
          color="neutral.600"
          fontSize="sm"
          lineHeight="1.7"
        >
          {authConfig.helper}
        </Text>
      )}
      <Formik
        initialValues={{ mobile: "", code: "" }}
        validationSchema={ValidationSchema}
        onSubmit={handleLogin}
      >
        {({ values }) => (
          <Form style={{ width: "100%", margin: 0 }}>
            <Field name="mobile">
              {({ field, form }) => (
                <FormControl
                  isInvalid={form.errors.mobile && form.touched.mobile}
                >
                  <InputGroup size="lg">
                    <Input
                      {...field}
                      placeholder="请输入手机号"
                      id="mobile"
                      type="tel"
                      autoComplete="tel"
                      focusBorderColor="primary.600"
                    />
                    <InputRightElement w="auto" pr={2}>
                      <Button
                        size="sm"
                        variant="ghost"
                        colorScheme="primary"
                        isLoading={sending}
                        isDisabled={countdown > 0}
                        onClick={() => handleSendCode(values.mobile)}
                      >
                        {countdown > 0 ? `${countdown}s` : "发送验证码"}
                      </Button>
                    </InputRightElement>
                  </InputGroup>
                  <Box display="flex" mt={2}>
                    &nbsp;
                    <FormErrorMessage m={0}>
                      {form.errors.mobile}
                    </FormErrorMessage>
                  </Box>
                </FormControl>
              )}
            </Field>

            <Field name="code">
              {({ field, form }) => (
                <FormControl isInvalid={form.errors.code && form.touched.code}>
                  <InputGroup size="lg">
                    <Input
                      {...field}
                      id="code"
                      autoComplete="one-time-code"
                      focusBorderColor="primary.600"
                      placeholder="请输入验证码"
                    />
                  </InputGroup>
                  <Box display="flex" mt={2}>
                    &nbsp;
                    <FormErrorMessage m={0}>
                      {form.errors.code}
                    </FormErrorMessage>
                  </Box>
                </FormControl>
              )}
            </Field>

            <Button
              type="submit"
              w="full"
              fontSize="16px"
              fontWeight="normal"
              mt={5}
              h="54px"
              position="relative"
              layerStyle="primarybutton"
              colorScheme="primary"
              isLoading={isLoginLoading}
            >
              <Text as="span">{authConfig.submitLabel}</Text>
              <Text
                as="span"
                position="absolute"
                right={3}
                bottom={1.5}
                color="whiteAlpha.800"
                fontSize="10px"
                fontWeight="500"
                lineHeight="1"
                pointerEvents="none"
                whiteSpace="nowrap"
              >
                {authConfig.submitNote}
              </Text>
            </Button>
          </Form>
        )}
      </Formik>
    </Flex>
  );
}
