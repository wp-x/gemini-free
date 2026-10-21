export default {
  name: "Gemini Free",
  identifier: "dev.gemini-free.desktop",
  version: "0.1.0",
  main: "./cmd/gemini-free",
  icon: "logo.png",
  out: "dist",
  macos: {
    minimumSystemVersion: "12.0",
    infoPlist: {
      LSApplicationCategoryType: "public.app-category.developer-tools",
      CFBundleDevelopmentRegion: "zh_CN",
      CFBundleLocalizations: ["zh_CN", "en"],
    },
  },
};
