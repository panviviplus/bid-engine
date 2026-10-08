import React from "react";

export default function PlainLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div style={{ width: "100%", height: "100%", background: "#fff" }}>
      {children}
    </div>
  );
}
