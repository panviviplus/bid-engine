export function getAuthAssistActions(tabIndex) {
  if (tabIndex === 1) {
    return {
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
    };
  }
  return {
    left: null,
    right: {
      kind: "tab",
      label: "密码登录 →",
      targetIndex: 1,
    },
  };
}

export function validateRegistrationFields(values) {
  const errors = {};
  if (!String(values.mobile || "").trim()) errors.mobile = "必填";
  if (!String(values.code || "").trim()) errors.code = "必填";
  if (!String(values.password || "").trim()) errors.password = "必填";
  return errors;
}

export function validateRegistrationPasswordPair(passwordValue, confirmValue) {
  const password = String(passwordValue || "").trim();
  const confirm = String(confirmValue || "").trim();
  if (!password) return "请设置密码";
  if (!confirm) return "请确认密码";
  if (password !== confirm) return "两次输入的密码不一致";
  return "";
}

export function buildRegisterPayload(values) {
  return {
    nickname: String(values.nickname || "").trim(),
    company_name: String(values.companyName || "").trim(),
    mobile: String(values.mobile || "").trim(),
    sms_code: String(values.code || "").trim(),
    password: String(values.password || "").trim(),
  };
}
