import type { Metadata, Viewport } from "next";
import type { ReactNode } from "react";
import "./globals.css";

export const metadata: Metadata = {
  title: "BridgeYok — Main bridge bareng",
  description: "Ruang bridge online gratis untuk bermain bersama teman, dibangun sederhana dan tahan terputus.",
  openGraph: {
    title: "BridgeYok — Main bridge bareng",
    description: "Ruang bridge online gratis untuk bermain bersama teman.",
    locale: "id_ID",
    type: "website"
  },
  robots: {
    index: false,
    follow: false
  }
};

export const viewport: Viewport = {
  colorScheme: "light",
  themeColor: "#f4f0e7"
};

export default function RootLayout({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <html lang="id">
      <body>{children}</body>
    </html>
  );
}
