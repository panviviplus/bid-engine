export function isUnifiedSmsAuthEnabled(value) {
  if (value === true) return true;
  if (typeof value !== "string") return false;
  const normalized = value.trim().toLowerCase();
  return normalized === "true" || normalized === "1";
}

export function getSmsAuthConfig(unifiedAuthEnabled) {
  if (unifiedAuthEnabled) {
    return {
      endpoint: "/auth/sms",
      scene: "auth",
      helper: "未注册手机号验证后将自动创建账号并登录。",
      includeLoginType: false,
      submitLabel: "验证并登录",
      submitNote: "(新用户自动注册)",
      showSignupLink: false,
    };
  }
  return {
    endpoint: "/login",
    scene: "login",
    helper: "",
    includeLoginType: true,
    submitLabel: "登录",
    submitNote: "(新用户自动注册)",
    showSignupLink: true,
  };
}
