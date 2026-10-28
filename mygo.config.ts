export default {
  name: "Gemini Free",
  identifier: "dev.gemini-free.desktop",
  version: "2.0.0",
  main: "./cmd/gemini-free",
  icon: "logo.png",
  out: "dist",
  macos: {
    minimumSystemVersion: "12.0",
    infoPlist: {
      LSUIElement: true,
      LSApplicationCategoryType: "public.app-category.developer-tools",
      CFBundleDevelopmentRegion: "zh_CN",
      CFBundleLocalizations: ["zh_CN", "en"],
    },
  },
};
