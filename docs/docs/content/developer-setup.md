# Developer setup
The app is a Go backend that server-renders the admin UI (HTML templates with JS, built with [esgun](https://github.com/oddship/esgun)).


### Pre-requisites
- `go`
- Python 3 (for generating the frontend icon sprite)
- Postgres database. If there is no local installation, the demo docker DB can be used for development (`docker compose up demo-db`)


### First time setup
`git clone https://github.com/knadh/listmonk.git`. The project uses go.mod, so it's best to clone it outside the Go src path.

1. Copy `config.toml.sample` as `config.toml` and add your config.
2. Build the frontend and the binary: `make dist` (builds the SSR admin and the Go binary, embedding the assets). Once the binary is built, run `./listmonk --install` to run the DB setup. For subsequent dev runs, use `make run`.

> [mailhog](https://github.com/mailhog/MailHog) is an excellent standalone mock SMTP server (with a UI) for testing and dev.


### Running the Docker dev environment

- Run `make build-dev-docker` once to build the dev container which provides Go and Python, with Postgres and MailHog as companion services.
- Run `make run-dev-docker` to start it and enter its shell.
- Inside the shell, run `make run` which builds static assets, initializes a fresh DB, and starts listmonk on `:9000`.
- For subsequent changes, simply Ctrl-C and re-run `make run` for changes to Go and static files to be picked up.


Run `make stop-dev-docker` to stop the container and `make rm-dev-docker` to completely remove it.

### Working on the frontend

`make build-frontend` installs the locked npm dependencies with the esgun version pinned in `go.mod`, bundles the JavaScript, and compresses the admin assets. It also copies ALTCHA to the public frontend. Dependencies are stored in `static/admin/vendor/node_modules`; intermediate output is in `static/admin/.build`. Neither directory is committed.

Run `make watch-frontend` alongside `make run` to rebuild and compress assets when frontend sources change. The Go driver preserves the mounted `dist` directory in Docker.

To manage dependencies, run these commands from the repository root:

```sh
go tool esgun add <package> --dir static/admin
go tool esgun remove <package> --dir static/admin
go tool esgun update --dir static/admin
go tool esgun outdated --dir static/admin
go tool esgun audit --dir static/admin
```

Commit both `package.json` and `package-lock.json` after changing dependencies. Builds use `esgun ci`, which rejects lockfile drift. Use `make watch-frontend` for development so compression and the public ALTCHA copy run after each bundle.

### Frontend tests

listmonk maintains a Playwright suite in `tests/` for end-to-end testing the admin UI. It runs against the Docker dev environment with Postgres and MailHog. Playwright requires Node.js and npm, installed separately in the dev container; these are not required to build listmonk.

#### Running

Follow the steps described above to run and enter the Docker dev environment.

Inside the dev container shell at `/app`:

```sh
make dist

cd tests
npm install
npx playwright install --with-deps chromium
npm test
```

# Production build
Run `make dist` to build the SSR admin frontend and the Go binary, embedding the static assets into a single self-contained binary, `listmonk`.
