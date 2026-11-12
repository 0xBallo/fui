# FUI — Specifiche tecniche

Nov 12, 2026 · @Mattia Ballo

**Versione 1.2** — 12 novembre 2026 · allineata alle FS 1.2, con i temi Catppuccin della TUI; le modifiche rispetto alle versioni precedenti sono nella sezione Cronologia delle versioni

## Panoramica e principi tecnici

FUI è un unico binario Go che espone CLI e TUI sopra lo stesso core di dominio puro, con SQLite incluso e nessuna dipendenza di rete. Questo documento traduce le Specifiche funzionali (FS) in scelte di implementazione, convenzioni di codice, flusso Git e strategia anti-regressione, ed è allineato alla versione 1.2 delle FS (moduli, beni, ricerca con filtri, reset dell'archivio).

| Principio delle FS | Conseguenza tecnica |
| --- | --- |
| Local-first | Nessun import di `net/http` nel binario `fui`, verificato da un test; archivio in un solo file SQLite |
| Un core, due interfacce | CLI e TUI sono adattatori sottili sopra gli stessi use case in `internal/app`; nessuna logica nei comandi o nelle schermate |
| Scriptabile | Ogni use case restituisce dati strutturati; il rendering (tabella, JSON, CSV) è un livello separato con output coperto da golden test |
| Importi esatti | Tipo `Money` su `int64` in unità minime; `float64` vietato nel dominio da linter, salvo il pacchetto delle simulazioni |
| Tastiera prima di tutto | Keymap dichiarative per schermata, generate anche nell'aiuto `?` |
| Anteprima delle scritture | Pattern Plan/Apply: ogni scrittura produce prima un piano di modifiche, che `--dry-run` mostra e `--yes` applica |
| Modularità | Un registro dei moduli decide quali comandi, schermate e dati sono esposti; lo schema dell'archivio resta completo, così disattivare un modulo non cancella nulla |

Gli obiettivi misurabili (avvio CLI < 100 ms, TUI < 300 ms, import di 10.000 righe < 5 s, ricerca con filtri < 200 ms su 100.000 transazioni) sono verificati da benchmark in CI, non solo dichiarati.

## Stack e dipendenze

Lo stack resta quello deciso nelle FS (Go, Bubble Tea, SQLite senza cgo); attorno si aggiungono poche librerie mature, ognuna con un motivo preciso. Ogni nuova dipendenza richiede un ADR breve: meno dipendenze significa meno superficie per vulnerabilità e rotture.

| Area | Scelta | Motivo |
| --- | --- | --- |
| Linguaggio | Go, ultima versione stabile, fissata con `toolchain` in `go.mod` | Build riproducibili; si supportano le ultime due minor |
| CLI | `spf13/cobra` | Sottocomandi annidati, `--help` per comando, completamento bash, zsh e fish già incluso |
| Prompt interattivi | `charmbracelet/huh` | Stesso ecosistema della TUI; usato solo se c'è un terminale e manca `--no-input` |
| TUI | `charmbracelet/bubbletea`, `bubbles`, `lipgloss` | Architettura Elm; liste, input, help e keymap pronti |
| Archivio | `modernc.org/sqlite` | SQLite in puro Go, `CGO_ENABLED=0`, cross-compilazione banale |
| Query | `sqlc` (generazione da SQL) | Query scritte in SQL e verificate in fase di build; nessun ORM |
| Migrazioni | `pressly/goose` con `embed.FS` | Migrazioni SQL numerate dentro il binario |
| Configurazione | `pelletier/go-toml/v2` | Parser TOML rigoroso, decodifica in struct tipate |
| Log | `log/slog` (stdlib) | Log strutturati, redazione dei dati sensibili tramite `LogValuer` |
| Excel (connettore Widiba) | `xuri/excelize` | Lettura XLSX solo nel binario del connettore, non in `fui` |
| Test | `testing`, `google/go-cmp`, `rogpeppe/go-internal/testscript`, `pgregory.net/rapid`, `charmbracelet/x/exp/teatest` | Unit, end-to-end CLI, property-based, snapshot della TUI |
| Qualità | `golangci-lint`, `gofumpt`, `govulncheck` | Lint, formattazione, vulnerabilità note |
| Release | `goreleaser` | Binari firmati per Linux e macOS, checksum, SBOM |
| Ricerca testuale | FTS5 di SQLite con tokenizer trigram + `golang.org/x/text` per la normalizzazione | Corrispondenza parziale senza distinzione di maiuscole e accenti entro 200 ms su 100.000 transazioni |
| Palette della TUI | `catppuccin/go` | Palette ufficiale dei quattro flavor come costanti Go, licenza MIT; usata solo da `internal/tui/theme` |

Importi, date senza orario, tassi di cambio e quantità di titoli usano tipi propri in `internal/domain`, non librerie esterne: sono il cuore della correttezza e devono restare piccoli e testati a fondo.

## Architettura

FUI segue un'architettura esagonale (ports and adapters): il dominio al centro non conosce né SQLite né il terminale, gli use case orchestrano tramite porte, e CLI, TUI e adattatori stanno ai bordi. Le regole di dipendenza sono imposte da `depguard` in CI, non solo documentate.

```text
   CLI · internal/cli          TUI · internal/tui
             \                      /
              v                    v
        Use case · internal/app            <── porte ──   Adattatori
        (Plan/Apply, Unit of Work,                        - Store SQLite (sqlc, goose)
         porte: Store, Clock, Source,                     - Runner connettori
         Backup, Config)                                  - Config e log
                    |                                     - Backup e clock
                    v
        Dominio · internal/domain          <── tipi ───   Adattatori
        (funzioni pure, solo stdlib)
```

CLI e TUI chiamano gli stessi use case; store, runner dei connettori, configurazione, backup e orologio implementano le porte definite in `internal/app` e vengono assemblati solo in `cmd/fui/main.go`.

**Regole di dipendenza**

- `internal/domain` importa solo la libreria standard.
- `internal/app` importa il dominio e definisce le porte; non importa nessun adattatore.
- `internal/cli` e `internal/tui` importano `app` e `domain`, mai `store` o `connector`.
- `pkg/fuiproto` non importa nulla da `internal`.
- `internal/module` importa solo la libreria standard ed è usabile da tutti gli strati.
- `cmd/fui/main.go` è la composition root: l'unico punto che conosce tutte le implementazioni.

**Layout del repository**

```text
fui/
├── cmd/
│   ├── fui/                      main: composition root
│   ├── fui-connector-revolut/
│   └── fui-connector-widiba/
├── internal/
│   ├── domain/                   tipi valore, entità, servizi puri
│   │   └── simulate/             simulazioni (unico posto con float64)
│   ├── app/                      use case, porte, pipeline di import
│   ├── store/sqlite/             repository, migrations/, queries/ (sqlc)
│   ├── connector/                runner esterni, connettore CSV integrato
│   ├── cli/                      comandi cobra e renderer
│   ├── tui/                      root, screens/, components/
│   ├── config/  backup/  clock/  skill/
│   ├── module/                   registro dei moduli, dipendenze, preset
│   └── testutil/                 fixture e generatori di archivi
├── pkg/fuiproto/                 protocollo fui/1, codec, validatore, conformance/
├── docs/adr/
├── tools/                        anonymize e altri script di sviluppo
└── testdata/
```

Un solo modulo Go per tutto il repository: `fui` e i connettori condividono `pkg/fuiproto` e si rilasciano insieme, con la stessa versione.

## Moduli

I moduli delle FS sono un registro statico nel codice che governa cosa è esposto, non cosa esiste: lo schema dell'archivio è sempre completo e un modulo disattivato nasconde comandi, schermate e dati senza toccarli. Così attivare o disattivare un modulo non richiede migrazioni e non può perdere dati.

**Registro** — `internal/module` definisce un tipo `module.ID` e una tabella costante con, per ogni modulo, dipendenze, attivo di default e preset di appartenenza. Il registro è puro e testato a parte.

| Modulo | Dipende da | Default | Preset `expense` |
| --- | --- | --- | --- |
| `core` | — | sempre | sì |
| `budget` | — | sì | sì |
| `recurring` | — | sì | sì |
| `loans` | `recurring` | no | no |
| `investments` | — | no | no |
| `crypto` | `investments` | no | no |
| `assets` | — | no | no |
| `analytics` | — | sì | no |
| `simulations` | `recurring` | no | no |
| `ai` | `simulations` | no | no |

- **Attivazione:** `Enable(id)` restituisce l'insieme chiuso rispetto alle dipendenze, mostrato come avviso; `Disable(id)` restituisce i moduli attivi che ne dipendono e richiede conferma. Entrambe sono use case Plan/Apply che scrivono solo `modules.enabled` nella configurazione.
- **Validazione:** all'avvio un `modules.enabled` incoerente (dipendenza mancante, id sconosciuto) produce un errore di configurazione con il nome del modulo; `core` non si può disattivare.
- **Porta applicativa:** gli use case ricevono un `module.Set` immutabile e chiamano `Require(id)` all'inizio; un modulo spento produce l'errore `module_disabled` (codice di uscita 1) con il comando per attivarlo.

**Effetti di un modulo disattivato**

| Dove | Comportamento | Come |
| --- | --- | --- |
| CLI | Comandi nascosti da `--help` e `fui help --json`; se invocati falliscono con `module_disabled` | Annotazione `module` su ogni comando cobra, applicata da un hook della radice |
| TUI | Schermate e riquadri della Dashboard assenti; i tasti delle altre schermate non cambiano | Il router registra solo le schermate dei moduli attivi; i tasti numerici restano fissi |
| Dati | Nessuna cancellazione; esclusi da Dashboard, report e patrimonio netto | Le query di aggregazione ricevono il `module.Set` e filtrano per tipo di conto ed entità |
| Import | Record di moduli spenti (es. `schedule`, `installment`) segnalati nell'anteprima, con scelta tra attivare il modulo o saltarli | Il validatore dello stream classifica ogni record per modulo |
| Agenti | La skill legge `fui module list --json` | Output stabile con id, stato e dipendenze |

`fui init` accetta `--preset expense|full|custom` e `--modules`; in un terminale, senza flag, chiede il profilo con un prompt.

## Modello dati e schema SQLite

Lo schema usa tabelle `STRICT`, chiavi esterne attive, importi come `INTEGER` in unità minime e date come `TEXT` ISO `YYYY-MM-DD`. Le invarianti che SQLite sa esprimere (vincoli `CHECK`, `UNIQUE`, `FOREIGN KEY`) stanno nello schema; le altre stanno nel dominio e sono coperte da test.

**Convenzioni di schema**

- Chiave primaria `id INTEGER PRIMARY KEY`; gli id esterni dei connettori stanno in colonne separate.
- Nomi di tabelle al plurale, colonne in `snake_case`, chiavi esterne `<entità>_id`.
- Nomi utente (conti, categorie, tag) `UNIQUE COLLATE NOCASE`.
- Enum come `TEXT` con `CHECK (kind IN (...))`, leggibili nei dump.
- `created_at` e `updated_at` in UTC ISO 8601; non entrano mai nei calcoli, che usano solo le date contabili.
- Tassi di cambio come intero scalato a 10^9; quantità di titoli e cripto come intero con una scala per titolo (colonna `quantity_scale`, es. 8 per le cripto).

| Tabella | Contenuto | Vincoli e indici chiave |
| --- | --- | --- |
| `accounts` | Conti, incluso il tipo finanziamento | `kind` con `CHECK`; `archived_at` nullable |
| `transactions` | Movimenti con stato, data contabile, operazione e valuta | `UNIQUE(account_id, external_id)`; indici `(account_id, date, id)`, `(category_id, date)`, `(fingerprint)` |
| `transaction_splits` | Righe di split | Somma = importo della transazione, verificata nel dominio e da un test di integrità |
| `transfer_groups` | Collega i due lati di un trasferimento e l'eventuale commissione | Somma dei lati = − commissione |
| `categories`, `tags`, `transaction_tags` | Classificazione | `ON DELETE RESTRICT` sulle categorie usate |
| `payees`, `payee_aliases` | Beneficiari normalizzati e alias per gli import | Alias unico per beneficiario |
| `budgets`, `budget_amounts`, `budget_moves` | Tetti e buste, variazioni mensili, assegnazioni e spostamenti | Una categoria coperta da un solo budget per periodo |
| `recurrences` | Modelli di transazione periodica | Regola serializzata in colonne esplicite, non JSON |
| `rules` | Regole di categorizzazione | Condizioni e azioni in JSON validato, `priority` per l'ordine |
| `fx_rates` | Tassi manuali per valuta e data | `UNIQUE(currency, date)` |
| `securities`, `security_prices`, `security_ops` | Titoli e cripto con scala della quantità, prezzi manuali, operazioni (v0.3, moduli investments e crypto) | `security_ops.cash_tx_id` collega il movimento di liquidità |
| `loans`, `loan_schedules`, `loan_installments`, `loan_events` | Finanziamenti con piani versionati (v0.2) | `UNIQUE(loan_id, valid_from)` sui piani |
| `import_batches`, `connector_account_refs` | Lotti di import annullabili, associazioni `ref` → conto | `source_sha256` per riconoscere un file già importato |
| `saved_reports`, `dismissed_insights` | Report salvati, segnalazioni ignorate | Nessuno particolare |
| `assets`, `asset_valuations` | Beni (immobili, veicoli, altro) e valutazioni manuali per data (v0.3, modulo assets) | `loan_id` nullable verso `loans`; `UNIQUE(asset_id, date)` sulle valutazioni |
| `transactions_fts` | Indice FTS5 trigram su descrizione, beneficiario e note normalizzati | Tabella esterna sincronizzata da trigger su `transactions` |

Il saldo non è memorizzato: si calcola come saldo iniziale più somma dei movimenti, sfruttando l'indice `(account_id, date, id)`. Se i benchmark lo richiederanno si aggiungerà una cache per mese, invalidata dalle scritture e verificata da un test che la confronta con il calcolo completo.

**Ricerca testuale** — ogni transazione ha una colonna `search_text` calcolata in Go: descrizione, beneficiario e note in minuscolo, senza accenti (decomposizione NFD e rimozione dei segni diacritici) e con spazi normalizzati. Su quella colonna lavora l'indice FTS5 con tokenizer trigram, che supporta la corrispondenza parziale; la stessa funzione normalizza il testo cercato, così indice e query non possono divergere. Per testi più corti di tre caratteri si ricade su `LIKE` sulla stessa colonna.

**Moduli e schema** — tutte le tabelle esistono a prescindere dai moduli attivi: la disattivazione agisce sulle query, mai sullo schema.

## Core di dominio

Il dominio è codice puro e deterministico: niente I/O, niente orologio, niente variabili globali, solo funzioni e tipi valore che si testano in isolamento. È il pacchetto con la copertura di test più alta e le regole di lint più severe.

**Tipi valore**

| Tipo | Rappresentazione | Regole |
| --- | --- | --- |
| `Money` | `int64` in unità minime + `Currency` | Somma e sottrazione restituiscono errore su valute diverse o overflow; nessuna conversione implicita |
| `Currency` | Codice ISO 4217 con numero di decimali | Costruito solo da una tabella nota; decimali da ISO (EUR 2, JPY 0) |
| `Date` | Anno, mese, giorno senza orario | Nessun `time.Time` nel dominio; conversione dal fuso locale solo agli estremi |
| `Period` | Intervallo di date chiuso | Calcolo del mese contabile con `month_start_day` (giorni 29–31 limitati all'ultimo giorno del mese) |
| `Rate` | Intero scalato a 10^9 | Conversione valutaria con arrotondamento unico per transazione |
| `AccountID`, `CategoryID`, … | Tipi distinti su `int64` | Impossibile passare l'id di una categoria al posto di un conto |
| `Quantity` | `int64` + scala del titolo | Operazioni solo tra quantità con la stessa scala; valore = quantità × prezzo con `big.Rat` e un solo arrotondamento finale |
| `TxFilter` | Struct immutabile con periodo, conti, categorie, tag, beneficiario, importi, stato, natura, lotto, testo, ordinamento e limite | Filtri diversi in AND, valori ripetuti dello stesso filtro in OR; `--min` e `--max` sul valore assoluto |

I costruttori validano e restituiscono errore ("parse, don't validate"): un valore di tipo `Money` o `Date` esistente è sempre valido. La politica di arrotondamento è unica, metà lontano dallo zero, implementata in un solo punto con test dedicati.

**Servizi di dominio**

- **Saldo:** saldo iniziale + somma dei movimenti fino a una data; è la base di tutti gli altri calcoli.
- **Impronta e deduplicazione:** `fingerprint = sha256(account | date | amount | descrizione normalizzata)`; classificazione in nuova, duplicata o dubbia (stesso conto e importo entro ±2 giorni).
- **Abbinamento trasferimenti:** cerca il movimento opposto per IBAN della controparte, poi per importo e data entro 2 giorni; propone, non decide.
- **Motore delle regole:** applica le regole in ordine di `priority`; la prima che imposta un campo vince per quel campo; restituisce anche quale regola ha agito, per l'anteprima e per `fui rule test`.
- **Budget:** calcolo di speso, previsto, residuo e riporto per tetti e buste; esclusione di trasferimenti, movimenti di investimento e quote capitale.
- **Ammortamento:** piano alla francese calcolato con `math/big.Rat` e arrotondato ai centesimi solo alla fine di ogni rata; l'ultima rata assorbe il residuo, così il capitale rimborsato coincide al centesimo.
- **Autonomia e previsione:** mediana delle uscite mensili su 12 mesi completi; previsione per giorno a partire da ricorrenze, rate e spesa stimata.
- **Filtro delle transazioni:** un solo `TxFilter` alimenta `fui tx list`, `fui tx search`, `fui export`, la schermata Transazioni e i report; il dominio offre anche `Match(tx)` in memoria, usato come oracolo nei test della traduzione in SQL.
- **Patrimonio netto:** liquidità + posizioni + ultima valutazione dei beni − debiti residui, considerando solo i moduli attivi; un bene collegato a un mutuo mostra anche il valore netto del debito.

```latex
R = C \cdot \frac{i}{1 - (1 + i)^{-n}}
```

Rata costante del piano alla francese: C capitale residuo, i tasso periodale, n numero di rate rimanenti.

**Errori di dominio** — sentinelle (`ErrAmbiguousName`, `ErrCurrencyMismatch`, `ErrReconciled`) e un tipo `domain.Error` con codice stabile. Il codice diventa la chiave `code` nell'errore JSON e determina il codice di uscita della CLI.

## CLI

Ogni comando è un adattatore sottile: legge flag e argomenti, costruisce una richiesta, chiama uno use case e passa il risultato a un renderer. Un comando non contiene logica di business e non supera di regola le 60 righe.

**Struttura**

- Un file per comando in `internal/cli`, nominato `<nome>_<verbo>.go` (es. `tx_list.go`, `budget_assign.go`), con l'albero cobra che rispecchia la tabella dei comandi delle FS.
- Flag comuni (`--json`, `--csv`, `--no-input`, `--dry-run`, `--yes`, `--db`, `--verbose`) definiti una volta come persistent flag della radice.
- `fui` senza argomenti avvia la TUI; il punto di ingresso è lo stesso, cambia solo l'adattatore.

**Plan/Apply** — ogni use case di scrittura ha due fasi: `Plan` calcola un insieme di modifiche senza scrivere, `Apply` le esegue in una transazione. `--dry-run` stampa il piano ed esce con 0; senza `--dry-run` la CLI applica il piano, chiedendo conferma solo per le modifiche di massa se c'è un terminale. Lo stesso piano alimenta l'anteprima della TUI e le regole di sicurezza della skill per agenti.

**Output**

| Formato | Regole |
| --- | --- |
| Tabella (predefinito) | Colonne allineate, importi formattati secondo `locale`, colori solo su terminale e senza `NO_COLOR` |
| `--json` | Liste come array, dettagli come oggetto; chiavi `snake_case`; importi come intero in unità minime con campo `currency`; date `YYYY-MM-DD`; nessun campo rimosso o rinominato senza una versione major |
| `--csv` | RFC 4180, intestazione sempre presente, separatore virgola e punto decimale indipendenti dal locale |

**Errori e codici di uscita**

| Codice | Quando | Esempi di `code` JSON |
| --- | --- | --- |
| 0 | Successo, incluso `--dry-run` | — |
| 1 | Errore di input dell'utente | `invalid_amount`, `ambiguous_name`, `missing_field`, `module_disabled`, `confirmation_required` |
| 2 | Errore di dati o di I/O | `db_locked`, `db_corrupted`, `connector_failed`, `migration_failed` |

Con `--json` l'errore va su stderr come `{"error":{"code":"ambiguous_name","message":"…","details":{"candidates":["Fineco","Fineco Risparmio"]}}}`. La mappatura da errore di dominio a codice di uscita sta in un'unica funzione, coperta da un test che elenca tutti i codici.

**Riferimenti per nome** — risoluzione in tre passi: corrispondenza esatta senza distinzione di maiuscole, poi prefisso unico, poi sottostringa unica. Se un passo trova più candidati, FUI esce con `ambiguous_name` e li elenca.

**Importi e date** — un parser unico per importi (virgola o punto secondo `locale`, segno, separatore delle migliaia opzionale) e uno per date (assolute, `today`, `yesterday`, `-3d`, `-2w`, `-1m`), usati da CLI e TUI e coperti da fuzzing.

**Ricerca e filtri** — i flag dei filtri (`--from`, `--to`, `--month`, `--year`, `-a`, `-c`, `--uncategorized`, `-t`, `--payee`, `--min`, `--max`, `--income`, `--expense`, `--status`, `--kind`, `--batch`, `--sort`, `--desc`, `--limit`) sono definiti una volta in `internal/cli/filterflags.go` e montati su `tx list`, `tx search` ed `export`. Producono un `TxFilter`, risolvendo i nomi con le stesse regole dei riferimenti; flag incompatibili (es. `--month` con `--from`) danno `invalid_filter`. L'output in tabella termina con numero di righe e somma degli importi; in JSON lo stesso riepilogo sta in un oggetto `summary` accanto alle righe.

**Reset** — `fui reset` è uno use case Plan/Apply come gli altri: il piano elenca cosa viene cancellato e cosa conservato con `--keep-settings`. In un terminale la conferma richiede di digitare `reset`; senza terminale o con `FUI_AGENT=1` serve `--yes`, altrimenti l'errore è `confirmation_required`. Se l'archivio non esiste, il comando delega a `init`.

**Moduli** — `fui module list / enable / disable` e `fui init --preset|--modules` sono comandi del nucleo; `fui asset add / list / value` appartiene al modulo `assets`. L'annotazione del modulo su ogni comando cobra è verificata da un test che fallisce se un comando ne è privo.

**Modalità agente** — `FUI_AGENT=1` equivale a `--no-input --json` e fa fallire con `confirmation_required` ogni scrittura senza `--yes`. `fui help --json` è generato visitando l'albero cobra e gli schemi JSON dei tipi di output, quindi non può divergere dal binario; il suo output è un golden file versionato.

## TUI

La TUI segue l'architettura Elm di Bubble Tea con una regola ferma: `Update` e `View` non fanno mai I/O. Le letture e le scritture sull'archivio partono come `tea.Cmd` e tornano come messaggi, così l'interfaccia resta fluida anche con 100.000 transazioni.

**Struttura dei modelli**

- `tui.Root` possiede il router delle schermate, la barra di stato, l'aiuto e la keymap globale; inoltra i messaggi alla schermata attiva.
- Ogni schermata (`dashboard`, `transactions`, `accounts`, `budget`, `reports`, `import`, `settings`) è un pacchetto in `internal/tui/screens` che implementa un'interfaccia `Screen` con `Init`, `Update`, `View`, `KeyMap` e `Title`.
- Componenti riusabili (tabella virtualizzata, form con autocompletamento, barra di avanzamento, grafico a barre testuale, dialogo di conferma) stanno in `internal/tui/components` e non conoscono il dominio.
- Le schermate ricevono gli use case per interfaccia nel costruttore, come la CLI.

**Liste grandi** — la tabella delle transazioni carica pagine di 200 righe con paginazione per chiave `(date, id)` e non per `OFFSET`, mantenendo in memoria al massimo tre pagine attorno alla selezione. Il rendering riguarda solo le righe visibili.

**Tastiera e aiuto** — ogni azione è una `key.Binding` di `bubbles/key` con descrizione; l'aiuto `?` e la riga di suggerimenti si generano dalle keymap, quindi non possono divergere dalle scorciatoie reali. Un test verifica che nessuna schermata abbia due azioni sullo stesso tasto.

**Ricerca e filtri** — `/` aggiorna i risultati mentre si digita: ogni tasto riavvia un timer di 150 ms e annulla la query precedente tramite `context`, così arriva a schermo solo l'ultimo risultato. `f` apre un pannello che costruisce lo stesso `TxFilter` della CLI; i filtri attivi restano in un'intestazione sopra la lista e si rimuovono singolarmente.

**Moduli** — il router registra solo le schermate dei moduli attivi e la Dashboard compone i riquadri dallo stesso registro. Dalla schermata Impostazioni si attivano e disattivano i moduli; al cambio la TUI ricostruisce router e Dashboard senza riavvio.

**Layout e tema**

- Dimensioni lette da `tea.WindowSizeMsg`; sotto 80×24 la TUI mostra solo un avviso con la dimensione minima.
- Colori dalla palette [Catppuccin](https://catppuccin.com/palette), con scelta del tema nelle Impostazioni o selezione automatica chiara/scura in base al terminale (dettagli sotto).
- Nessuna informazione solo cromatica: segno meno sulle uscite, etichetta `OVER` sui budget sforati, simboli per gli stati delle transazioni.

**Temi** — i quattro flavor di Catppuccin diventano temi della TUI: Latte (chiaro), Frappé, Macchiato e Mocha (scuri). Il valore predefinito è `auto`: all'avvio FUI rileva lo sfondo del terminale e usa Latte su sfondo chiaro, Mocha su sfondo scuro; le due scelte sono configurabili.

| Valore di `theme` | Comportamento |
| --- | --- |
| `auto` (predefinito) | Rileva lo sfondo del terminale e applica `theme_light` o `theme_dark` |
| `latte`, `frappe`, `macchiato`, `mocha` | Flavor fisso, indipendente dal terminale |
| `terminal` | Solo i 16 colori ANSI del terminale, come fallback per terminali senza truecolor o per chi preferisce il proprio schema |
| `high-contrast` | Variante ad alto contrasto: sfondo `Crust`, testo `Text`, accenti saturi, scelta in automatico tra Latte e Mocha come per `auto` |

**Rilevamento dello sfondo** — all'avvio la TUI interroga il colore di sfondo del terminale con la sequenza OSC 11, con un timeout di 100 ms per restare entro i 300 ms di apertura. Se il terminale non risponde, usa la variabile `COLORFGBG`; in mancanza di entrambi assume uno sfondo scuro. Se il terminale notifica un cambio di tema durante l'esecuzione, la TUI passa al flavor corrispondente senza riavvio.

**Token semantici** — i componenti non usano mai colori della palette direttamente: il pacchetto `internal/tui/theme` espone token con un significato, e ogni tema li mappa sui colori del proprio flavor.

| Token | Colore Catppuccin | Uso |
| --- | --- | --- |
| `Base`, `Surface`, `Selection` | Base, Surface0, Surface1 | Sfondo, pannelli, riga selezionata |
| `Text`, `Muted`, `Border` | Text, Subtext0, Overlay0 | Testo, testo secondario, bordi |
| `Accent` | Mauve | Schermata attiva, focus, titoli |
| `Income`, `Expense` | Green, Red | Entrate e uscite (sempre insieme al segno) |
| `Warning`, `Over` | Yellow, Red | Budget oltre soglia e sforati (sempre con etichetta) |
| `Pending`, `Reconciled` | Peach, Teal | Stati delle transazioni (sempre con simbolo) |
| `Series1…8` | Blue, Sapphire, Teal, Green, Yellow, Peach, Maroon, Mauve | Serie dei grafici e sparkline |

**Profondità di colore e `NO_COLOR`** — Lip Gloss riduce i colori esadecimali a 256 colori quando il terminale non supporta il truecolor; con soli 16 colori FUI passa al tema `terminal`. Con `NO_COLOR` impostato non si usano colori, qualunque sia il tema.

**Scelta del tema** — nella schermata Impostazioni una voce Aspetto elenca i temi con anteprima immediata; la scelta si salva in `theme` (oppure con `fui config set theme mocha`). Un tema scelto esplicitamente non viene mai sovrascritto dal rilevamento automatico.

**Salvataggio** — ogni modifica chiama subito lo use case corrispondente; un errore compare nella barra di stato e la vista ricarica i dati dall'archivio, senza stato ottimistico da riconciliare.

## Connettori e protocollo fui/1

Connettori integrati ed esterni producono lo stesso flusso di record `fui/1`; da quel punto in poi l'import è un unico codice, testato una volta sola. I tipi del protocollo stanno in `pkg/fuiproto`, l'unico pacchetto pubblico, così anche connettori di terzi scritti in Go riusano codec e validatore.

**Interfaccia comune** — `connector.Source` restituisce un iteratore di record (`iter.Seq2[fuiproto.Record, error]`). Il connettore CSV integrato la implementa in-process; il runner degli esterni la implementa avviando un processo. Il formato `.fui.jsonl` letto da file è solo un'altra `Source`.

**Runner dei connettori esterni**

- Avvio con `exec.CommandContext`, timeout predefinito di 120 s configurabile, directory di lavoro temporanea.
- Ambiente ridotto: niente `FUI_DB` né altre variabili di FUI; il connettore riceve solo la propria configurazione su stdin.
- Lettura di stdout riga per riga in streaming, con limite di 1 MiB per riga; stderr salvato e mostrato con `--verbose`.
- Il processo non è in sandbox: l'isolamento è per contratto (nessun accesso all'archivio), come già dichiarato nelle FS.

**Validazione dello stream** — un automa a stati verifica: `meta` come prima riga e major di protocollo 1; ogni `account` dichiarato prima dei record che lo usano; valuta di ogni transazione uguale a quella del conto; importi interi; `schedule` prima delle sue `installment`; `end` come ultima riga con conteggi coincidenti. Una violazione strutturale annulla l'import; una riga non valida diventa un errore nell'anteprima.

**Pipeline di import**

1. Lettura dalla `Source` e validazione dello stream; i record dei moduli disattivati vengono marcati per l'anteprima, che chiede se attivare il modulo o saltarli.
2. Risoluzione dei `ref` in conti tramite `connector_account_refs`, con richiesta all'utente al primo uso.
3. Normalizzazione di descrizioni e beneficiari tramite alias.
4. Deduplicazione per id esterno, poi per impronta; marcatura delle righe dubbie.
5. Abbinamento di trasferimenti e di transazioni in sospeso da ricorrenze o rate.
6. Applicazione delle regole di categorizzazione.
7. Costruzione del piano, mostrato come anteprima o con `--dry-run`.
8. Backup automatico, poi scrittura in una sola transazione SQL con un nuovo `import_batch`.

I passi 1–7 non scrivono nulla e sono funzioni del dominio o dell'applicazione testate senza database dove possibile.

**Connettori Revolut e Widiba** — scritti in Go nello stesso repository (`cmd/fui-connector-revolut`, `cmd/fui-connector-widiba`) e rilasciati insieme a `fui`. Ognuno ha una cartella `testdata` con export reali resi anonimi (nomi, IBAN, descrizioni sostituiti in modo coerente) e l'output `.jsonl` atteso come golden file. Le colonne Widiba si trovano cercando il testo dell'intestazione, mai per posizione.

**Suite di conformità** — `fuiproto/conformance` esegue un connettore qualsiasi su file di prova e verifica `describe`, codici di uscita, struttura dello stream e stabilità degli id tra due esecuzioni. La usano i connettori del repository e la possono usare quelli di terzi.

## Persistenza, migrazioni e backup

Ogni use case di scrittura è una sola transazione SQL: o si applica tutto o niente, e un'interruzione forzata lascia l'archivio nello stato precedente. Le migrazioni vanno solo in avanti e sono sempre precedute da un backup.

**Connessione** — all'apertura FUI imposta `foreign_keys = ON`, `journal_mode = WAL`, `synchronous = FULL` (le scritture sono rare, la durabilità conta più della velocità), `busy_timeout = 5000`. Una sola connessione di scrittura e un piccolo pool di lettura; due processi `fui` concorrenti si serializzano invece di corrompere.

**Unit of Work** — lo strato applicativo usa una porta `Store` con `WithTx(ctx, func(Repos) error) error`. I repository non aprono transazioni da soli; lo use case decide i confini, il repository esegue query generate da `sqlc`.

**Migrazioni**

- File SQL numerati in `internal/store/sqlite/migrations`, incorporati nel binario con `embed.FS` ed eseguiti da `goose` all'avvio.
- Solo in avanti: niente migrazioni `down` in produzione; per tornare indietro si ripristina il backup creato prima della migrazione.
- La versione dello schema sta in `PRAGMA user_version`; un archivio più recente del binario non viene aperto (errore `db_newer_than_binary`, codice 2).
- Ogni migrazione che trasforma dati ha un test con un archivio di partenza in `testdata` e i valori attesi dopo la migrazione.

**Backup**

- `VACUUM INTO` produce una copia coerente anche con l'archivio aperto, nominata `fui-AAAAMMGG-HHMMSS-<motivo>.db` (motivo: `manual`, `import`, `migrate`, `bulk`, `restore`, `reset`).
- Backup automatico prima di import, migrazioni, modifiche di massa, `restore` e `reset`; si conservano gli ultimi `backup.keep` (predefinito 10).
- Permessi `600` sui file e `700` sulle directory, impostati alla creazione e non corretti dopo.

**Ripristino** — `fui restore` controlla il file con `PRAGMA integrity_check` e la versione dello schema, fa un backup dell'archivio corrente, scrive in un file temporaneo nella stessa directory e lo sostituisce con un `rename` atomico. Il dump completo in JSON o CSV segue lo stesso percorso dopo aver ricostruito un archivio temporaneo.

**Reset** — `fui reset` costruisce l'archivio nuovo in un file temporaneo nella stessa directory: esegue le migrazioni, crea le categorie predefinite e, con `--keep-settings`, copia categorie, tag, regole, profili di import e associazioni dei connettori dall'archivio corrente. Prima del `rename` atomico crea un backup con motivo `reset`, che la rotazione di `backup.keep` non cancella mai; resta finché l'utente non lo elimina. Il file di configurazione non viene toccato.

## Configurazione, log e sicurezza

La configurazione è una struct tipata validata all'avvio; i log non contengono mai importi, descrizioni o beneficiari; nessun codice di FUI apre connessioni di rete, e un test lo verifica sul grafo delle dipendenze.

**Configurazione**

- `internal/config` definisce una struct `Config` con i valori predefiniti in codice; il file TOML viene decodificato in modalità rigorosa.
- Precedenza: flag > variabili `FUI_*` > file > predefiniti, risolta in un'unica funzione con test per ogni livello.
- Chiavi sconosciute producono un avviso, valori non validi un errore con il nome della chiave.
- Percorsi: rispetto di `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`; in loro assenza `~/.config/fui`, `~/.local/share/fui` e `~/.local/state/fui` su Linux e macOS, come indicato nelle FS.
- `modules.enabled` è una lista di id validata contro il registro dei moduli; le dipendenze mancanti sono un errore, non vengono aggiunte in silenzio.
- `theme` accetta `auto` (predefinito), `latte`, `frappe`, `macchiato`, `mocha`, `terminal` o `high-contrast`; `theme_light` (predefinito `latte`) e `theme_dark` (predefinito `mocha`) scelgono i flavor usati da `auto`. Un valore sconosciuto è un errore con l'elenco dei temi validi.

**Log**

- `log/slog` su file in `~/.local/state/fui/fui.log`, livello `warn` di default, `debug` con `--verbose` o `FUI_LOG=debug`.
- `Money`, descrizioni e beneficiari implementano `slog.LogValuer` e si registrano come `[redacted]`; solo `FUI_LOG_SENSITIVE=1` li rende in chiaro, per diagnosi locali.
- Nessun log su stdout: stdout è riservato all'output dei comandi, così gli script non si rompono.

**Sicurezza**

- Un test esegue `go list -deps ./cmd/fui` e fallisce se compaiono `net/http` o `net/rpc`; `depguard` vieta gli stessi import in `internal/`.
- `govulncheck` e `go mod verify` in CI; aggiornamenti delle dipendenze proposti automaticamente da Renovate o Dependabot, mai applicati senza CI verde.
- Input dei connettori trattato come non fidato: limiti di dimensione, validazione completa, nessuna interpolazione in SQL (solo parametri, garantito da `sqlc`).
- La skill per gli agenti è incorporata nel binario con `embed`, con la versione di FUI scritta in fase di build.

## Code convention e clean code

La base sono [Effective Go](https://go.dev/doc/effective_go) e le [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments); le regole sotto le precisano per FUI e, dove possibile, sono imposte da un linter invece che affidate alla review. Codice, identificatori, commenti e messaggi di commit sono in inglese, come l'interfaccia.

**Formattazione e lint**

- `gofumpt` e `goimports` con prefisso locale del modulo; il codice non formattato non passa la CI.
- `golangci-lint` con almeno: `staticcheck`, `revive`, `errcheck`, `errorlint`, `wrapcheck` (solo ai confini dei pacchetti), `gosec`, `gocyclo` (soglia 15), `funlen` (60 righe, 40 istruzioni), `depguard`, `forbidigo`, `exhaustive`, `nolintlint`.
- Ogni `//nolint` indica la regola e il motivo; `nolintlint` rifiuta quelli generici.

**Regole imposte da linter specifiche di FUI**

| Regola | Strumento | Ambito |
| --- | --- | --- |
| Niente `float32` o `float64` | `forbidigo` | `internal/domain`, `internal/app`, `internal/store` (eccezione: `internal/domain/simulate`) |
| Niente `time.Now()` | `forbidigo` | Ovunque tranne l'adattatore `clock` |
| Il dominio importa solo la stdlib | `depguard` | `internal/domain` |
| CLI e TUI non importano lo store | `depguard` | `internal/cli`, `internal/tui` |
| Niente `net/http` | `depguard` + test sul grafo | `internal/`, `cmd/fui` |
| `switch` esaustivi sugli enum | `exhaustive` | Ovunque |
| Niente colori letterali (`lipgloss.Color`, palette Catppuccin) fuori dai temi | `forbidigo` + `depguard` | `internal/tui` tranne `internal/tui/theme` |

**Nomi e pacchetti**

- Pacchetti brevi, minuscoli, al singolare e con un solo scopo; vietati `util`, `common`, `helpers`, `misc`.
- Interfacce piccole e definite dal consumatore (le porte stanno in `internal/app`, non accanto alle implementazioni).
- Nomi senza ripetere il pacchetto: `budget.Engine`, non `budget.BudgetEngine`.
- Errori sentinella `ErrXxx`, tipi di errore `XxxError`; acronimi in maiuscolo (`IBAN`, `ID`, `JSON`).
- File in `snake_case.go`; test accanto al codice in `_test.go`.

**Funzioni ed errori**

- `context.Context` come primo parametro di ogni funzione che fa I/O o può durare a lungo.
- Più di quattro parametri diventano una struct di opzioni; niente parametri booleani che cambiano il comportamento, si preferiscono due funzioni.
- Errori arricchiti con contesto e `%w` (`fmt.Errorf("import widiba row %d: %w", n, err)`); confronto solo con `errors.Is` e `errors.As`.
- Un errore si gestisce una volta: o si registra o si restituisce, mai entrambi.
- Niente `panic` fuori da `main` e dagli invarianti impossibili; niente `init()` con effetti collaterali; niente variabili globali mutabili.

**Principi di clean code applicati**

- **Functional core, imperative shell:** la logica sta in funzioni pure del dominio; I/O e orchestrazione negli use case e negli adattatori.
- **Responsabilità singola:** un pacchetto, un motivo per cambiare; un comando CLI, uno use case.
- **Dipendenze esplicite:** tutto passa dai costruttori e si assembla in `cmd/fui/main.go`, l'unico punto che conosce tutte le implementazioni.
- **Tipi che impediscono errori:** id tipizzati, `Money` con valuta, `Date` senza orario; uno stato non valido non deve essere rappresentabile.
- **Niente astrazioni premature:** un'interfaccia nasce quando c'è un secondo implementatore o un test che ne ha bisogno.
- **Commenti sul perché:** ogni simbolo esportato ha un commento godoc; dentro le funzioni si spiega solo ciò che il codice non dice.

**Codice generato** — l'output di `sqlc` e gli schemi JSON stanno nel repository; la CI li rigenera e fallisce se c'è differenza, così il codice generato non diverge mai dalla sorgente.

## Flusso Git

FUI usa il trunk-based development: `main` è sempre rilasciabile, i branch vivono al massimo pochi giorni e ogni commit è atomico, compila e passa i test. La storia resta lineare, così `git bisect` trova una regressione in pochi passi.

**Branch**

- `main` protetto: niente push diretti, CI verde obbligatoria, storia lineare, commit firmati (firma SSH).
- Branch di lavoro `<tipo>/<scope>-<descrizione>`, es. `feat/import-widiba-xlsx`, `fix/budget-rollover`; nati da `main` e riallineati con `rebase`, mai con merge.
- `release/vX.Y` solo quando serve correggere una versione già uscita mentre `main` è più avanti: la correzione entra prima in `main`, poi si porta con `cherry-pick -x`.
- Funzioni incomplete restano in `main` dietro un flag di configurazione `experimental.<nome>`, non in branch lunghi.

**Commit atomici**

- Un commit = un cambiamento logico, descrivibile in una riga senza "e".
- Ogni commit compila e passa `make check`; prima di aprire la PR si verifica con `git rebase -x 'make check' main`.
- Refactoring e cambi di comportamento in commit separati: il refactoring non cambia i test esistenti, il cambio di comportamento sì.
- Una migrazione di schema sta in un commit dedicato, insieme al suo test e al codice `sqlc` rigenerato.
- Un bug fix inizia con un test che riproduce il bug; test e correzione stanno nello stesso commit, così la storia non contiene commit rossi.
- Codice generato, golden file aggiornati e il cambiamento che li richiede stanno nello stesso commit.

**Messaggi: Conventional Commits** — formato `<tipo>(<scope>): <descrizione all'imperativo, minuscola, ≤ 72 caratteri>`, corpo che spiega il perché, footer con riferimenti alle issue.

| Tipo | Uso | Effetto sulla versione |
| --- | --- | --- |
| `feat` | Nuova funzione per l'utente | minor |
| `fix` | Correzione di un bug | patch |
| `perf` | Miglioramento di prestazioni senza cambi di comportamento | patch |
| `refactor` | Ristrutturazione senza cambi di comportamento | nessuno |
| `test` | Solo test | nessuno |
| `docs` | Documentazione, ADR, skill | nessuno |
| `build`, `ci`, `chore` | Build, pipeline, manutenzione | nessuno |

Scope tra `domain`, `app`, `cli`, `tui`, `store`, `import`, `connector/revolut`, `connector/widiba`, `proto`, `config`, `skill`, `module`. Un cambiamento incompatibile su output JSON, protocollo `fui/1`, configurazione o comandi usa `!` e il footer `BREAKING CHANGE:`. Il formato è controllato da un hook `commit-msg` e in CI.

**Pull request**

1. Branch aggiornato su `main`, `make check` verde in locale.
2. PR piccola, idealmente sotto le 400 righe modificate esclusi codice generato e golden file; altrimenti si divide.
3. Descrizione con cosa cambia, perché, come è stato verificato e lo use case o il requisito delle FS coinvolto.
4. CI verde su tutti i job obbligatori.
5. Revisione con checklist (sotto); da sviluppatore unico, auto-revisione del diff completo il giorno dopo per le modifiche al dominio e allo schema.
6. Merge con "Rebase and merge", che conserva i commit atomici; "Squash" solo se la PR contiene un unico cambiamento logico.

**Checklist di revisione**

- [ ] Ogni nuovo comportamento ha un test; ogni bug corretto ha un test di regressione.
- [ ] Nessuna logica di business in CLI, TUI o store.
- [ ] Golden file aggiornati solo dove l'output doveva cambiare, con diff letto riga per riga.
- [ ] Migrazioni con test e backup automatico preservato.
- [ ] Messaggi di errore chiari con codice stabile; aiuto `--help` con esempio per i comandi nuovi.
- [ ] Nessun dato personale reale in `testdata`.

**Hook locali** — gestiti con `lefthook` e versionati: `pre-commit` esegue `gofumpt` e il lint sui file toccati, `commit-msg` controlla il formato, `pre-push` esegue `make check`. Gli hook velocizzano il ciclo ma non sostituiscono la CI, che resta l'unico controllo vincolante.

**Versioni e rilasci** — SemVer con versioni `0.x` fino alla stabilità del formato JSON e del protocollo: in `0.x` un cambio incompatibile aumenta la minor. Un rilascio è un tag annotato e firmato `vX.Y.Z` su `main`; il changelog si genera dai commit con `git-cliff`, e la pipeline pubblica i binari.

## Test e prevenzione delle regressioni

Le regressioni si prevengono su tre fronti: invarianti verificate automaticamente su input generati, contratti esterni congelati in golden file, e una regola di processo per cui nessun bug viene corretto senza un test che lo riproduca. Tutto gira in CI a ogni PR, i controlli più lenti ogni notte.

**Livelli di test**

| Livello | Strumento | Cosa copre | Quando |
| --- | --- | --- | --- |
| Unit del dominio | `testing` table-driven, `go-cmp` | Tipi valore, saldi, budget, regole, ammortamento, autonomia | Ogni PR |
| Property-based | `rapid` | Invarianti su input generati (elenco sotto) | Ogni PR; più iterazioni di notte |
| Fuzzing | Fuzzing nativo di Go | Parser di importi e date, profili CSV, decoder `fui/1`, parser Revolut e Widiba | 30 s per target a ogni PR; 30 min di notte |
| Integrazione store | SQLite reale in directory temporanea | Repository, query `sqlc`, vincoli dello schema | Ogni PR |
| Migrazioni | Archivi di partenza in `testdata` | Aggiornamento da ogni versione rilasciata all'attuale | Ogni PR |
| CLI end-to-end | `testscript` (file `.txtar`) | Comandi reali su archivi temporanei: stdout, stderr, codici di uscita; almeno uno script per ogni UC-01…UC-10 | Ogni PR |
| Golden file | Confronto con `-update` esplicito | Output JSON e CSV, `help --json`, report, stream dei connettori, schema SQL | Ogni PR |
| TUI | `teatest` a 80×24 + test di `Update` | Snapshot delle schermate, sequenze di tasti, stato del modello | Ogni PR |
| Conformità connettori | `fuiproto/conformance` | Protocollo, id stabili, codici di uscita | Ogni PR |
| Prestazioni | `go test -bench` + `benchstat` | Obiettivi non funzionali su un archivio sintetico di 50.000 transazioni | Di notte; confronto con `main` sulle PR che toccano store e import |
| Mutation testing | `gremlins` | Qualità dei test del dominio | Settimanale |

**Invarianti verificate con test property-based**

- La somma delle righe di uno split è uguale all'importo della transazione.
- I due lati di un trasferimento sommano a meno la commissione; il patrimonio netto non cambia per un trasferimento senza commissione.
- Saldo a una data = saldo iniziale + somma dei movimenti fino a quella data, qualunque sia l'ordine di inserimento.
- Importare due volte lo stesso file non crea nuove transazioni (idempotenza della deduplicazione).
- Export in formato FUI e reimport in un archivio vuoto producono gli stessi conti, movimenti, categorie e tag (round-trip).
- `import undo` riporta l'archivio allo stato precedente all'import.
- In un piano di ammortamento la somma delle quote capitale è uguale al capitale e il debito residuo finale è zero.
- Con le buste, totale delle buste più "Da assegnare" = somma dei saldi dei conti di liquidità inclusi.
- Formattare e poi analizzare un importo o una data restituisce lo stesso valore, per ogni locale supportato.
- Disattivare e riattivare un modulo lascia l'archivio identico byte per byte.
- Per ogni `TxFilter` generato, la query SQL restituisce esattamente le transazioni per cui `Match(tx)` in memoria è vero, nello stesso ordine.
- `fui reset --keep-settings` conserva categorie, tag, regole, profili e associazioni dei connettori, e lascia vuote tutte le altre tabelle.
- L'attivazione di un modulo produce sempre un insieme chiuso rispetto alle dipendenze; nessuna sequenza di attivazioni e disattivazioni lascia una dipendenza mancante.
- Per ogni tema, ogni token di testo (`Text`, `Muted`, `Income`, `Expense`, `Warning`, `Over`, `Pending`, `Reconciled`) ha un contrasto WCAG di almeno 4,5:1 con `Base`, e di almeno 7:1 nel tema `high-contrast`; un token che non rispetta la soglia fa fallire il test.
- Ogni tema definisce tutti i token: aggiungere un token senza mapparlo in ogni flavor fa fallire i test.

**Matrice dei moduli** — gli script `testscript` girano con i preset `expense` e `full` e con ogni modulo opzionale spento uno alla volta; un golden file di `fui help --json` per preset verifica che i comandi compaiano e spariscano come previsto. Le prestazioni della ricerca (< 200 ms su 100.000 transazioni) sono un benchmark dedicato tra quelli notturni.

**Temi** — gli snapshot `teatest` della Dashboard e della lista Transazioni si generano per ogni tema (Latte, Frappé, Macchiato, Mocha, `terminal`, `high-contrast`) e con `NO_COLOR`. Il rilevamento dello sfondo è testato con risposte OSC 11 simulate (chiaro, scuro, nessuna risposta entro il timeout) e con `COLORFGBG`.

**Golden file come contratto** — output JSON, `help --json`, stream dei connettori e schema SQL sono i contratti verso script, agenti e archivi esistenti. Ogni modifica a un golden file compare nel diff della PR e va motivata nel messaggio di commit; un golden cambiato per un `fix` o un `refactor` è un segnale d'allarme in revisione.

**Determinismo** — orologio iniettato (`clock.Fixed` nei test), nessuna dipendenza da ordine delle mappe, seed fissi per i generatori. La CI esegue la suite con `TZ=UTC` e con `TZ=Pacific/Kiritimati` (UTC+14) per scovare errori sulle date, e con i locale `it-IT` e `en-US`.

**Regole di copertura** — copertura minima dell'80% sul totale e del 90% su `internal/domain`; le righe modificate in una PR devono essere coperte almeno all'80%. La soglia è un minimo, non un obiettivo: conta quello che verificano le asserzioni, controllato dal mutation testing.

**Regola dei bug** — ogni bug segnalato diventa prima un test rosso (unit, property o `testscript`), poi la correzione. Gli input che il fuzzing trova problematici restano nel corpus in `testdata/fuzz` per sempre.

**Dati di test** — generatori di archivi sintetici in `internal/testutil/fixture` (conti, anni di movimenti, ricorrenze, finanziamenti) con seed fisso; gli export bancari reali entrano nel repository solo dopo l'anonimizzazione con lo script `tools/anonymize`.

## CI/CD e distribuzione

La pipeline gira su GitHub Actions ed esegue gli stessi target del `Makefile` usati in locale: se `make check` passa sul portatile, passa anche in CI. Un tag `vX.Y.Z` su `main` produce un rilascio completo senza passi manuali.

**Target del Makefile**

| Target | Cosa fa |
| --- | --- |
| `make fmt` | `gofumpt` e `goimports` su tutto il codice |
| `make lint` | `golangci-lint run` |
| `make generate` | `sqlc generate`, schemi JSON, golden di `help --json` |
| `make test` | Unit, integrazione, property, `testscript`, TUI, con `-race` |
| `make check` | `fmt` in modalità verifica, `generate` senza differenze, `lint`, `test` |
| `make bench` | Benchmark degli obiettivi non funzionali |
| `make build` | Binari `fui` e connettori in `dist/` con versione incorporata |

**Job sulle pull request** (tutti obbligatori)

| Job | Contenuto |
| --- | --- |
| `lint` | Formattazione, `go mod tidy` senza differenze, codice generato aggiornato, `golangci-lint`, formato dei commit |
| `test` | `make test` su `ubuntu-latest` e `macos-latest`, con le ultime due versioni di Go e due fusi orari |
| `e2e` | `testscript` sul binario compilato e suite di conformità dei connettori |
| `fuzz-short` | 30 s per ogni target di fuzzing |
| `security` | `govulncheck`, `go mod verify`, test sull'assenza di `net/http` |
| `build` | Compilazione per tutte le piattaforme di rilascio e per Windows, quest'ultima solo come segnale non vincolante |
| `coverage` | Soglie complessiva, del dominio e delle righe modificate |

**Job notturni** — fuzzing lungo, property test con più iterazioni, benchmark con confronto rispetto all'ultimo rilascio (fallimento se un obiettivo non funzionale non è rispettato), mutation testing settimanale. Un job notturno fallito apre automaticamente una issue.

**Rilascio**

1. Tag annotato e firmato `vX.Y.Z` su un commit di `main` con CI verde.
2. `goreleaser` compila `fui` e i connettori con `CGO_ENABLED=0`, `-trimpath` e versione, commit e data incorporati, per `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`.
3. Pubblica archivi, `checksums.txt`, SBOM e firme `cosign`, più il changelog generato da `git-cliff`.
4. Aggiorna il tap Homebrew; i completamenti di shell e la pagina di manuale sono inclusi negli archivi.

Il binario riporta la propria versione con `fui --version`, la stessa scritta nella skill per gli agenti.

## Decisioni tecniche e punti aperti

Le decisioni architetturali si registrano come ADR in `docs/adr/NNNN-titolo.md` (formato MADR: contesto, opzioni, decisione, conseguenze); un ADR non si modifica, si sostituisce con uno nuovo che lo dichiara superato. Le prime da scrivere sono quelle già prese in questo documento.

| ADR | Decisione |
| --- | --- |
| 0001 | Architettura esagonale con dominio puro e regole di dipendenza imposte da `depguard` |
| 0002 | Importi `int64` in unità minime, tassi a 10^9, quantità con scala per titolo, arrotondamento metà lontano dallo zero in un solo punto |
| 0003 | `sqlc` al posto di un ORM; migrazioni `goose` solo in avanti con backup automatico |
| 0004 | Pattern Plan/Apply per tutte le scritture, base di `--dry-run`, anteprime TUI e sicurezza degli agenti |
| 0005 | Protocollo `fui/1` in `pkg/fuiproto` come unico pacchetto pubblico |
| 0006 | Trunk-based development, Conventional Commits, merge con rebase |
| 0007 | Golden file come contratto per output JSON, `help --json`, stream dei connettori e schema |
| 0008 | Percorsi XDG anche su macOS per coerenza con le FS |
| 0009 | Moduli come registro statico che agisce su esposizione e query; schema sempre completo, nessuna migrazione legata ai moduli |
| 0010 | Ricerca testuale con FTS5 trigram su una colonna normalizzata in Go; un unico `TxFilter` per CLI, TUI, export e report |
| 0011 | Reset come ricostruzione in file temporaneo con rename atomico e backup escluso dalla rotazione |
| 0012 | Temi della TUI dalla palette Catppuccin tramite token semantici; tema `auto` con rilevamento dello sfondo (OSC 11, poi `COLORFGBG`, poi scuro) |

**Da decidere**

- [ ] Confermare la politica di arrotondamento per le conversioni valutarie (proposta: metà lontano dallo zero, una volta per transazione).
- [ ] Scegliere la major di Bubble Tea da adottare all'avvio e il criterio per gli aggiornamenti delle major dell'ecosistema Charm.
- [ ] Decidere se firmare e notarizzare i binari macOS, necessario per evitare l'avviso di Gatekeeper fuori da Homebrew.
- [ ] Definire l'integrazione del modello AI locale (runtime, processo separato o libreria), oggi indicata come da definire nelle FS.
- [ ] Valutare se aggiungere un isolamento dei connettori esterni (es. permessi ridotti del processo) dopo l'MVP.
- [ ] Fissare la soglia di regressione dei benchmark che blocca una PR (proposta: 10% su `add` e import), dopo aver misurato il rumore dei runner di CI.
- [ ] Verificare che il build di `modernc.org/sqlite` in uso includa FTS5 con il tokenizer trigram; in caso contrario ripiegare su `LIKE` sulla colonna normalizzata e misurare i 200 ms.
- [ ] Confermare i nomi dei preset (`expense`, `full`, `custom`) e degli id dei moduli, che diventano parte del contratto della CLI.
- [ ] Aggiornare le FS: la chiave `theme` ha ora valore predefinito `auto` con i flavor Catppuccin, mentre le FS 1.2 indicano che il tema segue i colori del terminale.
- [ ] Verificare come la major di Bubble Tea scelta espone il colore di sfondo e la notifica del cambio di tema, che determinano l'implementazione del rilevamento.

## Cronologia delle versioni

**Versione 1.2** — 12 novembre 2026, temi della TUI basati su Catppuccin

| Sezione | Modifica |
| --- | --- |
| Stack e dipendenze | Nuova dipendenza `catppuccin/go` per la palette |
| TUI | Temi Latte, Frappé, Macchiato e Mocha; tema `auto` con rilevamento dello sfondo del terminale; temi `terminal` e `high-contrast`; token semantici; scelta del tema dalle Impostazioni con anteprima |
| Configurazione, log e sicurezza | Chiavi `theme`, `theme_light` e `theme_dark` |
| Code convention e clean code | Divieto di colori letterali fuori da `internal/tui/theme` |
| Test e prevenzione delle regressioni | Test di contrasto WCAG per ogni tema, completezza dei token, snapshot per tema e test del rilevamento dello sfondo |
| Decisioni tecniche e punti aperti | ADR 0012; punti aperti su aggiornamento delle FS e rilevamento con la major di Bubble Tea |

**Versione 1.1** — 12 novembre 2026, allineata alle FS 1.2 (moduli, beni, ricerca con filtri, reset dell'archivio)

| Sezione | Modifica |
| --- | --- |
| Panoramica e principi tecnici | Nuovo principio di modularità; obiettivo di ricerca con filtri < 200 ms su 100.000 transazioni |
| Stack e dipendenze | Ricerca testuale con FTS5 trigram e `golang.org/x/text` per la normalizzazione |
| Architettura | Pacchetto `internal/module` nel layout del repository e nelle regole di dipendenza |
| Moduli | Nuova sezione: registro statico, dipendenze, preset, effetti della disattivazione su CLI, TUI, dati, import e agenti |
| Modello dati e schema SQLite | Tabelle `assets`, `asset_valuations` e `transactions_fts`; colonna `search_text` normalizzata; scala della quantità per titolo al posto di 10^8 fisso; schema indipendente dai moduli attivi |
| Core di dominio | Nuovi tipi `Quantity` e `TxFilter`; servizi di filtro delle transazioni (con oracolo in memoria) e patrimonio netto con i beni |
| CLI | Flag di filtro condivisi da `tx list`, `tx search` ed `export`; `fui reset` con conferma digitata o `--yes`; comandi `module` e `asset`; errore `module_disabled` |
| TUI | Ricerca live con annullamento della query precedente, pannello filtri, gestione dei moduli dalle Impostazioni senza riavvio |
| Connettori e protocollo fui/1 | L'anteprima segnala i record dei moduli disattivati e chiede se attivarli o saltarli |
| Persistenza, migrazioni e backup | Reset come ricostruzione in file temporaneo con rename atomico; backup `reset` escluso dalla rotazione |
| Configurazione, log e sicurezza | Chiave `modules.enabled` validata contro il registro dei moduli |
| Flusso Git | Nuovo scope `module` |
| Test e prevenzione delle regressioni | Quattro nuovi invarianti (moduli, filtro SQL contro oracolo, reset, dipendenze); matrice dei moduli; benchmark della ricerca |
| Decisioni tecniche e punti aperti | ADR 0009–0011; ADR 0002 aggiornato per la scala delle quantità; due nuovi punti aperti (FTS5 in `modernc.org/sqlite`, nomi di preset e moduli) |

**Versione 1.0** — 11 novembre 2026: prima versione delle specifiche tecniche, basata sulle FS 1.0.
