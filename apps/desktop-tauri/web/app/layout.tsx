import "./styles.css";
import type { Metadata } from "next";
import { Fraunces, Source_Sans_3 } from "next/font/google";
import type { ReactNode } from "react";

const displayFont = Fraunces({
  subsets: ["latin"],
  variable: "--font-display",
});
const bodyFont = Source_Sans_3({ subsets: ["latin"], variable: "--font-body" });

export const metadata: Metadata = {
  title: "Concordance",
  description: "Local literary concordance and textual analysis",
};

export default function RootLayout({
  children,
}: Readonly<{ children: ReactNode }>) {
  return (
    <html lang="en">
      <body className={`${displayFont.variable} ${bodyFont.variable}`}>
        {children}
      </body>
    </html>
  );
}
