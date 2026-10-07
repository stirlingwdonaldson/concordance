import "@fontsource-variable/schibsted-grotesk";
import "@fontsource-variable/newsreader";
import "./styles.css";
import type { Metadata } from "next";
import type { ReactNode } from "react";

export const metadata: Metadata = {
  title: "Concordance",
  description: "See how words are used across the books and reports you read.",
};

export default function RootLayout({
  children,
}: Readonly<{ children: ReactNode }>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
