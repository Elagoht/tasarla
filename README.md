# kanban

A [collage](https://collage.furkanbaytekin.dev/en/) project, scaffolded by
`collage new --template minimal`.

```
go mod tidy
collage dev       # http://localhost:6060, reloading as you edit
collage build     # -> bin/kanban
collage export    # -> dist/, static files
```

`main.go` is how `collage dev` and `collage export` run this program: if you
rewrite it, keep both working. The guide starts at
https://collage.furkanbaytekin.dev/en/docs/introduction/.

## Plugins

What the framework leaves out comes as plugins, each its own module. A few worth
adding early:

- [collage-htmlcheck](https://github.com/Elagoht/collage-htmlcheck) checks the
  HTML you render — a missing `alt`, a skipped heading, a link to nothing — over the
  page in development and in `collage export`'s report.
- [collage-sitemap](https://github.com/Elagoht/collage-sitemap),
  [collage-robots](https://github.com/Elagoht/collage-robots) and
  [collage-feed](https://github.com/Elagoht/collage-feed) for search engines and
  feed readers.
- [collage-secure](https://github.com/Elagoht/collage-secure) for security headers,
  [collage-flash](https://github.com/Elagoht/collage-flash) for a "saved" message
  after a form, [collage-i18n](https://github.com/Elagoht/collage-i18n) for
  translations.

`go get` one, add it to `Plugins` in `main.go`, and its settings go in
`plugins-config.json`. All of them:
https://collage.furkanbaytekin.dev/en/docs/plugins/.

## Geliştirme

```bash
docker compose up -d                 # PostgreSQL, localhost:55432
cp .env.example .env.development      # OIDC ve anahtarları doldur
collage dev                           # http://localhost:6060

export KANBAN_TEST_DATABASE_URL='postgres://kanban:kanban@localhost:55432/kanban?sslmode=disable'
go test ./... -race                   # veritabanı testleri bu değişken yoksa atlanır
```

Callback adresi IdP'de kayıtlı redirect URI'dir ve `OIDC_REDIRECT_URL` ile verilir; `BASE_URL` ile aynı origin'de olmalıdır. Çıkış sonrası dönüş adresi: `<BASE_URL>/login`. Kayıtlı adres `localhost` değilse gerçek giriş yerelde denenemez; testler sahte issuer kullanır.
