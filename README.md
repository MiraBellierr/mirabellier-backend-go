# mirabellier backend (◕‿◕) ✨

backend API for [mirabellier.com](https://mirabellier.com) — go + gin + sqlite ♡

## what's in here

- question of the day with carry-forward & discord webhook
- blog posts, comments, likes
- guestbook
- anime feed from myanimelist
- character shrines
- daily quotes scraper
- arena game system
- discord oauth login
- seo pages & sitemap

## quick start

copy the example env and fill in your stuff:

```
cp .env.example .env
```

then run it:

```
go run ./cmd/server
```

it starts on whatever port you set (default 5000) ♪

## building

```
go build ./cmd/server
```

for a smaller binary:

```
go build -ldflags="-s -w" ./cmd/server
```

## project layout

```
cmd/server/      entry point
internal/
  auth/          discord login & sessions
  qotd/          question of the day
  quotes/        daily quotes
  posts/         blog
  guestbook/     guestbook
  anime/         myanimelist feed
  shrines/       character shrines
  arena/         arena game
  seo/           seo pages & sitemap
  embed/         og image cards
  database/      migrations
```

IF YOU SEE THIS, YOU ARE CUTE !!! (｡•ᴗ•)♡
