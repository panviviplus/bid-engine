"use client";

import {
  Badge,
  Flex,
  Text,
  Grid,
  GridItem,
  Box,
  Link,
  Tooltip,
  Popover,
  PopoverTrigger,
  PopoverContent,
  PopoverBody,
  PopoverArrow,
  HStack,
  Avatar,
  Center,
  Divider,
  IconButton,
  Menu,
  MenuButton,
  MenuItem,
  MenuList,
  Portal,
  Image,
} from "@chakra-ui/react";
import {
  FiChevronLeft,
  FiChevronRight,
  FiChevronDown,
  FiChevronUp,
  FiEdit3,
  FiKey,
  FiLogOut,
  FiMessageCircle,
} from "react-icons/fi";
import { motion, MotionConfig, useReducedMotion } from "framer-motion";
import { usePathname, useRouter } from "next/navigation";
import { useState, useEffect } from "react";
import NextLink from "next/link";
import useAxios from "axios-hooks";
import { NAVIGATION_LIST } from "@/router";
import { useAppContext } from "@/contexts/app-context";
import { useCustomToast } from "@/hooks/useCustomToast";
import { USER_ROLE } from "@/types/user";
import { useIntelUnreadCount } from "@/service/intel";
import { findBestMatchHref, nodeIsActive } from "./nav-match.mjs";
import NavIcon from "./nav-icon";
import ProfileModal from "./ProfileModal";
import ChangePasswordModal from "./ChangePasswordModal";

const MotionBox = motion(Box);

// 导航角标：目前只有“订阅与提醒”使用（未读提醒数），30s 轮询由 hook 负责。
function NavBadge({ value }) {
  if (!value || value <= 0) return null;
  const text = value > 99 ? "99+" : String(value);
  return (
    <Badge
      position="relative"
      zIndex={2}
      ml="auto"
      mr={1}
      display="inline-flex"
      alignItems="center"
      justifyContent="center"
      flexShrink={0}
      minW="20px"
      h="20px"
      borderRadius="md"
      px={1.5}
      py={0}
      fontSize="11px"
      fontWeight="700"
      lineHeight="1"
      fontVariantNumeric="tabular-nums"
      bg="error.500"
      color="white"
      aria-label={`未读 ${value} 条`}
    >
      {text}
    </Badge>
  );
}

// 行级状态样式（与 theme/default.js token 对齐）
const ROW_HOVER_BG = "whiteAlpha.100";
const FOCUS_RING = "0 0 0 2px var(--chakra-colors-gold-400)";
const PILL_TRANSITION = {
  opacity: { duration: 0.15 },
  scale: { duration: 0.22, ease: [0.16, 1, 0.3, 1] },
};
// 导航 Link 通用样式：去除 Chakra Link 默认 hover 下划线
const NAV_LINK_STYLE = {
  display: "block",
  borderRadius: "lg",
  textDecoration: "none",
  _hover: { textDecoration: "none" },
  _focusVisible: { boxShadow: FOCUS_RING, outline: "none" },
};

// 选中态整行蓝渐变胶囊层（置于行内容之下，opacity + scale 过渡）
function NavPill({ show }) {
  const reduced = useReducedMotion();
  return (
    <MotionBox
      aria-hidden
      position="absolute"
      inset={0}
      borderRadius="lg"
      bg="primary.500"
      style={{
        boxShadow: "0 4px 14px rgba(30, 58, 95, 0.30)",
        pointerEvents: "none",
      }}
      initial={false}
      animate={{
        opacity: show ? 1 : 0,
        scale: !reduced && !show ? 0.96 : 1,
      }}
      transition={PILL_TRANSITION}
      zIndex={0}
    />
  );
}

// 行内图标容器：颜色随行状态切换，hover 时轻微放大。
// 胶囊行（叶子/折叠父级/反馈/浮层子级）激活时图标为白色；
// 展开态父级无胶囊，需用 activeColor 指定图标激活色，避免白底白图标。
function NavIconSlot({ active, activeColor = "white", isCollapsed, children }) {
  return (
    <Flex
      position="relative"
      zIndex={1}
      align="center"
      mr={isCollapsed ? 0 : 2.5}
      color={active ? activeColor : "whiteAlpha.600"}
      transition="color 0.15s ease-out, transform 0.15s ease-out"
      _groupHover={
        active ? undefined : { color: "white", transform: "scale(1.06)" }
      }
    >
      {children}
    </Flex>
  );
}

function NavNode({
  nav,
  depth = 0,
  parentIndex = "",
  openMap,
  setOpenMap,
  asPath,
  bestHref,
  isCollapsed,
  badges = {},
}) {
  const { href, label, icon, children, badge } = nav;
  const badgeValue = badge ? badges[badge] || 0 : 0;
  const nodeKey = parentIndex ? `${parentIndex}-${label}` : label;
  const isOpen = !!openMap[nodeKey];
  const active = nodeIsActive(nav, asPath, bestHref);

  useEffect(() => {
    if (active && !isOpen) {
      setOpenMap((prev) => ({ ...prev, [nodeKey]: true }));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [active, nodeKey]);

  const rowPl = isCollapsed ? 0 : `${0.8 + depth * 2.5}rem`;
  const rowJustify = isCollapsed ? "center" : "flex-start";
  const rowPx = isCollapsed ? 2 : undefined;

  if (!children?.length) {
    const row = (
      <Link as={NextLink} href={href} aria-label={label} {...NAV_LINK_STYLE}>
        <Flex
          role="group"
          position="relative"
          align="center"
          h="44px"
          pl={rowPl}
          px={rowPx}
          mx={1.5}
          my={0.5}
          justify={rowJustify}
          borderRadius="lg"
          cursor="pointer"
          transition="transform 0.08s ease-out, color 0.15s ease-out, background-color 0.15s ease-out"
          fontSize="sm"
          fontWeight={active ? "600" : "400"}
          color={active ? "white" : "whiteAlpha.700"}
          _hover={active ? undefined : { bg: ROW_HOVER_BG, color: "white" }}
          _active={{ transform: "scale(0.98)" }}
        >
          <NavPill show={active} />
          {icon && (
            <NavIconSlot active={active} isCollapsed={isCollapsed}>
              <NavIcon name={icon} isActive={active} />
            </NavIconSlot>
          )}
          {!isCollapsed && (
            <Text position="relative" zIndex={1} noOfLines={1}>
              {label}
            </Text>
          )}
          {!isCollapsed && <NavBadge value={badgeValue} />}
        </Flex>
      </Link>
    );
    return isCollapsed ? (
      <Tooltip label={label} placement="right" openDelay={800}>
        <Box>{row}</Box>
      </Tooltip>
    ) : (
      row
    );
  }

  // Parent node with children
  if (isCollapsed) {
    return (
      <Popover
        placement="right-start"
        trigger="hover"
        openDelay={150}
        gutter={8}
      >
        <PopoverTrigger>
          <Flex
            data-group
            position="relative"
            align="center"
            justify="center"
            h="44px"
            mx={1.5}
            my={0.5}
            borderRadius="lg"
            tabIndex={0}
            role="button"
            cursor="pointer"
            transition="transform 0.08s ease-out, color 0.15s ease-out, background-color 0.15s ease-out"
            color={active ? "white" : "whiteAlpha.600"}
            _hover={active ? undefined : { bg: ROW_HOVER_BG, color: "white" }}
            _active={{ transform: "scale(0.98)" }}
            _focusVisible={{ boxShadow: FOCUS_RING, outline: "none" }}
          >
            <NavPill show={active} />
            {icon && (
              <NavIconSlot active={active} isCollapsed>
                <NavIcon name={icon} isActive={active} />
              </NavIconSlot>
            )}
          </Flex>
        </PopoverTrigger>
        <PopoverContent
          w="180px"
          _focus={{ boxShadow: "none", outline: "none" }}
          border="1px solid"
          borderColor="neutral.100"
          boxShadow="0 8px 24px rgba(0,0,0,0.1)"
          bg="white"
          borderRadius="xl"
        >
          <PopoverArrow bg="white" />
          <PopoverBody py={2}>
            <Text
              fontSize="sm"
              fontWeight="600"
              mb={2}
              px={2}
              color="neutral.800"
            >
              {label}
            </Text>
            {children.map((child, idx) => {
              const childActive = nodeIsActive(child, asPath, bestHref);
              return (
                <Link
                  key={idx}
                  as={NextLink}
                  href={child.href}
                  aria-label={child.label}
                  {...NAV_LINK_STYLE}
                >
                  <Flex
                    role="group"
                    position="relative"
                    align="center"
                    h="44px"
                    px={2}
                    mx={1}
                    my={0.5}
                    borderRadius="lg"
                    fontSize="sm"
                    fontWeight={childActive ? "600" : "400"}
                    color={childActive ? "white" : "neutral.600"}
                    transition="transform 0.08s ease-out, color 0.15s ease-out, background-color 0.15s ease-out"
                    _hover={
                      childActive
                        ? undefined
                        : { bg: "neutral.50", color: "neutral.900" }
                    }
                    _active={{ transform: "scale(0.98)" }}
                  >
                    <NavPill show={childActive} />
                    <Text position="relative" zIndex={1} noOfLines={1}>
                      {child.label}
                    </Text>
                    <NavBadge
                      value={child.badge ? badges[child.badge] || 0 : 0}
                    />
                  </Flex>
                </Link>
              );
            })}
          </PopoverBody>
        </PopoverContent>
      </Popover>
    );
  }

  return (
    <>
      <Flex
        data-group
        position="relative"
        align="center"
        justify="space-between"
        h="44px"
        pl="0.8rem"
        mx={1.5}
        my={0.5}
        borderRadius="lg"
        tabIndex={0}
        role="button"
        aria-expanded={isOpen}
        cursor="pointer"
        fontSize="sm"
        fontWeight={active ? "600" : "500"}
        color={active ? "gold.300" : "whiteAlpha.700"}
        onClick={() =>
          setOpenMap((prev) => ({ ...prev, [nodeKey]: !prev[nodeKey] }))
        }
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            setOpenMap((prev) => ({ ...prev, [nodeKey]: !prev[nodeKey] }));
          }
        }}
        transition="transform 0.08s ease-out, color 0.15s ease-out, background-color 0.15s ease-out"
        _hover={
          active ? { color: "gold.300" } : { bg: ROW_HOVER_BG, color: "white" }
        }
        _active={{ transform: "scale(0.98)" }}
        _focusVisible={{ boxShadow: FOCUS_RING, outline: "none" }}
      >
        <Flex position="relative" zIndex={1} align="center">
          {icon && (
            <NavIconSlot
              active={active}
              activeColor="gold.300"
              isCollapsed={false}
            >
              <NavIcon name={icon} isActive={active} />
            </NavIconSlot>
          )}
          <Text>{label}</Text>
        </Flex>
        <Box
          position="relative"
          zIndex={1}
          mr={2}
          color={active ? "gold.300" : "whiteAlpha.500"}
          transition="transform 0.2s ease-in-out"
          transform={isOpen ? "rotate(180deg)" : undefined}
        >
          <FiChevronDown size={16} />
        </Box>
      </Flex>
      <Grid
        templateRows={isOpen ? "1fr" : "0fr"}
        overflow="hidden"
        transition="grid-template-rows 0.2s cubic-bezier(0.65, 0, 0.35, 1)"
      >
        <GridItem overflow="hidden">
          {children.map((child, idx) => (
            <Box key={idx}>
              <NavNode
                key={idx}
                nav={child}
                depth={depth + 1}
                parentIndex={nodeKey}
                openMap={openMap}
                setOpenMap={setOpenMap}
                asPath={asPath}
                bestHref={bestHref}
                isCollapsed={isCollapsed}
                badges={badges}
              />
            </Box>
          ))}
        </GridItem>
      </Grid>
    </>
  );
}

function UserActionsMenu({
  isCollapsed,
  userProfile,
  feedbackActive,
  onEditProfile,
  onChangePassword,
  onLogout,
  isLoggingOut,
}) {
  const userName = userProfile?.name || "用户";
  const avatar = (
    <Avatar
      size="sm"
      name={userName}
      src={userProfile?.avatarUrl}
      border="2px solid"
      borderColor="whiteAlpha.300"
      bg="primary.600"
      color="white"
      pointerEvents="none"
    />
  );

  return (
    <Menu
      autoSelect={false}
      isLazy
      placement={isCollapsed ? "right-end" : "top-end"}
      gutter={10}
    >
      {isCollapsed ? (
        <Center minH="64px">
          <MenuButton
            as={IconButton}
            aria-label={`${userName}，展开用户菜单`}
            icon={avatar}
            variant="ghost"
            h="44px"
            w="44px"
            minW="44px"
            p={0}
            borderRadius="full"
            _hover={{ bg: "whiteAlpha.100" }}
            _active={{ bg: "whiteAlpha.200", transform: "translateY(1px)" }}
            _focusVisible={{ boxShadow: FOCUS_RING, outline: "none" }}
          />
        </Center>
      ) : (
        <Flex minH="72px" px={3} py={2.5} align="center" gap={2} minW={0}>
          <HStack spacing={2.5} flex={1} minW={0}>
            {avatar}
            <Text
              minW={0}
              color="white"
              fontSize="sm"
              fontWeight="600"
              noOfLines={1}
            >
              {userName}
            </Text>
          </HStack>
          <MenuButton
            as={IconButton}
            aria-label="展开用户菜单"
            icon={<FiChevronUp size={18} />}
            variant="ghost"
            h="44px"
            w="44px"
            minW="44px"
            color="whiteAlpha.700"
            borderRadius="lg"
            _hover={{ bg: "whiteAlpha.100", color: "white" }}
            _active={{ bg: "whiteAlpha.200", transform: "translateY(1px)" }}
            _focusVisible={{ boxShadow: FOCUS_RING, outline: "none" }}
          />
        </Flex>
      )}

      <Portal>
        <MenuList
          minW="216px"
          px={2}
          py={2}
          bg="workbench.paper"
          color="neutral.700"
          border="1px solid"
          borderColor="workbench.line"
          borderRadius="xl"
          boxShadow="0 16px 36px rgba(3, 17, 30, 0.18)"
          zIndex={40}
        >
          <MenuItem
            icon={<FiEdit3 size={16} />}
            minH="44px"
            borderRadius="lg"
            onClick={onEditProfile}
            _hover={{ bg: "primary.50", color: "primary.700" }}
            _focus={{ bg: "primary.50", color: "primary.700" }}
          >
            编辑资料
          </MenuItem>
          <MenuItem
            icon={<FiKey size={16} />}
            minH="44px"
            borderRadius="lg"
            onClick={onChangePassword}
            _hover={{ bg: "primary.50", color: "primary.700" }}
            _focus={{ bg: "primary.50", color: "primary.700" }}
          >
            修改密码
          </MenuItem>
          <MenuItem
            as={NextLink}
            href="/file-feedback"
            icon={<FiMessageCircle size={16} />}
            minH="44px"
            borderRadius="lg"
            bg={feedbackActive ? "primary.50" : undefined}
            color={feedbackActive ? "primary.700" : undefined}
            fontWeight={feedbackActive ? "600" : "400"}
            aria-current={feedbackActive ? "page" : undefined}
            _hover={{ bg: "primary.50", color: "primary.700" }}
            _focus={{ bg: "primary.50", color: "primary.700" }}
          >
            我要反馈
          </MenuItem>
          <Divider my={2} borderColor="workbench.line" />
          <MenuItem
            icon={<FiLogOut size={16} />}
            minH="44px"
            borderRadius="lg"
            color="error.600"
            isDisabled={isLoggingOut}
            onClick={onLogout}
            _hover={{ bg: "error.50" }}
            _focus={{ bg: "error.50" }}
          >
            {isLoggingOut ? "退出中…" : "退出登录"}
          </MenuItem>
        </MenuList>
      </Portal>
    </Menu>
  );
}

export default function LeftNav({ children, isCollapsed, onToggleCollapse }) {
  const pathname = usePathname();
  const router = useRouter();
  const showToast = useCustomToast();
  const [openMap, setOpenMap] = useState({});
  const [isProfileOpen, setIsProfileOpen] = useState(false);
  const [isPwdOpen, setIsPwdOpen] = useState(false);
  const [isSidebarHovered, setIsSidebarHovered] = useState(false);
  const { userProfile = {}, mutateUserProfile } = useAppContext();
  const { isMember, isCompanyOwner, isAdmin } = userProfile;
  const [{ loading: isLoggingOut }, logout] = useAxios(
    { method: "GET", url: "/logout" },
    { manual: true },
  );
  // “订阅与提醒”的未读角标：30s 轮询，不阻塞导航渲染
  const { unread } = useIntelUnreadCount();
  const navBadges = { intel_unread: unread };

  const handleLogout = async () => {
    try {
      const response = await logout();
      if (response.data.code === 0) {
        router.push("/signin");
        return;
      }
      showToast({ title: response.data.message || "服务异常，请稍后再试。" });
    } catch {
      showToast({ status: "error", title: "服务异常，请稍后再试。" });
    }
  };

  const filterNav = () => {
    const deepFilter = (nodes, predicate) =>
      nodes
        .map((n) => {
          if (!predicate(n)) return null;
          if (n.children?.length) {
            const filteredChildren = deepFilter(n.children, predicate);
            if (!filteredChildren.length && !n.href) return null;
            return { ...n, children: filteredChildren };
          }
          return n;
        })
        .filter(Boolean);

    const hideSystemForNonAdmin = (n) => {
      if (n?.label === "系统管理") return Boolean(isAdmin);
      if (typeof n?.href === "string" && n.href.startsWith("/system"))
        return Boolean(isAdmin);
      return true;
    };
    if (isMember) {
      return deepFilter(
        NAVIGATION_LIST,
        (n) =>
          hideSystemForNonAdmin(n) && !n.denyRoles?.includes(USER_ROLE.MEMBER),
      );
    }
    if (isCompanyOwner) {
      return deepFilter(
        NAVIGATION_LIST,
        (n) =>
          hideSystemForNonAdmin(n) &&
          !n.denyRoles?.includes(USER_ROLE.OWNER.COMPANY),
      );
    }
    return deepFilter(NAVIGATION_LIST, hideSystemForNonAdmin);
  };

  const navList = filterNav();
  const bestHref = findBestMatchHref(navList, pathname);
  const feedbackActive =
    pathname === "/file-feedback" || pathname?.startsWith("/file-feedback/");

  return (
    <>
      <Flex
        className="app-sidebar"
        direction="column"
        h="full"
        bg="workbench.control"
        borderRight="1px solid"
        borderColor="whiteAlpha.200"
        w="full"
        overflow="hidden"
        onMouseEnter={() => setIsSidebarHovered(true)}
        onMouseLeave={() => setIsSidebarHovered(false)}
      >
        {/* Logo 区域 */}
        <Flex
          h="16"
          align="center"
          justify={isCollapsed ? "center" : "space-between"}
          px={isCollapsed ? 2 : 3}
          borderBottom="1px solid"
          borderColor="whiteAlpha.200"
          flexShrink={0}
        >
          <Link
            as={NextLink}
            href="/"
            aria-label="返回标擎首页"
            minH="44px"
            minW={0}
            flex={isCollapsed ? undefined : 1}
            display="flex"
            alignItems="center"
            gap={2.5}
            px={2}
            borderRadius="lg"
            textDecoration="none"
            _hover={{ bg: "whiteAlpha.100", textDecoration: "none" }}
            _active={{ transform: "translateY(1px)" }}
            _focusVisible={{ boxShadow: FOCUS_RING, outline: "none" }}
          >
            <Image
              src="/bid-engine-mark.png"
              alt=""
              boxSize="36px"
              borderRadius="md"
              objectFit="cover"
              flexShrink={0}
            />
            {!isCollapsed && (
              <Text
                color="white"
                fontSize="lg"
                fontWeight="800"
                letterSpacing="tight"
                noOfLines={1}
              >
                标擎
              </Text>
            )}
          </Link>
          {!isCollapsed && (
            <Tooltip label="收起侧栏" placement="bottom" openDelay={800}>
              <IconButton
                className="sidebar-collapse-toggle"
                aria-label="收起侧栏"
                icon={<FiChevronLeft size={18} />}
                variant="ghost"
                h="44px"
                w="44px"
                minW="44px"
                ml={2}
                flexShrink={0}
                color="whiteAlpha.700"
                borderRadius="lg"
                onClick={onToggleCollapse}
                transition="transform 0.08s ease-out, color 0.15s ease-out, background-color 0.15s ease-out"
                _hover={{ bg: ROW_HOVER_BG, color: "white" }}
                _active={{ transform: "translateY(1px)" }}
                _focusVisible={{ boxShadow: FOCUS_RING, outline: "none" }}
              />
            </Tooltip>
          )}
        </Flex>

        {/* 导航列表 */}
        <MotionConfig reducedMotion="user">
          <Box flex={1} overflowY="auto" py={2} className="thin-scrollbars-mid">
            {isCollapsed && (
              <Flex
                className="sidebar-collapse-toggle"
                as="button"
                type="button"
                aria-label="展开侧栏"
                aria-expanded={false}
                minH="44px"
                w="calc(100% - 12px)"
                mx={1.5}
                mb={1}
                px={2}
                align="center"
                justify="center"
                gap={2.5}
                border="1px solid"
                borderRadius="lg"
                bg={isSidebarHovered ? "gold.400" : "transparent"}
                color={
                  isSidebarHovered ? "workbench.control" : "whiteAlpha.700"
                }
                borderColor={isSidebarHovered ? "gold.400" : "transparent"}
                cursor="pointer"
                onClick={onToggleCollapse}
                transition="transform 0.08s ease-out, color 0.15s ease-out, background-color 0.15s ease-out, border-color 0.15s ease-out"
                _hover={{
                  bg: "gold.400",
                  color: "workbench.control",
                  borderColor: "gold.400",
                }}
                _active={{ transform: "translateY(1px)" }}
                _focusVisible={{ boxShadow: FOCUS_RING, outline: "none" }}
              >
                <FiChevronRight size={18} />
              </Flex>
            )}
            {navList?.map((nav, idx) => (
              <Box key={idx} w="full">
                <NavNode
                  nav={nav}
                  openMap={openMap}
                  setOpenMap={setOpenMap}
                  asPath={pathname}
                  bestHref={bestHref}
                  isCollapsed={isCollapsed}
                  badges={navBadges}
                />
              </Box>
            ))}
          </Box>

          <Box mt="auto" borderTop="1px solid" borderColor="whiteAlpha.200">
            <UserActionsMenu
              isCollapsed={isCollapsed}
              userProfile={userProfile}
              feedbackActive={feedbackActive}
              onEditProfile={() => setIsProfileOpen(true)}
              onChangePassword={() => setIsPwdOpen(true)}
              onLogout={handleLogout}
              isLoggingOut={isLoggingOut}
            />
          </Box>
        </MotionConfig>

        {children}
      </Flex>

      <ProfileModal
        isOpen={isProfileOpen}
        onClose={() => setIsProfileOpen(false)}
        userProfile={userProfile}
        onSaved={mutateUserProfile}
      />
      <ChangePasswordModal
        isOpen={isPwdOpen}
        onClose={() => setIsPwdOpen(false)}
      />
    </>
  );
}
