"use client";

import React from "react";
import { PageViewport } from "@/components/layout/responsive-page";
import RuleLibrary from "@/components/audit/rule-library";

export default function BidAuditRulesPage() {
  return (
    <PageViewport w="full" className="thin-scrollbars" bg="workbench.canvas">
      <RuleLibrary />
    </PageViewport>
  );
}
