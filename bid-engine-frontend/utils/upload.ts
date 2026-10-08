import { FaFileImage } from "react-icons/fa";
import { isString } from "lodash";
import PDFSVG from "@/components/svg/upload/filelogo/pdf";
import WordSVG from "@/components/svg/upload/filelogo/word";
import XLSXSVG from "@/components/svg/upload/filelogo/xlsx";
import PPTSVG from "@/components/svg/upload/filelogo/ppt";
import CsvSVG from "@/components/svg/upload/filelogo/csv";
import TxtSVG from "@/components/svg/upload/filelogo/txt";
import FileDefaultSVG from "@/components/svg/upload/filelogo/file-default";
import UrlSVG from "@/components/svg/upload/filelogo/url";
import MdSVG from "@/components/svg/upload/filelogo/md";
import VideoSVG from "@/components/svg/upload/filelogo/video";

export const getUploadBytes = (total, uploaded) => {
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
export const getUploadMessage = (file) => {
  if (file.error) {
    return "上传失败，请检查网络后重试";
  }
  return getUploadBytes(file.progress.bytesTotal, file.progress.bytesUploaded);
};

const MEETING_TYPE = [
  50,
  51,
  52,
  53,
  54,
  55,
  80,
  81,
  82,
  "mp4",
  "wmv",
  "mkv",
  "avi",
  "flv",
  "m4s",
  "wav",
  "mp3",
  "m4a",
];

export const getFileIcon = (extension) => {
  const formatExtension = isString(extension)
    ? extension.toLowerCase().replace(".", "")
    : extension;
  if (MEETING_TYPE.includes(formatExtension)) return VideoSVG;
  switch (formatExtension) {
    case "pdf":
    case ".pdf":
    case 1:
      return PDFSVG;
    case "doc":
    case "docx":
    case ".doc":
    case ".docx":
    case 2:
    case 3:
      return WordSVG;
    case "xls":
    case "xlsx":
    case ".xls":
    case ".xlsx":
    case 6:
    case 7:
      return XLSXSVG;
    case "ppt":
    case "pptx":
    case ".ppt":
    case ".pptx":
    case 4:
    case 5:
      return PPTSVG;
    case "csv":
    case ".csv":
    case 8:
      return CsvSVG;
    case "txt":
    case ".txt":
    case 9:
      return TxtSVG;
    case "url":
    case ".url":
    case "urlpdf":
    case 100:
    case 101:
      return UrlSVG;
    case ".png":
    case ".jpg":
    case ".jpeg":
    case "png":
    case "jpg":
    case "jpeg":
    case ".svg":
    case "svg":
    case 10:
    case 11:
      return FaFileImage;
    case 12:
    case "md":
    case ".md":
      return MdSVG;
    default:
      return FileDefaultSVG;
  }
};
