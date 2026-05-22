export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        ink: "#1b1e1a",
        paper: "#f7f3ea",
        parchment: "#f2e8d5",
        panel: "#fcf8f1",
        line: "#d9cfbf",
        pine: "#2e4a1d",
        leaf: "#4b7a2c",
        gold: "#c9a227",
        rust: "#c4503a",
        moss: "#8a8f85",
        mist: "#eee6d6",
        board: {
          light: "#f2e8d5",
          dark: "#b88b5a",
          highlight: "#e1df91",
          selected: "#7aae6a",
          lastmove: "#d6c97a",
        },
      },
      fontFamily: {
        display: ["Playfair Display", "serif"],
        body: ["Source Sans 3", "system-ui", "sans-serif"],
        mono: ["IBM Plex Mono", "ui-monospace", "monospace"],
        sans: ["Source Sans 3", "system-ui", "sans-serif"],
      },
    },
  },
  plugins: [],
};
