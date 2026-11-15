const { Notice, Plugin, PluginSettingTab, Setting, requestUrl } = require("obsidian");

const SECRET_ID = "plain-blog-deploy-hook";

function validHookURL(value) {
  try {
    const url = new URL(value);
    return url.protocol === "https:" &&
      url.host === "api.cloudflare.com" &&
      !url.username && !url.password && !url.search && !url.hash &&
      /^\/client\/v4\/workers\/builds\/deploy_hooks\/[a-zA-Z0-9-]+$/.test(url.pathname);
  } catch (_) {
    return false;
  }
}

class BlogBuildSettings extends PluginSettingTab {
  display() {
    this.containerEl.empty();
    let hookURL = "";
    new Setting(this.containerEl)
      .setName("Deploy Hook URL")
      .setDesc("Store the hook in the Obsidian keychain on this device.")
      .addText((text) => {
        text.inputEl.type = "password";
        text.setPlaceholder("Paste the Deploy Hook URL").onChange((value) => {
          hookURL = value.trim();
        });
      })
      .addButton((button) => button.setButtonText("Save").onClick(() => {
        if (!validHookURL(hookURL)) {
          new Notice("Plain Blog Build: Enter a valid Cloudflare Deploy Hook URL.");
          return;
        }
        this.app.secretStorage.setSecret(SECRET_ID, hookURL);
        hookURL = "";
        new Notice("Plain Blog Build: Deploy Hook saved.");
        this.display();
      }));
  }
}

module.exports = class BlogBuildPlugin extends Plugin {
  onload() {
    this.addSettingTab(new BlogBuildSettings(this.app, this));
    this.addCommand({
      id: "build",
      name: "Start blog build",
      callback: () => void this.startBuild(),
    });
  }

  async startBuild() {
    if (this.buildRequestPending) return;
    const hookURL = this.app.secretStorage.getSecret(SECRET_ID);
    if (!hookURL || !validHookURL(hookURL)) {
      new Notice("Plain Blog Build: Set the Deploy Hook URL in plugin settings.");
      return;
    }
    this.buildRequestPending = true;
    try {
      const response = await requestUrl({ url: hookURL, method: "POST", throw: false });
      const result = response.json;
      if (response.status < 200 || response.status >= 300 || !result?.success || !result.result?.build_uuid) {
        throw new Error("Build request failed");
      }
      new Notice("Plain Blog Build: Build request accepted. Check Cloudflare for the result.");
    } catch (_) {
      new Notice("Plain Blog Build: Build request failed. Check the hook and try again.");
    } finally {
      this.buildRequestPending = false;
    }
  }
};
