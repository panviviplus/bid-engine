import assert from "node:assert/strict";
import test from "node:test";

import * as authCardState from "./auth-card-state.mjs";

const { buildRegisterPayload, validateRegistrationFields } = authCardState;

test("routes login assist actions from the active authentication method", () => {
  assert.equal(typeof authCardState.getAuthAssistActions, "function");

  assert.deepEqual(authCardState.getAuthAssistActions(0), {
    left: null,
    right: {
      kind: "tab",
      label: "密码登录 →",
      targetIndex: 1,
    },
  });
  assert.deepEqual(authCardState.getAuthAssistActions(1), {
    left: {
      kind: "tab",
      label: "← 验证码登录",
      targetIndex: 0,
    },
    right: {
      kind: "signup",
      label: "去注册",
      prompt: "还没有账号？",
    },
  });
});

test("requires mobile code and password but leaves profile fields optional", () => {
  assert.deepEqual(
    validateRegistrationFields({
      nickname: "",
      companyName: "",
      mobile: "",
      code: "",
      password: "",
    }),
    { mobile: "必填", code: "必填", password: "必填" },
  );

  assert.deepEqual(
    validateRegistrationFields({
      nickname: "",
      companyName: "",
      mobile: "13800138000",
      code: "123456",
      password: "Secret123",
    }),
    {},
  );
});

test("requires a confirmed password on the standalone registration flow", () => {
  assert.equal(
    typeof authCardState.validateRegistrationPasswordPair,
    "function",
  );
  assert.equal(
    authCardState.validateRegistrationPasswordPair("", ""),
    "请设置密码",
  );
  assert.equal(
    authCardState.validateRegistrationPasswordPair("Secret123", ""),
    "请确认密码",
  );
  assert.equal(
    authCardState.validateRegistrationPasswordPair("Secret123", "Other123"),
    "两次输入的密码不一致",
  );
  assert.equal(
    authCardState.validateRegistrationPasswordPair("Secret123", "Secret123"),
    "",
  );
});

test("builds the existing register API payload with trimmed values", () => {
  assert.deepEqual(
    buildRegisterPayload({
      nickname: "  张三 ",
      companyName: "  澜舟科技 ",
      mobile: " 13800138000 ",
      code: " 123456 ",
      password: " Secret123 ",
    }),
    {
      nickname: "张三",
      company_name: "澜舟科技",
      mobile: "13800138000",
      sms_code: "123456",
      password: "Secret123",
    },
  );
});
