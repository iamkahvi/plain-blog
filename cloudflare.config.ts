import { defineConfig } from "cf/config";

export default defineConfig({
  worker: {
    name: "plain-blog",
    compatibilityDate: "2026-11-04",
    workersDev: true,
    previewUrls: false,
    observability: {
      logs: {
        enabled: true,
        invocationLogs: true,
        persist: true,
      },
    },
  },
});
