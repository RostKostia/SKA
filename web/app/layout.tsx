import type { Metadata } from "next";
import { Commissioner, Source_Serif_4 } from "next/font/google";
import "./globals.css";

const commissioner = Commissioner({
  variable: "--font-commissioner",
  subsets: ["latin", "greek", "cyrillic"],
});

const sourceSerif = Source_Serif_4({
  variable: "--font-source-serif",
  subsets: ["latin", "greek", "cyrillic"],
});

export const metadata: Metadata = {
  title: "Kerdos",
  description: "Invoices, reminders and a cash forecast for small businesses.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className={`${commissioner.variable} ${sourceSerif.variable}`}>
      <body>
        {/* Roughens the edges of rubber-stamp ink. */}
        <svg width="0" height="0" aria-hidden="true" focusable="false" style={{ position: "absolute" }}>
          <filter id="stamp-rough">
            <feTurbulence type="fractalNoise" baseFrequency="0.9" numOctaves="2" result="noise" />
            <feDisplacementMap in="SourceGraphic" in2="noise" scale="1.6" />
          </filter>
        </svg>
        {children}
      </body>
    </html>
  );
}
