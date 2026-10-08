"use client";

import React from "react";
import PdfPreview from "@/components/common/pdf-preview";
import { useSearchParams } from "next/navigation";

export default function Page({ params }: { params: { id: string } }) {
  const searchParams = useSearchParams();
  const attachmentPath = searchParams.get("attachmentPath") || undefined;
  if (attachmentPath) {
    return <PdfPreview attachmentPath={attachmentPath} />;
  }
  return <PdfPreview src={`/zb/template/preview/${params.id}`} />;
}
