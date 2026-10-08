import { extendTheme } from "@chakra-ui/react";

// ============================================================
// Smart-Bid Professional Design System
// 专业、人性化、智能化的投标行业配色与组件体系
// ============================================================

const colors = {
  // ---- 深海智能工作台语义表面 ----
  workbench: {
    canvas: "#F2F5F7",
    paper: "#FFFFFF",
    control: "#0B1B2B",
    controlRaised: "#132B40",
    line: "#DCE4EA",
    text: "#182636",
    muted: "#66798B",
    signal: "#D4A853",
    evidence: "#E5B94F",
  },
  // ---- SignIn 深色品牌叙事表面（V9 冻结设计） ----
  signin: {
    canvas: "#07111F",
    panel: "rgba(11, 28, 48, 0.82)",
    surface: "#132842",
    surfaceStrong: "#19314F",
    paper: "#F8FAFC",
    text: "#F8FAFC",
    textMuted: "#B9C7D8",
    line: "rgba(255, 255, 255, 0.14)",
    bloomPrimary: "rgba(30, 58, 95, 0.42)",
    bloomSecondary: "rgba(8, 145, 178, 0.22)",
    glass: "rgba(255, 255, 255, 0.12)",
    glassHover: "rgba(255, 255, 255, 0.18)",
  },
  // ---- 主色体系：深海蓝 (Professional Navy) ----
  primary: {
    50: "#F0F4FA",
    100: "#D9E2F3",
    200: "#B3C5E7",
    300: "#8DA8DB",
    400: "#668BCF",
    500: "#406EC3",
    600: "#1E3A5F", // 主色 — 深蓝沉稳
    700: "#172E4C",
    800: "#102239",
    900: "#0A1626",
  },

  // ---- 辅助色：金色点缀 (Gold Accent) ----
  gold: {
    50: "#FDF8F0",
    100: "#F9EDD5",
    200: "#F3DBAB",
    300: "#EDC981",
    400: "#E7B757",
    500: "#D4A853", // 主金 — 用于 AI badge, CTA
    600: "#B8923A",
    700: "#9C7B2D",
    800: "#806520",
    900: "#644E13",
  },

  // ---- 功能色体系 ----
  success: {
    50: "#ECFDF5",
    100: "#D1FAE5",
    200: "#A7F3D0",
    300: "#6EE7B7",
    400: "#34D399",
    500: "#059669",
    600: "#047857",
    700: "#065F46",
    800: "#064E3B",
    900: "#022C22",
  },
  warning: {
    50: "#FFFBEB",
    100: "#FEF3C7",
    200: "#FDE68A",
    300: "#FCD34D",
    400: "#FBBF24",
    500: "#D97706",
    600: "#B45309",
    700: "#92400E",
    800: "#78350F",
    900: "#451A03",
  },
  error: {
    50: "#FFF1F2",
    100: "#FFE4E6",
    200: "#FECDD3",
    300: "#FDA4AF",
    400: "#FB7185",
    500: "#DC2626",
    600: "#B91C1C",
    700: "#991B1B",
    800: "#7F1D1D",
    900: "#450A0A",
  },
  info: {
    50: "#F0F9FF",
    100: "#E0F2FE",
    200: "#BAE6FD",
    300: "#7DD3FC",
    400: "#38BDF8",
    500: "#0284C7",
    600: "#0369A1",
    700: "#075985",
    800: "#0C4A6E",
    900: "#082F49",
  },

  // ---- 中性色：Slate 系 ----
  neutral: {
    50: "#F8FAFC",
    100: "#F1F5F9",
    200: "#E2E8F0",
    300: "#CBD5E1",
    400: "#94A3B8",
    500: "#64748B",
    600: "#475569",
    700: "#334155",
    800: "#1E293B",
    900: "#0F172A",
    r85: "rgba(15, 23, 42, 0.85)",
    r65: "rgba(15, 23, 42, 0.65)",
  },

  // ---- AI 品牌渐变色 ----
  brand: {
    50: "#F0F4FA",
    100: "#D9E2F3",
    200: "#B3C5E7",
    300: "#8DA8DB",
    400: "#668BCF",
    500: "#406EC3",
    600: "#1E3A5F",
    700: "#172E4C",
    800: "#102239",
    900: "#0A1626",
    gradient: "linear-gradient(135deg, #1E3A5F 0%, #406EC3 100%)",
    glow: "0 0 16px rgba(30, 58, 95, 0.2)",
    aiGradient: "linear-gradient(135deg, #1E3A5F 0%, #D4A853 100%)",
  },

  // ---- 保持兼容的旧 key (逐步迁移后可废弃) ----
  blue: {
    50: "#F0F4FA",
    100: "#D9E2F3",
    200: "#B3C5E7",
    300: "#8DA8DB",
    400: "#668BCF",
    500: "#406EC3",
    600: "#1E3A5F",
    700: "#172E4C",
    800: "#102239",
    900: "#0A1626",
  },
  orange: {
    100: "#ffefcbff",
  },
  cyan: {
    50: "#ECFEFF",
    100: "#CFFAFE",
    200: "#A5F3FC",
    300: "#67E8F9",
    400: "#22D3EE",
    500: "#06B6D4",
    600: "#0891B2",
    700: "#0E7490",
    800: "#155E75",
    900: "#164E63",
  },
  purple: {
    100: "#EDE9FE",
    500: "#A78BFA",
    600: "#8B5CF6",
    700: "#7C3AED",
  },

  // ---- 辅助语义色 (保持兼容) ----
  O: {
    200: "#FFF7ED",
    300: "#FFEDD5",
    400: "#FED7AA",
    500: "#F97316",
    600: "#EA580C",
    650: "#C2410C",
    700: "#9A3412",
  },
  P: {
    200: "#F3E8FF",
    300: "#E9D5FF",
    400: "#D8B4FE",
    500: "#A855F7",
    600: "#9333EA",
    700: "#7E22CE",
  },
  R: {
    200: "#FEE2E2",
    300: "#FECACA",
    400: "#FCA5A5",
    500: "#EF4444",
    600: "#DC2626",
    700: "#B91C1C",
  },
  G: {
    200: "#DCFCE7",
    300: "#BBF7D0",
    400: "#86EFAC",
    500: "#22C55E",
    600: "#16A34A",
    700: "#15803D",
  },
  BR: {
    200: "#CCFBF1",
    300: "#99F6E4",
    400: "#5EEAD4",
    500: "#2DD4BF",
    600: "#14B8A6",
    700: "#0F766E",
  },
  Z: {
    100: "#F0F2F7",
    200: "#E2E7F0",
    300: "#A7B6D2",
    400: "#A7B6D2",
    500: "#899EC3",
    600: "#6B85B3",
    700: "#556A8F",
    800: "#404F6B",
    900: "#2A3547",
  },

  // ---- 按钮专用色 ----
  primaryButton: {
    50: "#F0F4FA",
    100: "#D9E2F3",
    200: "#B3C5E7",
    300: "#8DA8DB",
    400: "#668BCF",
    500: "#406EC3",
    600: "#1E3A5F",
    700: "#172E4C",
    800: "#102239",
  },

  // ---- 图标与特殊色 ----
  iconColor: "#94A3B8",
  checkIconColor: "#059669",
};

const shadows = {
  signinCard: "0 24px 58px rgba(0, 0, 0, 0.30)",
  signinGlow: "0 0 18px rgba(242, 188, 87, 0.90)",
  signinGlowSoft: "0 0 12px rgba(242, 188, 87, 0.60)",
  signinFloating: "0 18px 50px rgba(0, 0, 0, 0.25)",
};

const fonts = {
  body: 'Inter, PingFang SC, Helvetica Neue, Hiragino Sans GB, Microsoft YaHei, Arial, sans-serif, "Apple Color Emoji", "Segoe UI Emoji", "Segoe UI Symbol"',
  heading:
    'Inter, PingFang SC, Helvetica Neue, Hiragino Sans GB, Microsoft YaHei, Arial, sans-serif, "Apple Color Emoji", "Segoe UI Emoji", "Segoe UI Symbol"',
};

const textStyles = {
  pageTitle: {
    fontSize: "1.75rem",
    fontWeight: "700",
    letterSpacing: "-0.02em",
    color: "neutral.900",
  },
  sectionTitle: {
    fontSize: "1.25rem",
    fontWeight: "600",
    letterSpacing: "-0.01em",
    color: "neutral.800",
  },
  cardTitle: {
    fontSize: "1rem",
    fontWeight: "600",
    color: "neutral.800",
  },
  body: {
    fontSize: "0.875rem",
    fontWeight: "400",
    lineHeight: "1.75",
    color: "neutral.600",
  },
  caption: {
    fontSize: "0.75rem",
    fontWeight: "400",
    color: "neutral.400",
  },
  link: {
    color: "primary.600",
    _hover: {
      color: "primary.500",
      cursor: "pointer",
      textDecoration: "underline",
    },
  },
  // Hallmark · component: list design system · genre: modern-minimal · theme: Cobalt (adapted)
  // 统计条等宽数字（科技感：tabular-nums 对齐）
  statValue: {
    fontWeight: "700",
    fontVariantNumeric: "tabular-nums",
  },
  // 状态/元信息微标签（大写 + 宽字距，仪器面板感）
  monoLabel: {
    fontSize: "xs",
    letterSpacing: "0.08em",
    textTransform: "uppercase",
    color: "neutral.400",
    fontVariantNumeric: "tabular-nums",
  },
};

const layerStyles = {
  // 毛玻璃面板 — 仅用于 Header
  glass: {
    bg: "rgba(255, 255, 255, 0.85)",
    backdropFilter: "blur(8px)",
    borderBottom: "1px solid rgba(0, 0, 0, 0.04)",
  },
  // 标准卡片
  card: {
    bg: "white",
    borderRadius: "xl",
    boxShadow: "0 1px 3px rgba(0, 0, 0, 0.06)",
    border: "1px solid",
    borderColor: "neutral.100",
    transition: "all 0.2s ease",
    _hover: {
      boxShadow: "0 4px 12px rgba(0, 0, 0, 0.08)",
      borderColor: "primary.200",
    },
  },
  // 主按钮
  primarybutton: {
    color: "white",
    bg: "primary.600",
    borderRadius: "lg",
    fontWeight: "600",
    shadow: "sm",
    transition: "all 0.2s",
    _hover: {
      bg: "primary.700",
      shadow: "0 4px 12px rgba(30, 58, 95, 0.25)",
      transform: "translateY(-1px)",
    },
    _active: {
      bg: "primary.800",
      transform: "translateY(0)",
      shadow: "none",
    },
    _disabled: {
      bg: "neutral.100",
      color: "neutral.400",
      cursor: "not-allowed",
      boxShadow: "none",
      _hover: {
        transform: "none",
        boxShadow: "none",
      },
    },
  },
  // AI 按钮 — 金色渐变
  aiButton: {
    color: "white",
    bgGradient: "linear(to-r, primary.600, gold.500)",
    borderRadius: "lg",
    fontWeight: "600",
    shadow: "0 2px 8px rgba(30, 58, 95, 0.2)",
    transition: "all 0.2s",
    _hover: {
      bgGradient: "linear(to-r, primary.700, gold.600)",
      shadow: "0 4px 16px rgba(30, 58, 95, 0.3)",
      transform: "translateY(-1px)",
    },
    _active: {
      transform: "translateY(0)",
      shadow: "none",
    },
  },
  // 文字按钮
  textbutton: {
    color: "neutral.500",
    backgroundColor: "transparent",
    borderRadius: "lg",
    _hover: {
      color: "neutral.700",
      backgroundColor: "neutral.50",
    },
    _active: {
      color: "primary.600",
      backgroundColor: "primary.50",
    },
    _disable: {
      color: "neutral.300",
      backgroundColor: "transparent",
    },
  },
  // ===== Hallmark · component: list page tokens · genre: modern-minimal · theme: Cobalt (adapted) =====
  // 统计条 chip：圆点（状态主题色）+ 标签 + 等宽数字
  statChip: {
    align: "center",
    gap: 2,
    bg: "white",
    px: 3.5,
    py: 2,
    borderRadius: "full",
    boxShadow: "sm",
    fontSize: "sm",
    border: "1px solid",
    borderColor: "neutral.100",
  },
  // 胶囊分段筛选容器
  filterPillGroup: {
    gap: 1.5,
    bg: "white",
    p: 1,
    borderRadius: "full",
    boxShadow: "sm",
  },
  // 批量操作栏（多选模式浮层）
  batchBar: {
    align: "center",
    justify: "space-between",
    gap: 3,
    p: 3,
    mb: 4,
    bg: "white",
    borderRadius: "xl",
    border: "1px solid",
    borderColor: "primary.200",
    boxShadow: "0 4px 20px rgba(30, 58, 95, 0.10)",
  },
  // 卡片活动态顶部氛围条容器（渐变由组件以主题 token 指定）
  ambientTopBar: {
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    h: "3px",
    borderTopRadius: "xl",
    overflow: "hidden",
    pointerEvents: "none",
  },
};

const styles = {
  global: {
    html: {
      overflowX: "clip",
    },
    "@supports (-webkit-touch-callout: none)": {
      "#app": {
        height: "-webkit-fill-available",
        minHeight: "-webkit-fill-available",
      },
    },
    body: {
      bg: "neutral.50",
      overflowX: "clip",
      overflowY: "hidden",
      lineHeight: "1.75",
      color: "neutral.700",
      letterSpacing: "0.01em",
    },
    ".thin-scrollbars::-webkit-scrollbar": {
      backgroundColor: "transparent",
      w: "6px",
      h: "6px",
    },
    ".thin-scrollbars::-webkit-scrollbar-thumb": {
      backgroundColor: "neutral.300",
      borderRadius: "full",
    },
    ".thin-scrollbars-mid::-webkit-scrollbar": {
      backgroundColor: "transparent",
      w: "4px",
      h: "4px",
    },
    "::-webkit-scrollbar": {
      backgroundColor: "transparent",
      w: "6px",
      h: "6px",
    },
    "::-webkit-scrollbar-track": {
      backgroundColor: "transparent",
    },
    "::-webkit-scrollbar-thumb": {
      backgroundColor: "neutral.300",
      borderRadius: "full",
    },
    "::-webkit-scrollbar-thumb:hover": {
      backgroundColor: "neutral.400",
    },
    "&::-webkit-scrollbar-button": {
      display: "none",
    },
  },
};

const components = {
  Button: {
    baseStyle: {
      borderRadius: "lg",
      fontWeight: "600",
      transition: "all 0.2s ease",
    },
    variants: {
      solid: (props) => {
        const solidBg =
          {
            gray: "white",
            primary: "primary.600",
          }[props.colorScheme] || `${props.colorScheme}.600`;
        return {
          bg: solidBg,
          color: props.colorScheme === "gray" ? "neutral.700" : "white",
          border: props.colorScheme === "gray" ? "1px solid" : "none",
          borderColor: "neutral.200",
          boxShadow:
            props.colorScheme === "gray" ? "sm" : "0 1px 3px rgba(0,0,0,0.1)",
          _hover: {
            bg:
              props.colorScheme === "gray"
                ? "neutral.50"
                : `${props.colorScheme}.700`,
            shadow: "md",
            transform: "translateY(-1px)",
          },
          _active: {
            transform: "translateY(0)",
            shadow: "none",
          },
        };
      },
      ghost: {
        _hover: {
          bg: "neutral.100",
          color: "primary.600",
        },
      },
      outline: {
        borderColor: "neutral.200",
        color: "neutral.600",
        _hover: {
          bg: "neutral.50",
          borderColor: "neutral.300",
        },
      },
    },
  },
  Menu: {
    baseStyle: {
      list: {
        borderRadius: "xl",
        boxShadow:
          "0 10px 15px -3px rgba(0, 0, 0, 0.08), 0 4px 6px -2px rgba(0, 0, 0, 0.04)",
        border: "1px solid",
        borderColor: "neutral.100",
        p: 2,
        bg: "white",
      },
      item: {
        borderRadius: "md",
        transition: "all 0.1s",
        _hover: {
          bg: "primary.50",
          color: "primary.700",
        },
        _focus: {
          bg: "primary.50",
        },
      },
    },
  },
  Table: {
    variants: {
      simple: {
        th: {
          borderBottom: "2px solid",
          borderColor: "neutral.100",
          color: "neutral.500",
          fontSize: "xs",
          textTransform: "uppercase",
          letterSpacing: "wider",
          fontWeight: "600",
          bg: "neutral.50",
          py: 3,
          px: 4,
        },
        td: {
          borderBottom: "1px solid",
          borderColor: "neutral.50",
          fontSize: "sm",
          color: "neutral.700",
          py: 3,
          px: 4,
        },
        tr: {
          transition: "all 0.15s",
          _hover: {
            bg: "primary.50",
          },
        },
      },
    },
  },
  Badge: {
    baseStyle: {
      borderRadius: "full",
      px: 3,
      py: 0.5,
      textTransform: "none",
      fontWeight: "medium",
    },
  },
  Card: {
    baseStyle: {
      container: {
        bg: "white",
        borderRadius: "xl",
        boxShadow: "0 1px 3px rgba(0,0,0,0.06)",
        border: "1px solid",
        borderColor: "neutral.100",
      },
      header: {
        pb: 2,
      },
      body: {
        pt: 2,
      },
    },
  },
  Input: {
    baseStyle: {
      field: {
        borderRadius: "lg",
        borderColor: "neutral.200",
        _hover: {
          borderColor: "neutral.300",
        },
        _focus: {
          borderColor: "primary.400",
          boxShadow: "0 0 0 3px rgba(30, 58, 95, 0.1)",
        },
      },
    },
    // 键盘焦点环与 Select 保持同一套（Chakra 默认的 1px 蓝环不属于本设计系统）
    variants: {
      outline: {
        field: {
          _focusVisible: {
            borderColor: "primary.400",
            boxShadow: "0 0 0 3px rgba(30, 58, 95, 0.1)",
          },
        },
      },
    },
  },
  // Hallmark · component: dropdown trigger · genre: modern-minimal · theme: BidEngine deep-sea/gold
  // 统一口径：下拉触发器与同尺寸 Input 同构（高度/内边距/圆角/边框/焦点环），并有 hover·focus·disabled·invalid 状态。
  // Chakra 默认箭头是 20px 的 currentColor，在 24–32px 的紧凑下拉里最扎眼，这里按尺寸收敛为 12–16px 的 workbench.muted。
  Select: {
    baseStyle: {
      field: {
        bg: "workbench.paper",
        color: "workbench.text",
        fontWeight: "500",
        borderRadius: "lg",
        borderColor: "workbench.line",
        cursor: "pointer",
        transitionProperty: "border-color, box-shadow, background-color",
        transitionDuration: "150ms",
        transitionTimingFunction: "cubic-bezier(0.16, 1, 0.3, 1)",
        _hover: {
          borderColor: "neutral.300",
        },
        _focus: {
          borderColor: "primary.400",
          boxShadow: "0 0 0 3px rgba(30, 58, 95, 0.1)",
        },
        _disabled: {
          bg: "neutral.50",
          color: "neutral.400",
          opacity: 1,
          cursor: "not-allowed",
          _hover: {
            borderColor: "workbench.line",
          },
        },
      },
      icon: {
        color: "workbench.muted",
        insetEnd: "2.5",
        _disabled: {
          opacity: 0.4,
        },
      },
    },
    // variant 在 Chakra 的合并顺序里晚于 baseStyle，边框与状态必须在这里再声明一次才稳定生效
    variants: {
      outline: {
        field: {
          bg: "workbench.paper",
          borderColor: "workbench.line",
          _hover: {
            bg: "workbench.paper",
            borderColor: "neutral.300",
          },
          _focus: {
            borderColor: "primary.400",
            boxShadow: "0 0 0 3px rgba(30, 58, 95, 0.1)",
          },
          // Chakra 默认的 :focus-visible 是 1px 蓝色环，会盖掉设计系统的焦点样式
          _focusVisible: {
            borderColor: "primary.400",
            boxShadow: "0 0 0 3px rgba(30, 58, 95, 0.1)",
          },
          _disabled: {
            bg: "neutral.50",
            color: "neutral.400",
            opacity: 1,
            cursor: "not-allowed",
          },
        },
      },
    },
    sizes: {
      xs: {
        field: { paddingInlineEnd: "6" },
        icon: { fontSize: "xs", insetEnd: "2" },
      },
      sm: {
        field: { paddingInlineEnd: "7" },
        icon: { fontSize: "sm" },
      },
      md: {
        field: { paddingInlineEnd: "8" },
        icon: { fontSize: "md" },
      },
      lg: {
        field: { paddingInlineEnd: "10" },
        icon: { fontSize: "lg" },
      },
    },
  },
};

const mdx = {
  h1: {
    mt: { base: "2rem", md: "4rem" },
    mb: ".25rem",
    lineHeight: 1.2,
    fontWeight: "bold",
    fontSize: "1.875rem",
    letterSpacing: "-.025em",
  },
  h2: {
    mt: "3rem",
    mb: "0.5rem",
    lineHeight: 1.3,
    fontWeight: "semibold",
    fontSize: "1.5rem",
    letterSpacing: "-.025em",
    "& + h3": {
      mt: "1.5rem",
    },
  },
  h3: {
    mt: "2rem",
    lineHeight: 1.25,
    fontWeight: "semibold",
    fontSize: "1.25rem",
    letterSpacing: "-.025em",
  },
  h4: {
    mt: "2rem",
    lineHeight: 1.375,
    fontWeight: "semibold",
    fontSize: "1.125rem",
  },
  a: {
    color: "primary.500",
    fontWeight: "bold",
    _hover: {
      color: "primary.600",
      cursor: "pointer",
    },
  },
  p: {
    mt: "1.25rem",
    lineHeight: 1.7,
    "blockquote &": {
      mt: 0,
    },
  },
  hr: {
    my: "4rem",
  },
  blockquote: {
    bg: "orange.100",
    borderWidth: "1px",
    borderColor: "orange.200",
    rounded: "lg",
    px: "1.25rem",
    py: "1rem",
    my: "1.5rem",
  },
  ul: {
    my: "1rem",
    ml: "1.25rem",
    "blockquote &": { my: 0 },
    "& > * + *": {
      mt: "0.25rem",
    },
  },
  code: {
    rounded: "sm",
    px: "1",
    fontSize: "0.875em",
    py: "2px",
    whiteSpace: "nowrap",
    lineHeight: "normal",
  },
};

const chatMdx = {
  h1: {
    my: "2rem",
    lineHeight: 1.2,
    fontWeight: "bold",
    fontSize: "1.875rem",
    letterSpacing: "-.025em",
  },
  h2: {
    my: "1rem",
    lineHeight: 1.3,
    fontWeight: "semibold",
    fontSize: "1.5rem",
    letterSpacing: "-.025em",
    "& + h3": {
      mt: "1.5rem",
    },
  },
  h3: {
    my: "0.75rem",
    lineHeight: 1.25,
    fontWeight: "semibold",
    fontSize: "1.25rem",
    letterSpacing: "-.025em",
  },
  h4: {
    my: "0.5rem",
    lineHeight: 1.375,
    fontWeight: "semibold",
    fontSize: "1.125rem",
  },
  a: {
    color: "primary.600",
    fontWeight: "medium",
    _hover: {
      color: "primary.500",
      cursor: "pointer",
    },
  },
  p: {
    lineHeight: "1.75",
    "blockquote &": {
      mt: 0,
    },
    fontSize: "sm",
  },
  ul: {
    mt: "0.25rem",
    ml: "1.25rem",
    "blockquote &": { my: 0 },
    "& > * + *": {
      mt: "0.25rem",
    },
    fontSize: "sm",
  },
  ol: {
    mt: "0.25rem",
    ml: "2rem",
    "blockquote &": { my: 0 },
    "& > * + *": {
      mt: "0.25rem",
    },
    fontSize: "sm",
  },
};

const editorMdx = {
  h1: {
    fontWeight: "bold",
    fontSize: "2rem",
    letterSpacing: "-.025em",
  },
  h2: {
    fontWeight: "semibold",
    fontSize: "1.75rem",
    letterSpacing: "-.025em",
  },
  h3: {
    fontWeight: "semibold",
    fontSize: "1.5rem",
    letterSpacing: "-.025em",
  },
  h4: {
    fontWeight: "semibold",
    fontSize: "1.25rem",
    letterSpacing: "-.025em",
  },
  h5: {
    fontWeight: "semibold",
    fontSize: "1.125rem",
    letterSpacing: "-.025em",
  },
  a: {
    color: "primary.600",
    fontWeight: "medium",
    _hover: {
      color: "primary.500",
      cursor: "pointer",
    },
  },
  ul: {
    ml: "1.25rem",
    "blockquote &": { my: 0 },
    "& > * + *": {
      mt: "0.25rem",
    },
    fontSize: "sm",
  },
  ol: {
    ml: "2rem",
    "blockquote &": { my: 0 },
    "& > * + *": {
      mt: "0.25rem",
    },
    fontSize: "sm",
  },
  code: {
    bg: "rgba(#616161, 0.1)",
    color: "#616161",
  },
  pre: {
    background: "#0D0D0D",
    color: "#FFF",
    padding: " 0.75rem 1rem",
    borderRadius: "0.5rem",
    code: {
      color: "inherit",
      padding: 0,
      background: "none",
      fontSize: "0.8rem",
    },
  },
  img: {
    maxWidth: "100%",
    height: "auto",
  },
  blockquote: {
    bg: "orange.100",
    borderWidth: "1px",
    borderColor: "orange.200",
    rounded: "lg",
    px: "1rem",
    py: "0.5rem",
  },
  hr: {
    border: "none",
    borderTop: "2px solid rgba(#0D0D0D, 0.1)",
    margin: "2rem 0",
  },
  ".tableWrapper": {
    margin: "0.5rem 0",
    overflowX: "auto",
  },
  "&.resize-cursor": {
    cursor: "col-resize",
  },
  table: {
    borderCollapse: "collapse",
    tableLayout: "auto",
    w: "full",
    minW: "max-content !important",
    overflow: "hidden",
    "td, th": {
      border: "1px solid #E2E7F0",
      verticalAlign: "top",
      boxSizing: "border-box",
      position: "relative",
      padding: "0 10px",
      bgColor: "white",
      "> *": {
        marginBottom: "0",
      },
    },
    th: {
      bgColor: "Z.100",
      fontWeight: "bold",
      textAlign: "left",
    },
    ".selectedCell": {
      background: "neutral.100",
      borderColor: "neutral.200",
      zIndex: "2",
      pointerEvents: "none",
    },
    ".column-resize-handle": {
      position: "absolute",
      pointerEvents: "none",
      right: "0px",
      top: "0",
      bottom: "-2px",
      width: "2px",
      padding: "0",
      backgroundColor: "primary.600",
      lineHeight: "1",
    },
  },
};

const theme = extendTheme({
  colors,
  shadows,
  fonts,
  textStyles,
  layerStyles,
  components,
  styles,
  mdx,
  chatMdx,
  editorMdx,
  breakpoints: {
    default: "0px",
    xl1440: "1440px",
    xl1520: "1520px",
    xl1600: "1600px",
    xl1680: "1680px",
    xl1760: "1760px",
    xl1840: "1840px",
    xl1920: "1920px",
    xl2000: "2000px",
  },
});

export default theme;
