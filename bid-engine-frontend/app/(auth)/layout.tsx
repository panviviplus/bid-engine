import React from "react";

export default function AuthLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  // eslint-disable-next-line react/jsx-no-useless-fragment
  return <>{children}</>;
}
