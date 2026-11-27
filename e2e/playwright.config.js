const { defineConfig } = require("@playwright/test");

const address = "127.0.0.1:18082";
const baseURL = `http://${address}`;

module.exports = defineConfig({
  testDir: ".",
  workers: 1,
  use: {
    baseURL,
    browserName: "chromium",
    locale: "en-US",
    launchOptions: { executablePath: process.env.E2E_CHROMIUM_PATH },
  },
  webServer: {
    command: "../bin/go-todo --env development serve",
    env: { ADDR: address },
    url: `${baseURL}/up`,
    reuseExistingServer: false,
  },
});
