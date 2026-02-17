module.exports = {
  content: ["./internal/view/**/*.templ", "./internal/view/**/*.go"],
  theme: {
    extend: {
      colors: {
        primary: 'var(--color-primary)',
        bg: 'var(--color-bg)',
      }
    }
  },
  plugins: [
  ],
}

