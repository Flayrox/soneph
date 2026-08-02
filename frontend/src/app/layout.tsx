import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "soneph | HQ Music Automation",
  description: "High Quality  Downloader & l'app Musique / iTunes Sync Dashboard",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="fr" className="dark">
      <body className="bg--dark text-white antialiased min-h-screen selection:bg--green selection:text-black">
        {children}
      </body>
    </html>
  );
}
