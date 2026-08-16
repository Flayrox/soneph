import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        apple: {
          pink: "#fa2d48",
          pinkHover: "#ff3b56",
          darkBg: "#161618",
          sidebar: "#1e1e20",
          card: "#252528",
          hover: "#323236",
          border: "rgba(255, 255, 255, 0.08)",
          subtext: "#98989d",
        },
      },
      fontFamily: {
        sans: [
          "-apple-system",
          "BlinkMacSystemFont",
          "SF Pro Display",
          "SF Pro Text",
          "Inter",
          "sans-serif",
        ],
      },
      backdropBlur: {
        '3xl': '64px',
      },
    },
  },
  plugins: [],
};
export default config;
