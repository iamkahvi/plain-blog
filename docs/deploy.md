# Production setup

Cloudflare builds `plain-blog` from `main` and deploys the output with `wrangler.jsonc`. Each build uses [Obsidian Headless](https://obsidian.md/help/sync/headless) to download the `notes` vault in pull-only mode. The generator uses only `blog-content/` by default.

## Cloudflare setup

Use Node.js 22.12 or later locally and in Cloudflare Workers Builds. The `notes` vault uses end-to-end encryption. The build needs an Obsidian account token and the vault encryption password.

1. Run `npm ci` from the repository root.
2. Run `npx ob login` to sign in to Obsidian Headless.
3. On macOS, copy the token to the clipboard with this command:

   ```sh
   pbcopy < "$HOME/.obsidian-headless/auth_token"
   ```

   On Linux, the token file is `${XDG_CONFIG_HOME:-$HOME/.config}/obsidian-headless/auth_token`.

4. In Cloudflare, open **Workers & Pages → plain-blog → Settings → Build**.
5. Set these commands:

   | Field | Value |
   | --- | --- |
   | Build command | `./scripts/build-obsidian.sh` |
   | Deploy command | `npx wrangler deploy --config wrangler.jsonc` |

6. Under **Build variables and secrets**, add these values for production:

   | Name | Type | Value |
   | --- | --- | --- |
   | `OBSIDIAN_VAULT` | Variable | `notes` |
   | `OBSIDIAN_AUTH_TOKEN` | Secret | The contents of the Headless `auth_token` file |
   | `OBSIDIAN_VAULT_PASSWORD` | Secret | The vault encryption password, not the Obsidian account password |

7. After `scripts/build-obsidian.sh` is on `main`, start a production build.
8. Check the build log for `Obsidian Sync download complete.`
9. Confirm that Cloudflare deployed the site.

Leave `OBSIDIAN_CONTENT_DIR` unset to use `blog-content/`. To use a different folder, set the variable to its path relative to the vault root. If more than one vault has the name `notes`, run `npx ob sync-list-remote`. Use the vault ID instead of the name.

The token gives access to the Obsidian account, not only the blog folder. Do not add these secrets to Worker runtime variables, preview builds, or the repository.

The build fails if Obsidian Sync fails or if the blog folder does not exist. The build script deletes the downloaded vault when it exits. Obsidian Sync logs can include private file names and vault paths. Do not share build logs without a review.

If you switch from Artifacts, keep the existing Deploy Hook for `plain-blog` on `main`. You do not need to change the Artifacts Sync setup.

## Git hook

Use the [Git hook command in the README](../README.md#production-builds) once per clone. The hook rebuilds and stages `bin/blog-linux-amd64` after changes to Go source, dependencies, or generator build scripts. It uses the staged source and keeps local changes to the executable that you did not stage.

Stage the changed files. Run `git commit` without file paths or `--only`. The executable build uses Go 1.27.3 and downloads it if necessary. The production build rejects an executable that does not match the source hash, even if you bypass the hook.

## Start a build from Obsidian

Plain Blog Build works on macOS and iOS. It starts a Cloudflare build for `main`. It does not sync files. A build request does not confirm deployment.

### Set up the command on another device

Copy the files below to `.obsidian/plugins/plain-blog-build/` in the vault if your Mac does not have the plugin:

- `obsidian-blog-build/manifest.json`
- `obsidian-blog-build/main.js`

1. On both devices, enable **Installed community plugin list** under **Settings → Sync → Vault configuration sync**.
2. Wait until Obsidian Sync completes on both devices.
3. Restart Obsidian on the other device.
4. Enable **Plain Blog Build** under **Settings → Community plugins**.
5. Paste the Deploy Hook URL in the plugin settings on that device.

If you do not have a hook, create one for `plain-blog` on `main` under **Settings → Builds → Deploy Hooks** in Cloudflare. The URL is a credential. The plugin keeps it in the device's Obsidian keychain, not in the vault. Replace the hook if someone else gets the URL.

### Start the build

1. Wait until Obsidian Sync shows that the vault is up to date.
2. Run **Plain Blog Build: Start blog build** in the command palette.
3. Check the Worker build history for the result.
