"use client";

import { useState, useCallback } from "react";
import {
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  Button,
  FormControl,
  FormLabel,
  Input,
  Text,
  HStack,
} from "@chakra-ui/react";
import { ViewIcon, ViewOffIcon } from "@chakra-ui/icons";
import useAxios from "axios-hooks";
import { useCustomToast } from "@/hooks/useCustomToast";

const PASSWORD_REGEX = /^[A-Za-z0-9!@#$%^&*()_+\-=\[\]{}|;':",./<>?]{8,128}$/;

function validatePasswordStrength(pwd: string): {
  valid: boolean;
  score: "weak" | "medium" | "strong";
  message: string;
} {
  if (!pwd) {
    return { valid: false, score: "weak", message: "" };
  }
  if (!PASSWORD_REGEX.test(pwd)) {
    return {
      valid: false,
      score: "weak",
      message: "密码格式不正确：8-128位，仅支持数字、大小写字母和英文标点符号",
    };
  }
  let types = 0;
  if (/[A-Z]/.test(pwd)) types++;
  if (/[a-z]/.test(pwd)) types++;
  if (/[0-9]/.test(pwd)) types++;
  if (/[^A-Za-z0-9]/.test(pwd)) types++;
  if (types < 3) {
    return {
      valid: false,
      score: "medium",
      message: "需包含大写字母、小写字母、数字、特殊符号中的至少3种",
    };
  }
  return {
    valid: true,
    score: pwd.length >= 12 ? "strong" : "medium",
    message: "",
  };
}

const strengthColor: Record<string, string> = {
  weak: "red.400",
  medium: "yellow.500",
  strong: "green.500",
};

interface Props {
  isOpen: boolean;
  onClose: () => void;
}

export default function ChangePasswordModal({ isOpen, onClose }: Props) {
  const showToast = useCustomToast();
  const [oldPassword, setOldPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showOld, setShowOld] = useState(false);
  const [showNew, setShowNew] = useState(false);

  const [{ loading }, changePassword] = useAxios(
    {
      method: "PUT",
      url: "/user/password",
      headers: { "Content-Type": "application/json" },
      withCredentials: true,
    },
    { manual: true },
  );

  const strength = validatePasswordStrength(newPassword);
  const confirmError =
    confirmPassword && newPassword !== confirmPassword
      ? "两次输入的新密码不一致"
      : "";

  const handleSubmit = useCallback(async () => {
    if (!oldPassword.trim()) {
      showToast({ title: "请输入旧密码" });
      return;
    }
    if (!strength.valid) {
      showToast({ title: strength.message || "新密码格式不正确" });
      return;
    }
    if (newPassword !== confirmPassword) {
      showToast({ title: "两次输入的新密码不一致" });
      return;
    }
    if (oldPassword === newPassword) {
      showToast({ title: "新密码不能与旧密码相同" });
      return;
    }
    try {
      const res = await changePassword({
        data: {
          old_password: oldPassword,
          new_password: newPassword,
          confirm_password: confirmPassword,
        },
      });
      if (res.data?.code !== 0) {
        showToast({ title: res.data?.message || "修改失败" });
        return;
      }
      showToast({ title: "密码修改成功" });
      setOldPassword("");
      setNewPassword("");
      setConfirmPassword("");
      onClose();
    } catch {
      showToast({ title: "修改失败，请稍后再试" });
    }
  }, [
    oldPassword,
    newPassword,
    confirmPassword,
    strength,
    changePassword,
    showToast,
    onClose,
  ]);

  const handleClose = () => {
    setOldPassword("");
    setNewPassword("");
    setConfirmPassword("");
    onClose();
  };

  return (
    <Modal isOpen={isOpen} onClose={handleClose} size="md" isCentered>
      <ModalOverlay />
      <ModalContent borderRadius="xl">
        <ModalHeader fontSize="lg" fontWeight="600">
          修改登录密码
        </ModalHeader>
        <ModalBody>
          <FormControl mb={4}>
            <FormLabel fontSize="sm">旧密码</FormLabel>
            <Input
              type={showOld ? "text" : "password"}
              placeholder="请输入当前密码"
              value={oldPassword}
              onChange={(e) => setOldPassword(e.target.value)}
            />
          </FormControl>
          <FormControl mb={4}>
            <FormLabel fontSize="sm">新密码</FormLabel>
            <Input
              type={showNew ? "text" : "password"}
              placeholder="请输入新密码"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
            />
            {newPassword && (
              <HStack mt={1} spacing={2}>
                <Text fontSize="xs" color={strengthColor[strength.score]}>
                  {strength.score === "weak"
                    ? "弱"
                    : strength.score === "medium"
                      ? "中"
                      : "强"}
                </Text>
                {strength.message && (
                  <Text fontSize="xs" color="red.400">
                    {strength.message}
                  </Text>
                )}
              </HStack>
            )}
            <Text fontSize="xs" color="gray.400" mt={1}>
              8-128位，需包含大写字母、小写字母、数字、特殊符号中至少3种
            </Text>
          </FormControl>
          <FormControl mb={2}>
            <FormLabel fontSize="sm">确认新密码</FormLabel>
            <Input
              type="password"
              placeholder="请再次输入新密码"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
            />
            {confirmError && (
              <Text fontSize="xs" color="red.400" mt={1}>
                {confirmError}
              </Text>
            )}
          </FormControl>
        </ModalBody>
        <ModalFooter>
          <Button variant="ghost" mr={3} onClick={handleClose}>
            取消
          </Button>
          <Button
            colorScheme="primary"
            isLoading={loading}
            onClick={handleSubmit}
          >
            确认修改
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
}
