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
          bg: "#0f1117",
          surface: "#1a1d27",
          border: "#2a2d3e",
          accent: "#6c63ff",
          gold: "#f5c518",
        },
      },
      fontFamily: {
        mono: ["JetBrains Mono", "Fira Code", "monospace"],
      },
    },
  },
  plugins: [],
};
