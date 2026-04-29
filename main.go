// TTWW: Getränke- & Snackverwaltung
//
// Dieses Programm dient der digitalen Erfassung und Abrechnung von Getränken
// und Snacks während der TTWW-Veranstaltung. Nutzerinnen können ihren Konsum
// bequem über eine Weboberfläche eintragen. Für jeden Tag wird eine CSV-Datei
// mit allen Einträgen erzeugt. Zusätzlich wird pro Nutzerin eine
// Statistikdatei angelegt, die eine gruppierte Übersicht über den
// bisherigen Verbrauch liefert.
//
// Die wichtigsten Funktionen:
// - Einfache Erfassung von Getränken und Snacks pro Person
// - Tägliche CSV-Dateien zur Nachvollziehbarkeit
// - Individuelle Statistikseiten für jede Nutzerin (/bar-view/<benutzer>)
// - Übersicht nach Eintrag auf der Danke-Seite
// - Gesamtabrechnung über die Seite /abrechnung
//
// Änderungen 2026 (2026-04-20):
// - io/ioutil (deprecated seit Go 1.16) ersetzt durch os.ReadDir,
//   os.ReadFile und os.WriteFile
// - Preisliste (prices) an neue Getränkepreise 2026 anpassen
// - Teilnehmerinnenliste in users.txt aktualisieren
// - links.html entsprechend der neuen users.txt neu erstellen
// - Standardmenge auf 1 vorbelegt (value="1" im Formular)
// - Doppelklick-Schutz auf dem Eintragen-Button per JavaScript
// - HTTP Basic Auth für die /abrechnung-Seite
//
// Copyright (C) 2025 Heike Jurzik
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

var allowedUsers = make(map[string]string)
var userOrder []string // 👈 Neue Slice für Reihenfolge

// Preisliste (Kurzformen, passend zum Dropdown value)
var prices = map[string]float64{
    "GerolsteinerClassic":       3.00,
    "GerolsteinerNaturell":      3.00,
    "CocaCola":                  3.00,
    "ColaLight":                 3.00,
    "LimoSchorle":               3.00,
    "Bitburger":                 2.80,
    "Kölsch":                    2.80,
    "Fassbrause":                2.80,
    "Radler":                    2.80,
    "Grauburgunder":             21.00,
    "RoterRiesling":             18.50,
    "Rosé":                      19.50,
    "Rotwein":                   19.50,
    "Sekt":                      21.00,
    "Secco":                     18.00,
    "Chips":                     2.90,
    "Salzstangen":               2.90,
    "Erdnüsse":                  2.90,
    "Schokoriegel":              2.90,
}

// Lädt die Benutzerliste aus users.txt (Format: nutzername=voller Name)
func loadUsers(filename string) error {
    file, err := os.Open(filename)
    if err != nil {
        return err
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := scanner.Text()
        if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
            continue // Leere Zeilen oder Kommentare überspringen
        }
        parts := strings.SplitN(line, "=", 2)
        if len(parts) != 2 {
            continue // ungültige Zeile
        }
        key := strings.TrimSpace(parts[0])
        value := strings.TrimSpace(parts[1])
        allowedUsers[key] = value
        userOrder = append(userOrder, key) // 👈 Reihenfolge merken
    }

    return scanner.Err()
}

// Baut stats/<user>.csv neu aus allen data-*.csv Dateien
func rebuildStats(username string) {
	stats := make(map[string]int)
	files, _ := os.ReadDir(".")
	for _, file := range files {
		name := file.Name()
		if strings.HasPrefix(name, "data-") && strings.HasSuffix(name, ".csv") {
			f, err := os.Open(name)
			if err != nil {
				continue
			}
			reader := csv.NewReader(f)
			records, _ := reader.ReadAll()
			for _, row := range records {
				if len(row) >= 5 && row[1] == username {
					drink := row[3]
					var amt int
					fmt.Sscanf(row[4], "%d", &amt)
					stats[drink] += amt
				}
			}
			f.Close()
		}
	}

	statsDir := "stats"
	os.MkdirAll(statsDir, 0755)
	statFile, _ := os.Create(fmt.Sprintf("%s/%s.csv", statsDir, username))
	writer := csv.NewWriter(statFile)
	writer.Write([]string{"Getränk", "Menge"})
	for k, v := range stats {
		writer.Write([]string{k, fmt.Sprintf("%d", v)})
	}
	writer.Flush()
	statFile.Close()
}
// Löscht den letzten Eintrag für eine Benutzerin aus allen data-*.csv-Dateien
func removeLastEntry(username string) bool {
	files, _ := os.ReadDir(".")
	// Von der neuesten zur ältesten Datei rückwärts durchsuchen
	for i := len(files) - 1; i >= 0; i-- {
		name := files[i].Name()
		if !strings.HasPrefix(name, "data-") || !strings.HasSuffix(name, ".csv") {
			continue
		}

		path := name
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		lines := strings.Split(strings.TrimSpace(string(content)), "\n")
		found := false

		for j := len(lines) - 1; j >= 0; j-- {
			cols := strings.Split(lines[j], ",")
			if len(cols) >= 2 && cols[1] == username {
				lines = append(lines[:j], lines[j+1:]...) // Zeile entfernen
				found = true
				break
			}
		}

		if found {
			output := strings.Join(lines, "\n")
			if output != "" {
				output += "\n"
			}
			os.WriteFile(path, []byte(output), 0644)
			rebuildStats(username)
			return true
		}
	}
	return false
}

// Format: Komma statt Punkt als Dezimaltrenner für deutsche Anzeige
func formatEuro(value float64) string {
	return strings.Replace(fmt.Sprintf("%.2f €", value), ".", ",", 1)
}

const abrechnungUser = "schluckwartin"
const abrechnungPass = "geheim123"

func basicAuth(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        user, pass, ok := r.BasicAuth()
        if !ok || user != abrechnungUser || pass != abrechnungPass {
            w.Header().Set("WWW-Authenticate", `Basic realm="KSI-Abrechnung"`)
            http.Error(w, "Nicht autorisiert", http.StatusUnauthorized)
            return
        }
        next(w, r)
    }
}

func main() {
	err := loadUsers("users.txt")
	if err != nil {
		fmt.Println("Fehler beim Laden der Benutzerinnen-Liste:", err)
		os.Exit(1)
	}

	// Statische Dateien
	http.Handle("/style.css", http.FileServer(http.Dir(".")))
	http.Handle("/favicon.png", http.FileServer(http.Dir(".")))
	http.Handle("/ksi-bg.jpg", http.FileServer(http.Dir(".")))

	// Danke-Ansicht mit Löschen
	http.HandleFunc("/bar-undo/", func(w http.ResponseWriter, r *http.Request) {
		username := strings.ToLower(strings.TrimPrefix(r.URL.Path, "/bar-undo/"))
		realname, ok := allowedUsers[username]
		if !ok {
			http.Error(w, "Unbekannte Benutzerin", http.StatusForbidden)
			return
		}

		if removeLastEntry(username) {
			renderThankYouPage(w, username, realname, "/bar/"+username, "Dein letzter Eintrag wurde gelöscht")
		} else {
			http.Error(w, "Kein Eintrag gefunden zum Löschen.", http.StatusNotFound)
		}
	})

        http.HandleFunc("/bar-view/", func(w http.ResponseWriter, r *http.Request) {
    username := strings.ToLower(strings.TrimPrefix(r.URL.Path, "/bar-view/"))
    realname, ok := allowedUsers[username]
    if !ok {
        http.Error(w, "Unbekannte Benutzerin", http.StatusForbidden)
        return
    }

    statsPath := fmt.Sprintf("stats/%s.csv", username)
    stats := make(map[string]int)

    if file, err := os.Open(statsPath); err == nil {
        reader := csv.NewReader(file)
        records, _ := reader.ReadAll()
        for _, row := range records[1:] {
            if len(row) == 2 {
                drink := row[0]
                var amt int
                fmt.Sscanf(row[1], "%d", &amt)
                stats[drink] = amt
            }
        }
        file.Close()
    }

    var tableRows string
    var total float64
    for drink, count := range stats {
        price := prices[drink]
        itemTotal := float64(count) * price
        total += itemTotal
        tableRows += fmt.Sprintf(
            `<tr><td>%s</td><td>%d</td><td>%s</td><td>%s</td></tr>`,
            drink,
            count,
            formatEuro(price),
            formatEuro(itemTotal),
        )
    }

    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Verbrauch von %s</title>
  <link rel="stylesheet" href="/style.css">
  <link rel="icon" href="/favicon.png" type="image/png">
</head>
<body>
<div class="success">
  <p><a class="button-orange" href="/bar/%s">Zurück zur Startseite</a></p>
  <h2>Verbrauch von %s</h2>
  <div class="scrollbar">
  <table>
    <tr><th>Getränk/Snack</th><th>Menge</th><th>Preis</th><th>Gesamt</th></tr>
    %s
    <tfoot>
      <tr><td colspan="3">Summe</td><td>%s</td></tr>
    </tfoot>
  </table>
  </div>
</div>
</body>
</html>
`, realname, username, realname, tableRows, formatEuro(total))
})


	// Formularseite
	http.HandleFunc("/bar/", func(w http.ResponseWriter, r *http.Request) {
		username := strings.ToLower(strings.TrimPrefix(r.URL.Path, "/bar/"))
		realname, ok := allowedUsers[username]
		if !ok {
			http.Error(w, "Unbekannte Benutzerin", http.StatusForbidden)
			return
		}

		if r.Method == http.MethodPost {
			drink := r.FormValue("drink")
			amount := r.FormValue("amount")

			// Eingabe prüfen
			if strings.TrimSpace(amount) == "" {
				http.Error(w, "Bitte gib eine Menge ein.", http.StatusBadRequest)
				return
			}

			var added int
			n, err := fmt.Sscanf(amount, "%d", &added)
			if err != nil || n != 1 || added < 1 {
				http.Error(w, "Ungültige Menge: Bitte gib eine ganze Zahl größer 0 ein.", http.StatusBadRequest)
				return
			}

			date := time.Now().Format("2006-01-02")
			filename := "data-" + date + ".csv"

			f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				http.Error(w, "Fehler beim Schreiben", 500)
				return
			}
			defer f.Close()

			writer := csv.NewWriter(f)
			writer.Write([]string{date, username, realname, drink, amount})
			writer.Flush()

			rebuildStats(username)
			renderThankYouPage(w, username, realname, r.URL.Path, "Dein Eintrag wurde gespeichert")
			return
		}
                // HTML-Formular anzeigen
w.Header().Set("Content-Type", "text/html; charset=utf-8")
fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Getränke und Snacks beim TTWW</title>
  <link rel="stylesheet" href="/style.css">
  <link rel="icon" href="/favicon.png" type="image/png">
  <link href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.5.0/css/all.min.css" rel="stylesheet">
</head>
<body>
<div class="container">
  <h1>Hallo, %s!</h1>
  <form method="POST">
    <label for="drink"><span>🍾 🍺 🍷 🍫 🥜</span><br/><br/>Getränk/Snack auswählen:</label>
    <div class="input-icon">
      <select name="drink" id="drink">
        <option value="GerolsteinerClassic">Gerolsteiner Classic (0,75&nbsp;l), 3,00&nbsp;€</option>
        <option value="GerolsteinerNaturell">Gerolsteiner Naturell (0,75&nbsp;l), 3,00&nbsp;€</option>
        <option value="CocaCola">Coca Cola (0,33&nbsp;l), 3,00&nbsp;€</option>
        <option value="ColaLight">Coca Cola Light (0,33&nbsp;l), 3,00&nbsp;€</option>
        <option value="LimoSchorle">Proviant Limo/Schorle (0,33&nbsp;l), 3,00&nbsp;€</option>
        <option value="Bitburger">Bitburger Pils/Drive (0,33&nbsp;l), 2,80&nbsp;€</option>
        <option value="Kölsch">Gaffel Kölsch (0,33&nbsp;l), 2,80&nbsp;€</option>
        <option value="Fassbrause">Fassbrause (0,33&nbsp;l), 2,80&nbsp;€</option>
        <option value="Radler">Radler (0,33&nbsp;l), 2,80&nbsp;€</option>
        <option value="Grauburgunder">Grauburgunder, Weingut Pieper (0,75&nbsp;l), 21,00&nbsp;€</option>
        <option value="RoterRiesling">Roter Riesling, Weingut Ernst (0,75&nbsp;l), 18,50&nbsp;€</option>
        <option value="Rosé">Rosé, Cuvée Preaquile (0,75&nbsp;l), 19,50&nbsp;€</option>
        <option value="Rotwein">Rotwein, L'Odalet Cabernet (0,75&nbsp;l), 19,50&nbsp;€</option>
        <option value="Sekt">Sekt, Schloss Affaltrach (0,75&nbsp;l), 21,00&nbsp;€</option>
        <option value="Secco">Secco vom Hiss (0,75&nbsp;l), 18,00&nbsp;€</option>
        <option value="Chips">Chips, 2,90&nbsp;€</option>
        <option value="Salzstangen">Salzstangen, 2,90&nbsp;€</option>
        <option value="Erdnüsse">Erdnüsse, 2,90&nbsp;€</option>
        <option value="Schokoriegel">Bio Schokoriegel, 2,90&nbsp;€</option>
      </select>
    </div>

    <label for="amount"><span>🔢</span> Menge auswählen:</label>
    <div class="input-icon">
      <input type="number" name="amount" id="amount" min="1" step="1" value="1">
    </div>

    <input type="submit" value="Eintragen" onclick="this.disabled=true; this.value='Wird gespeichert …'; this.form.submit();">

    <div class="help-container">
      <span class="help-icon" onclick="openHelpModal()">?</span>
      <span class="contact-icon" onclick="openContactModal()">@</span>
      <a href="/bar-view/%s" class="view-icon" title="Verbrauch anzeigen">∑</a>
   </div>
 
  </form>
</div>

<!-- Modals -->
<div id="helpModal" class="modal">
  <div class="modal-content">
    <span class="close" onclick="closeHelpModal()">&times;</span>
    <h2>Hilfe</h2>
    <ol>
      <li>Wähle ein Getränk oder einen Snack aus der Liste aus.</li>
      <li>Gib die Menge ein (ganze Zahl).</li>
      <li>Klicke auf <strong>Eintragen</strong>, um zu speichern.</li>
      <li>Mit dem Button <strong>Letzten Eintrag löschen</strong> kannst Du Deinen letzten Eintrag löschen.</li>
    </ol>
  </div>
</div>

<div id="contactModal" class="modal">
  <div class="modal-content">
    <span class="close" onclick="closeContactModal()">&times;</span>
    <h2>Kontakt zur Schluckwartin</h2>
    <p>
      📧 <a href="mailto:post@heikejurzik.de">post@heikejurzik.de</a><br>
      📞 <a href="tel:+491772780503">+49 177 278 0503</a>
    </p>
  </div>
</div>

<script>
function openHelpModal() {
  document.getElementById("helpModal").style.display = "block";
}
function closeHelpModal() {
  document.getElementById("helpModal").style.display = "none";
}
function openContactModal() {
  document.getElementById("contactModal").style.display = "block";
}
function closeContactModal() {
  document.getElementById("contactModal").style.display = "none";
}
</script>

<footer class="footer">
  <p>
    © 2025 Heike Jurzik · Dieses Projekt steht unter der
    <a href="https://www.gnu.org/licenses/gpl-3.0.de.html" target="_blank" rel="license noopener">GPLv3</a>.
  </p>
</footer>
</body>
</html>
`, realname, username)
	})

	// Abrechnungsseite für die Schluckwartin (passwortgeschützt)
    http.HandleFunc("/abrechnung", basicAuth(handleAbrechnungPage))

	fmt.Println("Server läuft auf http://localhost:8888")
	http.ListenAndServe(":8888", nil)
}
        // Seite nach dem Speichern oder Löschen eines Eintrags
        func renderThankYouPage(w http.ResponseWriter, username, realname, path, message string) {
	statsPath := fmt.Sprintf("stats/%s.csv", username)
	stats := make(map[string]int)

	if file, err := os.Open(statsPath); err == nil {
		reader := csv.NewReader(file)
		records, _ := reader.ReadAll()
		for _, row := range records[1:] {
			if len(row) == 2 {
				drink := row[0]
				var amt int
				fmt.Sscanf(row[1], "%d", &amt)
				stats[drink] = amt
			}
		}
		file.Close()
	}

	var tableRows string
	var total float64
	for k, v := range stats {
		price := prices[k]
		itemTotal := float64(v) * price
		total += itemTotal
		tableRows += fmt.Sprintf("<tr><td>%s</td><td>%d</td><td>%s</td><td>%s</td></tr>\n", k, v, formatEuro(price), formatEuro(itemTotal))
	}

	// Zeige den Undo-Button nur, wenn überhaupt ein Eintrag existiert
	undoButton := ""
	files, _ := os.ReadDir(".")
	found := false
	for i := len(files) - 1; i >= 0 && !found; i-- {
		if !strings.HasPrefix(files[i].Name(), "data-") {
			continue
		}
		f, _ := os.Open(files[i].Name())
		rows, _ := csv.NewReader(f).ReadAll()
		f.Close()
		for j := len(rows) - 1; j >= 0; j-- {
			if len(rows[j]) >= 2 && rows[j][1] == username {
				found = true
				break
			}
		}
	}
	if found {
		undoButton = fmt.Sprintf(`<p style="margin-top: 2em;"><a class="button-pink" href="/bar-undo/%s">Letzten Eintrag löschen</a></p>`, username)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Deine Übersicht</title>
  <link rel="stylesheet" href="/style.css">
  <link rel="icon" href="/favicon.png" type="image/png">
</head>
<body>
<div class="success">
  <p>%s, %s!</p>
  <p><a class="button-orange" href="%s">Zurück</a></p>
  <h2>Bisher getrunken/gesnackt:</h2>
  <div class="scrollbar">
  <table>
    <tr><th>Getränk/Snack</th><th>Menge</th><th>Preis</th><th>Gesamt</th></tr>
    %s
    <tfoot>
      <tr><td colspan="3">Summe</td><td>%s</td></tr>
    </tfoot>
  </table>
  </div>
  %s
</div>
<footer class="footer">
  <p>
    © 2025 Heike Jurzik · Dieses Projekt steht unter der
    <a href="https://www.gnu.org/licenses/gpl-3.0.de.html" target="_blank" rel="license noopener">GPLv3</a>.
  </p>
</footer>
</body>
</html>
`, message, realname, path, tableRows, formatEuro(total), undoButton)
}

// Neue Unterseite: /abrechnung
func handleAbrechnungPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>KSI-Abrechnung</title>
  <link rel="stylesheet" href="/style.css">
  <link rel="icon" href="/favicon.png" type="image/png">
</head>
<body class="no-bg">
  <h1>KSI-Abrechnung</h1>
  <table>
    <tr><th>Name</th>`, )

	// Überschrift: Alle Getränke in alphabetischer Reihenfolge
	drinks := make([]string, 0, len(prices))
	for k := range prices {
		drinks = append(drinks, k)
	}
	sort.Strings(drinks)
	for _, d := range drinks {
		fmt.Fprintf(w, "<th>%s</th>", d)
	}
	fmt.Fprint(w, "<th>Summe</th></tr>\n")

	var grandTotal float64

	// Alle Nutzerinnen durchgehen
	for _, username := range userOrder {
        realname := allowedUsers[username]
		statsPath := fmt.Sprintf("stats/%s.csv", username)
		file, err := os.Open(statsPath)
		if err != nil {
			continue
		}
		reader := csv.NewReader(file)
		records, _ := reader.ReadAll()
		file.Close()

		drinkCounts := make(map[string]int)
		for _, row := range records[1:] {
			if len(row) == 2 {
				drink := row[0]
				var count int
				fmt.Sscanf(row[1], "%d", &count)
				drinkCounts[drink] = count
			}
		}

		fmt.Fprintf(w, "<tr><td>%s</td>", realname)
		userTotal := 0.0
		for _, d := range drinks {
			count := drinkCounts[d]
			if count == 0 {
				fmt.Fprint(w, "<td></td>")
			} else {
				fmt.Fprintf(w, "<td>%d</td>", count)
				userTotal += float64(count) * prices[d]
			}
		}
		fmt.Fprintf(w, "<td>%s</td></tr>\n", formatEuro(userTotal))
		grandTotal += userTotal
	}

	// Gesamtsumme aller Einträge
	fmt.Fprintf(w, `<tfoot><tr><td colspan="%d">Gesamtsumme</td><td>%s</td></tr></tfoot>`, len(drinks)+1, formatEuro(grandTotal))
	fmt.Fprint(w, "</table></body></html>")
}
