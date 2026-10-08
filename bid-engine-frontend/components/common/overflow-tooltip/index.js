import { memo, useEffect, useRef, useState } from "react";
import { Text, Tooltip, Flex } from "@chakra-ui/react";
// import { CopyIcon } from "@chakra-ui/icons";
// import { copyToClipboard } from "@/utils";
// import { useCustomToast } from "@/hooks/useCustomToast";

function OverflowTooltip({ children, ...props }) {
  const textElementRef = useRef(null);
  const [isOverflow, setIsOverflow] = useState(false);
  // const showToast = useCustomToast();

  /**
   * Check whether the element is overflow or not
   */
  const compareSize = () => {
    const element = textElementRef.current;
    const compare = element
      ? element.offsetWidth < element.scrollWidth ||
        element.offsetHeight < element.scrollHeight
      : false;
    setIsOverflow(compare);
  };
  useEffect(() => {
    compareSize();
  }, []);

  // const handleCopy = async () => {
  //   if (!children) return;
  //   try {
  //     await copyToClipboard(String(children));
  //     showToast({ status: "success", title: "复制成功" });
  //   } catch (e) {
  //     showToast({ status: "error", title: "复制失败，请手动复制" });
  //   }
  // };

  const content = (
    <Flex align="center">
      <Text
        wordBreak="break-word"
        flex={1}
        alignItems="center"
        fontSize="14px"
        noOfLines={1}
        ref={textElementRef}
        {...props}
      >
        {children ?? "-"}
      </Text>
      {/* {children && (
        <IconButton
          aria-label="复制"
          icon={<CopyIcon />}
          size="xs"
          variant="ghost"
          color="gray.400"
          ml={1}
          onClick={(e) => {
            e.stopPropagation();
            e.preventDefault();
            handleCopy();
          }}
        />
      )} */}
    </Flex>
  );

  return isOverflow ? (
    <Tooltip hasArrow label={children}>
      {content}
    </Tooltip>
  ) : (
    content
  );
}

export default memo(OverflowTooltip);
