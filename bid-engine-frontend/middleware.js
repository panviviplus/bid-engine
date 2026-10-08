import { NextResponse } from "next/server";

export const config = {
  matcher:
    "/((?!api|_next/static|_next/image|bid-engine-logo.ico|images|logo).*)",
};

const redirectPath = (url, redirectUrl) => {
  url.pathname = redirectUrl;
  return NextResponse.redirect(url);
};

const toSigninPage = (url) => {
  return redirectPath(url, "/signin");
};

export default async function middleware(req) {
  const { pathname } = req.nextUrl;
  const url = req.nextUrl.clone();

  // -----------------------------------------------------------------------
  // FOR STATIC PAGE VIEWING ONLY - BYPASS AUTHENTICATION
  // -----------------------------------------------------------------------
  // 当您想要查看静态页面样式且没有后端服务时，请保留此部分并注释掉下方的原始逻辑。
  return NextResponse.next({
    headers: {
      "x-middleware-pass": "true",
    },
  });
  // -----------------------------------------------------------------------

  /* ORIGINAL AUTHENTICATION LOGIC (COMMENTED OUT FOR DEBUGGING)
  try {
    const hasAuthCookie = req.cookies.has("uid") && req.cookies.has("token");
    if (!hasAuthCookie) {
      if (!pathname.startsWith("/signin")) {
        return toSigninPage(url);
      }
    }

    if (hasAuthCookie) {
      const userInfo = await fetch(`${process.env.USR_SERVER}/info`, {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
          cookie: req.headers.get("cookie"),
        },
      });

      // 未登录/无权限处理
      if ([401, 403].includes(userInfo.status)) {
        if (!pathname.startsWith("/signin")) {
          return toSigninPage(url);
        }
      }

      // 登录态正常，处理登录页重定向
      if (userInfo.status === 200) {
        const user = await userInfo.json();
        if (user.code === -1) {
          if (!pathname.startsWith("/signin")) {
            return toSigninPage(url);
          }
        } else if (user.code === 0 && pathname.startsWith("/signin")) {
          return redirectPath(url, "/");
        }
      }
    }

    return NextResponse.next({
      headers: {
        "x-middleware-pass": "true",
      },
    });
  } catch (error) {
    console.log("middleware error", error);
    return NextResponse.next();
  }
  */
}
