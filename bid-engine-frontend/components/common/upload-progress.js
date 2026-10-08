import {
  Box,
  Flex,
  // Progress,
  Text,
  Icon,
  Tooltip,
  Collapse,
  Spacer,
  keyframes,
  useDisclosure,
} from "@chakra-ui/react";
import { RepeatIcon, CheckCircleIcon, WarningTwoIcon } from "@chakra-ui/icons";
import { getFileIcon } from "@/utils/upload";
import ExpandSVG from "@/components/svg/upload/expand";
import CloseSVG from "@/components/svg/upload/close";
import RefreshSVG from "@/components/svg/upload/refresh";

export default function UploadProgress({ uploadFiles }) {
  // 所有的uppy实例
  const uppys = uploadFiles?.map((item) => item.uppy);
  // 所有uppy实例中的文件
  const files = uploadFiles?.map((item) => item.files).flat();
  // 文件总数量
  const allFileLength = files.length;

  const spin = keyframes`from { transform: rotate(0deg);} to {transform: rotate(180deg);}`;
  const animation = `${spin} infinite 1s linear`;
  const { isOpen: isFileUploadStatusOpen, onToggle: onFileUploadStatusToggle } =
    useDisclosure();
  // 获取单个Uppy实例中  上传成功的数量和上传完成的数量
  const getUppyStatus = (uppy) => {
    const { erroredFiles: erroredFilesPDF, completeFiles: completeFilesPDF } =
      uppy.getObjectOfFilesPerState();
    // 上传成功的文件
    const successLength = completeFilesPDF.filter((file) => {
      return file?.response?.body?.code === 0;
    })?.length;
    // 完成的文件（成功或失败）
    const completeLength = erroredFilesPDF.length + completeFilesPDF.length;
    return { successLength, completeLength };
  };
  // 所有 Uppy实例
  let allCompleteLength = 0;
  let allSuccessLength = 0;
  uppys.forEach((uppy) => {
    const { successLength, completeLength } = getUppyStatus(uppy);
    allCompleteLength += completeLength;
    allSuccessLength += successLength;
  });
  const isAllErrored = allCompleteLength - allSuccessLength === allFileLength;
  const isUploadEnding = allCompleteLength === allFileLength;
  return (
    files?.length > 0 && (
      <Box
        pos="fixed"
        w={{ base: "280px", md: "360px" }}
        maxH="500px"
        overflowY="auto"
        className="thin-scrollbars"
        bottom={3}
        right={2}
        zIndex="tooltip"
        bg="neutral.800"
        borderRadius="8px"
        boxShadow="0px 4px 12px 0px rgba(0,0,0,0.10)"
      >
        <Flex
          align="center"
          py={{ base: 1, md: 3 }}
          px={{ base: "10px", md: 6 }}
          gap={4}
          bgColor="neutral.800"
          borderBottomWidth="1px"
          pos="sticky"
          top={0}
          zIndex="sticky"
        >
          {!isUploadEnding && (
            <Flex align="center" fontSize="lg">
              <RepeatIcon
                color="primary.600"
                boxSize={{ base: 5, md: 6 }}
                animation={animation}
              />
              <Box ml={2} fontWeight="medium">
                上传中{allSuccessLength}/{allFileLength}
              </Box>
            </Flex>
          )}

          {isUploadEnding && !isAllErrored && (
            <Flex align="center" fontSize="lg">
              <CheckCircleIcon color="G.600" boxSize={{ base: 5, md: 6 }} />
              <Box ml={2} fontWeight="medium">
                上传成功{allSuccessLength}/{allFileLength}
              </Box>
            </Flex>
          )}
          {isAllErrored && (
            <Flex align="center" fontSize="lg">
              <WarningTwoIcon
                color="red"
                boxSize={{ base: 5, md: 6 }}
                _hover={{ cursor: "pointer" }}
              />
              <Box ml={2} fontWeight="medium">
                上传失败
              </Box>
            </Flex>
          )}

          <Spacer />
          <ExpandSVG
            boxSize={{ base: 5, md: 6 }}
            _hover={{ cursor: "pointer" }}
            onClick={onFileUploadStatusToggle}
            transform={isFileUploadStatusOpen ? "" : "rotate(180deg)"}
          />
          <CloseSVG
            boxSize={{ base: 5, md: 6 }}
            _hover={{ cursor: "pointer" }}
            color="gray.500"
            onClick={() => {
              uppys.forEach((uppy) => {
                uppy.cancelAll("user");
              });
            }}
          />
        </Flex>
        <Collapse in={!isFileUploadStatusOpen} animateOpacity>
          {uploadFiles?.map((uppy) =>
            uppy.files?.map((file, index) => {
              const isFileError =
                file.error ||
                (file.progress.uploadComplete &&
                  file?.response?.body?.code !== 0);
              return (
                <Box key={index} mb={1}>
                  <Flex
                    px={{ base: "10px", md: 6 }}
                    py={{ base: 1, md: 3 }}
                    align="center"
                    gap={2}
                  >
                    <Icon
                      as={getFileIcon(file.extension)}
                      boxSize="40px"
                      mr="2px"
                      color="#7cbd58"
                    />

                    <Box flex={1}>
                      <Text
                        noOfLines={1}
                        wordBreak="break-all"
                        color="neutral.600"
                        fontSize={{ base: "sm", md: "md" }}
                      >
                        {file.name}
                      </Text>
                      {/* <Text
                        color={isFileError ? "red" : "neutral.400"}
                        fontSize="xs"
                      >
                        {getUploadMessage(file)}
                      </Text> */}
                    </Box>
                    {/* 上传失败--可重试，可删除 */}
                    {isFileError && (
                      <Flex gap={1}>
                        <Tooltip label="重新上传" hasArrow placement="auto">
                          <Flex align="center">
                            <RefreshSVG
                              color="neutral.400"
                              _hover={{ cursor: "pointer" }}
                              onClick={() => {
                                uppy.uppy.retryUpload(file.id);
                              }}
                            />
                          </Flex>
                        </Tooltip>
                        <Tooltip label="取消上传" hasArrow placement="auto">
                          <Flex align="center">
                            <CloseSVG
                              _hover={{ cursor: "pointer" }}
                              color="neutral.400"
                              onClick={() => {
                                uppy.uppy.removeFile(
                                  file.id,
                                  "removed-by-user",
                                );
                              }}
                            />
                          </Flex>
                        </Tooltip>
                      </Flex>
                    )}
                    {/* 上传完成-上传成功 */}
                    {!file.error && file?.response?.body?.code === 0 && (
                      <CheckCircleIcon color="G.600" boxSize={4} />
                    )}
                    {/* 上传中--可删除 */}
                    {!file.error && !file.progress.uploadComplete && (
                      <CloseSVG
                        _hover={{ cursor: "pointer" }}
                        color="gray.500"
                        onClick={() => {
                          uppy.uppy.removeFile(file.id, "removed-by-user");
                        }}
                      />
                    )}
                  </Flex>
                  {/*
                   *上传成功/上传中--green
                   *上传失败--red
                   */}
                  {/* <Progress
                    h="4px"
                    value={file.progress.percentage}
                    mx={{ base: "10px", md: 5 }}
                    bg="neutral.200"
                    borderRadius="16px"
                    colorScheme={isFileError ? "red" : "green"}
                  /> */}
                </Box>
              );
            }),
          )}
        </Collapse>
      </Box>
    )
  );
}

export function UploadProgressWithSingleUppy({ uppy, uploadFiles }) {
  const spin = keyframes`from { transform: rotate(0deg);} to {transform: rotate(180deg);}`;
  const animation = `${spin} infinite 1s linear`;
  const { isOpen: isFileUploadStatusOpen, onToggle: onFileUploadStatusToggle } =
    useDisclosure();
  const { erroredFiles: erroredFilesPDF, completeFiles: completeFilesPDF } =
    uppy.getObjectOfFilesPerState();
  // 上传成功
  const successfulFiles = completeFilesPDF.filter((file) => {
    return file?.response?.body?.code === 0;
  });
  const allFileLength = erroredFilesPDF.length + completeFilesPDF.length;

  const isAllErrored =
    allFileLength - successfulFiles.length === uploadFiles.length;

  const isUploadEnding = allFileLength === uploadFiles.length;

  const getUploadBytes = (total, uploaded) => {
    if (total <= 1024) {
      return `${uploaded}B/${total}B`;
    }
    if (total > 1024 && total < 1024 * 1024) {
      return `${(uploaded / 1024).toFixed(2)}K/${(total / 1024).toFixed(2)}K`;
    }
    if (total >= 1024 * 1024) {
      return `${(uploaded / 1024 / 1024).toFixed(2)}M/${(
        total /
        1024 /
        1024
      ).toFixed(2)}M`;
    }
  };

  const getUploadMessage = (file) => {
    if (file.progress.uploadComplete && file?.response?.body?.code !== 0) {
      return (
        file?.response?.body?.message ||
        "发生错误，请检查文件格式或联系客服人员"
      );
    }
    if (file.error) {
      return "上传失败，请检查网络后重试";
    }
    return getUploadBytes(
      file.progress.bytesTotal,
      file.progress.bytesUploaded,
    );
  };
  return (
    uploadFiles.length > 0 && (
      <Box
        pos="fixed"
        w="360px"
        maxH="500px"
        overflowY="auto"
        bottom={3}
        right={2}
        zIndex="tooltip"
        bg="neutral.800"
        borderRadius="8px"
        boxShadow="0px 4px 12px 0px rgba(0,0,0,0.10)"
      >
        <Flex
          align="center"
          py={3}
          px={6}
          gap={4}
          bgColor="neutral.800"
          borderBottomWidth="1px"
          pos="sticky"
          top={0}
          zIndex="sticky"
        >
          {!isUploadEnding && (
            <Flex align="center" fontSize="lg">
              <RepeatIcon
                color="primary.600"
                boxSize={6}
                animation={animation}
              />
              <Box ml={2} fontWeight="medium">
                上传中{successfulFiles.length}/{uploadFiles.length}
              </Box>
            </Flex>
          )}

          {isUploadEnding && !isAllErrored && (
            <Flex align="center" fontSize="lg">
              <CheckCircleIcon color="G.600" boxSize={6} />
              <Box ml={2} fontWeight="medium">
                上传成功{successfulFiles.length}/{uploadFiles.length}
              </Box>
            </Flex>
          )}
          {isAllErrored && (
            <Flex align="center" fontSize="lg">
              <WarningTwoIcon
                color="red"
                boxSize={6}
                _hover={{ cursor: "pointer" }}
                onClick={() => {
                  uppy.retryAll();
                }}
              />
              <Box ml={2} fontWeight="medium">
                上传失败
              </Box>
            </Flex>
          )}

          <Spacer />
          <ExpandSVG
            boxSize={6}
            _hover={{ cursor: "pointer" }}
            onClick={onFileUploadStatusToggle}
            transform={isFileUploadStatusOpen ? "" : "rotate(180deg)"}
          />
          <CloseSVG
            boxSize={5}
            _hover={{ cursor: "pointer" }}
            color="gray.500"
            onClick={() => {
              uppy.cancelAll("user");
            }}
          />
        </Flex>
        <Collapse in={!isFileUploadStatusOpen} animateOpacity>
          {uploadFiles?.map((file, index) => {
            const isFileError =
              file.error ||
              (file.progress.uploadComplete &&
                file?.response?.body?.code !== 0);
            return (
              <Box key={index} mb={1}>
                <Flex px={6} py={3} align="center" gap={2}>
                  <Icon as={getFileIcon(file.extension)} boxSize="48px" />
                  <Box flex={1}>
                    <Text noOfLines={1} wordBreak="break-all">
                      {file.name}
                    </Text>
                    <Text color={isFileError ? "red" : "neutral.400"}>
                      {getUploadMessage(file)}
                    </Text>
                  </Box>
                  {/* 上传失败--可重试，可删除 */}
                  {isFileError && (
                    <Flex gap={1}>
                      <Tooltip label="重新上传" hasArrow placement="auto">
                        <Flex align="center">
                          <RefreshSVG
                            color="neutral.400"
                            _hover={{ cursor: "pointer" }}
                            onClick={() => {
                              uppy.retryUpload(file.id);
                            }}
                          />
                        </Flex>
                      </Tooltip>
                      <Tooltip label="取消上传" hasArrow placement="auto">
                        <Flex align="center">
                          <CloseSVG
                            _hover={{ cursor: "pointer" }}
                            color="neutral.400"
                            onClick={() => {
                              uppy.removeFile(file.id, "removed-by-user");
                            }}
                          />
                        </Flex>
                      </Tooltip>
                    </Flex>
                  )}
                  {/* 上传完成-上传成功 */}
                  {!file.error && file?.response?.body?.code === 0 && (
                    <CheckCircleIcon color="G.600" boxSize={4} />
                  )}
                  {/* 上传中--可删除 */}
                  {!file.error && !file.progress.uploadComplete && (
                    <CloseSVG
                      _hover={{ cursor: "pointer" }}
                      color="gray.500"
                      onClick={() => {
                        uppy.removeFile(file.id, "removed-by-user");
                      }}
                    />
                  )}
                </Flex>
                {/*
                 *上传成功/上传中--green
                 *上传失败--red
                 */}
                {/* <Progress
                  h="4px"
                  value={file.progress.percentage}
                  mx={5}
                  bg="neutral.200"
                  borderRadius="16px"
                  colorScheme={isFileError ? "red" : "green"}
                /> */}
              </Box>
            );
          })}
        </Collapse>
      </Box>
    )
  );
}
