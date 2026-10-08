"use client";

import React from "react";
import { Box, Flex, Text, Tooltip, keyframes } from "@chakra-ui/react";
import {
  FiCheck,
  FiX,
  FiLoader,
  FiCircle,
} from "react-icons/fi";
import { StageStep } from "./types";

const pulse = keyframes`
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
`;

function StepIcon({ status }: { status: string }) {
  if (status === "succeeded") {
    return (
      <Flex
        w="22px"
        h="22px"
        borderRadius="full"
        bg="success.500"
        color="white"
        align="center"
        justify="center"
      >
        <FiCheck size={13} />
      </Flex>
    );
  }
  if (status === "failed") {
    return (
      <Flex
        w="22px"
        h="22px"
        borderRadius="full"
        bg="error.500"
        color="white"
        align="center"
        justify="center"
      >
        <FiX size={13} />
      </Flex>
    );
  }
  if (status === "running") {
    return (
      <Flex
        w="22px"
        h="22px"
        borderRadius="full"
        bg="primary.500"
        color="white"
        align="center"
        justify="center"
        animation={`${pulse} 1.2s ease-in-out infinite`}
      >
        <FiLoader size={13} />
      </Flex>
    );
  }
  return (
    <Flex
      w="22px"
      h="22px"
      borderRadius="full"
      border="2px solid"
      borderColor="neutral.300"
      color="neutral.300"
      align="center"
      justify="center"
    >
      <FiCircle size={9} />
    </Flex>
  );
}

export default function StageProgress({
  steps,
  progress,
}: {
  steps: StageStep[];
  progress?: number;
}) {
  return (
    <Flex align="center" gap={0} flexWrap="wrap" rowGap={2}>
      {steps.map((st, i) => {
        const done = st.status === "succeeded";
        const failed = st.status === "failed";
        const active = st.status === "running";
        return (
          <React.Fragment key={st.stage}>
            <Tooltip
              label={`${st.label} · ${statusLabel(st.status)}`}
              hasArrow
              placement="top"
            >
              <Flex align="center" gap={1.5} cursor="default">
                <StepIcon status={st.status} />
                <Text
                  fontSize="12px"
                  fontWeight={active ? "600" : "500"}
                  color={
                    failed
                      ? "error.600"
                      : active
                        ? "primary.600"
                        : done
                          ? "success.600"
                          : "neutral.400"
                  }
                >
                  {st.label}
                </Text>
              </Flex>
            </Tooltip>
            {i < steps.length - 1 && (
              <Box
                flex="0 0 28px"
                h="2px"
                mx={2}
                bg={done ? "success.400" : "neutral.200"}
                borderRadius="full"
              />
            )}
          </React.Fragment>
        );
      })}
      {typeof progress === "number" && (
        <Text ml={3} fontSize="12px" color="neutral.400">
          {progress}%
        </Text>
      )}
    </Flex>
  );
}

function statusLabel(status: string) {
  switch (status) {
    case "succeeded":
      return "已完成";
    case "failed":
      return "失败";
    case "running":
      return "进行中";
    case "skipped":
      return "已跳过";
    default:
      return "待开始";
  }
}
