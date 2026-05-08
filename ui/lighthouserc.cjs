const path = require("node:path");

const chromeUserDataDir = path.resolve(__dirname, ".chrome-lighthouse").replace(/\\/g, "/");
const lhciPort = 4174;
const startServerCommand =
  process.platform === "win32"
    ? `set PORT=${lhciPort}&& pnpm run build && node tests/e2e/serve-admin-ui.mjs`
    : `PORT=${lhciPort} pnpm run build && PORT=${lhciPort} node tests/e2e/serve-admin-ui.mjs`;

module.exports = {
  ci: {
    collect: {
      startServerCommand,
      startServerReadyPattern: "Admin UI test server listening",
      startServerReadyTimeout: 60000,
      url: [`http://127.0.0.1:${lhciPort}/admin/ui/`],
      numberOfRuns: 1,
      settings: {
        preset: "desktop",
        chromeFlags: `--headless=new --no-sandbox --disable-gpu --disable-dev-shm-usage --user-data-dir=${chromeUserDataDir}`
      }
    },
    assert: {
      assertions: {
        "categories:performance": ["error", { minScore: 0.9 }],
        "categories:accessibility": ["error", { minScore: 0.95 }],
        "categories:best-practices": ["error", { minScore: 0.9 }]
      }
    },
    upload: {
      target: "filesystem",
      outputDir: "./.lighthouseci"
    }
  }
};
