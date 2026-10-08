"use client";

import { motion, useReducedMotion } from "framer-motion";
import { memo } from "react";

/*
 * Hallmark · component: nav-icon · genre: modern-minimal · theme: brand(navy/gold preserved)
 * states: default · hover · focus · active · active-route · collapsed · reduced-motion
 *
 * 侧边导航线性图标：24px 网格 / 2px 描边 / 圆角端点 / currentColor 继承，
 * 与全站 react-icons/fi（Feather）同一套笔触语言。
 */

const GLYPH_PATHS = {
  // 招标情报站：雷达 + 信号波（情报扫描）
  "nav-intel": [
    "M12 3a9 9 0 1 0 9 9",
    "M12 7a5 5 0 1 0 5 5",
    "M12 11a1 1 0 1 0 1 1",
    "M18.5 3.5 21 6l-3.2 3.2",
  ],
  // 招标解析：文档 + 放大镜
  "nav-analysis": [
    "M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8l-5-5z",
    "M14 3v5h5",
    "M8.5 14.5a3 3 0 1 0 6 0a3 3 0 1 0-6 0",
    "M14 17l2.5 2.5",
  ],
  // 投标文件生成：文档 + 笔
  "nav-gen": [
    "M12.5 22H18a2 2 0 0 0 2-2V7l-5-5H6a2 2 0 0 0-2 2v9.5",
    "M14 2v4a2 2 0 0 0 2 2h4",
    "M13.378 15.626a1 1 0 1 0-3.004-3.004l-5.01 5.012a2 2 0 0 0-.506.854l-.837 2.87a.5.5 0 0 0 .62.62l2.87-.837a2 2 0 0 0 .854-.506z",
  ],
  // 投标文件审核：文档 + 对勾
  "nav-audit": [
    "M4 22h14a2 2 0 0 0 2-2V7l-5-5H6a2 2 0 0 0-2 2v4",
    "M14 2v4a2 2 0 0 0 2 2h4",
    "m3 15 2 2 4-4",
  ],
  // 素材库：文件夹
  "nav-material": [
    "M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z",
  ],
  // 系统管理：齿轮
  "nav-system": [
    "M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z",
    "M12 12m-3 0a3 3 0 1 0 6 0a3 3 0 1 0-6 0",
  ],
};

function NavIcon({ name, isActive = false, size = 20, strokeWidth = 2 }) {
  const reducedMotion = useReducedMotion();
  const paths = GLYPH_PATHS[name];
  if (!paths) return null;

  return (
    <svg
      viewBox="0 0 24 24"
      width={size}
      height={size}
      fill="none"
      stroke="currentColor"
      strokeWidth={strokeWidth}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
      style={{ display: "block", flexShrink: 0 }}
    >
      {isActive && !reducedMotion
        ? paths.map((d, i) => (
            <motion.path
              key={`${name}-${i}-active`}
              d={d}
              pathLength={1}
              initial={{ pathLength: 0 }}
              animate={{ pathLength: 1 }}
              transition={{
                duration: 0.4,
                ease: [0.16, 1, 0.3, 1],
                delay: i * 0.04,
              }}
            />
          ))
        : paths.map((d, i) => <path key={`${name}-${i}`} d={d} />)}
    </svg>
  );
}

export default memo(NavIcon);
