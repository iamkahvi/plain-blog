# Site

## Local preview

Use Go 1.21 or later. Use Node.js 22 or later for the JavaScript tests.

Start the local server:

```sh
go run . dev
```

Open <http://127.0.0.1:8080/>. The server reloads open pages after changes to content, `public/`, `site.yaml`, or `templates/layout.html`. Restart the server after changes to Go code. If a build fails, the server keeps the last valid site.

Run the tests:

```sh
go test ./...
node --test
```

For a local build, run `go run . build`. This command replaces `dist/`.

## Content

Local commands use `content/` by default. For example, the home page uses `content/home/home.md`.

Set `BLOG_CONTENT_DIR` in the shell to use a different directory. The local server also reads `.env.local`, but the `build` command does not. The server uses the shell value if you set the variable in both places.

See [Content reference](docs/content.md) for file formats and site features.

## Production builds

Set up the Git hook once per clone:

```sh
git config --local core.hooksPath .githooks
```

Push to `main` to start a Cloudflare build. To start a build from Obsidian, wait until Obsidian Sync shows that the vault is up to date. Then run **Plain Blog Build: Start blog build**.

The build uses the latest Git revision on `main` and reads `blog-content/` from the synced vault by default. Check the Worker build history for the result. Obsidian Sync alone does not start a build.

See [Production setup](docs/deploy.md) for Cloudflare, Obsidian, and Git hook details.
