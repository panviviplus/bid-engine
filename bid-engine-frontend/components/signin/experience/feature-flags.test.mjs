import test from "node:test";
import assert from "node:assert/strict";

import { getSmsAuthConfig, isUnifiedSmsAuthEnabled } from "./feature-flags.mjs";

test("keeps unified SMS authentication disabled by default", () => {
  assert.equal(isUnifiedSmsAuthEnabled(undefined), false);
  assert.equal(isUnifiedSmsAuthEnabled(""), false);
  assert.equal(isUnifiedSmsAuthEnabled("false"), false);
  assert.equal(isUnifiedSmsAuthEnabled("0"), false);
});

test("enables unified SMS authentication only for explicit true values", () => {
  assert.equal(isUnifiedSmsAuthEnabled("true"), true);
  assert.equal(isUnifiedSmsAuthEnabled(" TRUE "), true);
  assert.equal(isUnifiedSmsAuthEnabled("1"), true);
  assert.equal(isUnifiedSmsAuthEnabled(true), true);
});

test("keeps legacy SMS login live until unified authentication is enabled", () => {
  assert.deepEqual(getSmsAuthConfig(false), {
    endpoint: "/login",
    scene: "login",
    helper: "",
    includeLoginType: true,
    submitLabel: "登录",
    submitNote: "(新用户自动注册)",
    showSignupLink: true,
  });
  assert.deepEqual(getSmsAuthConfig(true), {
    endpoint: "/auth/sms",
    scene: "auth",
    helper: "未注册手机号验证后将自动创建账号并登录。",
    includeLoginType: false,
    submitLabel: "验证并登录",
    submitNote: "(新用户自动注册)",
    showSignupLink: false,
  });
});
