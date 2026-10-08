// ShadowTooltip.tsx
import React, {
  useRef,
  useEffect,
  useState,
  ReactElement,
  cloneElement,
} from "react";
import { createPortal } from "react-dom";

type Placement = "top" | "bottom" | "left" | "right";

interface ShadowTooltipProps {
  children: ReactElement;
  placement?: Placement;
  gap?: number;
  content?: string; // 提示文本，默认空
}

const DEFAULT_STYLE = `
  .tooltip {
    position: absolute;
    background: #333;
    color: #fff;
    padding: 6px 10px;
    border-radius: 4px;
    font-size: 12px;
    white-space: nowrap;
    pointer-events: none;
    z-index: 9999;
    visibility: hidden;
    opacity: 0;
    transition: opacity 0.15s ease-in-out;
  }
  .tooltip.visible {
    visibility: visible;
    opacity: 1;
  }
  .tooltip::after {
    content: '';
    position: absolute;
    width: 0;
    height: 0;
    border: 5px solid transparent;
  }
  .tooltip.top::after {
    top: 100%;
    left: 50%;
    transform: translateX(-50%);
    border-top-color: #333;
  }
  .tooltip.bottom::after {
    bottom: 100%;
    left: 50%;
    transform: translateX(-50%);
    border-bottom-color: #333;
  }
  .tooltip.left::after {
    left: 100%;
    top: 50%;
    transform: translateY(-50%);
    border-left-color: #333;
  }
  .tooltip.right::after {
    right: 100%;
    top: 50%;
    transform: translateY(-50%);
    border-right-color: #333;
  }
`;

function getPosition(
  trigger: DOMRect,
  placement: Placement,
  gap: number,
): { top: number; left: number } {
  const { top, left, width, height } = trigger;
  switch (placement) {
    case "top":
      return { top: top - height - 10 - gap, left: left + width / 2 };
    case "bottom":
      return { top: top + height + gap, left: left + width / 2 };
    case "left":
      return { top: top + height / 2, left: left - gap };
    case "right":
      return { top: top + height / 2, left: left + width + gap };
    default:
      return { top: top + height / 2, left: left + width / 2 };
  }
}

export default function ShadowTooltip({
  children,
  placement = "top",
  gap = 16,
  content = "",
}: ShadowTooltipProps) {
  const [open, setOpen] = useState(false);
  const triggerRef = useRef<HTMLElement>(null);
  const shadowHostRef = useRef<HTMLDivElement | null>(null);
  const tooltipRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!shadowHostRef.current) {
      const host = document.createElement("div");
      document.body.appendChild(host);
      shadowHostRef.current = host;

      const shadow = host.attachShadow({ mode: "open" });
      const style = document.createElement("style");
      style.textContent = DEFAULT_STYLE;
      shadow.appendChild(style);

      const tooltip = document.createElement("div");
      tooltip.className = `tooltip ${placement}`;
      shadow.appendChild(tooltip);
      tooltipRef.current = tooltip;
    }
  }, [placement]);

  useEffect(() => {
    const tooltip = tooltipRef.current;
    if (!tooltip) return;
    tooltip.textContent = content;
    tooltip.classList.toggle("visible", open);
    if (!open) return;

    const rect = triggerRef.current!.getBoundingClientRect();
    const { top, left } = getPosition(rect, placement, gap);
    tooltip.style.top = `${top}px`;
    tooltip.style.left = `${left}px`;
    tooltip.style.transform = ["top", "bottom"].includes(placement)
      ? "translateX(-50%)"
      : "translateY(-50%)";
  }, [open, content, placement, gap]);

  useEffect(() => {
    return () => {
      shadowHostRef.current?.remove();
    };
  }, []);

  return (
    <>
      {cloneElement(children, {
        ref: triggerRef,
        onMouseEnter: () => setOpen(true),
        onMouseLeave: () => setOpen(false),
        onFocus: () => setOpen(true),
        onBlur: () => setOpen(false),
      })}
      {/* 用 createPortal 把 tooltip 渲染到 body，避免被父级 overflow 裁剪 */}
      {createPortal(null, document.body)}
    </>
  );
}
