import type { Metadata } from "next";
import "./styles.css";

export const metadata: Metadata = {
  title: "Rivo",
  description: "Open-source video platform built around creator trust",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
