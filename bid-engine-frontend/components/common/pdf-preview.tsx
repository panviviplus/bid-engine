import dynamic from "next/dynamic";

const PdfPreview = dynamic(() => import("./pdf-preview-client"), {
  ssr: false,
});

export default PdfPreview;
