import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "./globals.css";

const inter = Inter({
  variable: "--font-inter",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "AegisRL — Rate Limiter Dashboard",
  description:
    "Real-time monitoring dashboard for AegisRL distributed rate limiting engine. View request metrics, latency, circuit breaker status, and API key management.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className={`${inter.variable} dark`}>
      <body className="min-h-screen bg-[#0a0a0f] text-white antialiased font-[family-name:var(--font-inter)]">
        {children}
      </body>
    </html>
  );
}
