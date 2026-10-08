import React from "react";
import Layout from "@/components/layout";

export default function AuthLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  // eslint-disable-next-line react/jsx-no-useless-fragment
  return <Layout>{children}</Layout>;
}
