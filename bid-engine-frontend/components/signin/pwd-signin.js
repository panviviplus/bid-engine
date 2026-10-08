import React from "react";
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

export default function SmsLogin() {
  // const mobileRegex = /^(999)\d{8}$/;
  const router = useRouter();
  const showToast = useCustomToast();

  // 表单验证
  const ValidationSchema = Yup.object().shape({
    code: Yup.string().trim().required("必填"),
    // .matches(/^\d{6}$/, "验证码为6位数字"),
    tel: Yup.string().trim().required("必填"),
    // .matches(mobileRegex, "请输入正确的账号"),
  });

  // 登录
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
        params: {
          mode: "saas",
          mobile: values.tel,
          sms_code: values.code,
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
      <Formik
        initialValues={{
          tel: "",
          code: "",
        }}
        validationSchema={ValidationSchema}
        onSubmit={handleLogin}
      >
        <Form style={{ width: "100%", margin: 0 }}>
          <Field name="tel">
            {({ field, form }) => (
              <FormControl isInvalid={form.errors.tel && form.touched.tel}>
                <InputGroup size="lg">
                  <Input
                    {...field}
                    placeholder="请输入账号"
                    id="tel"
                    type="tel"
                    autoComplete="false"
                    focusBorderColor="primary.600"
                  />
                </InputGroup>
                <Box display="flex" mt={2}>
                  &nbsp;
                  <FormErrorMessage m={0}>{form.errors.tel}</FormErrorMessage>
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
                    autoComplete="false"
                    focusBorderColor="primary.600"
                    placeholder="请输入密码"
                  />
                </InputGroup>
                <Box display="flex" mt={2}>
                  &nbsp;
                  <FormErrorMessage m={0}>{form.errors.code}</FormErrorMessage>
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
