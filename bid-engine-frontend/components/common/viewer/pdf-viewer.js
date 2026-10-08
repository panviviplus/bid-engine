import React, {
  useState,
  useEffect,
  useRef,
  useState as ReactUseState,
} from "react";
import { Spinner, Center, Flex, IconButton, Box } from "@chakra-ui/react";
import { ChevronLeftIcon, ChevronRightIcon } from "@chakra-ui/icons";
import {
  IoIosAddCircleOutline,
  IoIosRemoveCircleOutline,
} from "react-icons/io";
import { Worker, Viewer, SpecialZoomLevel } from "@react-pdf-viewer/core";
import { defaultLayoutPlugin } from "@react-pdf-viewer/default-layout";
import { highlightPlugin } from "@react-pdf-viewer/highlight";
import { pageNavigationPlugin } from "@react-pdf-viewer/page-navigation";
import { zoomPlugin } from "@react-pdf-viewer/zoom";
import { searchPlugin } from "@react-pdf-viewer/search";
import Empty from "@/components/common/empty";
import {
  getHighlightNavigationTarget,
  mapHighlightAreas,
} from "./pdf-navigation.mjs";
// import { scrollbarStyle } from "@/utils/style";

/**
 * PDF 查看器组件
 * @param {object} props
 * @param {string} [props.url]
 * @param {any} [props.highlightAreas]
 * @param {any} [props.onPagesContainerReady]
 * @param {boolean} [props.showHighlights]
 * @param {string} [props.pdfH]
 * @param {any} [props.onJumpToPageRef]
 * @param {any} [props.onSearchRef]
 * @param {((selection: object) => void) | null} [props.onTextSelection]
 * @param {((controls: object) => void) | null} [props.externalToolbar] 外部工具条回调：接收缩放/翻页控制项
 * @param {string} [props.defaultZoom] 打开文档后的初始缩放，默认适配页宽（SpecialZoomLevel.ActualSize 为 100%）
 */
function PDFViewer({
  url,
  highlightAreas,
  onPagesContainerReady,
  showHighlights = true,
  pdfH = "calc(100vh - 324px)",
  onJumpToPageRef,
  onSearchRef,
  onTextSelection = null,
  externalToolbar = null,
  defaultZoom = SpecialZoomLevel.PageWidth,
}) {
  const [rootEl, setRootEl] = ReactUseState();
  const [isDocumentLoaded, setIsDocumentLoaded] = useState(false);

  const newHighlightAreas = Array.isArray(highlightAreas)
    ? highlightAreas?.flatMap((arr) => arr?.highlight_areas) // 数组，一对多引用
    : highlightAreas?.highlight_areas; // 对象，一对一引用

  const zoomPluginInstance = zoomPlugin();
  const { zoomTo } = zoomPluginInstance;

  const pageNavigationPluginInstance = pageNavigationPlugin();
  const { jumpToPage } = pageNavigationPluginInstance;

  const searchPluginInstance = searchPlugin();

  // 外部工具条模式：把缩放/翻页控制项转发给父级（如审核详情页的面板头部一行展示）
  // 这些 render-prop 组件为插件实例装饰器（store 已注入），可在任意位置渲染
  const externalToolbarControls = externalToolbar
    ? {
        ZoomOut: zoomPluginInstance.ZoomOut,
        Zoom: zoomPluginInstance.Zoom,
        ZoomIn: zoomPluginInstance.ZoomIn,
        GoToPreviousPage: pageNavigationPluginInstance.GoToPreviousPage,
        GoToNextPage: pageNavigationPluginInstance.GoToNextPage,
        CurrentPageLabel: pageNavigationPluginInstance.CurrentPageLabel,
        NumberOfPages: pageNavigationPluginInstance.NumberOfPages,
      }
    : null;
  const forwardedToolbarRef = useRef(false);
  useEffect(() => {
    if (
      externalToolbar &&
      externalToolbarControls &&
      !forwardedToolbarRef.current
    ) {
      forwardedToolbarRef.current = true;
      externalToolbar(externalToolbarControls);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [externalToolbar]);

  // 暴露 jumpToPage 给父组件（1-based 页码 → 0-based pageIndex）
  useEffect(() => {
    if (onJumpToPageRef) {
      onJumpToPageRef.current = (pageNo) => {
        if (pageNo > 0) jumpToPage(pageNo - 1);
      };
    }
  }, [onJumpToPageRef, jumpToPage]);

  // 暴露搜索方法给父组件（用 source_text 搜索定位）
  useEffect(() => {
    if (onSearchRef) {
      onSearchRef.current = (keyword) => {
        if (!keyword) return;
        searchPluginInstance.highlight(keyword);
      };
    }
  }, [onSearchRef, searchPluginInstance]);

  const renderToolbar = externalToolbar
    ? () => null
    : (Toolbar) => (
        <Toolbar>
          {(slots) => {
            const {
              // CurrentPageInput,
              // CurrentScale,
              CurrentPageLabel,
              GoToNextPage,
              GoToPreviousPage,
              // NumberOfPages,
              Zoom,
              ZoomIn,
              ZoomOut,
            } = slots;
            return (
              <Flex
                w="full"
                justify="flex-end"
                align="center"
                px={3}
                py={1}
                bg="white"
                display={{ base: "none", md: "flex" }}
              >
                {/* 缩放 */}
                <Flex>
                  <ZoomOut>
                    {(props) => (
                      <IconButton
                        size="sm"
                        bg="transparent"
                        _hover={{
                          bg: "transparent",
                        }}
                        isDisabled={props.isDisabled}
                        icon={<IoIosRemoveCircleOutline fontSize="16px" />}
                        onClick={props.onClick}
                      />
                    )}
                  </ZoomOut>
                  <Zoom />
                  <ZoomIn>
                    {(props) => (
                      <IconButton
                        size="sm"
                        bg="transparent"
                        _hover={{
                          bg: "transparent",
                        }}
                        isDisabled={props.isDisabled}
                        icon={<IoIosAddCircleOutline fontSize="16px" />}
                        onClick={props.onClick}
                      />
                    )}
                  </ZoomIn>
                </Flex>
                {/* 翻页 */}
                <GoToPreviousPage>
                  {(props) => (
                    <IconButton
                      aria-label="previous page"
                      size="sm"
                      bg="transparent"
                      _hover={{
                        bg: "transparent",
                      }}
                      isDisabled={props.isDisabled}
                      icon={<ChevronLeftIcon fontSize="24px" />}
                      onClick={props.onClick}
                    />
                  )}
                </GoToPreviousPage>
                <CurrentPageLabel>
                  {(props) => (
                    <span>{`${props.currentPage + 1} / ${
                      props.numberOfPages
                    }`}</span>
                  )}
                </CurrentPageLabel>
                <GoToNextPage>
                  {(props) => (
                    <IconButton
                      aria-label="next page"
                      size="sm"
                      bg="transparent"
                      _hover={{
                        bg: "transparent",
                      }}
                      isDisabled={props.isDisabled}
                      icon={<ChevronRightIcon fontSize="24px" />}
                      onClick={props.onClick}
                    />
                  )}
                </GoToNextPage>
              </Flex>
            );
          }}
        </Toolbar>
      );

  const renderHighlights = (props) => {
    if (!showHighlights) {
      return null;
    }
    const mappedAreas = mapHighlightAreas(newHighlightAreas);
    return (
      <Box>
        {mappedAreas
          ?.filter((area) => area.pageIndex === props.pageIndex)
          ?.map((area, idx) => {
            return (
              <Box
                key={idx}
                style={{
                  background: area.active ? "#E5B94F" : "rgba(212,168,83,.08)",
                  border: area.active
                    ? "1px solid rgba(156,123,45,.45)"
                    : "1px solid rgba(212,168,83,.55)",
                  opacity: area.active ? 0.44 : 1,
                  transformOrigin: "left center",
                  animation: area.active
                    ? "evidenceScan 260ms cubic-bezier(.16,1,.3,1) both"
                    : "none",
                  ...props.getCssProperties(area, props.rotation),
                }}
              />
            );
          })}
      </Box>
    );
  };

  const defaultLayoutPluginInstance = defaultLayoutPlugin({
    sidebarTabs: () => [],
    renderToolbar,
  });
  const highlightPluginInstance = highlightPlugin({
    renderHighlights,
  });

  const { jumpToHighlightArea } = highlightPluginInstance;

  const renderError = (error) => {
    console.log(error);
    let message = "";
    switch (error.name) {
      case "InvalidPDFException":
        message = "文件无效或已损坏！";
        break;
      case "MissingPDFException":
        message = "抱歉，您访问的文件不存在！";
        break;
      case "UnexpectedResponseException":
        message = "服务器开小差啦！";
        break;
      default:
        message = "未知错误，无法加载文档！";
        break;
    }
    return <Empty type="bid-error" message={message} />;
  };

  const renderPage = (props) => {
    return (
      <>
        {props.canvasLayer.children}
        {props.textLayer.children}
      </>
    );
  };

  useEffect(() => {
    if (!isDocumentLoaded) {
      return;
    }

    zoomTo(defaultZoom);

    if (!jumpToHighlightArea) {
      return;
    }

    const targetArea = getHighlightNavigationTarget(
      newHighlightAreas,
      isDocumentLoaded,
    );
    if (!targetArea) return;
    const timeoutId = setTimeout(() => {
      jumpToHighlightArea(targetArea);
    }, 100);

    return () => {
      clearTimeout(timeoutId);
    };
  }, [
    isDocumentLoaded,
    jumpToHighlightArea,
    newHighlightAreas,
    zoomTo,
    defaultZoom,
  ]);

  useEffect(() => {
    if (!rootEl || !onPagesContainerReady) {
      return;
    }
    const inner = rootEl.querySelector(".rpv-core__inner-pages");
    if (inner) {
      onPagesContainerReady(inner);
    }
  }, [rootEl, onPagesContainerReady, isDocumentLoaded]);

  return (
    <>
      {!url && (
        <Center h="full" w="full">
          <Empty type="empty" />
        </Center>
      )}
      {url && (
        <Worker workerUrl="/pdf/pdf.worker.js">
          <Flex
            direction="column"
            w="full"
            flex={1}
            overflow="hidden"
            h={pdfH}
            ref={(dom) => {
              if (dom) {
                setRootEl(dom);
              }
            }}
            onMouseUp={() => {
              if (!onTextSelection || !rootEl) return;
              const selection = window.getSelection();
              const quote = selection?.toString().trim();
              if (!selection || !quote || selection.rangeCount === 0) return;
              const range = selection.getRangeAt(0);
              const anchor =
                range.commonAncestorContainer.nodeType === Node.TEXT_NODE
                  ? range.commonAncestorContainer.parentElement
                  : range.commonAncestorContainer;
              const page = anchor?.closest?.(".rpv-core__page-layer");
              if (!page) return;
              const pages = Array.from(
                rootEl.querySelectorAll(".rpv-core__page-layer"),
              );
              const pageIndex = pages.indexOf(page);
              const pageRect = page.getBoundingClientRect();
              const rect = range.getBoundingClientRect();
              if (pageIndex < 0 || !pageRect.width || !pageRect.height) return;
              onTextSelection({
                quote,
                page_no: pageIndex + 1,
                left: Math.max(0, (rect.left - pageRect.left) / pageRect.width),
                top: Math.max(0, (rect.top - pageRect.top) / pageRect.height),
                width: Math.min(1, rect.width / pageRect.width),
                height: Math.min(1, rect.height / pageRect.height),
              });
            }}
          >
            <Flex
              flex={1}
              direction="column"
              h="100%"
              sx={{
                ".rpv-default-layout__container": {
                  borderWidth: "0px",
                },
                ".rpv-default-layout__body": {
                  paddingTop: externalToolbar ? "0" : { base: "0", md: `40px` },
                  bg: "transparent",
                },
                ".rpv-default-layout__toolbar": {
                  height: externalToolbar ? "0" : { base: "0", md: `40px` },
                },
                ".rpv-core__inner-pages": {
                  paddingTop: { base: "0", md: "12px" },
                  bg: "transparent",
                  flex: 1,
                  // ...scrollbarStyle,
                },
                ".rpv-core__inner-page": {
                  bg: "transparent",
                },
              }}
            >
              <Viewer
                fileUrl={url}
                transformGetDocumentParams={(options) => ({
                  ...options,
                  disableAutoFetch: true,
                  disableStream: true,
                })}
                characterMap={{
                  isCompressed: true,
                  url: "/pdfFont/cmaps/",
                }}
                plugins={[
                  defaultLayoutPluginInstance,
                  highlightPluginInstance,
                  zoomPluginInstance,
                  pageNavigationPluginInstance,
                  searchPluginInstance,
                ]}
                withCredentials
                httpHeaders={{
                  "Content-Type": "multipart/form-data",
                }}
                defaultScale={isDocumentLoaded && defaultZoom}
                renderError={renderError}
                renderPage={renderPage}
                renderLoader={() => <Spinner color="primary.300" size="lg" />}
                onDocumentLoad={async () => {
                  setIsDocumentLoaded(true);
                }}
              />
            </Flex>
          </Flex>
        </Worker>
      )}
    </>
  );
}

export default PDFViewer;
