"use client";

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
  InputGroup,
  InputLeftElement,
  VStack,
  HStack,
  Box,
  Center,
  Avatar,
  Text,
} from "@chakra-ui/react";
import { EditIcon, EmailIcon, PhoneIcon } from "@chakra-ui/icons";
import useAxios from "axios-hooks";
import { useCustomToast } from "@/hooks/useCustomToast";
import { useEffect, useRef, useState } from "react";
import FeishuBindingPanel from "./FeishuBindingPanel";

export default function ProfileModal({
  isOpen,
  onClose,
  userProfile,
  onSaved,
}) {
  const showToast = useCustomToast();
  const fileInputRef = useRef(null);
  const [selectedAvatarFile, setSelectedAvatarFile] = useState(null);
  const [feishuBound, setFeishuBound] = useState(false);
  const prevOpenRef = useRef(false);

  const [form, setForm] = useState({
    nickname: "",
    mobile: "",
    email: "",
    avatarUrl: "",
  });

  // 当弹窗从关闭→打开时，用最新的 userProfile 初始化表单
  // 依赖 userProfile 确保 save → mutate → re-fetch 后的新数据能正确回显
  useEffect(() => {
    const justOpened = !prevOpenRef.current && isOpen;
    prevOpenRef.current = isOpen;
    if (!justOpened) return;
    setForm({
      nickname: userProfile?.nickname || userProfile?.name || "",
      mobile: userProfile?.mobile || "",
      email: userProfile?.email || "",
      avatarUrl: userProfile?.avatarUrl || "",
    });
    setSelectedAvatarFile(null);
  }, [isOpen, userProfile]);

  const [{ loading: isSaving }, saveProfile] = useAxios(
    { method: "PUT", url: "/user/profile", withCredentials: true },
    { manual: true },
  );

  const handlePickAvatar = () => fileInputRef.current?.click?.();

  const handleAvatarFileChange = async (e) => {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    if (!/\.(jpg|jpeg|png|gif)$/.test(file.name.toLowerCase())) {
      showToast({ title: "仅支持 jpg/jpeg/png/gif 格式" });
      return;
    }
    if (file.size > 5 * 1024 * 1024) {
      showToast({ title: "头像大小不能超过5MB" });
      return;
    }
    setSelectedAvatarFile(file);
    setForm((prev) => ({ ...prev, avatarUrl: URL.createObjectURL(file) }));
  };

  const handleSave = async () => {
    try {
      const fd = new FormData();
      fd.append("nickname", form.nickname || "");
      fd.append("mobile", form.mobile || "");
      fd.append("email", form.email || "");
      if (selectedAvatarFile) fd.append("file", selectedAvatarFile);
      const res = await saveProfile({ data: fd });
      if (res.data.code !== 0) {
        showToast({ title: res.data.message || "保存失败" });
        return;
      }
      showToast({ status: "success", title: "保存成功" });
      onClose();
      await onSaved?.();
      setSelectedAvatarFile(null);
    } catch {
      showToast({ status: "error", title: "保存失败，请稍后再试" });
    }
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} isCentered motionPreset="scale">
      <ModalOverlay bg="blackAlpha.200" backdropFilter="blur(4px)" />
      <ModalContent
        borderRadius="2xl"
        overflow="hidden"
        boxShadow="0 16px 48px rgba(0,0,0,0.15)"
      >
        <ModalHeader
          bg="primary.600"
          color="white"
          fontSize="md"
          fontWeight="600"
          py={4}
        >
          编辑资料
        </ModalHeader>
        <ModalBody bg="white" py={6} maxH="70vh" overflowY="auto">
          <VStack spacing={5} align="stretch">
            <HStack spacing={4}>
              <Box
                role="group"
                position="relative"
                onClick={handlePickAvatar}
                cursor="pointer"
              >
                <Avatar
                  size="lg"
                  name={form.nickname || userProfile?.name}
                  src={form.avatarUrl}
                  borderWidth="2px"
                  borderColor="primary.100"
                />
                <Center
                  position="absolute"
                  inset={0}
                  borderRadius="full"
                  bg="blackAlpha.400"
                  opacity={0}
                  transition="opacity 0.15s"
                  _groupHover={{ opacity: 1 }}
                >
                  <EditIcon color="white" boxSize={5} />
                </Center>
              </Box>
              <Box>
                <input
                  ref={fileInputRef}
                  type="file"
                  accept="image/*"
                  style={{ display: "none" }}
                  onChange={handleAvatarFileChange}
                />
                <Text fontSize="xs" color="neutral.400">
                  支持 jpg/png/gif，≤5MB
                </Text>
              </Box>
            </HStack>

            <FormControl>
              <FormLabel fontSize="sm" color="neutral.600">
                用户姓名
              </FormLabel>
              <InputGroup size="lg">
                <InputLeftElement pointerEvents="none">
                  <EditIcon color="neutral.400" />
                </InputLeftElement>
                <Input
                  value={form.nickname}
                  onChange={(e) =>
                    setForm((p) => ({ ...p, nickname: e.target.value }))
                  }
                  placeholder="请输入姓名"
                  borderRadius="xl"
                  bg="neutral.50"
                  _focus={{ bg: "white" }}
                />
              </InputGroup>
            </FormControl>

            {(!feishuBound || userProfile?.mobile) && <FormControl>
              <FormLabel fontSize="sm" color="neutral.600">
                手机号
              </FormLabel>
              <InputGroup size="lg">
                <InputLeftElement pointerEvents="none">
                  <PhoneIcon color="neutral.400" />
                </InputLeftElement>
                <Input
                  value={form.mobile}
                  onChange={(e) =>
                    setForm((p) => ({ ...p, mobile: e.target.value }))
                  }
                  placeholder="请输入手机号"
                  borderRadius="xl"
                  bg="neutral.50"
                  _focus={{ bg: "white" }}
                />
              </InputGroup>
            </FormControl>}

            <FormControl>
              <FormLabel fontSize="sm" color="neutral.600">
                邮箱
              </FormLabel>
              <InputGroup size="lg">
                <InputLeftElement pointerEvents="none">
                  <EmailIcon color="neutral.400" />
                </InputLeftElement>
                <Input
                  value={form.email}
                  onChange={(e) =>
                    setForm((p) => ({ ...p, email: e.target.value }))
                  }
                  placeholder="请输入邮箱"
                  borderRadius="xl"
                  bg="neutral.50"
                  _focus={{ bg: "white" }}
                />
              </InputGroup>
            </FormControl>
          </VStack>
          <Box mt={5}>
            <FeishuBindingPanel isOpen={isOpen} onStateChange={setFeishuBound} />
          </Box>
        </ModalBody>
        <ModalFooter bg="white" borderTop="1px solid" borderColor="neutral.50">
          <Button variant="ghost" mr={3} onClick={onClose} borderRadius="xl">
            取消
          </Button>
          <Button
            colorScheme="primary"
            onClick={handleSave}
            isLoading={isSaving}
            loadingText="保存中"
            borderRadius="xl"
          >
            保存
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
}
