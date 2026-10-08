import Providers from "@/providers";
import { PublicEnvScript } from "next-runtime-env";
import "@react-pdf-viewer/core/lib/styles/index.css";
import "@react-pdf-viewer/default-layout/lib/styles/index.css";
import "@react-pdf-viewer/highlight/lib/styles/index.css";
import "@/styles/globals.css";

export const viewport = {
  width: "device-width",
  initialScale: 1,
  viewportFit: "cover",
};

function LocaleLayout({ children }) {
  return (
    <html lang="zh">
      <head>
        <PublicEnvScript />
        <title>标擎BidEngine - AI招投标工作台</title>
        <link rel="icon" href="/bid-engine-logo.ico" sizes="any" />
        <link rel="icon" type="image/png" sizes="32x32" href="/favicon-32.png" />
        <link rel="icon" type="image/png" sizes="16x16" href="/favicon-16.png" />
        <link rel="apple-touch-icon" sizes="180x180" href="/apple-touch-icon.png" />
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossOrigin="anonymous" />
        <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet" />
      </head>
      <body className="h-full flex">
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}

export default LocaleLayout;
