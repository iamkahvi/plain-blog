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

Open <http://127.0.0.1:8080/>. Changes to the content directory, `public/`, `site.yaml`, and `templates/layout.html` rebuild the site and reload open pages. Restart the server after Go code changes. A failed build leaves the last valid site available.

Run the tests:

```sh
go test ./...
node --test
```

## Font selection

Geist Serif is the default font. Add `?font=baskerville`, `?font=geist`, or `?font=times` to a page URL to select a font. Internal links retain the selected parameter. Font selection requires JavaScript.

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

Without `BLOG_CONTENT_DIR`, local commands use the repository's `content/` directory.

## Content structure

Each page uses `content/<name>/<name>.md`. Its YAML front matter needs a `title` and a root-relative `permalink` that ends in `/`. The generator uses the permalink as the route. Page routes must be unique. The navigation contains `home`, `writing`, `book-shelf`, and `projects`, in that order.

`content/home/home.md` must use `permalink: /`. Its `title` sets the home heading and site title. The `writing` and `book-shelf` pages contain the introductions for their respective lists.

Posts use `content/writing/posts/*.md`; newsletters use `content/writing/newsletter/*.md`. Each file needs a `title` and a `date` in `YYYY-MM-DD` format. Entries publish by default. Set `published: false` to omit an entry. Both collections use the writing page permalink plus the filename, so filenames must be unique across both directories.

Books use `content/book-shelf/books/*.md`. Each file needs `title`, `author`, and `date` in `YYYY-MM-DD` format. The book shelf lists books by date, newest first. Books with the same date sort by title. The page does not show dates before 2019.

Root Markdown files with `render: raw` in their front matter are copied to the site root without front matter. For example, `content/resume.md` becomes `/resume.md`.

If an entry has a `substack` URL, the lists link to it in a new tab. Add `?local` to use its local page instead. Internal links retain this parameter. This option requires JavaScript; without it, the lists link to Substack. Entries without a `substack` URL always link to their local pages.

Absolute HTTP and HTTPS links and protocol-relative web links have an icon and open in a new tab by default. This includes links to the site's own domain. An explicit link target takes precedence.

Obsidian comments use `%% ... %%`. The generator removes them from rendered Markdown and fails the build if a comment is not closed. Code examples keep literal comment markers. Files with `render: raw` are copied without changes and do not remove comments.

Place static files in `public/`. For example, `public/assets/photo.jpg` becomes `/assets/photo.jpg`. Markdown and its embedded HTML are trusted source content.

## Search metadata

Pages and posts can set `description` in their front matter. If it is absent, the generator uses the first non-empty paragraph. It omits the meta description when neither is available.

Set `domain` in `site.yaml` to the site hostname, without a scheme or path. The generator uses HTTPS with this domain for canonical links and `/sitemap.xml`. Published local pages and posts appear in the sitemap. Entries with a `substack` URL have no canonical link and do not appear in the sitemap. Their index policy is not yet set. Drafts, raw files, and assets do not appear in the sitemap. Submit the sitemap URL to Google Search Console after deployment.
