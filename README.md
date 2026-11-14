# Site

Build the static site with Go 1.21 or later:

```sh
go run . build
```

The build replaces `dist/`. Deploy its contents to a static host that serves directory index files.

Start the local server:

```sh
go run . dev
```

Open <http://127.0.0.1:8080/>. Changes to `content/`, `public/`, `layout.html`, and `style.css` rebuild the site and reload open pages. Restart the server after Go code changes. A failed build leaves the last valid site available.

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

Posts use `content/posts/*.md`; newsletters use `content/newsletter/*.md`. Each file needs YAML front matter with a title and a `YYYY-MM-DD` date. Set `published: false` to omit an entry. Both use `/writing/<filename>/`, so filenames must be unique across both directories.

If an entry has a `substack` URL, the lists link to it in a new tab. Its local page remains available. External web links open in a new tab by default. Links to `kahvipatel.com` and `www.kahvipatel.com` stay in the same tab. An explicit link target takes precedence. The site also includes `content/about/about.md`, `content/projects/projects.md`, and `content/pronounce/pronounce.md`.

Place static files in `public/`. For example, `public/assets/photo.jpg` becomes `/assets/photo.jpg`. Some existing content refers to local assets that are not present in this repository. Add those files to `public/` to restore those links. Markdown and its embedded HTML are trusted source content.
