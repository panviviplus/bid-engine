"use client";

import React from "react";
import { ArrowUpIcon } from "@chakra-ui/icons";
import {
  Badge,
  Box,
  Button,
  Card,
  CardBody,
  Container,
  Divider,
  Flex,
  Heading,
  HStack,
  IconButton,
  SimpleGrid,
  Text,
  VStack,
} from "@chakra-ui/react";
import { useRouter } from "next/navigation";
import AvatarSVG from "@/components/svg/head/avatar";
import { useCustomToast } from "@/hooks/useCustomToast";
import { AUTH_PAGE_GUTTERS } from "@/components/layout/responsive-layout.mjs";

type ModuleItem = { title: string; desc: string };

const MODULES: ModuleItem[] = [
  { title: "招标解析", desc: "关键条款/评分点结构化提取，进度可追踪。" },
  { title: "标书生成", desc: "目录到正文一体化编辑，支持协作与导出。" },
  { title: "合规审核", desc: "对照招标要求校验，输出问题清单与建议。" },
  { title: "知识库", desc: "资质/业绩/方案/段落沉淀复用，越用越强。" },
];

const hoverLift = {
  transition: "all 0.2s",
  _hover: { transform: "translateY(-2px)", boxShadow: "0 22px 60px rgba(0,0,0,0.18)" },
  _active: { transform: "translateY(-1px)" },
} as const;

type Props = {
  formTitle: string;
  children: React.ReactNode;
  brandTitle?: string;
  brandSubtitle?: string;
};

export default function AuthCanvas({
  formTitle,
  children,
  brandTitle = "标 擎",
  brandSubtitle = "AI 驱动 · 智慧投标",
}: Props) {
  const router = useRouter();
  const showToast = useCustomToast();
  const scrollerRef = React.useRef<HTMLDivElement | null>(null);
  const [showBackToTop, setShowBackToTop] = React.useState(false);

  React.useEffect(() => {
    const el = scrollerRef.current;
    if (!el) return;
    const onScroll = () => {
      setShowBackToTop(el.scrollTop > el.clientHeight * 0.85);
    };
    onScroll();
    el.addEventListener("scroll", onScroll, { passive: true });
    return () => el.removeEventListener("scroll", onScroll);
  }, []);

  const scrollToTop = () => {
    const el = scrollerRef.current;
    if (!el) return;
    el.scrollTo?.({ top: 0, behavior: "smooth" }) ?? (el.scrollTop = 0);
  };

  const handleProtectedNav = (href: string) => {
    const isAuthed =
      typeof document !== "undefined" &&
      document.cookie.includes("uid=") &&
      document.cookie.includes("token=");
    if (isAuthed) { router.push(href); return; }
    showToast({ title: "请先登录后访问该页面" });
    if (typeof window !== "undefined" && window.location.pathname !== "/signin") {
      router.push("/signin");
    }
  };

  return (
    <Flex ref={scrollerRef} flex={1} direction="column" bg="gray.900"
      bgImage="radial-gradient(at 20% 10%, rgba(37,99,235,0.35) 0px, transparent 55%), radial-gradient(at 80% 90%, rgba(6,182,212,0.25) 0px, transparent 55%)"
      bgSize="cover" overflow="auto" position="relative">
      <Box position="absolute" top="-12%" left="-12%" w="60%" h="60%" bg="primary.900" filter="blur(140px)" opacity={0.45} pointerEvents="none" />
      <Box position="absolute" bottom="-12%" right="-12%" w="60%" h="60%" bg="cyan.900" filter="blur(140px)" opacity={0.35} pointerEvents="none" />
      <Box position="absolute" inset={0} bgGradient="linear(to-b, transparent, rgba(0,0,0,0.15))" pointerEvents="none" />

      <Box position="fixed" right={{ base: 4, md: 6 }} bottom={{ base: 6, md: 8 }} zIndex={50}
        opacity={showBackToTop ? 1 : 0} transform={showBackToTop ? "translateY(0)" : "translateY(10px)"}
        transition="all 0.2s" pointerEvents={showBackToTop ? "auto" : "none"}>
        <IconButton aria-label="回到顶部" icon={<ArrowUpIcon />} onClick={scrollToTop} borderRadius="full"
          bg="rgba(255,255,255,0.14)" color="white" border="1px solid rgba(255,255,255,0.18)"
          backdropFilter="blur(14px)" boxShadow="0 18px 50px rgba(0,0,0,0.25)"
          _hover={{ bg: "rgba(255,255,255,0.18)", transform: "translateY(-1px)" }} _active={{ transform: "translateY(0)" }} />
      </Box>

      <Flex direction="column" minH="100dvh" justify="center" py={{ base: 8, md: 12 }}>
        <Container w="full" maxW="none" px={AUTH_PAGE_GUTTERS}>
          <Flex
            align="stretch"
            justify="space-between"
            gap={{ base: 8, xl: 12, "2xl": 16 }}
            direction={{ base: "column", lg: "row" }}
          >
            <VStack
              spacing={6}
              align="start"
              flex="1 1 0"
              minW={0}
              color="white"
              pt={{ base: 0, lg: 4 }}
            >
              <HStack spacing={3}>
                <Box w="44px" h="44px" borderRadius="2xl" bg="whiteAlpha.200" backdropFilter="blur(10px)"
                  display="flex" alignItems="center" justifyContent="center" border="1px solid rgba(255,255,255,0.12)">
                  <AvatarSVG width={22} height={22} />
                </Box>
                <Badge bg="whiteAlpha.200" color="white" borderRadius="full" px={3} py={1} fontSize="sm"
                  border="1px solid rgba(255,255,255,0.12)">AI驱动的投标工作台</Badge>
              </HStack>
              <Heading bgGradient="linear(to-r, white, cyan.100)" bgClip="text"
                fontSize={{ base: "3xl", md: "4xl", lg: "5xl" }} fontWeight="800" lineHeight="1.15">
                {brandTitle}<br />{brandSubtitle}
              </Heading>
              <Text fontSize={{ base: "md", md: "lg" }} color="gray.200" maxW="520px" lineHeight="1.9">
                将招标解析、标书撰写、合规审核、知识沉淀融为一体。以可复用资产与可控流程，提升交付质量与团队协作效率。
              </Text>
              <Flex gap={3} mt={1} wrap="wrap">
                {["AI 驱动", "流程可控", "团队协作", "资产沉淀"].map((tag) => (
                  <Flex key={tag} px={4} py={2} bg="whiteAlpha.200" borderRadius="full"
                    backdropFilter="blur(10px)" border="1px solid rgba(255,255,255,0.1)">
                    <Text fontSize="sm" fontWeight="600">{tag}</Text>
                  </Flex>
                ))}
              </Flex>
              <Box w="full" mt={2}>
                <Text fontSize="sm" color="gray.200" mb={3}>功能模块</Text>
                <SimpleGrid
                  minChildWidth={{ base: "100%", sm: "18rem", "2xl": "20rem" }}
                  spacing={3}
                  w="full"
                >
                  {MODULES.map((it) => (
                    <Card key={it.title} bg="rgba(255,255,255,0.92)" border="1px solid rgba(255,255,255,0.35)"
                      backdropFilter="blur(16px)" borderRadius="2xl" boxShadow="0 12px 30px rgba(0,0,0,0.12)" {...hoverLift}>
                      <CardBody maxW="65ch" p={4}>
                        <Text fontWeight="800" color="gray.800" fontSize="sm">{it.title}</Text>
                        <Text mt={1} color="gray.600" fontSize="sm" lineHeight="1.7" noOfLines={2}>{it.desc}</Text>
                      </CardBody>
                    </Card>
                  ))}
                </SimpleGrid>
              </Box>
            </VStack>
            <Box w={{ base: "full", lg: "clamp(28rem, 30vw, 38rem)" }} flexShrink={0}
              bg="rgba(255, 255, 255, 0.92)" backdropFilter="blur(20px)"
              borderRadius="3xl" boxShadow="0 20px 40px rgba(0,0,0,0.2)" p={{ base: 8, md: 10 }}
              border="1px solid rgba(255,255,255,0.6)" alignSelf="center"
              transition="all 0.2s" _hover={{ boxShadow: "0 26px 70px rgba(0,0,0,0.24)" }}>
              <Heading size="lg" mb={8} color="gray.900" textAlign="center" letterSpacing="0.5px">{formTitle}</Heading>
              {children}
            </Box>
          </Flex>
        </Container>
      </Flex>

      <Box
        bg="rgba(255,255,255,0.03)" backdropFilter="blur(12px)" borderTop="1px solid rgba(255,255,255,0.12)"
        py={{ base: 12, md: 20 }}>
        <Container w="full" maxW="none" px={AUTH_PAGE_GUTTERS}>
          <SimpleGrid columns={{ base: 1, md: 2, lg: 3 }} spacing={6}>
            <Card borderRadius="2xl" bgGradient="linear(to-b, rgba(255,255,255,0.94), rgba(239,246,255,0.92))"
              border="1px solid rgba(255,255,255,0.35)" backdropFilter="blur(16px)" boxShadow="0 16px 40px rgba(0,0,0,0.14)" {...hoverLift}>
              <CardBody maxW="65ch" p={{ base: 5, md: 6 }}>
                <Badge colorScheme="blue" borderRadius="full" px={3} py={1}>适配 To B</Badge>
                <Heading mt={3} fontSize={{ base: "lg", md: "xl" }} color="gray.900">像工作台一样组织页面</Heading>
                <VStack align="start" spacing={3} mt={4}>
                  <Text color="gray.700" fontSize="sm" lineHeight="1.9">
                    招投标工作本质是高频、强时间约束的协作交付。登录后的页面更适合用"功能直达 + 统一导航"的工作台方式组织。
                  </Text>
                  <Text color="gray.600" fontSize="sm" lineHeight="1.9">
                    你可以在固定路径中完成上传、解析、撰写、审核、导出，不必通过滚轮在模块间切换，减少误操作与注意力分散。
                  </Text>
                </VStack>
              </CardBody>
            </Card>
            <Card borderRadius="2xl" bgGradient="linear(to-b, rgba(255,255,255,0.94), rgba(245,240,255,0.92))"
              border="1px solid rgba(255,255,255,0.35)" backdropFilter="blur(16px)" boxShadow="0 16px 40px rgba(0,0,0,0.14)" {...hoverLift}>
              <CardBody maxW="65ch" p={{ base: 5, md: 6 }}>
                <Badge colorScheme="purple" borderRadius="full" px={3} py={1}>质量可控</Badge>
                <Heading mt={3} fontSize={{ base: "lg", md: "xl" }} color="gray.900">输出结果可追溯、可交接</Heading>
                <VStack align="start" spacing={3} mt={4}>
                  <Text color="gray.700" fontSize="sm" lineHeight="1.9">
                    招投标的风险往往来自"信息漏看、版本混乱、责任不清"。系统把关键动作围绕"协作—确认—导出"组织，减少临近截止时间的返工与争议。
                  </Text>
                  <Text color="gray.600" fontSize="sm" lineHeight="1.9">
                    终审确认、协作者管理等能力可嵌入到项目流转中，让交接更顺畅、过程更可控。
                  </Text>
                </VStack>
              </CardBody>
            </Card>
            <Card borderRadius="2xl" bgGradient="linear(to-b, rgba(255,255,255,0.94), rgba(255,245,235,0.92))"
              border="1px solid rgba(255,255,255,0.35)" backdropFilter="blur(16px)" boxShadow="0 16px 40px rgba(0,0,0,0.14)" {...hoverLift}>
              <CardBody maxW="65ch" p={{ base: 5, md: 6 }}>
                <Badge colorScheme="orange" borderRadius="full" px={3} py={1}>资产沉淀</Badge>
                <Heading mt={3} fontSize={{ base: "lg", md: "xl" }} color="gray.900">知识库复用，越用越稳</Heading>
                <VStack align="start" spacing={3} mt={4}>
                  <Text color="gray.700" fontSize="sm" lineHeight="1.9">
                    "写标"不是从零开始，而是持续沉淀并复用企业资产：资质、业绩、方案段落、常用表格与图片统一进入知识库。
                  </Text>
                  <Text color="gray.600" fontSize="sm" lineHeight="1.9">
                    标书生成时优先复用既有内容，把更多精力留给差异化与合规细节，让每次交付都更稳定。
                  </Text>
                </VStack>
                <Button variant="outline" size="sm" mt={5} onClick={() => handleProtectedNav("/material/knowledge")}>
                  浏览知识库
                </Button>
              </CardBody>
            </Card>
          </SimpleGrid>
          <SimpleGrid columns={{ base: 1, lg: 2 }} spacing={6} mt={6}>
            <Card borderRadius="2xl" bgGradient="linear(to-b, rgba(255,255,255,0.94), rgba(239,246,255,0.92))"
              border="1px solid rgba(255,255,255,0.35)" backdropFilter="blur(16px)" boxShadow="0 16px 40px rgba(0,0,0,0.14)" {...hoverLift}>
              <CardBody maxW="65ch" p={{ base: 5, md: 6 }}>
                <Badge colorScheme="teal" borderRadius="full" px={3} py={1}>效率收益</Badge>
                <Heading mt={3} fontSize={{ base: "lg", md: "xl" }} color="gray.900">把时间留给差异化内容</Heading>
                <Text mt={4} color="gray.700" fontSize="sm" lineHeight="1.9">
                  解析把信息从"散落在文档里"变成"可检索、可复用的结构化清单"；生成把写作从"复制粘贴"变成"组装与校验"；审核把风险从"最后一刻爆雷"前置到流程中消化。
                </Text>
                <SimpleGrid columns={{ base: 1, md: 3 }} spacing={3} mt={5}>
                  {[{ k: "更少漏看", v: "条款结构化" }, { k: "更少返工", v: "清单式审核" }, { k: "更稳交付", v: "资产复用" }].map((it) => (
                    <Box key={it.k} borderRadius="xl" bg="gray.50" border="1px solid" borderColor="gray.100" p={4}>
                      <Text color="gray.900" fontWeight="800" fontSize="sm">{it.k}</Text>
                      <Text mt={1} color="gray.600" fontSize="sm">{it.v}</Text>
                    </Box>
                  ))}
                </SimpleGrid>
              </CardBody>
            </Card>
            <Card borderRadius="2xl" bgGradient="linear(to-b, rgba(255,255,255,0.94), rgba(245,240,255,0.92))"
              border="1px solid rgba(255,255,255,0.35)" backdropFilter="blur(16px)" boxShadow="0 16px 40px rgba(0,0,0,0.14)" {...hoverLift}>
              <CardBody maxW="65ch" p={{ base: 5, md: 6 }}>
                <Badge colorScheme="gray" borderRadius="full" px={3} py={1}>示例视图</Badge>
                <Heading mt={3} fontSize={{ base: "lg", md: "xl" }} color="gray.900">像"清单"一样看懂风险点</Heading>
                <Text mt={4} color="gray.700" fontSize="sm" lineHeight="1.9">
                  招标要求常常分散且表述不一。把要求抽取成清单后，你能快速核对每一项是否覆盖、是否需要证明材料、是否存在冲突或遗漏。
                </Text>
                <VStack align="start" spacing={3} mt={5}>
                  {[{ t: "资格项", d: "资质/人员/业绩是否满足，证明材料是否齐全。" }, { t: "响应项", d: "技术参数/服务承诺/交付周期是否逐项响应。" }, { t: "格式项", d: "章节结构、表格/图片引用、页眉页脚与签章是否合规。" }].map((it) => (
                    <Box key={it.t} w="full" borderRadius="xl" bg="gray.50" border="1px solid" borderColor="gray.100" p={4}>
                      <HStack justify="space-between">
                        <Text color="gray.900" fontWeight="800" fontSize="sm">{it.t}</Text>
                        <Badge variant="subtle" colorScheme="green" borderRadius="full">可追踪</Badge>
                      </HStack>
                      <Text mt={1} color="gray.600" fontSize="sm" lineHeight="1.8">{it.d}</Text>
                    </Box>
                  ))}
                </VStack>
              </CardBody>
            </Card>
          </SimpleGrid>
        </Container>
      </Box>

      <Box h={{ base: 10, md: 14 }}
        bgGradient="linear(to-b, rgba(255,255,255,0.03), rgba(0,0,0,0.18))"
        borderTop="1px solid rgba(255,255,255,0.10)" borderBottom="1px solid rgba(255,255,255,0.14)" />

      <Box pt={{ base: 12, md: 20 }} pb={{ base: 12, md: 24 }}
        bg="rgba(0,0,0,0.16)" backdropFilter="blur(12px)">
        <Container w="full" maxW="none" px={AUTH_PAGE_GUTTERS}>
          <Flex direction="column" gap={8}>
            <Box>
              <Badge bg="whiteAlpha.200" color="white" borderRadius="full" px={3} py={1} border="1px solid rgba(255,255,255,0.12)">典型流程</Badge>
              <Heading mt={3} fontSize={{ base: "2xl", md: "3xl" }} color="white" letterSpacing="0.5px">三步完成一次投标交付</Heading>
              <Text mt={3} color="gray.200" maxW="900px" lineHeight="1.9">
                招投标行业的核心挑战是"时间紧 + 要求多 + 参与角色多"。解析—生成—审核贯通，减少反复切换工具与重复整理文档的时间；协作与终审确认内置在流程中，让交付更稳定。
              </Text>
            </Box>
            <SimpleGrid columns={{ base: 1, md: 2 }} spacing={6}>
              <Card borderRadius="2xl" bgGradient="linear(to-b, rgba(255,255,255,0.94), rgba(245,240,255,0.92))"
                border="1px solid rgba(255,255,255,0.35)" backdropFilter="blur(16px)" boxShadow="0 16px 40px rgba(0,0,0,0.14)" {...hoverLift}>
                <CardBody maxW="65ch" p={{ base: 5, md: 6 }}>
                  <Badge colorScheme="gray" borderRadius="full" px={3} py={1}>行业难点</Badge>
                  <Heading mt={3} fontSize={{ base: "lg", md: "xl" }} color="gray.900">评分点多、版本多、沟通多</Heading>
                  <VStack align="start" spacing={3} mt={4}>
                    <Text color="gray.700" fontSize="sm" lineHeight="1.9">
                      招标文件要求繁杂，评分点分散在不同章节，人工整理容易遗漏；多人协作写作时版本容易乱；临近截止时间沟通成本陡增。
                    </Text>
                    <Text color="gray.600" fontSize="sm" lineHeight="1.9">
                      系统的目标是把不确定性前置：把要求结构化、把过程可视化、把资产可复用化。
                    </Text>
                  </VStack>
                </CardBody>
              </Card>
              <Card borderRadius="2xl" bgGradient="linear(to-b, rgba(255,255,255,0.94), rgba(239,246,255,0.92))"
                border="1px solid rgba(255,255,255,0.35)" backdropFilter="blur(16px)" boxShadow="0 16px 40px rgba(0,0,0,0.14)" {...hoverLift}>
                <CardBody maxW="65ch" p={{ base: 5, md: 6 }}>
                  <Badge colorScheme="blue" borderRadius="full" px={3} py={1}>系统能力</Badge>
                  <Heading mt={3} fontSize={{ base: "lg", md: "xl" }} color="gray.900">把"写标"变成"组装与校验"</Heading>
                  <VStack align="start" spacing={3} mt={4}>
                    <Text color="gray.700" fontSize="sm" lineHeight="1.9">
                      解析阶段形成可追踪的结构化结果；生成阶段基于大纲与知识库快速复用；审核阶段对照要求输出清单式问题与建议。
                    </Text>
                    <Text color="gray.600" fontSize="sm" lineHeight="1.9">
                      让团队把精力集中在差异化内容与关键合规点上，而不是重复复制粘贴与格式整理。
                    </Text>
                  </VStack>
                </CardBody>
              </Card>
            </SimpleGrid>
            <SimpleGrid columns={{ base: 1, md: 3 }} spacing={6}>
              {[{ k: "01", t: "解析招标文件", d: "导入招标文件，自动提取关键条款、评分点与清单，形成结构化结果。" },
                { k: "02", t: "生成与协作撰写", d: "基于目录与知识库素材快速成稿，支持多人协作、角色权限与确认机制。" },
                { k: "03", t: "合规审核与导出", d: "对照招标要求逐项校验，修订后导出交付文档，减少返工与风险。" }].map((it) => (
                <Card key={it.k} borderRadius="2xl" bgGradient="linear(to-b, rgba(255,255,255,0.94), rgba(245,240,255,0.92))"
                  border="1px solid rgba(255,255,255,0.35)" backdropFilter="blur(16px)" boxShadow="0 16px 40px rgba(0,0,0,0.14)" {...hoverLift}>
                  <CardBody maxW="65ch" p={{ base: 5, md: 6 }}>
                    <HStack justify="space-between" align="start">
                      <Text color="gray.900" fontWeight="800" fontSize="xl">{it.k}</Text>
                      <Badge colorScheme="gray" borderRadius="full" px={3} py={1}>{it.t}</Badge>
                    </HStack>
                    <Text mt={4} color="gray.700" fontSize="sm" lineHeight="1.9">{it.d}</Text>
                  </CardBody>
                </Card>
              ))}
            </SimpleGrid>
            <SimpleGrid columns={{ base: 1, md: 3 }} spacing={4} mt={8} mb={{ base: 14, md: 18 }}>
              <Button onClick={() => router.push("/signin")} w="full" h="auto" py={4} px={5} borderRadius="xl"
                bg="rgba(255,255,255,0.92)" border="1px solid rgba(255,255,255,0.55)" backdropFilter="blur(18px)"
                boxShadow="0 10px 24px rgba(0,0,0,0.14)" justifyContent="flex-start" textAlign="left" whiteSpace="normal"
                _hover={{ transform: "translateY(-2px)", boxShadow: "0 16px 40px rgba(0,0,0,0.18)" }}>
                <VStack align="start" spacing={2} w="full">
                  <HStack w="full" justify="space-between">
                    <Text fontWeight="800" fontSize="lg" color="gray.900">立即登录</Text>
                    <Badge bg="blue.50" color="blue.700" borderRadius="full" px={3} py={1}>推荐</Badge>
                  </HStack>
                  <Text color="gray.600" fontSize="sm">进入工作台，从招标解析开始体验完整流程。</Text>
                </VStack>
              </Button>
              <Button onClick={() => router.push("/signup")} w="full" h="auto" py={4} px={5} borderRadius="xl"
                bg="rgba(255,255,255,0.92)" border="1px solid rgba(255,255,255,0.55)" backdropFilter="blur(18px)"
                boxShadow="0 10px 24px rgba(0,0,0,0.14)" justifyContent="flex-start" textAlign="left" whiteSpace="normal"
                _hover={{ transform: "translateY(-2px)", boxShadow: "0 16px 40px rgba(0,0,0,0.18)" }}>
                <VStack align="start" spacing={2} w="full">
                  <HStack w="full" justify="space-between">
                    <Text fontWeight="800" fontSize="lg" color="gray.900">注册账号</Text>
                    <Badge bg="purple.50" color="purple.700" borderRadius="full" px={3} py={1}>新用户</Badge>
                  </HStack>
                  <Text color="gray.600" fontSize="sm">快速创建企业账号，沉淀资质与业绩资产。</Text>
                </VStack>
              </Button>
              <Button onClick={() => handleProtectedNav("/")} w="full" h="auto" py={4} px={5} borderRadius="xl"
                bg="rgba(255,255,255,0.92)" border="1px solid rgba(255,255,255,0.55)" backdropFilter="blur(18px)"
                boxShadow="0 10px 24px rgba(0,0,0,0.14)" justifyContent="flex-start" textAlign="left" whiteSpace="normal"
                _hover={{ transform: "translateY(-2px)", boxShadow: "0 16px 40px rgba(0,0,0,0.18)" }}>
                <VStack align="start" spacing={2} w="full">
                  <HStack w="full" justify="space-between">
                    <Text fontWeight="800" fontSize="lg" color="gray.900">查看首页介绍</Text>
                    <Badge bg="gray.100" color="gray.700" borderRadius="full" px={3} py={1}>了解更多</Badge>
                  </HStack>
                  <Text color="gray.600" fontSize="sm">查看系统概览与模块入口（需先登录）。</Text>
                </VStack>
              </Button>
            </SimpleGrid>
          </Flex>
        </Container>
      </Box>

      <Box h={{ base: 10, md: 14 }}
        bgGradient="linear(to-b, rgba(0,0,0,0.16), rgba(0,0,0,0.28))"
        borderTop="1px solid rgba(255,255,255,0.14)" borderBottom="1px solid rgba(255,255,255,0.10)" />

      <Box py={{ base: 10, md: 12 }}
        bgGradient="linear(to-b, rgba(0,0,0,0.18), rgba(0,0,0,0.38))"
        backdropFilter="blur(14px)" borderTop="1px solid rgba(255,255,255,0.16)">
        <Container w="full" maxW="none" px={AUTH_PAGE_GUTTERS}>
          <Flex justify="space-between" align={{ base: "flex-start", md: "center" }} gap={6} wrap="wrap">
            <Text color="gray.200" fontSize="sm" fontWeight="600">标擎 · 智慧投标</Text>
            <Flex gap={5} wrap="wrap">
              {["法律声明", "Cookies 政策", "隐私政策", "廉正举报", "安全举报", "联系我们"].map((t) => (
                <Button key={t} as="a" href="#" variant="link" color="gray.200" fontSize="sm" fontWeight="500">{t}</Button>
              ))}
            </Flex>
          </Flex>
          <Divider my={6} borderColor="whiteAlpha.200" />
          <Text mt={6} color="gray.400" fontSize="xs">© {new Date().getFullYear()} 标擎</Text>
        </Container>
      </Box>
    </Flex>
  );
}
