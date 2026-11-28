import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "./globals.css";

const inter = Inter({
  variable: "--font-inter",
  subsets: ["latin", "greek", "cyrillic"],
});

export const metadata: Metadata = {
  title: "Kerdos",
  description: "Invoices, reminders and a cash forecast for small businesses.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className={inter.variable}>
      <body>
        {children}
      </body>
    </html>
  );
}
