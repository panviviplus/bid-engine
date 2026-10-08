"use client";

import { Box, Grid, GridItem, useMediaQuery } from "@chakra-ui/react";
import {
  useState,
  useCallback,
  createContext,
  useContext,
  useEffect,
  useMemo,
  useRef,
} from "react";
import LeftNav from "@/components/layout/left-nav";
import { useAppContext } from "@/contexts/app-context";
import {
  readSidebarPreference,
  writeSidebarPreference,
} from "@/components/layout/sidebar-preference.mjs";
import { getResponsivePageLayout } from "@/components/layout/responsive-layout.mjs";

const SIDEBAR_EXPANDED = "15rem";
const SIDEBAR_COLLAPSED = "4rem";

const SidebarCollapseContext = createContext({
  collapsed: true,
  toggleSidebar: () => {},
  collapseForDetail: () => {},
  restoreSidebar: () => {},
});

export function useSidebarCollapse() {
  return useContext(SidebarCollapseContext);
}

function Layout({ children }) {
  const { shellContent } = getResponsivePageLayout();
  const { userProfile = {} } = useAppContext();
  const userId = userProfile?.id || "";
  // 默认折叠；用户信息加载完成后按该用户偏好回填。
  const [collapsed, setCollapsed] = useState(true);
  const [isNarrowViewport] = useMediaQuery("(max-width: 47.99em)");
  // 本次会话用户是否已手动切换过侧栏：手动选择优先，避免被偏好回填覆盖
  const hasUserToggledRef = useRef(false);
  const prevUserIdRef = useRef("");
  const collapsedRef = useRef(true);
  const restoreValueRef = useRef(true);

  // 用户信息加载完成或切换账号后，应用该用户的侧栏偏好；
  // 无记录时保持默认折叠；本次会话已手动切换过则尊重手动选择。
  useEffect(() => {
    const prevUserId = prevUserIdRef.current;
    prevUserIdRef.current = userId;
    if (!userId || userId === prevUserId) return;
    if (prevUserId) hasUserToggledRef.current = false;
    if (hasUserToggledRef.current) return;
    const stored = readSidebarPreference(userId);
    setCollapsed(stored === null ? true : stored);
  }, [userId]);

  // 同步最新 collapsed，供 collapseForDetail 读取进入详情页前的状态
  useEffect(() => {
    collapsedRef.current = collapsed;
  }, [collapsed]);

  useEffect(() => {
    if (isNarrowViewport) setCollapsed(true);
  }, [isNarrowViewport]);

  // 用户手动切换：唯一写入偏好的入口
  const toggleSidebar = useCallback(() => {
    hasUserToggledRef.current = true;
    setCollapsed((prev) => {
      const next = !prev;
      writeSidebarPreference(userId, next);
      return next;
    });
  }, [userId]);

  // 详情页进入时自动折叠（不写入偏好）
  const collapseForDetail = useCallback(() => {
    restoreValueRef.current = collapsedRef.current;
    setCollapsed(true);
  }, []);

  // 离开详情页时恢复进入前的状态（不写入偏好）
  const restoreSidebar = useCallback(() => {
    setCollapsed(restoreValueRef.current);
  }, []);

  const sidebarCollapseValue = useMemo(
    () => ({ collapsed, toggleSidebar, collapseForDetail, restoreSidebar }),
    [collapsed, toggleSidebar, collapseForDetail, restoreSidebar],
  );

  return (
    <SidebarCollapseContext.Provider value={sidebarCollapseValue}>
      <Box w="full" h="100dvh" bg="workbench.canvas" overflow="hidden">
        <Grid
          w="full"
          h="full"
          templateAreas='"sidebar main"'
          gridTemplateColumns={`${collapsed ? SIDEBAR_COLLAPSED : SIDEBAR_EXPANDED} minmax(0, 1fr)`}
          transition="grid-template-columns 0.25s cubic-bezier(0.4, 0, 0.2, 1)"
        >
          <GridItem
            area="sidebar"
            zIndex={20}
            minW={0}
            bg="workbench.control"
            overflow="hidden"
          >
            <LeftNav isCollapsed={collapsed} onToggleCollapse={toggleSidebar} />
          </GridItem>

          <GridItem
            area="main"
            w="full"
            minW={0}
            minH={0}
            h="full"
            overflow="auto"
            bg="workbench.canvas"
          >
            <Box
              w={shellContent.width}
              h={shellContent.height}
              minW={shellContent.minWidth}
              minH={shellContent.minHeight}
            >
              {children}
            </Box>
          </GridItem>
        </Grid>
      </Box>
    </SidebarCollapseContext.Provider>
  );
}

export default Layout;
