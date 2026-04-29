# tabkeeper
A lightweight web app for tracking drinks and snacks at events: log
consumption, view stats, and generate a final bill.

Originally built for the **Texttreff Wochenende (TTWW)**, a German women's writing
retreat. The person in charge of drinks and the tab was lovingly called the
*Schluckwartin* – hence the app.

Written in Go, no database required, no framework, just a single binary and a few
CSV files.

> Code comments and the UI are in German, as the app was built for a German-speaking
> audience. The structure is simple enough to adapt to other languages.

---

## Requirements

- Go 1.16 or later (tested with Go 1.24 on Debian Trixie)
- A web server or any machine that can run a Go binary
- A browser

---

## Installation

```bash
git clone https://github.com/huhnix/tabkeeper.git
cd tabkeeper
mkdir -p stats
```

---

## Configuration

### 1. User list

Edit `users.txt` – one entry per line, format: `username=Full Name`

```
anna.mueller=Anna Müller
bettina.schmidt=Bettina Schmidt
```

Usernames are used as URL slugs: `/bar/anna.mueller`

### 2. Price list

Edit the `prices` map in `main.go` to match your event's drinks and prices.

### 3. Background image

Replace `ksi-bg.jpg` with your own image, or remove the reference in `style.css`.

### 4. Billing password

In `main.go`, set your own credentials for the billing page:

```go
const abrechnungUser = "schluckwartin"
const abrechnungPass = "geheim123"
```

---

## Running the app

```bash
go run main.go
```

The app listens on port `8888` by default.

---

## Routes

| Route | Description |
|---|---|
| `/bar/<username>` | Entry form for a participant |
| `/bar-view/<username>` | Consumption overview for a participant |
| `/bar-undo/<username>` | Delete the last entry for a participant |
| `/abrechnung` | Full billing overview (password protected) |
| `/links` | — (static file, see `links.html`) |

---

## File structure

```
tabkeeper/
├── main.go          # Application logic
├── style.css        # Stylesheet
├── users.txt        # Participant list (dummy version in repo)
├── links.html       # Static page with links to all bar pages
├── favicon.png      # Favicon
├── ksi-bg.jpg       # Background image
├── stats/           # Per-user stats (generated, not in repo)
└── data-YYYY-MM-DD.csv  # Daily logs (generated, not in repo)
```

---

## License

Copyright (C) 2025 Heike Jurzik

This program is free software, licensed under the
[GNU General Public License v3.0](https://www.gnu.org/licenses/gpl-3.0.html).
