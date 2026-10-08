/* eslint-disable no-unused-vars */
/* eslint-disable no-useless-return */
/* eslint-disable prefer-template */
/* eslint-disable prettier/prettier */
import React, { useEffect, useRef, useImperativeHandle, forwardRef, CSSProperties, useState, useCallback } from "react";
import ToolTip from "./ToolTip";

// 画布上的单个文字对象
type TextItem = {
  id: string;
  text: string;
  x: number;
  y: number;
  color: string;
  fontSize: number;
};

// 图形标注对象（矩形 / 圆形 / 马赛克）
type ShapeItem =
  | {
    id: string;
    type: "rect" | "circle";
    x: number;
    y: number;
    w: number;
    h: number;
    color: string;
    lineWidth: number;
    lineDash: number[];
    fill: "fill" | "stroke";
  }
  | {
    id: string;
    type: "mosaic";
    x: number;
    y: number;
    w: number;
    h: number;
    mosaicSize: number;
  };

export type ImageEditorExportData = {
  image: { width: number; height: number } | null;
  texts: TextItem[];
  shapes: ShapeItem[];
};

// 内部编辑器 API，提供给 React 组件使用
type EditorApi = {
  getEditImageBase64: () => string;
  loadImageFromFile: (file: File) => void;
  loadImageFromUrl: (url: string) => void;
  downloadImage: () => void;
  getExportData: () => ImageEditorExportData | null;
  exportData: () => ImageEditorExportData | null;
  setData: (data: ImageEditorExportData) => void;
  reset: () => void;
  destroy: () => void;
};

export type ImageEditorHandle = {
  loadImageFile: (file: File) => void;
  loadImageUrl: (url: string) => void;
  downloadImage: () => void;
  getExportData: () => ImageEditorExportData | null;
  exportData: () => ImageEditorExportData | null;
  reset: () => void;
  getEditImageBase64: () => string;
};

// ImageEditor 组件的属性定义
export type ImageEditorProps = {
  initialImageFile?: File;
  initialImageSrc: string;
  imgData?: ImageEditorExportData | null;
  onExport?: (data: ImageEditorExportData) => void;
  onDownload?: (dataUrl: string) => void;
  onImageLoaded?: (size: { width: number; height: number }) => void;
  className?: string;
  style?: CSSProperties;
  enableLeaveTip?: boolean;
  isReadOnly?: boolean;
  isLoading?: boolean;
};

// 组件内部使用的样式字符串
const imageEditorStyles = `
.reie-root {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  padding: 16px;
  box-sizing: border-box;
  background: #ffffff;
  color: #222222;
  position: relative;
}

.reie-loading-overlay {
  position: absolute;
  inset: 0;
  background: rgba(255, 255, 255, 0.75);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 30;
}

.reie-loading-spinner {
  width: 40px;
  height: 40px;
  border-radius: 9999px;
  border: 4px solid #e5e7eb;
  border-top-color: #3182ce;
  animation: reie-spin 0.8s linear infinite;
}

@keyframes reie-spin {
  to {
    transform: rotate(360deg);
  }
}

.reie-toolbar {
  margin-bottom: 8px;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  justify-content: end;
  position: sticky;
  top: 0;
  z-index: 20;
  background: #ffffff;
  padding-top: 8px;
  padding-bottom: 8px;
}

.reie-toolbar-row {
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 6px;
  flex-wrap: nowrap;
}

.reie-tool-group {
  display: inline-flex;
  align-items: end;
  gap: 10px;
  padding: 6px 10px;
  border-radius: 6px;
  background: #fafafa;
}
.reie-tool-group:hover {
  background: #eaeaea;
  cursor: pointer;
}

.reie-tool-group-item:hover {
  background: #d4d4d4ff;
  cursor: pointer;
}

.reie-tool-group-mode {
  position: relative;
}

.reie-tool-popup {
  position: absolute;
  top: 100%;
  left: 50%;
  transform: translateX(-50%);
  margin-top: 4px;
  background: #ffffff;
  border-radius: 8px;
  border: 1px solid #e0e0e0;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.12);
  padding: 8px 10px;
  z-index: 50;
  display: none;
  min-width: 180px;
}

.reie-tool-popup-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 6px;
}

.reie-tool-popup-title {
  font-size: 12px;
  color: #111;
  font-weight: 600;
}

.reie-tool-popup-close {
  border: none;
  background: transparent;
  width: 22px;
  height: 22px;
  line-height: 22px;
  border-radius: 6px;
  cursor: pointer;
  color: #666;
  padding: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.reie-tool-popup-close:hover {
  background: #f0f0f0;
  color: #111;
}

.reie-tool-popup.reie-tool-popup-visible {
  display: block;
}

.reie-tool-popup-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}

.reie-tool-popup-row:last-child {
  margin-bottom: 0;
}

.reie-tool-popup-label {
  font-size: 12px;
  color: #666;
  white-space: nowrap;
}

.reie-color-swatches {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.reie-color-swatch {
  width: 16px;
  height: 16px;
  border-radius: 4px;
  border: 1px solid rgba(0, 0, 0, 0.1);
  padding: 0;
  cursor: pointer;
  background: transparent;
}

.reie-color-swatch:hover {
  border-color: #1677ff;
}

.reie-color-swatch.reie-color-swatch-selected {
  border-color: #0d0101ff;
  box-shadow: 0 0 0 0.5px #000000;
}

.reie-popup-color-input {
  width: 32px;
  padding: 0;
}

.reie-btn {
  padding: 4px 12px;
  border-radius: 4px;
  border: 1px solid transparent;
  cursor: pointer;
  font-size: 13px;
  line-height: 1.4;
  white-space: nowrap;
  background: transparent;
  color: #222222;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition: background 0.15s ease, border-color 0.15s ease, color 0.15s ease, box-shadow 0.15s ease;
}

.reie-btn:hover {
  background: #f0f0f0;
}

.reie-btn-mode {
  background: #ffffff;
  border-color: #d0d7e2;
  color: #222222;
}

.reie-btn-mode.reie-btn-mode-active {
  background: #1677ff;
  border-color: #1677ff;
  color: #ffffff;
  box-shadow: 0 0 0 1px rgba(22, 119, 255, 0.4);
}

.reie-btn-primary {
  background: #1677ff;
  border-color: #1677ff;
  color: #ffffff;
}

.reie-btn-primary:hover {
  background: #165dff;
  border-color: #165dff;
}

.reie-btn-ghost {
  border-color: #d0d7e2;
  background: #ffffff;
  color: #1f2933;
}

.reie-btn-ghost:hover {
  background: #f3f6fb;
}

.reie-toolbar input[type="color"],
.reie-toolbar input[type="number"],
.reie-toolbar input[type="range"],
.reie-toolbar select {
  cursor: pointer;
  border-radius: 4px;
  border: 1px solid #e0e0e0;
  background: #ffffff;
  color: #222222;
  padding: 2px 4px;
  font-size: 12px;
}

.reie-toolbar input[type="range"] {
  padding: 0;
  background: transparent;
}

.reie-editor-wrap {
  flex: 1;
  min-height: 0;
}

.reie-canvas-inner {
  position: relative;
  height: 100%;
  background: #f8f4f4ff;
  display: block;
  overflow: auto;
  text-align: center;
  padding: 8px;
  box-sizing: border-box;
}

.reie-canvas-inner canvas {
  border: 1px solid #ddd;
  background: #ffffff;
  display: inline-block;
}

.reie-zoom-bar {
  position: absolute;
  left: 120px;
  top: 16px;
  padding: 2px;
  display: flex;
  align-items: center;
  font-size: 12px;
  z-index: 999;
  color: #676363;
  border-radius: 6px;
}

.reie-zoom-bar-image-size {
  color: #a9b6c2;
  left: 16px;
  top: 16px;
}

.reie-zoom-bar span {
  min-width: 40px;
  text-align: center;
}

.reie-zoom-bar button {
  cursor: pointer;
  background: transparent;
  width: 20px;
  height: 20px;
  line-height: 1px;
  border-radius: 8px;
  padding: 2px;
  font-size: 16px;
}

.reie-zoom-bar button:hover {
  background: #eaebedff;
}

.reie-delete-selection-btn {
  color: #676363;
  cursor: pointer;
  background: #ffffffff;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
  width: 34px;
  height: 26px;
  text-align: center;
  border-radius: 4px;
  padding: 6px;
  position: fixed;
  display: none;
  transform: translateX(-50%);
  z-index: 20;
}

.active-tools-bg {
  background: #e1dfdf;
  box-shadow: 0 0 0 1px rgba(132, 132, 132, 0.32);
}
`;

// 初始化核心图片编辑逻辑，绑定 canvas 与工具栏事件
function initImageEditor(
  container: HTMLDivElement,
  options: {
    onExport?: (data: ImageEditorExportData) => void;
    onDownload?: (dataUrl: string) => void;
    onImageLoaded?: (size: { width: number; height: number }) => void;
  },
): EditorApi {
  const canvas = container.querySelector(
    '[data-role="canvas"]',
  ) as HTMLCanvasElement;
  const mouseBtn = container.querySelector(
    '[data-role="mouse-btn"]',
  ) as HTMLButtonElement | null;
  const textBtn = container.querySelector(
    '[data-role="text-btn"]',
  ) as HTMLButtonElement | null;
  const textColorInput = container.querySelector(
    "#textColor",
  ) as HTMLInputElement | null;
  const textSizeInput = container.querySelector(
    '[data-role="text-size"]',
  ) as HTMLInputElement | null;
  const rectBtn = container.querySelector(
    '[data-role="rect-btn"]',
  ) as HTMLButtonElement | null;
  const rectColorInput = container.querySelector(
    "#rectColor",
  ) as HTMLInputElement | null;
  const rectWidthInput = container.querySelector(
    '[data-role="rect-width"]',
  ) as HTMLInputElement | null;
  const rectShapeSelect = container.querySelector(
    '[data-role="rect-shape"]',
  ) as HTMLSelectElement | null;
  const rectDashSelect = container.querySelector(
    '[data-role="rect-dash"]',
  ) as HTMLSelectElement | null;
  const rectFillSelect = container.querySelector(
    '[data-role="rect-fill"]',
  ) as HTMLSelectElement | null;
  const mosaicBtn = container.querySelector(
    '[data-role="mosaic-btn"]',
  ) as HTMLButtonElement | null;
  const resetBtn = container.querySelector(
    '[data-role="reset-btn"]',
  ) as HTMLButtonElement | null;
  const undoBtn = container.querySelector(
    '[data-role="undo-btn"]',
  ) as HTMLButtonElement | null;
  const redoBtn = container.querySelector(
    '[data-role="redo-btn"]',
  ) as HTMLButtonElement | null;
  const zoomOutBtn = container.querySelector(
    '[data-role="zoom-out-btn"]',
  ) as HTMLButtonElement | null;
  const zoomInBtn = container.querySelector(
    '[data-role="zoom-in-btn"]',
  ) as HTMLButtonElement | null;
  const zoomPercent = container.querySelector(
    '[data-role="zoom-percent"]',
  ) as HTMLSpanElement | null;
  const deleteSelectionBtn = container.querySelector(
    '[data-role="delete-selection-btn"]',
  ) as HTMLButtonElement | null;

  const canvasInner = container.querySelector(
    '.reie-canvas-inner',
  ) as HTMLDivElement | null;
  const toolbar = container.querySelector(
    '.reie-toolbar',
  ) as HTMLDivElement | null;
  const textPopup = container.querySelector(
    '[data-role="text-popup"]',
  ) as HTMLDivElement | null;
  const rectPopup = container.querySelector(
    '[data-role="rect-popup"]',
  ) as HTMLDivElement | null;
  const mosaicPopup = container.querySelector(
    '[data-role="mosaic-popup"]',
  ) as HTMLDivElement | null;
  const textPopupClose = container.querySelector(
    '[data-role="text-popup-close"]',
  ) as HTMLButtonElement | null;
  const rectPopupClose = container.querySelector(
    '[data-role="rect-popup-close"]',
  ) as HTMLButtonElement | null;
  const mosaicPopupClose = container.querySelector(
    '[data-role="mosaic-popup-close"]',
  ) as HTMLButtonElement | null;
  const colorSwatches = container.querySelectorAll(
    '.reie-color-swatch',
  ) as NodeListOf<HTMLButtonElement>;
  const mosaicSizeInput = container.querySelector(
    '[data-role="mosaic-size"]',
  ) as HTMLInputElement | null;

  const isReadOnlyMode = () => container.getAttribute('data-read-only') === 'true';
  const closePopup = (p: HTMLElement | null) => {
    if (!p) return;
    p.classList.remove('reie-tool-popup-visible');
    const active = document.activeElement as HTMLElement | null;
    if (active && p.contains(active)) {
      active.blur();
    }
  };
  const closeAllPopups = () => {
    closePopup(textPopup);
    closePopup(rectPopup);
    closePopup(mosaicPopup);
  };

  if (!canvas) {
    throw new Error('ImageEditor: canvas not found');
  }

  const ctx = canvas.getContext('2d', { willReadFrequently: true }) as CanvasRenderingContext2D;
  if (!ctx) {
    throw new Error('ImageEditor: canvas context not available');
  }

  // 当前编辑模式：'' | 'text' | 'rect' | 'mosaic'
  let mode = '';
  // 原始图像数据（当前画布内容）
  let baseImage: ImageData | null = null;
  // 初始图像快照，用于重置
  let originalImage: ImageData | null = null;
  // 文本和图形标注集合
  let texts: TextItem[] = [];
  let shapes: ShapeItem[] = [];
  // 当前选中的文字 / 图形
  let activeTextId: string | null = null;
  let activeShapeId: string | null = null;
  // 拖拽相关状态（文字 / 图形）
  let draggingText = false;
  let draggingShape = false;
  let dragTextOffsetX = 0;
  let dragTextOffsetY = 0;
  let dragShapeOffsetX = 0;
  let dragShapeOffsetY = 0;
  let justDraggedSomething = false;
  // 文本编辑框 DOM 及状态
  let textEditor: HTMLDivElement | null = null;
  let editingTextId: string | null = null;
  let pendingTextClick = false;
  let pendingTextClickId: string | null = null;
  let pendingTextClickStartX = 0;
  let pendingTextClickStartY = 0;

  // 选框绘制与拖拽状态
  let isDraggingSelection = false;
  let selectionPending = false;
  let selectionStartX = 0;
  let selectionStartY = 0;
  let selectionSnapshot: ImageData | null = null;

  let isPanning = false;
  let panStartX = 0;
  let panStartY = 0;
  let panStartScrollLeft = 0;
  let panStartScrollTop = 0;

  // 图形缩放状态
  let resizingShape = false;
  let resizeEdge = '';
  let resizeStartX = 0;
  let resizeStartY = 0;
  let autoFit = true;
  let resizeStartShape: { x: number; y: number; w: number; h: number } | null = null;

  // 控制点大小、命中范围与拖拽阈值
  const RESIZE_HANDLE_SIZE = 14;
  const RESIZE_HANDLE_HIT_SIZE = 14;
  const SELECT_DRAG_THRESHOLD = 6;

  // 撤销 / 重做历史
  let history: { texts: TextItem[]; shapes: ShapeItem[] }[] = [];
  let historyIndex = -1;
  // 视图缩放与马赛克配置
  let viewScale = 1;
  let mosaicSize = 15;
  // DOM 监听器与待应用的数据
  let resizeObserver: ResizeObserver | null = null;
  let pendingData: ImageEditorExportData | null = null;

  // 绘制选框时的边框样式
  const STROKESTYLE = "rgba(24, 82, 255, 1)";

  function getMosaicGray(id: string, col: number, row: number, sizeValue: number) {
    let h = 0;
    for (let i = 0; i < id.length; i += 1) {
      h = (h * 31 + id.charCodeAt(i)) | 0;
    }
    h ^= (col * 73856093) | 0;
    h ^= (row * 19349663) | 0;
    h ^= (sizeValue * 83492791) | 0;
    h >>>= 0;
    const base = 130;
    const range = 130;
    return base + (h % range);
  }

  // 深拷贝 ImageData，避免共享同一数据引用
  function cloneImageData(imageData: ImageData | null) {
    if (!imageData) return null;
    return new ImageData(
      new Uint8ClampedArray(imageData.data),
      imageData.width,
      imageData.height
    );
  }

  // 深拷贝文字列表，防止直接修改历史记录中的数据
  function cloneTexts(list: TextItem[]) {
    return list.map(t => ({
      id: t.id,
      text: t.text,
      x: t.x,
      y: t.y,
      color: t.color,
      fontSize: t.fontSize
    }));
  }

  // 深拷贝图形列表（矩形 / 圆形 / 马赛克）
  function cloneShapes(list: ShapeItem[]) {
    return list.map(s => {
      if (s.type === 'mosaic') {
        return {
          id: s.id,
          type: 'mosaic' as const,
          x: s.x,
          y: s.y,
          w: s.w,
          h: s.h,
          mosaicSize: s.mosaicSize
        };
      }
      return {
        id: s.id,
        type: s.type,
        x: s.x,
        y: s.y,
        w: s.w,
        h: s.h,
        color: s.color,
        lineWidth: s.lineWidth,
        lineDash: s.lineDash ? s.lineDash.slice() : [],
        fill: s.fill
      };
    });
  }

  // 将当前文本和图形状态压入历史栈，用于撤销 / 重做
  function pushHistory() {
    if (!baseImage) return;
    if (historyIndex < history.length - 1) {
      history = history.slice(0, historyIndex + 1);
    }
    history.push({
      texts: cloneTexts(texts),
      shapes: cloneShapes(shapes)
    });
    historyIndex = history.length - 1;
  }

  // 根据缩放和滚动位置更新文本编辑框的绝对位置
  function updateTextEditorPosition() {
    if (!textEditor) return;
    if (!baseImage || !editingTextId || !canvasInner) {
      textEditor.style.display = 'none';
      return;
    }
    const t = texts.find(item => item.id === editingTextId);
    if (!t) {
      textEditor.style.display = 'none';
      return;
    }
    const rect = canvas.getBoundingClientRect();
    const containerRect = canvasInner.getBoundingClientRect();
    if (!rect.width || !rect.height) {
      textEditor.style.display = 'none';
      return;
    }
    const size = t.fontSize || 30;
    const rawText = textEditor ? textEditor.textContent || '' : t.text || '';
    ctx.save();
    ctx.font = size + 'px sans-serif';
    const textWidth = ctx.measureText(rawText).width;
    ctx.restore();
    const left = t.x;
    const top = t.y - size;
    const right = t.x + textWidth;
    const bottom = t.y;
    const scaleX = rect.width / canvas.width;
    const scaleY = rect.height / canvas.height;
    const clientLeft = rect.left + left * scaleX;
    const clientTop = rect.top + top * scaleY;
    const width = (right - left) * scaleX;
    const height = (bottom - top) * scaleY;
    const scrollLeft = canvasInner.scrollLeft;
    const scrollTop = canvasInner.scrollTop;
    textEditor.style.left = clientLeft - containerRect.left + scrollLeft + 'px';
    textEditor.style.top = clientTop - containerRect.top + scrollTop + 'px';
    textEditor.style.width = Math.max(40, width) + 'px';
    textEditor.style.height = Math.max(24, height) + 'px';
    textEditor.style.fontSize = size * scaleY + 'px';
    textEditor.style.color = t.color || '#ff0000';
    textEditor.style.display = 'block';
  }
  // 根据 viewScale 更新 canvas 显示尺寸和缩放百分比
  function updateZoomView() {
    if (!baseImage) {
      (canvas.style as any).width = '';
      (canvas.style as any).height = '';
      if (zoomPercent) {
        zoomPercent.textContent = '100%';
      }
      return;
    }
    const displayWidth = baseImage.width * viewScale;
    const displayHeight = baseImage.height * viewScale;
    (canvas.style as any).width = displayWidth + 'px';
    (canvas.style as any).height = displayHeight + 'px';
    if (zoomPercent) {
      zoomPercent.textContent = Math.round(viewScale * 100) + '%';
    }
    updateTextEditorPosition();
  }

  // 设置视图缩放比例，并限制在 20% ~ 400% 之间
  function setViewScale(scale: number) {
    if (!baseImage) return;
    let percent = Math.round(scale * 100);
    if (!Number.isFinite(percent) || percent <= 0) {
      percent = 100;
    }
    if (percent < 20) {
      percent = 20;
    } else if (percent > 400) {
      percent = 400;
    }
    viewScale = percent / 100;
    autoFit = false;
    updateZoomView();
  }

  const handleCanvasWheel = (e: WheelEvent) => {
    if (!baseImage) return;
    if (!e.ctrlKey && !e.metaKey) {
      return;
    }
    e.preventDefault();
    if (e.deltaY > 0) {
      setViewScale(viewScale + 0.02);
    } else if (e.deltaY < 0) {
      setViewScale(viewScale - 0.02);
    }
  };

  // 计算文字的包围盒，用于命中检测与选中框
  function getTextBounds(t: TextItem | null) {
    if (!t) return null;
    const size = t.fontSize || 30;
    ctx.save();
    ctx.font = size + 'px sans-serif';
    const w = ctx.measureText(t.text || '').width;
    ctx.restore();
    return {
      left: t.x,
      top: t.y - size,
      right: t.x + w,
      bottom: t.y
    };
  }

  // 获取当前选中的文字或图形的包围盒
  function getActiveSelectionBounds() {
    if (activeShapeId) {
      const s = shapes.find(item => item.id === activeShapeId);
      if (s) {
        return {
          left: s.x,
          top: s.y,
          right: s.x + s.w,
          bottom: s.y + s.h
        };
      }
    }
    if (activeTextId) {
      const t = texts.find(item => item.id === activeTextId);
      if (t) {
        const bounds = getTextBounds(t);
        if (bounds) return bounds;
      }
    }
    return null;
  }

  // 更新删除按钮的位置，使其跟随当前选中的对象
  function updateDeleteButtonPosition() {
    if (!deleteSelectionBtn) return;
    if (isReadOnlyMode()) {
      deleteSelectionBtn.style.display = 'none';
      return;
    }
    if (!baseImage) {
      deleteSelectionBtn.style.display = 'none';
      return;
    }
    const bounds = getActiveSelectionBounds();
    if (!bounds) {
      deleteSelectionBtn.style.display = 'none';
      return;
    }
    const rect = canvas.getBoundingClientRect();
    if (!rect.width || !rect.height) {
      deleteSelectionBtn.style.display = 'none';
      return;
    }
    const scaleX = rect.width / canvas.width;
    const scaleY = rect.height / canvas.height;
    const centerX = (bounds.left + bounds.right) / 2;
    const bottomY = bounds.bottom;
    const clientX = rect.left + centerX * scaleX;
    const clientY = rect.top + bottomY * scaleY + 8;
    deleteSelectionBtn.style.display = 'block';
    deleteSelectionBtn.style.left = clientX + 'px';
    deleteSelectionBtn.style.top = clientY + 'px';
  }

  // 核心渲染函数：绘制底图、所有图形、文本和选中状态
  function render(opts?: { noOverlay?: boolean; noShadow?: boolean }) {
    const includeOverlay = !(opts && opts.noOverlay);
    const disableShadow = !!(opts && opts.noShadow);
    if (baseImage) {
      canvas.width = baseImage.width;
      canvas.height = baseImage.height;
      ctx.putImageData(baseImage, 0, 0);
    } else {
      ctx.clearRect(0, 0, canvas.width, canvas.height);
    }
    shapes.forEach(s => {
      if (s.type === 'rect' || s.type === 'circle') {
        ctx.save();
        ctx.strokeStyle = s.color || '#ff0000';
        ctx.lineWidth = s.lineWidth || 2;
        // if (disableShadow) {
        //   ctx.shadowColor = 'transparent';
        //   ctx.shadowBlur = 0;
        // } else {
        //   ctx.shadowColor = 'rgba(0, 0, 0, 0.25)';
        //   ctx.shadowBlur = 2;
        // }
        if (s.lineDash && s.lineDash.length) {
          ctx.setLineDash(s.lineDash);
        } else {
          ctx.setLineDash([]);
        }
        const fillMode = s.fill || 'stroke';
        if (fillMode === 'fill') {
          ctx.fillStyle = s.color || '#ff0000';
        }
        if (s.type === 'circle') {
          const cx = s.x + s.w / 2;
          const cy = s.y + s.h / 2;
          const radius = Math.min(Math.abs(s.w), Math.abs(s.h)) / 2;
          ctx.beginPath();
          ctx.arc(cx, cy, radius, 0, Math.PI * 2);
          if (fillMode === 'fill') {
            ctx.fill();
          }
          ctx.stroke();
        } else {
          if (fillMode === 'fill') {
            ctx.fillRect(s.x, s.y, s.w, s.h);
          }
          ctx.strokeRect(s.x, s.y, s.w, s.h);
        }
        ctx.restore();
      } else if (s.type === 'mosaic') {
        const size = Math.max(1, Math.round(s.mosaicSize || 10));
        const startX = Math.floor(s.x / size) * size;
        const startY = Math.floor(s.y / size) * size;
        const endX = Math.ceil((s.x + s.w) / size) * size;
        const endY = Math.ceil((s.y + s.h) / size) * size;
        for (let yy = startY; yy < endY; yy += size) {
          for (let xx = startX; xx < endX; xx += size) {
            const x0 = Math.max(xx, s.x);
            const y0 = Math.max(yy, s.y);
            const x1 = Math.min(xx + size, s.x + s.w);
            const y1 = Math.min(yy + size, s.y + s.h);
            const w = x1 - x0;
            const h = y1 - y0;
            if (w <= 0 || h <= 0) continue;
            const colIndex = Math.floor((xx - startX) / size);
            const rowIndex = Math.floor((yy - startY) / size);
            const gray = getMosaicGray(s.id, colIndex, rowIndex, size);
            ctx.fillStyle = 'rgb(' + gray + ',' + gray + ',' + gray + ')';
            ctx.fillRect(x0, y0, w, h);
          }
        }
      }
    });

    if (includeOverlay && activeShapeId) {
      const activeShape = shapes.find(item => item.id === activeShapeId);
      if (activeShape) {
        ctx.save();
        ctx.strokeStyle = STROKESTYLE;
        ctx.lineWidth = 2;
        ctx.setLineDash([8, 4]);
        ctx.strokeRect(activeShape.x - 2, activeShape.y - 2, activeShape.w + 4, activeShape.h + 4);
        const handleSize = RESIZE_HANDLE_SIZE;
        const radius = handleSize / 2;
        const cx = activeShape.x + activeShape.w / 2;
        const cy = activeShape.y + activeShape.h / 2;
        const handles = [
          { x: activeShape.x, y: activeShape.y },
          { x: activeShape.x + activeShape.w, y: activeShape.y },
          { x: activeShape.x, y: activeShape.y + activeShape.h },
          { x: activeShape.x + activeShape.w, y: activeShape.y + activeShape.h },
          { x: cx, y: activeShape.y },
          { x: cx, y: activeShape.y + activeShape.h },
          { x: activeShape.x, y: cy },
          { x: activeShape.x + activeShape.w, y: cy }
        ];
        ctx.setLineDash([]);
        ctx.fillStyle = '#ffffff';
        ctx.strokeStyle = STROKESTYLE;
        ctx.lineWidth = 2;
        handles.forEach(h => {
          ctx.beginPath();
          ctx.arc(h.x, h.y, radius, 0, Math.PI * 2);
          ctx.fill();
          ctx.stroke();
        });
        ctx.restore();
      }
    }

    texts.forEach(t => {
      if (editingTextId && t.id === editingTextId) return;
      const size = t.fontSize || 30;
      ctx.save();
      ctx.font = size + 'px sans-serif';
      ctx.fillStyle = t.color || '#ff0000';
      ctx.shadowColor = 'transparent';
      ctx.shadowBlur = 0;
      ctx.fillText(t.text, t.x, t.y);
      ctx.restore();
    });

    if (includeOverlay && activeTextId && activeTextId !== editingTextId) {
      const activeText = texts.find(item => item.id === activeTextId);
      if (activeText) {
        const bounds = getTextBounds(activeText);
        if (bounds) {
          ctx.save();
          ctx.strokeStyle = STROKESTYLE;
          ctx.lineWidth = 2;
          ctx.setLineDash([8, 4]);
          const w = bounds.right - bounds.left;
          const h = bounds.bottom - bounds.top;
          ctx.strokeRect(bounds.left, bounds.top, w, h + 4);
          ctx.restore();
        }
      }
    }

    updateZoomView();
    updateDeleteButtonPosition();
    updateTextEditorPosition();
  }

  // 提交文本编辑结果，写回 texts 并记录历史
  function commitTextEditor() {
    if (!editingTextId || !textEditor) return;
    const t = texts.find(item => item.id === editingTextId);
    const currentId = editingTextId;
    editingTextId = null;
    if (!t) {
      textEditor.style.display = 'none';
      textEditor.textContent = '';
      return;
    }
    const raw = textEditor.textContent || '';
    const clean = raw.replace(/\r\n/g, '\n').split('\n').join('');
    if (clean) {
      t.text = clean;
    } else {
      texts = texts.filter(item => item.id !== currentId);
      if (activeTextId === currentId) {
        activeTextId = null;
      }
    }
    textEditor.style.display = 'none';
    textEditor.textContent = '';
    render();
    pushHistory();
  }

  // 取消文本编辑，不保存当前内容
  function cancelTextEditor() {
    if (!textEditor) return;
    editingTextId = null;
    textEditor.style.display = 'none';
    textEditor.textContent = '';
    render();
  }

  // 首次需要编辑文字时创建一个可编辑的文本浮层
  function ensureTextEditor() {
    if (textEditor || !canvasInner) return;
    textEditor = document.createElement('div');
    textEditor.contentEditable = 'true';
    textEditor.style.position = 'absolute';
    textEditor.style.border = '1px dashed ' + STROKESTYLE;
    textEditor.style.background = 'rgba(255, 255, 255, 0.05)';
    textEditor.style.padding = '2px 4px';
    textEditor.style.minWidth = '40px';
    textEditor.style.minHeight = '24px';
    textEditor.style.outline = 'none';
    textEditor.style.display = 'none';
    textEditor.style.whiteSpace = 'pre';
    textEditor.style.boxSizing = 'border-box';
    textEditor.style.zIndex = '30';
    canvasInner.appendChild(textEditor);

    textEditor.addEventListener('blur', () => {
      commitTextEditor();
    });
    textEditor.addEventListener('keydown', e => {
      if (e.key === 'Enter') {
        e.preventDefault();
        commitTextEditor();
      } else if (e.key === 'Escape') {
        e.preventDefault();
        cancelTextEditor();
      }
    });
    textEditor.addEventListener('input', () => {
      updateTextEditorPosition();
    });
  }

  // 打开指定文字对象的编辑框并聚焦
  function openTextEditorFor(textObj: TextItem | null) {
    if (!textObj) return;
    ensureTextEditor();
    if (!textEditor) return;
    activeTextId = textObj.id;
    activeShapeId = null;
    editingTextId = textObj.id;
    textEditor.textContent = textObj.text || '';
    render();
    const anyEditor = textEditor as any;
    if (anyEditor && typeof anyEditor.focus === 'function') {
      try {
        anyEditor.focus({ preventScroll: true });
      } catch (e) {
        anyEditor.focus();
      }
    }
    justDraggedSomething = true;
  }

  // 切换当前工具模式，并同步更新工具栏激活样式
  function setMode(nextMode: '' | 'text' | 'rect' | 'mosaic') {
    let modeToSet = nextMode;
    if (isReadOnlyMode() && modeToSet !== '') {
      modeToSet = '';
    }
    mode = modeToSet;
    if (mouseBtn) {
      mouseBtn.classList.remove('reie-btn-mode-active');
      (mouseBtn.parentNode as HTMLElement)?.classList.remove('active-tools-bg');
    }
    if (textBtn) {
      textBtn.classList.remove('reie-btn-mode-active');
      (textBtn.parentNode as HTMLElement)?.classList.remove('active-tools-bg');
    }
    if (rectBtn) {
      rectBtn.classList.remove('reie-btn-mode-active');
      (rectBtn.parentNode as HTMLElement)?.classList.remove('active-tools-bg');
    }
    if (mosaicBtn) {
      mosaicBtn.classList.remove('reie-btn-mode-active');
      (mosaicBtn.parentNode as HTMLElement)?.classList.remove('active-tools-bg');
    }
    closeAllPopups();
    if (modeToSet === '') {
      if (mouseBtn) {
        mouseBtn.classList.add('reie-btn-mode-active');
        (mouseBtn.parentNode as HTMLElement)?.classList.add('active-tools-bg');
      }
    } else if (modeToSet === 'text') {
      if (textBtn) {
        textBtn.classList.add('reie-btn-mode-active');
        (textBtn.parentNode as HTMLElement)?.classList.add('active-tools-bg');
      }
      if (textPopup) {
        textPopup.classList.add('reie-tool-popup-visible');
      }
    } else if (modeToSet === 'rect') {
      if (rectBtn) {
        rectBtn.classList.add('reie-btn-mode-active');
        (rectBtn.parentNode as HTMLElement)?.classList.add('active-tools-bg');
      }
      if (rectPopup) {
        rectPopup.classList.add('reie-tool-popup-visible');
      }
    } else if (modeToSet === 'mosaic') {
      if (mosaicBtn) {
        mosaicBtn.classList.add('reie-btn-mode-active');
        (mosaicBtn.parentNode as HTMLElement)?.classList.add('active-tools-bg');
      }
      if (mosaicPopup) {
        mosaicPopup.classList.add('reie-tool-popup-visible');
      }
    }
  }

  // 在指定坐标处查找最上层的文字对象
  function findTextAt(x: number, y: number) {
    for (let i = texts.length - 1; i >= 0; i -= 1) {
      const t = texts[i];
      const size = t.fontSize || 30;
      ctx.save();
      ctx.font = size + 'px sans-serif';
      const w = ctx.measureText(t.text).width;
      ctx.restore();
      const h = size;
      const left = t.x;
      const right = t.x + w;
      const top = t.y - h;
      const bottom = t.y;
      if (x >= left && x <= right && y >= top && y <= bottom) {
        return t;
      }
    }
    return null;
  }

  // 在指定坐标处查找最上层的图形对象
  function findShapeAt(x: number, y: number) {
    for (let i = shapes.length - 1; i >= 0; i -= 1) {
      const s = shapes[i];
      const left = s.x;
      const right = s.x + s.w;
      const top = s.y;
      const bottom = s.y + s.h;
      if (x >= left && x <= right && y >= top && y <= bottom) {
        return s;
      }
    }
    return null;
  }

  // 判断当前鼠标命中了图形的哪条缩放边
  function getShapeResizeEdge(s: ShapeItem, x: number, y: number) {
    const hitRadius = RESIZE_HANDLE_HIT_SIZE / 2;
    const hitR2 = hitRadius * hitRadius;
    const cx = s.x + s.w / 2;
    const cy = s.y + s.h / 2;
    const points: { key: string; px: number; py: number }[] = [
      { key: 'top-left', px: s.x, py: s.y },
      { key: 'top-right', px: s.x + s.w, py: s.y },
      { key: 'bottom-left', px: s.x, py: s.y + s.h },
      { key: 'bottom-right', px: s.x + s.w, py: s.y + s.h },
      { key: 'top', px: cx, py: s.y },
      { key: 'bottom', px: cx, py: s.y + s.h },
      { key: 'left', px: s.x, py: cy },
      { key: 'right', px: s.x + s.w, py: cy }
    ];
    for (let i = 0; i < points.length; i += 1) {
      const item = points[i];
      const dx = x - item.px;
      const dy = y - item.py;
      if (dx * dx + dy * dy <= hitR2) {
        return item.key;
      }
    }
    return '';
  }

  // 将鼠标事件坐标转换为 canvas 内部坐标
  function getCanvasPoint(e: MouseEvent) {
    const rect = canvas.getBoundingClientRect();
    const scaleX = canvas.width / rect.width;
    const scaleY = canvas.height / rect.height;
    const x = (e.clientX - rect.left) * scaleX;
    const y = (e.clientY - rect.top) * scaleY;
    return { x, y };
  }

  // 从 URL 加载图片到画布，并根据 pendingData 恢复标注
  function loadImageFromUrl(url: string) {
    if (!url) return;
    const image = new Image();
    image.onload = () => {
      canvas.width = image.width;
      canvas.height = image.height;
      ctx.drawImage(image, 0, 0);
      baseImage = ctx.getImageData(0, 0, canvas.width, canvas.height);
      originalImage = cloneImageData(baseImage);
      if (options.onImageLoaded) {
        options.onImageLoaded({ width: image.width, height: image.height });
      }
      if (pendingData) {
        texts = cloneTexts(pendingData.texts || []);
        shapes = cloneShapes(pendingData.shapes || []) as ShapeItem[];
        pendingData = null;
      } else {
        texts = [];
        shapes = [];
      }
      activeTextId = null;
      activeShapeId = null;
      history = [];
      historyIndex = -1;
      if (canvasInner) {
        const rect = canvasInner.getBoundingClientRect();
        const scaleX = rect.width / image.width;
        viewScale = Math.min(scaleX, 1);
        autoFit = true;
      } else {
        viewScale = 1;
        autoFit = false;
      }
      pushHistory();
      render();
      if (url.startsWith('blob:')) {
        URL.revokeObjectURL(url);
      }
    };
    image.src = url;
  }

  // 从文件对象加载图片，内部通过 URL 方式实现
  function loadImageFromFile(file: File) {
    if (!file) return;
    const url = URL.createObjectURL(file);
    loadImageFromUrl(url);
  }

  // 外部设置导出数据（用于回显已有的文本和图形标注）
  function setData(data: ImageEditorExportData) {
    if (!data) return;
    pendingData = data;
    texts = cloneTexts(data.texts || []);
    shapes = cloneShapes(data.shapes || []) as ShapeItem[];
    activeTextId = null;
    activeShapeId = null;
    pushHistory();
    render();
  }

  function handlePanMove(e: MouseEvent) {
    if (!isPanning || !canvasInner) return;
    const dx = e.clientX - panStartX;
    const dy = e.clientY - panStartY;
    canvasInner.scrollLeft = panStartScrollLeft - dx;
    canvasInner.scrollTop = panStartScrollTop - dy;
  }

  function handlePanEnd() {
    if (!isPanning) return;
    isPanning = false;
    if (mode === '') {
      canvas.style.cursor = 'grab';
    } else {
      canvas.style.cursor = 'default';
    }
  }

  if (canvas) {
    canvas.addEventListener('click', e => {
      if (justDraggedSomething) {
        justDraggedSomething = false;
        return;
      }
      if (!baseImage) return;
       if (isReadOnlyMode()) return;
      const point = getCanvasPoint(e);
      const x = point.x;
      const y = point.y;
      const t = findTextAt(x, y);
      if (mode === 'text') {
        if (t) {
          activeTextId = t.id;
          activeShapeId = null;
          render();
          return;
        }
        const id = Date.now() + '_' + Math.random();
        const color = textColorInput?.value || '#ff0000';
        const fontSize = parseFloat(textSizeInput?.value || '') || 30;
        const newText: TextItem = {
          id,
          text: '',
          x,
          y,
          color,
          fontSize
        };
        texts.push(newText);
        activeTextId = id;
        activeShapeId = null;
        render();
        openTextEditorFor(newText);
        return;
      }
      if (t) {
        activeTextId = t.id;
        activeShapeId = null;
        render();
      }
    });

    canvas.addEventListener('dblclick', e => {
      if (!baseImage) return;
      if (isReadOnlyMode()) return;
      const point = getCanvasPoint(e);
      const x = point.x;
      const y = point.y;
      const t = findTextAt(x, y);
      if (!t) return;
      activeTextId = t.id;
      activeShapeId = null;
      render();
      openTextEditorFor(t);
    });

    canvas.addEventListener('mousedown', e => {
      if (!baseImage) return;
      const point = getCanvasPoint(e);
      const x = point.x;
      const y = point.y;
      const t = findTextAt(x, y);
      const s = findShapeAt(x, y);
      if (!isReadOnlyMode()) {
        if (t) {
          activeTextId = t.id;
          activeShapeId = null;
          pendingTextClick = true;
          pendingTextClickId = t.id;
          pendingTextClickStartX = x;
          pendingTextClickStartY = y;
          render();
          return;
        }
        if (activeShapeId) {
          const activeShape = shapes.find(item => item.id === activeShapeId);
          if (activeShape) {
            const activeEdge = getShapeResizeEdge(activeShape, x, y);
            if (activeEdge) {
              resizingShape = true;
              resizeEdge = activeEdge;
              resizeStartX = x;
              resizeStartY = y;
              resizeStartShape = {
                x: activeShape.x,
                y: activeShape.y,
                w: activeShape.w,
                h: activeShape.h
              };
              return;
            }
          }
        }
        if (s) {
          activeShapeId = s.id;
          activeTextId = null;
          const edge = getShapeResizeEdge(s, x, y);
          if (edge) {
            resizingShape = true;
            resizeEdge = edge;
            resizeStartX = x;
            resizeStartY = y;
            resizeStartShape = { x: s.x, y: s.y, w: s.w, h: s.h };
          } else {
            draggingShape = true;
            dragShapeOffsetX = x - s.x;
            dragShapeOffsetY = y - s.y;
          }
          render();
          return;
        }
        if (mode === 'rect' || mode === 'mosaic') {
          if (!t && !s && (activeTextId || activeShapeId)) {
            activeTextId = null;
            activeShapeId = null;
            render();
          }
          selectionPending = true;
          selectionStartX = x;
          selectionStartY = y;
          return;
        }
        if (!t && !s && mode !== 'rect' && mode !== 'mosaic') {
          if (activeTextId || activeShapeId) {
            activeTextId = null;
            activeShapeId = null;
            render();
          }
        }
      }
      if (!t && !s && mode === '' && canvasInner && e.button === 0) {
        isPanning = true;
        panStartX = e.clientX;
        panStartY = e.clientY;
        panStartScrollLeft = canvasInner.scrollLeft;
        panStartScrollTop = canvasInner.scrollTop;
        canvas.style.cursor = 'grabbing';
        e.preventDefault();
      }
    });

    canvas.addEventListener('mousemove', e => {
      if (!baseImage) return;
      if (isReadOnlyMode()) {
        if (isPanning) {
          canvas.style.cursor = 'grabbing';
        } else if (mode === '') {
          canvas.style.cursor = 'grab';
        } else {
          canvas.style.cursor = 'default';
        }
        return;
      }
      const point = getCanvasPoint(e);
      const x = point.x;
      const y = point.y;
      if (pendingTextClick && !draggingText) {
        const dx = Math.abs(x - pendingTextClickStartX);
        const dy = Math.abs(y - pendingTextClickStartY);
        if (dx >= SELECT_DRAG_THRESHOLD || dy >= SELECT_DRAG_THRESHOLD) {
          const t = texts.find(item => item.id === pendingTextClickId);
          if (t) {
            draggingText = true;
            dragTextOffsetX = x - t.x;
            dragTextOffsetY = y - t.y;
          }
          pendingTextClick = false;
        }
      }
      if (selectionPending && !isDraggingSelection) {
        const dx = Math.abs(x - selectionStartX);
        const dy = Math.abs(y - selectionStartY);
        if (dx >= SELECT_DRAG_THRESHOLD || dy >= SELECT_DRAG_THRESHOLD) {
          isDraggingSelection = true;
          selectionPending = false;
          selectionSnapshot = ctx.getImageData(0, 0, canvas.width, canvas.height);
        }
      }
      if (isPanning) {
        canvas.style.cursor = 'grabbing';
        return;
      }
      if (resizingShape) {
        const s = shapes.find(item => item.id === activeShapeId);
        if (!s || !resizeStartShape) return;
        const dx = x - resizeStartX;
        const dy = y - resizeStartY;
        const minSize = 10;
        let nx = resizeStartShape.x;
        let ny = resizeStartShape.y;
        let nw = resizeStartShape.w;
        let nh = resizeStartShape.h;
        if (resizeEdge === 'left') {
          nx = resizeStartShape.x + dx;
          nw = resizeStartShape.w - dx;
          if (nw < minSize) {
            nx = resizeStartShape.x + (resizeStartShape.w - minSize);
            nw = minSize;
          }
        } else if (resizeEdge === 'right') {
          nw = resizeStartShape.w + dx;
          if (nw < minSize) {
            nw = minSize;
          }
        } else if (resizeEdge === 'top') {
          ny = resizeStartShape.y + dy;
          nh = resizeStartShape.h - dy;
          if (nh < minSize) {
            ny = resizeStartShape.y + (resizeStartShape.h - minSize);
            nh = minSize;
          }
        } else if (resizeEdge === 'bottom') {
          nh = resizeStartShape.h + dy;
          if (nh < minSize) {
            nh = minSize;
          }
        } else if (resizeEdge === 'top-left') {
          nx = resizeStartShape.x + dx;
          ny = resizeStartShape.y + dy;
          nw = resizeStartShape.w - dx;
          nh = resizeStartShape.h - dy;
          if (nw < minSize) {
            nx = resizeStartShape.x + (resizeStartShape.w - minSize);
            nw = minSize;
          }
          if (nh < minSize) {
            ny = resizeStartShape.y + (resizeStartShape.h - minSize);
            nh = minSize;
          }
        } else if (resizeEdge === 'top-right') {
          ny = resizeStartShape.y + dy;
          nw = resizeStartShape.w + dx;
          nh = resizeStartShape.h - dy;
          if (nw < minSize) {
            nw = minSize;
          }
          if (nh < minSize) {
            ny = resizeStartShape.y + (resizeStartShape.h - minSize);
            nh = minSize;
          }
        } else if (resizeEdge === 'bottom-left') {
          nx = resizeStartShape.x + dx;
          nw = resizeStartShape.w - dx;
          nh = resizeStartShape.h + dy;
          if (nw < minSize) {
            nx = resizeStartShape.x + (resizeStartShape.w - minSize);
            nw = minSize;
          }
          if (nh < minSize) {
            nh = minSize;
          }
        } else if (resizeEdge === 'bottom-right') {
          nw = resizeStartShape.w + dx;
          nh = resizeStartShape.h + dy;
          if (nw < minSize) {
            nw = minSize;
          }
          if (nh < minSize) {
            nh = minSize;
          }
        }
        s.x = nx;
        s.y = ny;
        s.w = nw;
        s.h = nh;
        if (resizeEdge === 'left' || resizeEdge === 'right') {
          canvas.style.cursor = 'ew-resize';
        } else if (resizeEdge === 'top' || resizeEdge === 'bottom') {
          canvas.style.cursor = 'ns-resize';
        } else if (resizeEdge === 'top-left' || resizeEdge === 'bottom-right') {
          canvas.style.cursor = 'nwse-resize';
        } else if (resizeEdge === 'top-right' || resizeEdge === 'bottom-left') {
          canvas.style.cursor = 'nesw-resize';
        }
        render();
        return;
      }
      if (draggingText) {
        const t = texts.find(item => item.id === activeTextId);
        if (!t) return;
        t.x = x - dragTextOffsetX;
        t.y = y - dragTextOffsetY;
        render();
        return;
      }
      if (draggingShape) {
        const s = shapes.find(item => item.id === activeShapeId);
        if (!s) return;
        s.x = x - dragShapeOffsetX;
        s.y = y - dragShapeOffsetY;
        render();
        return;
      }
      if (isDraggingSelection && (mode === 'rect' || mode === 'mosaic')) {
        const currentX = x;
        const currentY = y;
        const sx = Math.min(selectionStartX, currentX);
        const sy = Math.min(selectionStartY, currentY);
        const w = Math.abs(currentX - selectionStartX);
        const h = Math.abs(currentY - selectionStartY);
        if (selectionSnapshot) {
          ctx.putImageData(selectionSnapshot, 0, 0);
        }
        if (w < 2 || h < 2) return;
        ctx.save();
        if (mode === 'rect') {
          ctx.strokeStyle = rectColorInput?.value || '#ff0000';
          ctx.lineWidth = parseFloat(rectWidthInput?.value || '') || 4;
          const dashValue = rectDashSelect ? rectDashSelect.value : 'dashed';
          if (dashValue === 'dashed') {
            ctx.setLineDash([6, 3]);
          } else {
            ctx.setLineDash([]);
          }
          const fillMode = rectFillSelect ? rectFillSelect.value : 'fill';
          if (fillMode === 'fill') {
            ctx.fillStyle = rectColorInput?.value || '#ff0000';
          }
        } else {
          ctx.strokeStyle = STROKESTYLE;
          ctx.lineWidth = 3;
          ctx.setLineDash([4, 4]);
        }
        if (rectShapeSelect && rectShapeSelect.value === 'circle' && mode === 'rect') {
          const cx = sx + w / 2;
          const cy = sy + h / 2;
          const radius = Math.min(w, h) / 2;
          ctx.beginPath();
          ctx.arc(cx, cy, radius, 0, Math.PI * 2);
          if (mode === 'rect') {
            const fillMode = rectFillSelect ? rectFillSelect.value : 'fill';
            if (fillMode === 'fill') {
              ctx.fill();
            }
          }
          ctx.stroke();
        } else {
          if (mode === 'rect') {
            const fillMode = rectFillSelect ? rectFillSelect.value : 'fill';
            if (fillMode === 'fill') {
              ctx.fillRect(sx, sy, w, h);
            }
          }
          ctx.strokeRect(sx, sy, w, h);
        }
        ctx.restore();
        return;
      }
      canvas.style.cursor = 'default';
      if (activeShapeId) {
        const activeShape = shapes.find(item => item.id === activeShapeId);
        if (activeShape) {
          const edge = getShapeResizeEdge(activeShape, x, y);
          if (edge === 'left' || edge === 'right') {
            canvas.style.cursor = 'ew-resize';
          } else if (edge === 'top' || edge === 'bottom') {
            canvas.style.cursor = 'ns-resize';
          } else if (edge === 'top-left' || edge === 'bottom-right') {
            canvas.style.cursor = 'nwse-resize';
          } else if (edge === 'top-right' || edge === 'bottom-left') {
            canvas.style.cursor = 'nesw-resize';
          } else if (
            x >= activeShape.x &&
            x <= activeShape.x + activeShape.w &&
            y >= activeShape.y &&
            y <= activeShape.y + activeShape.h
          ) {
            canvas.style.cursor = 'move';
          }
        }
      } else if (mode === 'rect' || mode === 'mosaic') {
        canvas.style.cursor = 'crosshair';
      } else if (mode === 'text') {
        canvas.style.cursor = 'text';
      } else if (mode === '') {
        canvas.style.cursor = 'grab';
      }
    });

    canvas.addEventListener('mouseup', e => {
      if (!baseImage) return;
      if (isReadOnlyMode()) return;
      const point = getCanvasPoint(e);
      const x = point.x;
      const y = point.y;
      if (pendingTextClick) {
        pendingTextClick = false;
        pendingTextClickId = null;
        return;
      }
      if (selectionPending && !isDraggingSelection) {
        selectionPending = false;
        return;
      }
      if (resizingShape) {
        resizingShape = false;
        justDraggedSomething = true;
        const s = shapes.find(item => item.id === activeShapeId);
        if (!s) return;
        render();
        pushHistory();
        return;
      }
      if (draggingText) {
        draggingText = false;
        justDraggedSomething = true;
        const t = texts.find(item => item.id === activeTextId);
        if (!t) return;
        t.x = x - dragTextOffsetX;
        t.y = y - dragTextOffsetY;
        render();
        pushHistory();
        return;
      }
      if (draggingShape) {
        draggingShape = false;
        justDraggedSomething = true;
        const s = shapes.find(item => item.id === activeShapeId);
        if (!s) return;
        s.x = x - dragShapeOffsetX;
        s.y = y - dragShapeOffsetY;
        render();
        pushHistory();
        return;
      }
      if (!isDraggingSelection || (mode !== 'rect' && mode !== 'mosaic')) {
        isDraggingSelection = false;
        return;
      }
      isDraggingSelection = false;
      const currentX = x;
      const currentY = y;
      const sx = Math.min(selectionStartX, currentX);
      const sy = Math.min(selectionStartY, currentY);
      const w = Math.abs(currentX - selectionStartX);
      const h = Math.abs(currentY - selectionStartY);
      if (w < 2 || h < 2) {
        render();
        return;
      }
      const id = Date.now() + '_' + Math.random();
      if (mode === 'rect') {
        const shapeKind = rectShapeSelect ? rectShapeSelect.value : 'rect';
        const dashValue = rectDashSelect ? rectDashSelect.value : 'dashed';
        const newShape: ShapeItem = {
          id,
          type: shapeKind === 'circle' ? 'circle' : 'rect',
          x: sx,
          y: sy,
          w,
          h,
          color: rectColorInput?.value || '#ff0000',
          lineWidth: parseFloat(rectWidthInput?.value || '') || 2,
          lineDash: dashValue === 'dashed' ? [6, 3] : [],
          fill: (rectFillSelect ? rectFillSelect.value : 'fill') as 'fill' | 'stroke'
        };
        shapes.push(newShape);
        activeShapeId = id;
        activeTextId = null;
      } else if (mode === 'mosaic') {
        const newShape: ShapeItem = {
          id,
          type: 'mosaic',
          x: sx,
          y: sy,
          w,
          h,
          mosaicSize: mosaicSize || 10
        };
        shapes.push(newShape);
        activeShapeId = id;
        activeTextId = null;
      }
      render();
      pushHistory();
    });
  }

  if (deleteSelectionBtn) {
    deleteSelectionBtn.addEventListener('click', () => {
      if (isReadOnlyMode()) return;
      if (!baseImage) return;
      const removed: { type: 'text' | 'shape'; id: string }[] = [];
      if (activeTextId) {
        removed.push({ type: 'text', id: activeTextId });
        texts = texts.filter(item => item.id !== activeTextId);
        activeTextId = null;
      }
      if (activeShapeId) {
        removed.push({ type: 'shape', id: activeShapeId });
        shapes = shapes.filter(item => item.id !== activeShapeId);
        activeShapeId = null;
      }
      if (!removed.length) {
        updateDeleteButtonPosition();
        return;
      }
      history.forEach(state => {
        removed.forEach(info => {
          if (info.type === 'text') {
            state.texts = state.texts.filter(t => t.id !== info.id);
          } else if (info.type === 'shape') {
            state.shapes = state.shapes.filter(s => s.id !== info.id);
          }
        });
      });
      render();
      pushHistory();
    });
  }

  if (textBtn) {
    textBtn.addEventListener('click', () => {
      setMode('text');
    });
  }

  if (mouseBtn) {
    mouseBtn.addEventListener('click', () => {
      setMode('');
    });
  }

  if (rectBtn) {
    rectBtn.addEventListener('click', () => {
      setMode('rect');
    });
  }

  if (mosaicBtn) {
    mosaicBtn.addEventListener('click', () => {
      setMode('mosaic');
    });
  }

  if (textPopupClose) {
    textPopupClose.addEventListener('click', (e) => {
      e.preventDefault();
      e.stopPropagation();
      closePopup(textPopup);
    });
  }
  if (rectPopupClose) {
    rectPopupClose.addEventListener('click', (e) => {
      e.preventDefault();
      e.stopPropagation();
      closePopup(rectPopup);
    });
  }
  if (mosaicPopupClose) {
    mosaicPopupClose.addEventListener('click', (e) => {
      e.preventDefault();
      e.stopPropagation();
      closePopup(mosaicPopup);
    });
  }

  function updateColorSwatchSelection(targetId: string) {
    const group = container.querySelector(
      '.reie-color-swatches[data-target="' + targetId + '"]',
    ) as HTMLElement | null;
    const input = container.querySelector('#' + targetId) as HTMLInputElement | null;
    if (!group || !input) return;
    const selectedColor = (input.value || '').toLowerCase();
    const buttons = group.querySelectorAll('.reie-color-swatch') as NodeListOf<HTMLButtonElement>;
    buttons.forEach(b => {
      const c = (b.getAttribute('data-color') || '').toLowerCase();
      if (c && c === selectedColor) {
        b.classList.add('reie-color-swatch-selected');
      } else {
        b.classList.remove('reie-color-swatch-selected');
      }
    });
  }

  colorSwatches.forEach(btn => {
    const color = btn.getAttribute('data-color') || '';
    const parent = btn.parentElement as HTMLElement | null;
    const targetId = parent ? parent.getAttribute('data-target') : '';
    if (color) {
      btn.style.backgroundColor = color;
    }
    btn.addEventListener('click', () => {
      if (!color || !targetId) return;
      const input = container.querySelector('#' + targetId) as HTMLInputElement | null;
      if (!input) return;
      input.value = color;
      updateColorSwatchSelection(targetId);
    });
  });

  if (textColorInput) {
    textColorInput.addEventListener('input', () => {
      updateColorSwatchSelection('textColor');
    });
  }
  if (rectColorInput) {
    rectColorInput.addEventListener('input', () => {
      updateColorSwatchSelection('rectColor');
    });
  }
  updateColorSwatchSelection('textColor');
  updateColorSwatchSelection('rectColor');

  if (mosaicSizeInput) {
    mosaicSize = parseFloat(mosaicSizeInput.value || '') || 10;
    mosaicSizeInput.addEventListener('input', () => {
      mosaicSize = parseFloat(mosaicSizeInput.value || '') || 10;
    });
  }

  // 键盘快捷键处理：模式切换、撤销重做、缩放等
  let hotkeysActive = false;
  const handleContainerEnter = () => {
    hotkeysActive = true;
  };
  const handleContainerLeave = () => {
    hotkeysActive = false;
    const active = document.activeElement as HTMLElement | null;
    if (active && container.contains(active)) {
      active.blur();
    }
  };

  container.addEventListener('mouseenter', handleContainerEnter);
  container.addEventListener('mouseleave', handleContainerLeave);

  const handleKeyDown = (e: KeyboardEvent) => {
    const active = document.activeElement as Node | null;
    if (!hotkeysActive && (!active || !container.contains(active))) {
      return;
    }
    const target = e.target as HTMLElement | null;
    const tag = target && target.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA' || (target && (target as any).isContentEditable)) {
      return;
    }
    if (isReadOnlyMode()) {
      if (e.key === '+' || e.key === '=') {
        zoomInBtn?.click();
        e.preventDefault();
        return;
      }
      if (e.key === '-' || e.key === '_') {
        zoomOutBtn?.click();
        e.preventDefault();
        return;
      }
      return;
    }
    if (e.key === 'Escape') {
      if (editingTextId) {
        cancelTextEditor();
        return;
      }
      if (draggingText || draggingShape || resizingShape || isDraggingSelection || selectionPending) {
        draggingText = false;
        draggingShape = false;
        resizingShape = false;
        isDraggingSelection = false;
        selectionPending = false;
        selectionSnapshot = null;
        render();
        return;
      }
      activeTextId = null;
      activeShapeId = null;
      setMode('');
      render();
      return;
    }
    if (e.metaKey || e.ctrlKey) {
      if (!e.shiftKey && (e.key === 'z' || e.key === 'Z')) {
        undoBtn?.click();
        e.preventDefault();
        return;
      }
      if (e.shiftKey && (e.key === 'z' || e.key === 'Z')) {
        redoBtn?.click();
        e.preventDefault();
        return;
      }
      if (e.key === '+' || e.key === '=') {
        zoomInBtn?.click();
        e.preventDefault();
        return;
      }
      if (e.key === '-' || e.key === '_') {
        zoomOutBtn?.click();
        e.preventDefault();
        return;
      }
      return;
    }
    if (!e.metaKey && !e.ctrlKey && (e.key === 't' || e.key === 'T')) {
      setMode('text');
      e.preventDefault();
      return;
    }
    if (!e.metaKey && !e.ctrlKey && (e.key === "r" || e.key === "R")) {
      setMode('rect');
      e.preventDefault();
      return;
    }
    if (!e.metaKey && !e.ctrlKey && (e.key === 'm' || e.key === 'M')) {
      setMode('mosaic');
      e.preventDefault();
      return;
    }
  };

  // 点击工具栏之外时关闭工具弹层
  const handleDocumentClick = (e: MouseEvent) => {
    if (!toolbar) return;
    const target = e.target as Node | null;
    if (target && toolbar.contains(target)) {
      return;
    }
    closeAllPopups();
  };

  document.addEventListener('keydown', handleKeyDown);
  document.addEventListener('click', handleDocumentClick);

  // 撤销按钮：从历史栈中回退一步
  if (undoBtn) {
    undoBtn.addEventListener('click', () => {
      if (isReadOnlyMode()) return;
      if (historyIndex <= 0) return;
      historyIndex -= 1;
      const state = history[historyIndex];
      if (!state) return;
      texts = cloneTexts(state.texts);
      shapes = cloneShapes(state.shapes || []);
      render();
    });
  }

  // 重做按钮：从历史栈中前进一步
  if (redoBtn) {
    redoBtn.addEventListener('click', () => {
      if (isReadOnlyMode()) return;
      if (historyIndex >= history.length - 1) return;
      historyIndex += 1;
      const state = history[historyIndex];
      if (!state) return;
      texts = cloneTexts(state.texts);
      shapes = cloneShapes(state.shapes || []);
      render();
    });
  }

  // 重置按钮：恢复为 originalImage 并清空所有标注
  if (resetBtn) {
    resetBtn.addEventListener('click', () => {
      if (isReadOnlyMode()) return;
      if (!originalImage) return;
      baseImage = cloneImageData(originalImage);
      texts = [];
      shapes = [];
      history = [];
      historyIndex = -1;
      if (canvasInner && baseImage) {
        const rect = canvasInner.getBoundingClientRect();
        const scaleX = rect.width / baseImage.width;
        viewScale = Math.min(scaleX, 1);
        autoFit = true;
      } else {
        viewScale = 1;
        autoFit = false;
      }
      pushHistory();
      render();
    });
  }

  // 缩放按钮：放大视图
  if (zoomInBtn) {
    zoomInBtn.addEventListener('click', () => {
      if (!baseImage) return;
      setViewScale(viewScale + 0.1);
    });
  }

  // 缩放按钮：缩小视图
  if (zoomOutBtn) {
    zoomOutBtn.addEventListener('click', () => {
      if (!baseImage) return;
      setViewScale(viewScale - 0.1);
    });
  }

  // 将当前画布导出为图片，并触发下载或回调
  function downloadImage() {
    if (!canvas.width || !canvas.height) return;
    render({ noOverlay: true, noShadow: true });
    const url = canvas.toDataURL('image/jpeg', 0.92);
    if (options.onDownload) {
      options.onDownload(url);
    } else {
      const a = document.createElement('a');
      a.href = url;
      a.download = 'edit.jpg';
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
    }
    render();
  }

  // 组装当前编辑结果（尺寸 + 文本 + 图形）
  function getExportData(): ImageEditorExportData | null {
    const payload: ImageEditorExportData = {
      image: baseImage
        ? {
          width: baseImage.width,
          height: baseImage.height
        }
        : null,
      texts: cloneTexts(texts),
      shapes: cloneShapes(shapes) as ShapeItem[],
    };
    return payload;
  }

  // 导出编辑数据，并触发 onExport 回调
  function exportData(): ImageEditorExportData | null {
    const payload = getExportData();
    if (!payload) return null;
    if (options.onExport) {
      options.onExport(payload);
    }
    return payload;
  }

  // 监听编辑区域大小变化，自动调整缩放与文本位置
  if (canvasInner) {
    resizeObserver = new ResizeObserver(() => {
      if (!baseImage) return;
      const rect = canvasInner.getBoundingClientRect();
      const scaleX = rect.width / baseImage.width;
      if (autoFit) {
        viewScale = Math.min(scaleX, 1);
      }
      updateZoomView();
    });
    resizeObserver.observe(canvasInner);
    canvasInner.addEventListener('scroll', () => {
      updateTextEditorPosition();
      updateDeleteButtonPosition();
    });
    canvasInner.addEventListener('wheel', handleCanvasWheel);
  }

  // 对外暴露的重置方法，恢复 baseImage 并清空标注
  function reset() {
    if (!originalImage) return;
    baseImage = cloneImageData(originalImage);
    texts = [];
    shapes = [];
    history = [];
    historyIndex = -1;
    if (canvasInner && baseImage) {
      const rect = canvasInner.getBoundingClientRect();
      const scaleX = rect.width / baseImage.width;
      viewScale = Math.min(scaleX, 1);
    } else {
      viewScale = 1;
    }
    pushHistory();
    render();
  }

  // 清理事件监听和观察器，组件卸载时调用
  function destroy() {
    document.removeEventListener('keydown', handleKeyDown);
    document.removeEventListener('click', handleDocumentClick);
    document.removeEventListener('mousemove', handlePanMove);
    document.removeEventListener('mouseup', handlePanEnd);
    container.removeEventListener('mouseenter', handleContainerEnter);
    container.removeEventListener('mouseleave', handleContainerLeave);
    if (resizeObserver) {
      resizeObserver.disconnect();
      resizeObserver = null;
    }
    if (canvasInner) {
      canvasInner.removeEventListener('wheel', handleCanvasWheel);
    }
  }

  // 获取编辑图片base64
  function getEditImageBase64() {
    if (!canvas.width || !canvas.height) return '';
    render({ noOverlay: true, noShadow: true });
    const url = canvas.toDataURL('image/png');
    render();
    return url;
  }

  document.addEventListener('mousemove', handlePanMove);
  document.addEventListener('mouseup', handlePanEnd);

  setMode('');

  return {
    getEditImageBase64,
    loadImageFromFile,
    loadImageFromUrl,
    downloadImage,
    getExportData,
    exportData,
    setData,
    reset,
    destroy,
  };
}

// 图片编辑器 React 组件，对外暴露简化的操作方法
export const ImageEditor = forwardRef<ImageEditorHandle, ImageEditorProps>(
  (props, ref) => {
    // 组件入参说明：
    // - initialImageFile: 初始图片文件（File）。优先级高于 initialImageSrc
    // - initialImageSrc: 初始图片地址（string URL）。未提供 initialImageFile 时使用
    // - onExport: 用户执行“导出”操作时的回调，返回编辑结果数据
    // - onDownload: 用户执行“下载”操作时的回调，返回导出的 dataURL
    // - className: 根容器的额外类名
    // - style: 根容器的内联样式
    const {
      initialImageFile,
      initialImageSrc,
      imgData,
      onExport,
      onDownload,
      onImageLoaded,
      className,
      style,
      isLoading = false,
      enableLeaveTip = true,
      isReadOnly = false,
    } = props;
    const containerRef = useRef<HTMLDivElement | null>(null);
    const apiRef = useRef<EditorApi | null>(null);
    const initialDataRef = useRef<ImageEditorExportData | null>(null);

    const [imageSize, setImageSize] = useState<{ width: number; height: number } | null>(null);
    const onImageLoadedCall = useCallback((size: { width: number; height: number }) => {
      setImageSize(size);
      if (onImageLoaded) {
        onImageLoaded(size);
      }
    }, [onImageLoaded]);
    useEffect(() => {
      if (!containerRef.current) return;
      const api = initImageEditor(containerRef.current, {
        onExport,
        onDownload,
        onImageLoaded: onImageLoadedCall,
      });
      apiRef.current = api;
      // 这里需要拿到图片的原始宽高
      
      return () => {
        api.destroy();
        apiRef.current = null;
      };
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    useEffect(() => {
      if (!apiRef.current) return;
      if (initialImageFile) {
        apiRef.current.loadImageFromFile(initialImageFile);
      } else if (initialImageSrc) {
        apiRef.current.loadImageFromUrl(initialImageSrc);
      }
    }, [initialImageFile, initialImageSrc]);

    useEffect(() => {
      if (!apiRef.current) return;
      if (!imgData) return;
      apiRef.current.setData(imgData);
      initialDataRef.current = imgData;
    }, [imgData]);

    useEffect(() => {
      if (!enableLeaveTip) return;
      if (!apiRef.current) return;
      if (initialDataRef.current) return;
      const data = apiRef.current.getExportData();
      if (data) {
        initialDataRef.current = data;
      }
    }, [enableLeaveTip, initialImageFile, initialImageSrc]);

    useEffect(() => {
      if (!enableLeaveTip) return;

      const handleBeforeUnload = (e: BeforeUnloadEvent) => {
        if (!apiRef.current) return;
        if (!enableLeaveTip) return;

        const current = apiRef.current.getExportData();
        const initial = initialDataRef.current;

        let edited = false;
        if (!initial) {
          edited =
            !!current &&
            ((current.texts && current.texts.length > 0) ||
              (current.shapes && current.shapes.length > 0));
        } else if (current) {
          edited = JSON.stringify(current) !== JSON.stringify(initial);
        }

        if (!edited) {
          return;
        }

        e.preventDefault();
        e.returnValue = "";
      };

      window.addEventListener("beforeunload", handleBeforeUnload);

      return () => {
        window.removeEventListener("beforeunload", handleBeforeUnload);
      };
    }, [enableLeaveTip]);

    useImperativeHandle(ref, () => ({
      loadImageFile(file: File) {
        apiRef.current?.loadImageFromFile(file);
      },
      loadImageUrl(url: string) {
        apiRef.current?.loadImageFromUrl(url);
      },
      downloadImage() {
        apiRef.current?.downloadImage();
      },
      getExportData() {
        return apiRef.current?.getExportData() ?? null;
      },
      exportData() {
        return apiRef.current?.exportData() ?? null;
      },
      reset() {
        apiRef.current?.reset();
      },
      getEditImageBase64() {
        return apiRef.current?.getEditImageBase64() ?? '';
      },
    }));
    return (
      <div
        ref={containerRef}
        className={`reie-root${className ? ' ' + className : ''}`}
        style={style}
        data-read-only={isReadOnly ? 'true' : 'false'}
      >
        {isLoading && (
          <div className="reie-loading-overlay">
            <div className="reie-loading-spinner" />
          </div>
        )}
        <div className="reie-toolbar">
          <div className="reie-toolbar-row">
            <div className="reie-zoom-bar reie-zoom-bar-image-size">
              {imageSize && (
                <span>
                  {imageSize.width} x {imageSize.height} px
                </span>
              )}
            </div>
            <div className="reie-zoom-bar">
              <button data-role="zoom-out-btn">-</button>
              <span data-role="zoom-percent">100%</span>
              <button data-role="zoom-in-btn">+</button>
            </div>
            <div className="reie-tool-group reie-tool-group-mode" title="鼠标">
              <ToolTip content="鼠标">
                <svg
                  data-role="mouse-btn"
                  stroke="currentColor"
                  fill="none"
                  strokeWidth="2"
                  viewBox="0 0 24 24"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  height="1em"
                  width="1em"
                  xmlns="http://www.w3.org/2000/svg"
                >
                  <path d="M12.034 12.681a.498.498 0 0 1 .647-.647l9 3.5a.5.5 0 0 1-.033.943l-3.444 1.068a1 1 0 0 0-.66.66l-1.067 3.443a.5.5 0 0 1-.943.033z" />
                  <path d="M21 11V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h6" />
                </svg>
              </ToolTip>
            </div>
            {!isReadOnly && (
              <>
                <div className="reie-tool-group reie-tool-group-mode" title="文字">
                  <ToolTip content="文字">
                    <svg
                      data-role="text-btn"
                      aria-label="文字"
                      stroke="currentColor"
                      fill="none"
                      strokeWidth="0"
                      viewBox="0 0 15 15"
                      height="1em"
                      width="1em"
                      xmlns="http://www.w3.org/2000/svg"
                    >
                      <title>文字</title>
                      <path
                        fillRule="evenodd"
                        clipRule="evenodd"
                        d="M3.94993 2.95002L3.94993 4.49998C3.94993 4.74851 3.74845 4.94998 3.49993 4.94998C3.2514 4.94998 3.04993 4.74851 3.04993 4.49998V2.50004C3.04993 2.45246 3.05731 2.40661 3.07099 2.36357C3.12878 2.18175 3.29897 2.05002 3.49993 2.05002H11.4999C11.6553 2.05002 11.7922 2.12872 11.8731 2.24842C11.9216 2.32024 11.9499 2.40682 11.9499 2.50002L11.9499 2.50004V4.49998C11.9499 4.74851 11.7485 4.94998 11.4999 4.94998C11.2514 4.94998 11.0499 4.74851 11.0499 4.49998V2.95002H8.04993V12.05H9.25428C9.50281 12.05 9.70428 12.2515 9.70428 12.5C9.70428 12.7486 9.50281 12.95 9.25428 12.95H5.75428C5.50575 12.95 5.30428 12.7486 5.30428 12.5C5.30428 12.2515 5.50575 12.05 5.75428 12.05H6.94993V2.95002H3.94993Z"
                        fill="currentColor"
                      />
                    </svg>
                  </ToolTip>
                  <div data-role="text-popup" className="reie-tool-popup">
                    <div className="reie-tool-popup-header">
                      <span className="reie-tool-popup-title">设置</span>
                      <button type="button" className="reie-tool-popup-close" data-role="text-popup-close" aria-label="关闭">×</button>
                    </div>
                    <div className="reie-tool-popup-row">
                      <span className="reie-tool-popup-label">颜色</span>
                      <div className="reie-color-swatches" data-target="textColor">
                        <button className="reie-color-swatch" data-color="#ff0000" aria-label="红色" />
                        <button className="reie-color-swatch" data-color="#fa8c16" aria-label="橙色" />
                        <button className="reie-color-swatch" data-color="#fadb14" aria-label="黄色" />
                        <button className="reie-color-swatch" data-color="#52c41a" aria-label="绿色" />
                        <button
                          className="reie-color-swatch"
                          data-color="#1677ff"
                          aria-label="蓝色"
                        />
                        <button className="reie-color-swatch" data-color="#722ed1" aria-label="紫色" />
                      </div>
                      <input
                        type="color"
                        defaultValue="#ff0000"
                        className="reie-popup-color-input"
                        id="textColor"
                      />
                    </div>
                    <div className="reie-tool-popup-row">
                      <span className="reie-tool-popup-label">大小</span>
                      <input
                        type="number"
                        defaultValue={30}
                        min={10}
                        max={100}
                        step={2}
                        data-role="text-size"
                      />
                    </div>
                  </div>
                </div>
                <div className="reie-tool-group reie-tool-group-mode" title="框选">
                  <ToolTip content="框选">
                    <svg data-role="rect-btn" aria-label="框选" stroke="currentColor" fill="currentColor" strokeWidth="0" viewBox="0 0 24 24" height="1em" width="1em" xmlns="http://www.w3.org/2000/svg"><path fill="none" d="M0 0h24v24H0V0zm24 24H0V0h24v24z" /><path d="M23 15h-2v2h2v-2zm0-4h-2v2h2v-2zm0 8h-2v2c1 0 2-1 2-2zM15 3h-2v2h2V3zm8 4h-2v2h2V7zm-2-4v2h2c0-1-1-2-2-2zM3 21h8v-6H1v4c0 1.1.9 2 2 2zM3 7H1v2h2V7zm12 12h-2v2h2v-2zm4-16h-2v2h2V3zm0 16h-2v2h2v-2zM3 3C2 3 1 4 1 5h2V3zm0 8H1v2h2v-2zm8-8H9v2h2V3zM7 3H5v2h2V3z" /></svg>
                  </ToolTip>
                  <div data-role="rect-popup" className="reie-tool-popup">
                    <div className="reie-tool-popup-header">
                      <span className="reie-tool-popup-title">设置</span>
                      <button type="button" className="reie-tool-popup-close" data-role="rect-popup-close" aria-label="关闭">×</button>
                    </div>
                    <div className="reie-tool-popup-row">
                      <span className="reie-tool-popup-label">颜色</span>
                      <div className="reie-color-swatches" data-target="rectColor">
                        <button className="reie-color-swatch" data-color="#ff0000" aria-label="红色" />
                        <button
                          className="reie-color-swatch"
                          data-color="#fa8c16"
                          aria-label="橙色"
                        />
                        <button className="reie-color-swatch" data-color="#fadb14" aria-label="黄色" />
                        <button
                          className="reie-color-swatch"
                          data-color="#52c41a"
                          aria-label="绿色"
                        />
                        <button
                          className="reie-color-swatch"
                          data-color="#1677ff"
                          aria-label="蓝色"
                        />
                        <button
                          className="reie-color-swatch"
                          data-color="#722ed1"
                          aria-label="紫色"
                        />
                      </div>
                      <input
                        type="color"
                        defaultValue="#ff0000"
                        className="reie-popup-color-input"
                        id="rectColor"
                      />
                    </div>
                    <div className="reie-tool-popup-row">
                      <span className="reie-tool-popup-label">边框</span>
                      <input
                        type="range"
                        min={1}
                        max={40}
                        defaultValue={5}
                        data-role="rect-width"
                      />
                    </div>
                    <div className="reie-tool-popup-row">
                      <span className="reie-tool-popup-label">形状</span>
                      <select data-role="rect-shape">
                        <option value="rect">矩形</option>
                        <option value="circle">圆形</option>
                      </select>
                    </div>
                    <div className="reie-tool-popup-row">
                      <span className="reie-tool-popup-label">线型</span>
                      <select data-role="rect-dash">
                        <option value="solid">实线</option>
                        <option value="dashed">虚线</option>
                      </select>
                    </div>
                    <div className="reie-tool-popup-row">
                      <span className="reie-tool-popup-label">填充</span>
                      <select data-role="rect-fill" defaultValue="fill">
                        <option value="fill">实心</option>
                        <option value="stroke">空心</option>
                      </select>
                    </div>
                  </div>
                </div>
                <div className="reie-tool-group reie-tool-group-mode" title="马赛克">
                  <ToolTip content="马赛克">
                    <svg
                      data-role="mosaic-btn"
                      aria-label="马赛克"
                      stroke="currentColor"
                      fill="currentColor"
                      strokeWidth="0"
                      viewBox="0 0 24 24"
                      height="1em"
                      width="1em"
                      xmlns="http://www.w3.org/2000/svg"
                    >
                      <title>马赛克</title>
                      <path fill="none" d="M0 0h24v24H0z" />
                      <path d="M3 5v14a2 2 0 0 0 2 2h6V3H5a2 2 0 0 0-2 2zm6 14H5V5h4v14zM19 3h-6v8h8V5c0-1.1-.9-2-2-2zm0 6h-4V5h4v4zM13 21h6c1.1 0 2-.9 2-2v-6h-8v8zm2-6h4v4h-4v-4z" />
                    </svg>
                  </ToolTip>
                  <div data-role="mosaic-popup" className="reie-tool-popup">
                    <div className="reie-tool-popup-header">
                      <span className="reie-tool-popup-title">设置</span>
                      <button type="button" className="reie-tool-popup-close" data-role="mosaic-popup-close" aria-label="关闭">×</button>
                    </div>
                    <div className="reie-tool-popup-row">
                      <span className="reie-tool-popup-label">模糊大小</span>
                      <input
                        type="range"
                        min={4}
                        max={40}
                        defaultValue={15}
                        data-role="mosaic-size"
                      />
                    </div>
                  </div>
                </div>
                <div className="reie-tool-group">
                  <ToolTip content="重置">
                    <div className="reie-tool-group-item" data-role="reset-btn" title="重置">
                      <svg
                        aria-label="重置"
                        stroke="currentColor"
                        fill="currentColor"
                        strokeWidth="0"
                        viewBox="0 0 24 24"
                        height="1em"
                        width="1em"
                        xmlns="http://www.w3.org/2000/svg"
                      >
                        <title>重置</title>
                        <path d="M22 12C22 17.5228 17.5229 22 12 22C6.4772 22 2 17.5228 2 12C2 6.47715 6.4772 2 12 2V4C7.5817 4 4 7.58172 4 12C4 16.4183 7.5817 20 12 20C16.4183 20 20 16.4183 20 12C20 9.53614 18.8862 7.33243 17.1346 5.86492L15 8V2L21 2L18.5535 4.44656C20.6649 6.28002 22 8.9841 22 12Z" />
                      </svg>
                    </div>
                  </ToolTip>
                  <ToolTip content="撤销">
                    <div className="reie-tool-group-item" data-role="undo-btn" title="撤销">
                      <svg stroke="currentColor" fill="currentColor" strokeWidth="0" viewBox="0 0 256 256" height="1em" width="1em" xmlns="http://www.w3.org/2000/svg"><path d="M232,184a8,8,0,0,1-16,0A88,88,0,0,0,65.78,121.78L43.4,144H88a8,8,0,0,1,0,16H24a8,8,0,0,1-8-8V88a8,8,0,0,1,16,0v44.77l22.48-22.33A104,104,0,0,1,232,184Z" /></svg>
                    </div>
                  </ToolTip>
                  <ToolTip content="恢复">
                    <div className="reie-tool-group-item" data-role="redo-btn" title="恢复">
                      <svg stroke="currentColor" fill="currentColor" strokeWidth="0" viewBox="0 0 256 256" height="1em" width="1em" xmlns="http://www.w3.org/2000/svg"><path d="M240,88v64a8,8,0,0,1-8,8H168a8,8,0,0,1,0-16h44.6l-22.36-22.21A88,88,0,0,0,40,184a8,8,0,0,1-16,0,104,104,0,0,1,177.54-73.54L224,132.77V88a8,8,0,0,1,16,0Z" /></svg>
                    </div>
                  </ToolTip>
                </div>
              </>
            )}
          </div>
        </div>
        <div className="reie-editor-wrap">
          <div className="reie-canvas-inner">
            <canvas data-role="canvas" />
            <svg
              className="reie-delete-selection-btn"
              data-role="delete-selection-btn"
              stroke="currentColor"
              fill="currentColor"
              strokeWidth="0"
              viewBox="0 0 24 24"
              height="1em"
              width="1em"
              xmlns="http://www.w3.org/2000/svg"
            >
              <path d="M7 4V2H17V4H22V6H20V21C20 21.5523 19.5523 22 19 22H5C4.44772 22 4 21.5523 4 21V6H2V4H7ZM6 6V20H18V6H6ZM9 9H11V17H9V9ZM13 9H15V17H13V9Z" />
            </svg>
          </div>
        </div>
        <style>{imageEditorStyles}</style>
      </div>
    );
  },
);
