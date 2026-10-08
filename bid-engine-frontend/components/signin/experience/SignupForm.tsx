"use client";

import {
  Box,
  Button,
  Flex,
  FormControl,
  FormErrorMessage,
  FormLabel,
  Input,
  InputGroup,
  InputRightElement,
  Text,
} from "@chakra-ui/react";
import useAxios from "axios-hooks";
import { Field, Form, Formik, type FieldProps } from "formik";
import { useRouter } from "next/navigation";
import { type RefObject, useEffect, useState } from "react";

import { useCustomToast } from "@/hooks/useCustomToast";

import {
  buildRegisterPayload,
  validateRegistrationFields,
} from "./auth-card-state.mjs";

type SignupFormProps = {
  onClose: () => void;
  initialFocusRef: RefObject<HTMLInputElement>;
};

type SignupValues = {
  nickname: string;
  companyName: string;
  mobile: string;
  code: string;
  password: string;
};

const initialValues: SignupValues = {
  nickname: "",
  companyName: "",
  mobile: "",
  code: "",
  password: "",
};

function validate(values: SignupValues) {
  const errors = validateRegistrationFields(values) as Partial<
    Record<keyof SignupValues, string>
  >;
  const password = values.password.trim();
  if (
    password &&
    !/^[A-Za-z0-9!@#$%^&*()_+\-=[\]{}|;':",./<>?]{8,128}$/.test(password)
  ) {
    errors.password = "请输入 8–128 位数字、字母或英文标点";
  } else if (password) {
    const groups = [
      /[A-Z]/.test(password),
      /[a-z]/.test(password),
      /[0-9]/.test(password),
      /[^A-Za-z0-9]/.test(password),
    ].filter(Boolean).length;
    if (groups < 3) {
      errors.password = "请至少组合大写字母、小写字母、数字或符号中的三类";
    }
  }
  return errors;
}

export default function SignupForm({
  onClose,
  initialFocusRef,
}: SignupFormProps) {
  const router = useRouter();
  const showToast = useCustomToast();
  const [countdown, setCountdown] = useState(0);
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
    if (!countdown) return undefined;
    const timer = window.setInterval(
      () => setCountdown((value) => (value > 0 ? value - 1 : 0)),
      1_000,
    );
    return () => window.clearInterval(timer);
  }, [countdown]);

  const handleSendCode = async (mobileValue: string) => {
    const mobile = mobileValue.trim();
    if (!mobile) {
      showToast({ title: "请先输入手机号" });
      return;
    }
    try {
      const response = await sendSms({
        data: { mobile, scene: "register" },
      });
      if (response.data?.code !== 0) {
        showToast({
          title: response.data?.message || "验证码发送失败，请稍后再试。",
        });
        return;
      }
      setCountdown(60);
      const debugCode = response.data?.data?.debugCode;
      showToast({
        status: "success",
        title: debugCode ? `验证码：${debugCode}` : "验证码已发送",
      });
    } catch {
      showToast({ title: "验证码发送失败，请稍后再试。" });
    }
  };

  const handleSubmit = async (values: SignupValues) => {
    try {
      const response = await register({ data: buildRegisterPayload(values) });
      if (response.data?.code !== 0) {
        showToast({
          id: "registerId",
          title: response.data?.message || "注册失败，请检查后重试。",
        });
        return;
      }
      router.replace("/");
    } catch {
      showToast({ title: "注册失败，请稍后再试。" });
    }
  };

  return (
    <Formik
      initialValues={initialValues}
      validate={validate}
      onSubmit={handleSubmit}
    >
      {({ values }) => (
        <Form style={{ width: "100%", margin: 0 }}>
          <Flex direction="column" gap={3}>
            <Field name="nickname">
              {({ field, form }: FieldProps<string, SignupValues>) => (
                <FormControl
                  isInvalid={Boolean(
                    form.errors.nickname && form.touched.nickname,
                  )}
                >
                  <FormLabel mb={1.5} fontSize="sm" fontWeight="700">
                    用户名{" "}
                    <Text as="span" color="neutral.400">
                      （选填）
                    </Text>
                  </FormLabel>
                  <Input
                    {...field}
                    ref={initialFocusRef}
                    size="lg"
                    autoComplete="name"
                    placeholder="用于展示和称呼"
                    focusBorderColor="primary.600"
                  />
                  <FormErrorMessage>{form.errors.nickname}</FormErrorMessage>
                </FormControl>
              )}
            </Field>

            <Field name="companyName">
              {({ field, form }: FieldProps<string, SignupValues>) => (
                <FormControl
                  isInvalid={Boolean(
                    form.errors.companyName && form.touched.companyName,
                  )}
                >
                  <FormLabel mb={1.5} fontSize="sm" fontWeight="700">
                    公司名{" "}
                    <Text as="span" color="neutral.400">
                      （选填）
                    </Text>
                  </FormLabel>
                  <Input
                    {...field}
                    size="lg"
                    autoComplete="organization"
                    placeholder="填写您所代表的组织"
                    focusBorderColor="primary.600"
                  />
                  <FormErrorMessage>{form.errors.companyName}</FormErrorMessage>
                </FormControl>
              )}
            </Field>

            <Field name="mobile">
              {({ field, form }: FieldProps<string, SignupValues>) => (
                <FormControl
                  isRequired
                  isInvalid={Boolean(form.errors.mobile && form.touched.mobile)}
                >
                  <FormLabel mb={1.5} fontSize="sm" fontWeight="700">
                    手机号
                  </FormLabel>
                  <Input
                    {...field}
                    size="lg"
                    type="tel"
                    inputMode="numeric"
                    autoComplete="tel"
                    placeholder="请输入手机号"
                    focusBorderColor="primary.600"
                  />
                  <Box minH="20px">
                    <FormErrorMessage>{form.errors.mobile}</FormErrorMessage>
                  </Box>
                </FormControl>
              )}
            </Field>

            <Field name="code">
              {({ field, form }: FieldProps<string, SignupValues>) => (
                <FormControl
                  isRequired
                  isInvalid={Boolean(form.errors.code && form.touched.code)}
                >
                  <FormLabel mb={1.5} fontSize="sm" fontWeight="700">
                    验证码
                  </FormLabel>
                  <InputGroup size="lg">
                    <Input
                      {...field}
                      inputMode="numeric"
                      autoComplete="one-time-code"
                      placeholder="请输入验证码"
                      focusBorderColor="primary.600"
                      pr="7.5rem"
                    />
                    <InputRightElement w="7.25rem" pr={2}>
                      <Button
                        type="button"
                        size="sm"
                        minH="44px"
                        variant="ghost"
                        colorScheme="primary"
                        whiteSpace="nowrap"
                        isLoading={sending}
                        isDisabled={countdown > 0}
                        onClick={() => handleSendCode(values.mobile)}
                      >
                        {countdown > 0 ? `${countdown}s` : "发送验证码"}
                      </Button>
                    </InputRightElement>
                  </InputGroup>
                  <Box minH="20px">
                    <FormErrorMessage>{form.errors.code}</FormErrorMessage>
                  </Box>
                </FormControl>
              )}
            </Field>

            <Field name="password">
              {({ field, form }: FieldProps<string, SignupValues>) => (
                <FormControl
                  isRequired
                  isInvalid={Boolean(
                    form.errors.password && form.touched.password,
                  )}
                >
                  <FormLabel mb={1.5} fontSize="sm" fontWeight="700">
                    密码
                  </FormLabel>
                  <Input
                    {...field}
                    size="lg"
                    type="password"
                    autoComplete="new-password"
                    placeholder="请设置登录密码"
                    focusBorderColor="primary.600"
                  />
                  <Box minH="20px">
                    {form.errors.password && form.touched.password ? (
                      <FormErrorMessage>
                        {form.errors.password}
                      </FormErrorMessage>
                    ) : (
                      <Text mt={2} color="neutral.500" fontSize="xs">
                        8–128 位，至少组合字母、数字或符号中的三类
                      </Text>
                    )}
                  </Box>
                </FormControl>
              )}
            </Field>
          </Flex>

          <Button
            type="submit"
            w="full"
            h="54px"
            mt={4}
            colorScheme="primary"
            layerStyle="primarybutton"
            isLoading={registering}
          >
            注册并登录
          </Button>

          <Text mt={4} textAlign="center" color="neutral.500" fontSize="sm">
            已有账号？{" "}
            <Text
              as="button"
              type="button"
              minH="44px"
              color="primary.600"
              fontWeight="700"
              whiteSpace="nowrap"
              onClick={onClose}
              _focusVisible={{
                outline: "none",
                boxShadow: "0 0 0 2px var(--chakra-colors-gold-400)",
                borderRadius: "md",
              }}
            >
              去登录
            </Text>
          </Text>
        </Form>
      )}
    </Formik>
  );
}
