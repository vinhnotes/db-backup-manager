# Product page on GitHub Pages

The static English product page lives in `docs/`: HTML, CSS, JavaScript, a favicon and screenshots in `docs/images/`. No dependencies or React frontend build are needed. The screenshots show the real app running with demo data (a throwaway Postgres, no real data).

## First publish

1. Commit and push the `docs/` files, `.github/workflows/pages.yml` and the docs to the repository's `main` branch.
2. In the GitHub repository, open **Settings → Pages → Build and deployment → Source** and choose **GitHub Actions**.
3. Open **Actions → Deploy product page → Run workflow**, select the `main` branch and run it. Later pushes that change `docs/` or the workflow deploy automatically.
4. Once the workflow succeeds, visit **https://vinhnotes.github.io/db-backup-manager/**. The actual URL also appears in the `github-pages` environment of the run.

The workflow only uploads `docs/`. Never put backend code, backup data, environment configuration or the admin app on Pages. This page only introduces the product; the backup manager itself must be self-hosted.

If you fork the repository, replace the `vinhnotes/db-backup-manager` URLs in `docs/index.html` and the docs with your own repository. Assets use relative paths, so they work with Pages URLs that are prefixed with the repository name.

## Local preview

From the project root:

```sh
python3 -m http.server 4173 --bind 127.0.0.1 --directory docs
```

Open **http://127.0.0.1:4173**. Edit content in `docs/index.html`, styling in `docs/styles.css` and the copy-command behavior in `docs/script.js`.

Reference: [GitHub — Using custom workflows with GitHub Pages](https://docs.github.com/en/pages/getting-started-with-github-pages/using-custom-workflows-with-github-pages).
