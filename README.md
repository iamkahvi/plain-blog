# Site

Build the static site with Go 1.21 or later:

```sh
go run . build
```

The build replaces `dist/`. Deploy its contents to a static host that serves directory index files.

Cloudflare Workers can build and deploy the site after each push to `main`. See [Obsidian content](#obsidian-content) to configure the content source. To deploy manually, install Node.js 24 and run:

```sh
npm ci
go run . build
npx cf deploy
```

Start the local server:

```sh
go run . dev
```

Open <http://127.0.0.1:8080/>. Changes to the content directory, `public/`, `layout.html`, and `style.css` rebuild the site and reload open pages. Restart the server after Go code changes. A failed build leaves the last valid site available.

## Obsidian content

The vault folder is the content source. The Cloudflare Artifacts Sync plugin copies it to an Artifacts repository. Workers Builds fetches that repository when a push to GitHub `main` starts a build. You can also start a build from Obsidian without a GitHub push. A content sync alone does not start a build.

1. Configure the plugin to sync the content folder to an Artifacts repository. Use the `main` branch. Artifacts requires a Workers Paid plan.
2. Use `cf builds workers update` to set the build command to `go run . build-artifacts`. Keep the existing deploy command.
3. Use the same command to set build variables `ARTIFACTS_ACCOUNT_ID` and `ARTIFACTS_REPO`. Set `ARTIFACTS_NAMESPACE` and `ARTIFACTS_BRANCH` if their defaults do not apply.
4. Set the build secret `ARTIFACTS_REPO_TOKEN` to a read-only repository token. Renew it before it expires.
5. Sync the plugin, then push to GitHub `main`. Check the Worker build log for the Artifacts content revision.

Build variables and secrets are separate from Worker runtime variables. The build fails if it cannot fetch Artifacts. It does not fall back to the copy in the GitHub checkout. Keep the repository token in production build secrets, not in the repository or preview builds. Replace the secret when you renew the token.

### Start a build from Obsidian

1. Run the Artifacts Sync **Sync now** command. Wait for it to finish.
2. Run **Plain Blog Build: Start blog build** in the command palette.
3. Check the Worker build history for the result.

A build request does not confirm deployment. The build uses the latest synced content and the latest GitHub `main` revision. Unsynced vault edits and uncommitted code changes are not included.

To set up the command, copy `obsidian-blog-build/manifest.json` and `obsidian-blog-build/main.js` to `.obsidian/plugins/plain-blog-build/` in the vault. Enable the plugin in Obsidian. Create a Workers Builds Deploy Hook for `plain-blog` on `main`, then add its URL in the plugin settings. The URL is a credential. The plugin stores it in the Obsidian keychain, not in plugin data. Configure the keychain on each device that does not have the secret. Do not commit or share the URL. Delete and replace the hook if its URL is exposed.

The plugin stores Git metadata in the selected folder. If Syncthing also syncs that folder, exclude the plugin's `.git` directory from Syncthing.

For a local build or server, set `BLOG_CONTENT_DIR` to the vault folder:

```sh
BLOG_CONTENT_DIR=/path/to/content go run . build
BLOG_CONTENT_DIR=/path/to/content go run . dev
```

For the local server, you can also set `BLOG_CONTENT_DIR` in `.env.local`:

```sh
BLOG_CONTENT_DIR=/path/to/content
```

The file is not tracked by Git. Shell environment variables take precedence. The `build` command does not read `.env.local`.

Without `BLOG_CONTENT_DIR`, local commands use the repository's `content/` directory. `scripts/sync-blog-content.sh` copies repository content to `BLOG_CONTENT_DIR`. Do not run it after the vault becomes the source of truth.

## Content structure

- `content/`
  - `about/`
    - `about.md` — required
      - `title`
  - `projects/`
    - `projects.md` — required
      - `title`
  - `pronounce/`
    - `pronounce.md` — required
      - `title`
  - `book-shelf/`
    - `book-shelf.md` — required; contains the page title and introduction
    - `books/` — optional
      - `<filename>.md` — each file needs `title`, `author`, and `date` (`YYYY-MM-DD`)
  - `posts/` — optional
    - `<filename>.md`
      - `title`
      - `date` — `YYYY-MM-DD`
  - `newsletter/` — optional
    - `<filename>.md`
      - `title`
      - `date` — `YYYY-MM-DD`

Books use `content/book-shelf/books/*.md`. The book shelf lists them by date, newest first. Each book body contains its notes. Books with the same date sort by title. As on the original site, the page does not show dates before 2019.

Posts use `content/posts/*.md`; newsletters use `content/newsletter/*.md`. Each file needs YAML front matter with a title and a `YYYY-MM-DD` date. Entries publish by default. Set `published: false` to omit a post or newsletter. The flag does not control books or fixed pages. Both use `/writing/<filename>/`, so filenames must be unique across both directories.

If an entry has a `substack` URL, the lists link to it in a new tab. Its local page remains available. External web links open in a new tab by default. Links to `kahvipatel.com` and `www.kahvipatel.com` stay in the same tab. An explicit link target takes precedence. The site also includes `content/about/about.md`, `content/projects/projects.md`, and `content/pronounce/pronounce.md`.

Place static files in `public/`. For example, `public/assets/photo.jpg` becomes `/assets/photo.jpg`. Some existing content refers to local assets that are not present in this repository. Add those files to `public/` to restore those links. Markdown and its embedded HTML are trusted source content.
