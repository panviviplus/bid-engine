"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

export default function TemplatesPage() {
  const router = useRouter();

  useEffect(() => {
    router.replace("/material/template");
  }, [router]);

  return null;
}
