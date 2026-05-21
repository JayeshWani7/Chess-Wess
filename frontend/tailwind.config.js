export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        board: {
          light: "#f0d9b5",
          dark: "#b58863",
          highlight: "#f6f669",
          selected: "#7fc97f",
          lastmove: "#cdd16e",
        },
        chrono: {
          bg: "#0b0f14",
          surface: "#131a24",
          border: "#243042",
          accent: "#3dd6c9",
          gold: "#f5c84c",
        },
      },
      fontFamily: {
        sans: ["Space Grotesk", "system-ui", "sans-serif"],
        mono: ["JetBrains Mono", "Fira Code", "monospace"],
      },
    },
  },
  plugins: [],
};
