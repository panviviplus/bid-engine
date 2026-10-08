import React, { useMemo } from "react";
import {
  Box,
  Flex,
  Button,
  Input,
  InputGroup,
  FormControl,
  FormErrorMessage,
} from "@chakra-ui/react";
import { Field, Form, Formik } from "formik";
import * as Yup from "yup";
import useAxios from "axios-hooks";
import { useCustomToast } from "@/hooks/useCustomToast";
import { useRouter } from "next/navigation";

export default function PasswordSignin() {
  const router = useRouter();
  const showToast = useCustomToast();

  const ValidationSchema = useMemo(
    () =>
      Yup.object().shape({
        mobile: Yup.string().trim().required("必填"),
        password: Yup.string().trim().required("必填"),
      }),
    [],
  );

  const [{ loading: isLoginLoading }, login] = useAxios(
    {
      method: "POST",
      url: "/login",
      headers: { "Content-Type": "application/json" },
      withCredentials: true,
    },
    { manual: true },
  );

  const handleLogin = async (values) => {
    try {
      const res = await login({
        data: {
          mobile: values.mobile,
          password: values.password,
          login_type: "password",
        },
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
      <Formik
        initialValues={{ mobile: "", password: "" }}
        validationSchema={ValidationSchema}
        onSubmit={handleLogin}
      >
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

          <Field name="password">
            {({ field, form }) => (
              <FormControl
                isInvalid={form.errors.password && form.touched.password}
              >
                <InputGroup size="lg">
                  <Input
                    {...field}
                    id="password"
                    type="password"
                    autoComplete="current-password"
                    focusBorderColor="primary.600"
                    placeholder="请输入密码"
                  />
                </InputGroup>
                <Box display="flex" mt={2}>
                  &nbsp;
                  <FormErrorMessage m={0}>
                    {form.errors.password}
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
            layerStyle="primarybutton"
            colorScheme="primary"
            isLoading={isLoginLoading}
          >
            登录
          </Button>
        </Form>
      </Formik>
    </Flex>
  );
}
