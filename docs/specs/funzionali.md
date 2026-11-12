# FUI — Specifiche funzionali

Nov 9, 2026 · @Mattia Ballo

**Versione 1.2** — 12 novembre 2026 · le modifiche rispetto alle versioni precedenti sono nella sezione Cronologia delle versioni

## Panoramica e obiettivi

FUI è un gestore di finanze personali da terminale, con una CLI scriptabile e una TUI interattiva, che conserva tutti i dati in locale senza alcun servizio cloud.

**Obiettivi**

- Registrare entrate, uscite e trasferimenti in pochi secondi, senza lasciare il terminale.
- Conoscere in ogni momento il saldo di ogni conto e il patrimonio netto.
- Controllare la spesa per categoria rispetto a un budget mensile.
- Importare gli estratti conto bancari senza ridigitare i movimenti.
- Garantire che nessun dato finanziario lasci la macchina.

**Principi guida**

- **Local-first:** un solo archivio sul disco dell'utente, nessuna connessione di rete necessaria.
- **Un core, due interfacce:** CLI e TUI usano la stessa logica di dominio e lo stesso archivio; nessuna funzione esiste solo in una delle due, salvo la navigazione interattiva.
- **Scriptabile:** ogni operazione è disponibile da CLI, con output leggibile da macchina (JSON, CSV).
- **Importi esatti:** nessun numero in virgola mobile; gli importi sono interi nell'unità minima della valuta (centesimi).
- **Tastiera prima di tutto:** ogni azione della TUI è raggiungibile senza mouse.

## Moduli

FUI è modulare: il nucleo è sempre attivo e ogni altra area si attiva solo se serve, così chi vuole un semplice expense tracker non vede investimenti, mutui o simulazioni.

| Modulo | Cosa attiva | Attivo di default | Dipende da |
| --- | --- | --- | --- |
| Nucleo | Conti di liquidità e carte, transazioni, trasferimenti, categorie, tag, regole, import ed export, backup, ricerca, report spese e flusso di cassa, KPI di autonomia | sempre | — |
| Budget | Tetti o buste, schermata Budget, report budget vs effettivo | sì | — |
| Ricorrenze | Ricorrenze e previsione di cassa | sì | — |
| Mutui e prestiti | Finanziamenti, piani di ammortamento, eventi, report debito residuo e interessi pagati | no | Ricorrenze |
| Investimenti | Conti di investimento, titoli, operazioni e posizioni | no | — |
| Crypto | Wallet e cripto-attività, con quantità frazionarie | no | Investimenti |
| Beni | Immobili, veicoli e altri beni con valutazioni manuali, collegabili a un mutuo | no | — |
| Analisi | Analisi automatiche e report salvati | sì | — |
| Simulazioni | Rendimento investimenti, PAC e flussi di cassa | no | Ricorrenze |
| AI | Funzioni AI con modello locale, inclusa la simulazione Monte Carlo | no | Simulazioni |

**Attivazione**

- Al primo avvio `fui init` chiede un profilo: Expense tracker (nucleo, budget, ricorrenze), Completo (tutti i moduli) o Personalizzato; da CLI si sceglie con `--preset` o `--modules`.
- In seguito i moduli si attivano e disattivano con `fui module enable` e `fui module disable` o dalla schermata Impostazioni; la scelta è salvata nella chiave `modules.enabled`.
- Attivare un modulo attiva anche quelli da cui dipende, con un avviso; disattivarlo chiede conferma se altri moduli attivi ne dipendono.

**Effetti di un modulo disattivato**

- I suoi comandi spariscono dall'aiuto e da `fui help --json`; se invocati falliscono con un messaggio che spiega come attivarlo.
- Le sue schermate e i suoi riquadri spariscono dalla TUI; i tasti delle altre schermate non cambiano.
- I dati non vengono cancellati: restano nell'archivio, esclusi da Dashboard, report e patrimonio netto, e ricompaiono riattivando il modulo.
- Se un import contiene record di un modulo disattivato (es. `schedule` e `installment` senza Mutui e prestiti), l'anteprima li segnala e chiede se attivare il modulo o saltarli.
- La skill per agenti legge i moduli attivi con `fui module list --json` e propone solo comandi disponibili.

## Modello di dominio

Il dominio ruota attorno a conti e transazioni; categorie, tag, budget, ricorrenze e regole servono a classificare e prevedere i movimenti.

| Entità | Descrizione | Attributi principali |
| --- | --- | --- |
| Conto | Contenitore di denaro con un saldo | nome, descrizione (opzionale), tipo (contanti, conto corrente, carta di credito, risparmio, investimento, finanziamento), valuta, saldo iniziale, data di apertura, archiviato |
| Transazione | Singolo movimento su un conto | data, importo con segno, conto, beneficiario, categoria, tag, note, stato (in sospeso, registrata, riconciliata), id di import |
| Trasferimento | Spostamento tra due conti dell'utente | due movimenti collegati di segno opposto, eventuale commissione |
| Split | Ripartizione di una transazione su più categorie | righe con categoria e importo; la somma eguaglia il totale |
| Categoria | Classificazione di entrate o uscite | nome, un solo livello, nessuna sottocategoria (es. Casa); la classificazione di dettaglio si fa con i tag, tipo entrata/uscita |
| Tag | Etichetta libera trasversale alle categorie | nome; relazione molti-a-molti con le transazioni |
| Beneficiario | Controparte normalizzata | nome, alias usati per riconoscerlo negli import |
| Budget | Tetto di spesa o busta su categorie o tag | modalità (tetto o busta), categorie o tag coperti, periodo (mese o anno), importo con eventuali variazioni per mese, assegnazioni e spostamenti (solo buste), riporto di avanzo e sforamento |
| Ricorrenza | Modello di transazione periodica | transazione modello, regola (ogni N giorni/settimane/mesi, giorno del mese), prossima data, registrazione automatica in stato in sospeso, confermata dall'utente o da un import |
| Regola | Classificazione automatica | condizioni su descrizione, importo, conto; azioni: imposta categoria, beneficiario, tag |
| Valuta | Valuta di un conto | codice ISO 4217, tassi di cambio manuali per data |
| Titolo | Strumento finanziario detenuto in un conto di investimento | nome, ticker o ISIN, tipo (azione, ETF, obbligazione, fondo, cripto), valuta, precisione della quantità (es. 8 decimali per le cripto, salvata come intero), prezzi manuali per data |
| Operazione su titolo | Acquisto, vendita o provento; la posizione è la somma delle operazioni e vale quantità × ultimo prezzo noto | data, conto, titolo, tipo (acquisto, vendita, dividendo o cedola), quantità, prezzo unitario, commissioni, movimento di liquidità collegato |
| Bene | Bene non finanziario dell'utente; entra nel patrimonio netto all'ultima valutazione | nome, tipo (immobile, veicolo, altro), data e valore di acquisto, valutazioni manuali per data, finanziamento collegato (opzionale) |
| Finanziamento | Mutuo o prestito, gestito come conto passività con saldo pari al debito residuo | tipo (mutuo, prestito), capitale erogato, data di erogazione, durata, tipo di tasso (fisso, variabile, misto), conto di addebito delle rate |
| Piano di ammortamento | Versione del piano delle rate, valida da una data | data di validità, origine (importato o calcolato), tasso; rate con scadenza, quota capitale, quota interessi, spese, debito residuo |
| Evento di finanziamento | Fatto che cambia il piano | tipo (variazione tasso, rinegoziazione, estinzione parziale), data, nuovi valori (tasso, rata, durata, capitale rimborsato) |

**Regole di business**

- Saldo di un conto = saldo iniziale + somma dei movimenti fino alla data richiesta.
- Importo negativo = uscita, positivo = entrata; la CLI accetta anche il tipo esplicito.
- I trasferimenti e i movimenti di investimento (acquisti, vendite e rimborsi di titoli o fondi) non compaiono come entrate o uscite nei report e nei budget.
- Una categoria con transazioni può essere eliminata solo riassegnandole o unendola a un'altra.
- Un conto archiviato sparisce dalle viste correnti ma resta nei report storici.
- Le date non hanno orario e seguono il fuso locale.

## Casi d'uso

L'unico attore è l'utente proprietario dei dati; i dieci casi d'uso sotto definiscono cosa FUI deve permettere di fare.

| ID | Caso d'uso | Interfaccia prevalente | Fase |
| --- | --- | --- | --- |
| UC-01 | Registrare una spesa al volo con un solo comando | CLI | MVP |
| UC-02 | Rivedere e categorizzare i movimenti del mese | TUI | MVP |
| UC-03 | Importare l'estratto conto CSV/XLSX della banca evitando duplicati | CLI e TUI | MVP |
| UC-04 | Vedere il saldo di tutti i conti e il patrimonio netto | CLI e TUI | MVP |
| UC-05 | Analizzare le spese per categoria e periodo ed esportarle | CLI e TUI | MVP |
| UC-06 | Fare backup e ripristino dell'archivio | CLI | MVP |
| UC-07 | Impostare e monitorare budget mensili per categoria o totali | TUI | v0.2 |
| UC-08 | Gestire spese ricorrenti (affitto, abbonamenti, stipendio, mutuo, etc) | CLI e TUI | v0.2 |
| UC-09 | Riconciliare un conto con il saldo dell'estratto | TUI | v0.2 |
| UC-10 | Usare FUI in script e pipeline (cron, jq, altri tool) | CLI | MVP |

## Requisiti funzionali — CLI

Il binario si chiama `fui`: senza argomenti apre la TUI, con un sottocomando esegue l'operazione ed esce.

| Comando | Scopo | Esempio |
| --- | --- | --- |
| `fui init` | Crea l'archivio e le categorie predefinite e sceglie i moduli | `fui init --currency EUR --preset expense` |
| `fui module list / enable / disable` | Mostra e cambia i moduli attivi | `fui module enable investments` |
| `fui add` | Aggiunge una transazione; data predefinita oggi | `fui add -12,50 Esselunga -c Spesa -a Contanti` |
| `fui tx list / search / show / edit / rm` | Elenca, cerca e modifica transazioni con filtri | `fui tx search amazon --from 2026-01-01 --min 50` |
| `fui transfer` | Sposta denaro tra due conti | `fui transfer 500 --from Fineco --to Risparmio` |
| `fui account add / list / edit / archive` | Gestisce i conti e mostra i saldi | `fui account list --json` |
| `fui category add / list / rename / merge / rm` | Gestisce le categorie | `fui category merge Bar Ristoranti` |
| `fui tag add / list / rm` | Gestisce i tag | `fui tag add vacanze-2026` |
| `fui budget set / list / status / assign / move` | Imposta e controlla i budget; con le buste assegna e sposta importi | `fui budget set Spesa 400 --month 2026-11` |
| `fui recurring add / list / run / skip / confirm` | Gestisce le ricorrenze | `fui recurring run --until today` |
| `fui loan add / show / schedule / event` | Gestisce mutui e prestiti, i piani di ammortamento e i loro eventi | `fui loan event Mutuo --rate 3,15 --from 2027-01-01` |
| `fui asset add / list / value` | Gestisce i beni e le loro valutazioni | `fui asset value Casa 320000 --date 2026-12-31` |
| `fui rule add / list / test / apply` | Gestisce le regole di categorizzazione | `fui rule test "AMAZON"` |
| `fui connector list / show / test` | Elenca i connettori installati, ne mostra i parametri e ne prova l'output senza importare | `fui connector test fineco-pdf estratto.pdf` |
| `fui import` | Importa movimenti tramite un connettore | `fui import ott.csv --connector csv --profile fineco --dry-run` |
| `fui export` | Esporta transazioni in CSV, JSON o formato FUI, oppure l'intero archivio | `fui export --format json --all` |
| `fui report run / save / list` | Esegue un report predefinito o salvato, ne salva uno nuovo | `fui report run categories --month 2026-10` |
| `fui insights` | Mostra le segnalazioni delle analisi automatiche | `fui insights --json` |
| `fui forecast` | Mostra la previsione del flusso di cassa | `fui forecast --horizon 12m` |
| `fui kpi` | Mostra i KPI, a partire dall'autonomia | `fui kpi runway --json` |
| `fui simulate investment / pac / cashflow / save / list / compare` | Calcola, salva e confronta scenari ipotetici senza toccare i dati | `fui simulate pac --monthly 200 --years 20 --rate 4,5` |
| `fui reconcile` | Riconcilia un conto con un saldo noto | `fui reconcile Fineco --balance 2340,18 --date 2026-10-31` |
| `fui backup / restore` | Copia e ripristina l'archivio | `fui backup` |
| `fui reset` | Azzera l'archivio e lo riporta allo stato iniziale, dopo un backup | `fui reset --keep-settings` |
| `fui skill install / show` | Installa o mostra la skill per gli agenti AI | `fui skill install --target ~/.claude/skills` |
| `fui help` | Mostra l'aiuto; con `--json` il riferimento completo per gli agenti | `fui help --json` |
| `fui config get / set` | Legge e modifica la configurazione | `fui config set locale it-IT` |

**Requisiti trasversali**

- **Output:** tabella leggibile di default; `--json` e `--csv` producono output stabile per gli script; colori disattivati se l'output non è un terminale o se è impostato `NO_COLOR`.
- **Codici di uscita:** 0 successo, 1 errore di input dell'utente, 2 errore di dati o di I/O.
- **Input interattivo:** se mancano campi obbligatori e c'è un terminale, FUI li chiede; con `--no-input` o senza terminale fallisce con un messaggio chiaro.
- **Riferimenti per nome:** conti, categorie e tag si indicano per nome con corrispondenza parziale; se il nome è ambiguo, FUI elenca le opzioni ed esce con errore.
- **Importi e date:** accetta virgola o punto decimale secondo il locale; date assolute (`2026-11-09`) e relative (`today`, `yesterday`, `-3d`).
- **Anteprima:** `--dry-run` su import, applicazione regole, ricorrenze e modifiche di massa.
- **Aiuto:** `--help` su ogni comando, con almeno un esempio; completamento per bash, zsh e fish.

**Ricerca e filtri delle transazioni** — `fui tx list` elenca le transazioni con filtri, `fui tx search <testo>` aggiunge la ricerca testuale; gli stessi filtri valgono per `fui export` e per la schermata Transazioni della TUI.

| Filtro | Opzioni CLI | Esempio |
| --- | --- | --- |
| Periodo | `--from`, `--to`, `--month`, `--year` | `--month 2026-10` |
| Conti | `-a`, `--account` (ripetibile) | `-a Widiba -a Revolut` |
| Categorie | `-c`, `--category` (ripetibile), `--uncategorized` | `--uncategorized` |
| Tag | `-t`, `--tag` (ripetibile) | `-t vacanze-2026` |
| Beneficiario | `--payee` | `--payee Esselunga` |
| Importo | `--min`, `--max` sul valore assoluto; `--income` o `--expense` | `--min 50 --expense` |
| Stato | `--status` (pending, booked, reconciled) | `--status pending` |
| Natura | `--kind` (payment, transfer, investment…) | `--kind transfer` |
| Lotto di import | `--batch` | `--batch 2026-11-10-01` |
| Testo | argomento di `fui tx search` | `fui tx search "amazon prime"` |

- Filtri diversi si combinano in AND; lo stesso filtro ripetuto vale come OR (es. due conti).
- La ricerca testuale guarda descrizione, beneficiario e note, senza distinguere maiuscole, minuscole e accenti, con corrispondenza parziale.
- I risultati si ordinano con `--sort` (date, amount, payee) e `--desc`, si limitano con `--limit` e mostrano in fondo il numero di righe e la somma degli importi.
- Nella TUI `/` avvia la ricerca testuale con risultati aggiornati mentre si digita e `f` apre il pannello con gli stessi filtri; i filtri attivi restano visibili in testa alla lista.
- Su un archivio di 100.000 transazioni una ricerca con filtri risponde in meno di 200 ms.

**Reset dell'archivio** — `fui reset` riporta l'archivio allo stato iniziale, come subito dopo `fui init`.

- Prima di azzerare, FUI crea sempre un backup, escluso dalla rotazione automatica finché l'utente non lo cancella.
- In un terminale FUI chiede di digitare `reset` per confermare; senza terminale serve `--yes`.
- Senza opzioni cancella tutto e ricrea le categorie predefinite; con `--keep-settings` conserva categorie, tag, regole, profili e associazioni dei connettori, e cancella conti, transazioni, budget, ricorrenze, finanziamenti, investimenti e simulazioni.
- Il file di configurazione non viene toccato.
- Se l'archivio non esiste, `fui reset` si comporta come `fui init`.

## Uso con agenti AI

La versione free include una skill che insegna agli agenti AI (Claude Code e altri compatibili con il formato skill) a usare FUI tramite la CLI: l'utente può chiedere in linguaggio naturale di importare un estratto, categorizzare i movimenti o preparare un report, e l'agente esegue i comandi `fui`.

**La skill**

- Una cartella con un file `SKILL.md`: quando usarla, riferimento dei comandi, formato dell'output JSON, flussi tipici (import mensile, revisione dei movimenti non categorizzati, stato dei budget, report) e regole di sicurezza.
- Viene distribuita dentro il binario e installata con `fui skill install`, che la scrive nella directory delle skill dell'agente (es. `~/.claude/skills/fui/`) o in un percorso indicato; con `fui skill show` si legge senza installarla.
- La versione della skill coincide con quella di FUI, così i comandi descritti corrispondono sempre al binario installato.

**Requisiti della CLI per gli agenti**

- `fui help --json` restituisce l'elenco completo di comandi, opzioni e schemi di output in forma leggibile da macchina.
- Con `--json` anche gli errori sono JSON su stderr, con codice e messaggio.
- Con `FUI_AGENT=1` FUI non chiede mai input, usa l'output JSON e rifiuta le scritture che non abbiano `--yes`.

**Regole di sicurezza nella skill**

1. Prima di ogni scrittura l'agente esegue il comando con `--dry-run` e mostra all'utente cosa cambierebbe.
2. Scrive solo dopo la conferma dell'utente, aggiungendo `--yes`.
3. Non cancella dati, non azzera l'archivio, non esegue `restore` e non modifica transazioni riconciliate senza una richiesta esplicita.
4. Preferisce dati aggregati (report, totali) ai movimenti riga per riga, per limitare quanti dati finiscono nel contesto del modello.

Se l'agente usa un modello remoto, i dati che legge escono dalla macchina: è una scelta dell'utente, coerente con il principio local-first perché FUI di suo non apre connessioni.

## Requisiti funzionali — TUI

La TUI si apre sulla Dashboard e offre sette schermate raggiungibili con un tasto; ogni modifica viene salvata subito, senza un comando "salva".

| Tasto | Schermata | Contenuto | Azioni principali |
| --- | --- | --- | --- |
| 1 | Dashboard | Saldi dei conti, patrimonio netto, autonomia in mesi, spese del mese contro budget, prossime ricorrenze | Aggiunta rapida, salto alla schermata di dettaglio |
| 2 | Transazioni | Lista filtrabile e ordinabile (data, conto, beneficiario, categoria, importo, stato) | Aggiungi, modifica, elimina, split, categorizzazione multipla, ricerca testuale e filtri |
| 3 | Conti | Elenco conti con saldo; dettaglio movimenti per conto | Aggiungi, archivia, riconcilia |
| 4 | Budget | Per tetto o busta: importo, speso, previsto, residuo con riporto, barra di avanzamento; con le buste anche il saldo Da assegnare | Imposta importo, mese precedente e successivo, copia dal mese prima, assegna e sposta tra buste |
| 5 | Report | Report predefiniti e salvati con grafici a barre testuali e tabelle, previsione di cassa | Cambia report e periodo, esporta |
| 6 | Import | Anteprima del file, mappatura colonne, duplicati evidenziati | Scegli connettore e profilo, includi o escludi righe, conferma |
| 7 | Impostazioni | Moduli attivi, categorie, tag, regole, ricorrenze, profili di import | CRUD su ogni elemento |

**Scorciatoie globali**

| Tasto | Azione |
| --- | --- |
| `j` / `k` o frecce | Muovi la selezione |
| `/` | Cerca nella vista corrente |
| `f` | Apri i filtri |
| `a` | Aggiungi (in Transazioni) o aggiunta rapida (ovunque) |
| `e` / `Invio` | Modifica o apri il dettaglio |
| `d` | Elimina, con conferma |
| `Spazio` | Seleziona per azioni multiple |
| `Esc` | Torna indietro o chiudi il pannello |
| `?` | Mostra le scorciatoie della schermata |
| `q` | Esci |

**Requisiti**

- Funziona in un terminale di almeno 80×24 caratteri e si adatta al ridimensionamento.
- Il mouse è opzionale; nessuna azione lo richiede.
- I form di inserimento offrono autocompletamento per conto, categoria, beneficiario e tag.
- Le liste restano fluide fino a 100.000 transazioni (caricamento a pagine o virtualizzato).
- Nessuna informazione è trasmessa solo dal colore: le uscite hanno anche il segno meno, i budget sforati anche un'etichetta.
- Il tema segue i colori del terminale; è disponibile una variante ad alto contrasto.

## Import, export e riconciliazione

Ogni import passa da un connettore: FUI ne include alcuni e permette di aggiungerne di propri; ogni import è atomico, deduplicato e annullabile.

**Connettori** — un connettore legge una sorgente di estratti conto e restituisce i movimenti in un formato comune; da lì in poi il flusso di import è identico per tutti.

| Tipo | Come funziona | Esempi |
| --- | --- | --- |
| Integrato | Compilato in FUI e configurato con un profilo | CSV (MVP), OFX, CAMT.053 |
| Esterno | Eseguibile separato, in qualsiasi linguaggio, che FUI avvia e con cui dialoga in JSON su stdin/stdout | Uno script che estrae i movimenti dal PDF della banca, l'export di un'altra app |

**Contratto dei connettori esterni**

- **Rilevamento:** eseguibili chiamati `fui-connector-<nome>` nella directory `~/.config/fui/connectors/` o nel `PATH`.
- **Protocollo:** il connettore parla il formato FUI descritto nella sezione dedicata; `describe` restituisce i metadati, `fetch` restituisce i movimenti.
- **Esito:** un codice di uscita diverso da 0, o un output senza record di chiusura, annulla l'import senza scrivere nulla; le righe non valide sono segnalate ed escluse dall'anteprima.
- **Isolamento:** il connettore non accede all'archivio; riceve solo la propria configurazione e restituisce solo movimenti.
- **Rete:** FUI non apre connessioni; un connettore esterno che lo fa è una scelta esplicita di chi lo installa.

**Banche iniziali** — Widiba e Revolut sono connettori esterni già nell'MVP: leggono l'export della banca e lo convertono nel formato FUI.

| Connettore | Input | Conversione nel formato FUI |
| --- | --- | --- |
| `fui-connector-revolut` | CSV dell'estratto Revolut (verificato su un export reale, vedi sotto) | Mappatura colonna per colonna nella tabella seguente |
| `fui-connector-widiba` | Excel "I miei movimenti conto" (verificato su un export reale, vedi sotto); Widiba offre anche PDF e Money/Quicken | Mappatura colonna per colonna e per causale nelle tabelle seguenti |

**Connettore Revolut** — l'export è un CSV in UTF-8 con intestazione in inglese, virgola come separatore, punto decimale e date ISO con ora. Non contiene IBAN né id di transazione, e i saldi sono coerenti ordinando le righe per Completed Date.

| Colonna Revolut | Campo FUI | Conversione |
| --- | --- | --- |
| Product + Currency | `account` | `revolut-<product>-<currency>`, es. `revolut-current-EUR`; un record account per ogni combinazione |
| Started Date | `operation_date` | Giorno in cui è partita l'operazione |
| Completed Date | `date` | Data contabile; determina l'ordine dei saldi |
| Description | `description`, `payee` | Per i Transfer il beneficiario è il nome dopo "Transfer from" o "Transfer to"; per i pagamenti con carta è la descrizione stessa |
| Amount | `amount` | Moltiplicato per 100 e arrotondato all'intero |
| Fee | movimento separato | Se diversa da 0 genera un movimento `fee` con id derivato da quello principale |
| Currency | `currency` | Invariata |
| State | `status` | COMPLETED → booked, PENDING → pending; REVERTED e DECLINED sono omessi |
| Balance | `balance_after` | L'ultimo saldo diventa anche un record `balance` |
| Type | `kind` | Card Payment → payment, Transfer → transfer; Card Refund → refund, Exchange → exchange, Topup → transfer da confermare su export che li contengano; altri → other |
| nessuna | `id` | sha256 di Started Date, Completed Date, Amount, Description e Balance |

- I bonifici in entrata da un altro conto dell'utente arrivano come `transfer`: durante l'import FUI cerca sull'altro conto un'uscita di pari importo entro 2 giorni e propone di collegarle come trasferimento.
- Lo stato PENDING e i tipi diversi da Card Payment e Transfer non compaiono nell'export di prova e vanno verificati quando capitano.

**Connettore Widiba** — l'export Excel ha un foglio "Movimenti conto" con un riepilogo in testa e poi la tabella dei movimenti in ordine di data decrescente. Non contiene id di transazione né saldo per riga, ma riporta IBAN, saldo iniziale e saldo finale del periodo.

| Parte del file | Contenuto | Uso nel formato FUI |
| --- | --- | --- |
| Riepilogo | Numero conto, intestatario, IBAN, saldi disponibile, contabile, iniziale e finale, filtri applicati | Record `account` con ref `widiba-<ultime 4 cifre IBAN>` e `iban`; saldo finale come record `balance`; controllo che saldo iniziale + somma degli importi = saldo finale |
| Tabella movimenti | Dalla riga di intestazione "DATA CONT." fino alla riga "Totale" | Un record `transaction` per riga |

| Colonna Widiba | Campo FUI | Conversione |
| --- | --- | --- |
| DATA CONT. | `date` | Data Excel convertita in YYYY-MM-DD |
| DATA VAL. | `value_date` | Come sopra; può precedere la data contabile |
| CAUSALE | `kind` | Vedi tabella delle causali; conservata anche in `raw` |
| DESCRIZIONE | `description`, `payee`, `counterparty_iban` | Spazi normalizzati; beneficiario dal testo dopo "A Favore" o "Ord:"; IBAN della controparte se presente |
| IMPORTO (€) | `amount` | Già con segno; moltiplicato per 100 e arrotondato; valuta EUR |
| nessuna | `id` | Codice dell'operazione presente nella descrizione (Bon. Ist., Bon. Sepa, Addebito Sdd N.); in mancanza, sha256 di date, causale, descrizione, importo e indice tra righe identiche |

| Causale | `kind` | Note |
| --- | --- | --- |
| Bonifico In Entrata | transfer | Entrata, salvo che l'ordinante sia un conto dell'utente |
| Bonifico Estero In Uscita, Pagamento Istantaneo | transfer | Se l'IBAN del beneficiario appartiene a un conto dell'utente, FUI propone di collegarlo come trasferimento |
| Addebito Diretto Sdd | payment | Il codice mandato aiuta a riconoscere le ricorrenze |
| Trasferimento A / Da Linea Libera | transfer | Spostamento automatico verso il deposito Linea Libera: il connettore genera anche il movimento opposto sul conto `widiba-linea-libera`, con lo stesso valore di `transfer` |
| Rimborso Titoli E/o Fondi Comuni | investment | Vendita di quote di fondi; in v0.3 diventa un'operazione su titolo |
| Altre causali | other | Da mappare quando compaiono |

- Le colonne si individuano dal testo dell'intestazione e non dalla posizione, perché il riepilogo sopra la tabella può cambiare altezza.
- L'export copre il periodo scelto sul sito della banca; export sovrapposti non creano duplicati grazie agli id.
- I trasferimenti tra conti propri si riconoscono confrontando `counterparty_iban` con gli IBAN dei conti dell'utente, oltre che per importo e data.

**Profilo del connettore CSV** — definisce: colonne di data, importo (unico o dare/avere separati), descrizione e id esterno; separatore, encoding, formato data, separatore decimale, righe da saltare, conto di destinazione.

**Flusso di import**

1. Lettura della sorgente con il connettore e il profilo indicati (o rilevamento guidato al primo uso).
2. Normalizzazione di date, importi e descrizioni.
3. Deduplicazione rispetto alle transazioni già presenti.
4. Applicazione delle regole di categorizzazione.
5. Anteprima con righe nuove, duplicate e dubbie.
6. Conferma e scrittura in un'unica operazione atomica, marcata con un id di lotto.

**Deduplicazione** — una riga è duplicata se coincide l'id esterno oppure l'impronta (conto, data, importo, descrizione normalizzata). Una riga con stesso conto e importo e data entro ±2 giorni viene segnalata come dubbia, non scartata. Una riga che corrisponde a una transazione in sospeso generata da una ricorrenza la conferma, invece di crearne una nuova.

**Annullamento** — `fui import undo <lotto>` rimuove tutte le transazioni di quel lotto, se non ancora riconciliate.

**Export** — transazioni filtrate in CSV, JSON o formato FUI; dump completo dell'archivio in JSON o CSV (un file per entità), reimportabile con `fui restore`, per portabilità e migrazione.

**Riconciliazione** — l'utente indica il saldo dell'estratto a una data; FUI mostra la differenza e le transazioni non ancora spuntate. Quando la differenza è zero, le transazioni fino a quella data passano allo stato riconciliata; modificarle in seguito richiede una conferma esplicita.

## Formato FUI

Il formato FUI è il contratto tra connettori e FUI: JSON Lines in UTF-8, un oggetto per riga, con versione di protocollo `fui/1`. Lo stesso formato si può importare direttamente da file con `fui import movimenti.fui.jsonl`, utile per provare un connettore o per dati preparati a mano.

**Comandi del connettore**

| Comando | Input | Output |
| --- | --- | --- |
| `describe` | nessuno | Un oggetto JSON con `protocol`, `name`, `version`, tipi di file accettati e parametri di configurazione (chiave, tipo, obbligatorietà, descrizione) |
| `fetch` | Su stdin un oggetto JSON con `protocol`, `config`, `file` (percorso o null) e `since` (data opzionale) | Su stdout i record descritti sotto; su stderr solo messaggi per l'utente, mostrati con `--verbose` |

**Codici di uscita** — 0 successo, 2 configurazione non valida, 3 sorgente illeggibile o formato non riconosciuto, qualsiasi altro valore errore generico. Con un codice diverso da 0 FUI non importa nulla.

**Tipi di record**

| `type` | Quando | Campi |
| --- | --- | --- |
| `meta` | Prima riga, obbligatoria | `protocol`, `connector`, `version`, `source` (nome file e sha256) |
| `account` | Prima dei record che lo usano | `ref`, `name`, `currency`, `kind` (checking, card, savings, investment, cash, loan), `iban` opzionale |
| `transaction` | Un record per movimento | Vedi tabella seguente |
| `balance` | Opzionale | `account`, `date`, `amount`: saldo dell'estratto a una data, usato per la riconciliazione |
| `schedule` | Prima delle rate di un piano | `account` (conto di tipo loan), `valid_from`, `rate`, `rate_type` (fixed, variable, mixed) |
| `installment` | Una per rata, dopo il proprio `schedule` | `account`, `due_date`, `principal`, `interest`, `fees`, `remaining` (importi interi in unità minime) |
| `error` | Per ogni riga sorgente non convertibile | `line`, `message`, `raw` |
| `end` | Ultima riga, obbligatoria | `transactions`, `errors`: conteggi per verificare che l'output sia completo |

**Campi del record transaction**

| Campo | Tipo | Obbligatorio | Significato |
| --- | --- | --- | --- |
| `account` | stringa | sì | `ref` di un record account dichiarato prima |
| `id` | stringa | sì | Id stabile tra export successivi; se la banca non lo fornisce, hash deterministico calcolato dal connettore |
| `date` | YYYY-MM-DD | sì | Data contabile |
| `operation_date` | YYYY-MM-DD | no | Data in cui è avvenuta l'operazione, se diversa da quella contabile (es. pagamento con carta) |
| `value_date` | YYYY-MM-DD | no | Data valuta |
| `amount` | intero | sì | Importo con segno in unità minime della valuta; negativo = uscita |
| `currency` | ISO 4217 | sì | Deve coincidere con la valuta del conto |
| `description` | stringa | sì | Testo della banca, con spazi normalizzati |
| `payee` | stringa | no | Controparte, se riconoscibile |
| `counterparty_iban` | stringa | no | IBAN della controparte, usato per riconoscere i trasferimenti tra conti dell'utente |
| `status` | `booked` o `pending` | sì | `pending` entra in FUI come in sospeso |
| `kind` | payment, transfer, exchange, fee, interest, refund, investment, other | no | Natura del movimento; predefinito other; `investment` indica acquisti, vendite e rimborsi di titoli o fondi |
| `transfer` | stringa | no | Stesso valore su due movimenti = un trasferimento tra conti |
| `original` | oggetto `amount` + `currency` | no | Importo nella valuta originale per pagamenti in valuta estera |
| `balance_after` | intero | no | Saldo dopo il movimento, usato per i controlli di coerenza |
| `category_hint` | stringa | no | Categoria proposta dalla banca; la usano solo le regole, non viene mai applicata direttamente |
| `category` | stringa | no | Categoria da applicare direttamente; la scrive l'export di FUI, mentre i connettori usano `category_hint` |
| `tags` | array di stringhe | no | Tag da applicare |
| `notes` | stringa | no | Note dell'utente |
| `raw` | oggetto | no | Campi originali della riga, conservati per diagnosi |

**Regole**

- FUI ignora i campi che non conosce, così i connettori possono aggiungerne senza rompere la compatibilità; una versione di protocollo con major diversa viene rifiutata.
- Al primo import di un `ref` sconosciuto, FUI chiede a quale conto associarlo o se crearne uno nuovo e salva l'associazione per quel connettore.
- Un output senza `meta` iniziale o senza `end` finale è considerato troncato e l'import è annullato.
- Se `balance_after` è presente, FUI verifica che la sequenza dei saldi sia coerente e segnala le discrepanze nell'anteprima.

**Esempio**

```json
{"type":"meta","protocol":"fui/1","connector":"revolut","version":"0.1.0","source":{"file":"revolut-2026-10.csv","sha256":"9f2c…"}}
{"type":"account","ref":"revolut-EUR","name":"Revolut EUR","currency":"EUR","kind":"checking"}
{"type":"account","ref":"revolut-USD","name":"Revolut USD","currency":"USD","kind":"checking"}
{"type":"transaction","account":"revolut-EUR","id":"rv-8c1e","date":"2026-10-03","amount":-1250,"currency":"EUR","description":"Esselunga Milano","status":"booked","kind":"payment"}
{"type":"transaction","account":"revolut-EUR","id":"rv-a41b","date":"2026-10-05","amount":-10000,"currency":"EUR","description":"Exchanged to USD","status":"booked","kind":"exchange","transfer":"fx-20261005-1"}
{"type":"transaction","account":"revolut-USD","id":"rv-a41c","date":"2026-10-05","amount":10842,"currency":"USD","description":"Exchanged from EUR","status":"booked","kind":"exchange","transfer":"fx-20261005-1"}
{"type":"balance","account":"revolut-EUR","date":"2026-10-31","amount":234018}
{"type":"end","transactions":3,"errors":0}
```

**Import ed export in formato FUI** — FUI legge e scrive il proprio formato: un file esportato si può reimportare nello stesso archivio o in un altro senza perdere nulla di movimenti, classificazione e finanziamenti.

- **Export:** `fui export --format fui` accetta gli stessi filtri di `fui tx list` (periodo, conti, categorie, tag) e scrive `meta` con connector `fui`, un record `account` per ogni conto coinvolto, le transazioni, piani e rate dei finanziamenti, il saldo finale di ogni conto come record `balance` e infine `end`.
- **Id:** ogni transazione esportata mantiene l'id originale dell'import se presente, altrimenti riceve `fui-<id interno>`; reimportare nello stesso archivio non crea quindi duplicati.
- **Contenuto:** i due lati di un trasferimento hanno lo stesso valore di `transfer`; categoria, tag e note vanno nei campi `category`, `tags` e `notes`; le transazioni riconciliate sono esportate come `booked`.
- **Import:** `fui import file.fui.jsonl` segue il flusso normale (deduplicazione, anteprima, lotto annullabile); i conti si associano per `ref` come con i connettori e, con `--create-accounts`, quelli mancanti sono creati dai record `account`.
- **Categorie e tag nuovi:** quelli presenti nel file ma assenti nell'archivio sono elencati nell'anteprima e creati alla conferma.
- **Differenza dal dump completo:** il formato FUI trasporta conti, movimenti e finanziamenti ed è pensato per scambiare dati tra archivi o con script esterni; il dump completo in JSON o CSV contiene anche budget, regole, ricorrenze, profili e configurazione ed è pensato per backup e ripristino.

## Mutui e prestiti

Mutui e prestiti sono conti passività con un piano di ammortamento versionato: il tasso fisso usa un unico piano, mentre variazioni di tasso e rinegoziazioni creano nuove versioni senza toccare le rate già pagate. La funzione arriva in v0.2.

**Origine del piano**

- **Importato:** il prospetto della banca entra tramite un connettore, con i record `schedule` e `installment` del formato FUI; è la via naturale per il tasso fisso.
- **Calcolato:** FUI genera un piano alla francese da capitale, tasso e durata; per il variabile ricalcola le rate future sul debito residuo quando l'utente inserisce il nuovo tasso.
- Se la banca comunica la rata esatta, quel valore prevale sul calcolo.

**Eventi** — variazione del tasso, rinegoziazione ed estinzione parziale creano una nuova versione del piano valida da una data. Le rate con scadenza precedente restano invariate, così lo storico e i report passati non cambiano.

**Collegamento con i movimenti**

1. Ogni rata del piano genera una transazione attesa in stato in sospeso sul conto di addebito.
2. Quando l'utente la conferma, o un import trova l'addebito corrispondente, FUI la divide: la quota capitale diventa un trasferimento verso il conto del finanziamento, interessi e spese diventano uscite in una categoria dedicata.
3. Se l'importo addebitato differisce da quello previsto, FUI lo segnala e propone di registrare una variazione di tasso.

**Regole**

- La quota capitale riduce il debito e non conta come spesa né nei budget.
- Il debito residuo entra nel patrimonio netto come valore negativo.
- Un'estinzione parziale chiede se ridurre la rata o la durata, poi genera la nuova versione del piano.

## Budget

FUI offre due modalità di budget, tetti di spesa e buste, scelte con la chiave di configurazione `budget.mode` (predefinita: tetti). Entrambe lavorano su periodi mensili o annuali e riportano al periodo successivo sia l'avanzo sia lo sforamento.

| Aspetto | Tetti di spesa | Buste |
| --- | --- | --- |
| Idea | Un limite fissato in anticipo per categoria o tag | Ogni euro entrato viene assegnato a una busta finché "Da assegnare" arriva a zero |
| Cosa misura | Quanto hai speso rispetto al limite | Quanto resta da spendere dei soldi realmente disponibili |
| Azione richiesta | Impostare i limiti una volta | Assegnare le entrate a ogni accredito |
| Adatto a | Tenere d'occhio alcune categorie | Controllo completo e accantonamenti per obiettivi |

**Tetti di spesa**

- Un tetto copre una o più categorie o tag, con periodo mensile o annuale e importo nella valuta base.
- L'importo mensile può avere variazioni per singoli mesi (es. Regali più alto a dicembre).
- FUI mostra speso, previsto, residuo e percentuale; avvisa al superamento di soglie configurabili, predefinite all'80% e al 100%.

**Buste**

1. Ogni entrata registrata confluisce nel saldo "Da assegnare".
2. L'utente assegna importi alle buste; una busta copre una o più categorie o tag.
3. Ogni uscita delle categorie o dei tag coperti scala dalla busta corrispondente.
4. Per spendere oltre il contenuto di una busta, l'utente sposta importi da un'altra busta; gli spostamenti sono registrati come movimenti di budget, non come transazioni.

- Una busta annuale ha un obiettivo per l'anno e suggerisce la quota mensile da assegnare: residuo diviso per i mesi rimanenti.
- Le uscite su categorie non coperte da alcuna busta sono segnalate come "non coperte".
- Un saldo "Da assegnare" negativo indica che si è assegnato più di quanto entrato e viene segnalato.
- Il totale delle buste più "Da assegnare" deve coincidere con la somma dei saldi dei conti di liquidità inclusi nel budget; investimenti e finanziamenti sono esclusi.

**Regole comuni**

- Lo speso è la somma delle uscite delle categorie o dei tag coperti, al netto dei rimborsi; trasferimenti e quote capitale dei finanziamenti sono esclusi; gli split contano per riga.
- Le transazioni in sospeso (ricorrenze e rate attese) compaiono come "previsto" e non scalano finché non sono confermate.
- A fine periodo avanzo e sforamento passano al periodo successivo: uno sforamento riduce il disponibile del periodo dopo.
- Il periodo mensile segue `month_start_day`; quello annuale l'anno solare.
- Una categoria può essere coperta al massimo da un tetto o da una busta per periodo, per evitare doppi conteggi; i tag possono sovrapporsi alle categorie e sono riportati a parte.

## Report e analytics

I report predefiniti rispondono alle domande ricorrenti, i report salvati permettono di crearne di propri, mentre analisi automatiche e previsione di cassa segnalano ciò che l'utente non ha chiesto esplicitamente. Tutti i report escludono i trasferimenti e sono esportabili in CSV o JSON.

| Report | Domanda a cui risponde | Parametri | Fase |
| --- | --- | --- | --- |
| Spese per categoria | Dove sono finiti i soldi? | periodo, livello di categoria, conti | MVP |
| Flusso di cassa | Quanto è entrato e uscito ogni mese? | intervallo di mesi, conti | MVP |
| Budget vs effettivo | Sto rispettando i budget? | mese | v0.2 |
| Saldo nel tempo | Come si è mosso il saldo di un conto? | conto, intervallo, granularità | v0.2 |
| Debito residuo | Quanto devo ancora e come cala nel tempo? | finanziamento, intervallo | v0.2 |
| Interessi pagati | Quanto ho pagato di interessi e spese ogni anno? | finanziamento, anno | v0.2 |
| Patrimonio netto | Quanto valgo nel complesso e come cambia? | intervallo, granularità | v0.2 |
| Principali beneficiari | Con chi spendo di più? | periodo, numero di righe | v0.2 |
| Confronto periodi | Spendo più o meno del mese o dell'anno scorso? | due periodi, categoria | v0.3 |

**Requisiti**

- Periodo predefinito: mese corrente; il mese contabile può iniziare in un giorno configurabile (es. il 27, giorno dello stipendio).
- Con più valute, i totali sono convertiti nella valuta base al tasso valido alla data della transazione.
- Le transazioni con split contano nella categoria di ciascuna riga per il relativo importo.
- In TUI i report mostrano barre orizzontali e sparkline; in CLI una tabella, con `--chart` per le barre testuali.

**Filtri e raggruppamenti comuni** — ogni report accetta periodo, conti, categorie, tag, beneficiari, intervallo di importi, stato e testo nella descrizione, e può raggruppare per categoria, tag, beneficiario, conto, settimana, mese o anno. Nella TUI ogni riga di un report apre le transazioni che la compongono.

**Report salvati** (v0.2)

- Un report salvato ha un nome, un report di base, filtri, raggruppamento e un periodo relativo (es. "ultimi 3 mesi", "anno corrente"), così resta aggiornato senza modifiche.
- Si crea dalla TUI salvando la vista corrente o da CLI con `fui report save`, e si esegue con `fui report run <nome>`.
- I report salvati compaiono nella schermata Report accanto ai predefiniti.

**Analisi automatiche** (v0.3)

| Analisi | Cosa segnala | Criterio predefinito |
| --- | --- | --- |
| Spese anomale | Un movimento molto più alto del solito | Importo oltre 2,5 volte la mediana degli ultimi 6 mesi per lo stesso beneficiario o categoria |
| Abbonamenti rilevati | Addebiti ricorrenti non ancora registrati come ricorrenze, e variazioni di prezzo di quelli noti | Stesso beneficiario, importo entro ±5%, almeno 3 occorrenze a intervallo regolare (settimanale, mensile, annuale) |
| Andamenti e variazioni | Categorie con spesa in crescita o in calo marcati | Variazione oltre ±25% rispetto alla media dei 3 periodi precedenti, sopra un importo minimo per escludere il rumore |

- Le analisi girano in locale all'apertura della Dashboard o con `fui insights`; tutte le soglie sono configurabili.
- Ogni segnalazione si può ignorare, e non ricompare, oppure trasformare in un'azione: un abbonamento rilevato diventa una ricorrenza con un tasto.

**Previsione di cassa** (v0.2)

- Orizzonte configurabile con `forecast.horizon`, predefinito 6 mesi.
- Parte dal saldo attuale dei conti di liquidità e somma ricorrenze, rate dei finanziamenti e spese variabili stimate dai budget o, in loro assenza, dalla media degli ultimi 3 mesi per categoria.
- Mostra il saldo previsto per conto e totale, per giorno o settimana, ed evidenzia il primo giorno in cui un conto scenderebbe sotto zero o sotto una soglia minima configurata.
- Si consulta nella schermata Report o con `fui forecast`.

**KPI: autonomia** (MVP) — indica quanto durerebbe la liquidità attuale se le entrate si fermassero, sulla base della spesa mensile mediana.

- **Liquidità:** somma dei saldi dei conti contanti, correnti e di risparmio, meno il debito delle carte di credito; investimenti e finanziamenti esclusi. Con `--include-investments` si aggiunge il valore delle posizioni.
- **Spesa mensile di riferimento:** mediana delle uscite mensili degli ultimi 12 mesi completi. Trasferimenti e movimenti di investimento sono esclusi; le rate dei finanziamenti sono incluse per intero, perché escono comunque dal conto. Con meno di 3 mesi di dati il KPI non viene calcolato.
- **Risultato:** liquidità divisa per la spesa mensile di riferimento, espressa in mesi e giorni insieme alla data stimata di azzeramento (es. "7 mesi e 12 giorni, fino al 23 giugno 2027").
- **Dove:** nella Dashboard e con `fui kpi runway`.

**Simulazioni** (v0.3) — il simulatore calcola scenari ipotetici senza mai scrivere transazioni nell'archivio. Ogni scenario ha un nome, si salva, si ricalcola e si confronta con altri, fino a tre affiancati.

| Simulazione | Parametri | Risultato |
| --- | --- | --- |
| Rendimento investimenti | Capitale iniziale (o valore attuale delle posizioni), rendimento annuo atteso, orizzonte, capitalizzazione mensile o annuale, costi annui in percentuale, aliquota sulle plusvalenze, inflazione | Valore nominale e reale anno per anno, guadagno lordo e netto |
| PAC | Versamento periodico e frequenza, durata, capitale iniziale opzionale, crescita annua del versamento, rendimento atteso, costi, aliquota, inflazione | Totale versato, valore finale nominale e reale, interessi maturati, tabella anno per anno |
| Flussi di cassa | La previsione di cassa più eventi ipotetici: spese o entrate una tantum, variazioni ricorrenti da una data (aumento di stipendio, nuovo affitto), stop delle entrate, nuovo finanziamento con capitale, tasso e durata | Saldo previsto mese per mese confrontato con lo scenario base, primo mese sotto zero, differenza a fine orizzonte, autonomia risultante |

- Rendimento investimenti e PAC accettano tre tassi (pessimista, base, ottimista) e ne mostrano gli esiti affiancati.
- I calcoli sono deterministici: tassi costanti per tutto l'orizzonte, senza simulare la volatilità; la simulazione Monte Carlo, che stima un intervallo di esiti tenendo conto della volatilità, fa parte delle funzioni AI della versione free con modello locale, mentre la premium permette di eseguirla con modelli di frontiera.
- Rendimento investimenti e PAC condividono lo stesso motore di calcolo: un PAC è un investimento con versamenti periodici.
- L'aliquota è un parametro della simulazione, non un calcolo fiscale; il calcolo delle imposte resta fuori perimetro.
- Le simulazioni si usano dalla schermata Report o con `fui simulate`.

## Persistenza, backup, sicurezza e configurazione

Tutti i dati stanno in un unico file locale; FUI non apre connessioni di rete e crea backup automatici prima di ogni operazione rischiosa.

**Archivio**

- Un solo file nella directory dati standard del sistema (su Linux `~/.local/share/fui/`), sovrascrivibile con `--db` o la variabile `FUI_DB`.
- SQLite compilato dentro il binario (driver proposto: modernc.org/sqlite, senza cgo), per transazioni atomiche e query veloci sui report; l'utente non installa nulla a parte e la portabilità è garantita dall'export completo in CSV/JSON.
- Lo schema è versionato; le migrazioni girano in automatico all'avvio, sempre precedute da un backup.
- Un'interruzione del processo non lascia mai l'archivio in uno stato parziale.

**Backup**

- `fui backup` crea una copia datata nella directory dei backup.
- Backup automatico prima di import, migrazioni, modifiche di massa, reset e `restore`.
- Conservazione configurabile, predefinita agli ultimi 10 backup.

**Sicurezza e privacy**

- Nessuna chiamata di rete; l'eventuale aggiornamento dei tassi di cambio sarà un comando esplicito, disattivato di default.
- File dati e backup creati con permessi `600`.
- Nessun importo, descrizione o beneficiario nei log di default.

**Configurazione** — file TOML nella directory di configurazione standard (`~/.config/fui/config.toml`), con precedenza flag > variabili d'ambiente > file > valori predefiniti.

| Chiave | Significato | Predefinito |
| --- | --- | --- |
| `currency` | Valuta base | EUR |
| `locale` | Separatore decimale, formato data e numeri | it-IT |
| `month_start_day` | Giorno di inizio del mese contabile | 1 |
| `week_start` | Primo giorno della settimana | lunedì |
| `theme` | Tema della TUI | terminale |
| `backup.keep` | Numero di backup conservati | 10 |
| `import.profiles` | Profili di import per banca | nessuno |

## Requisiti non funzionali

I valori sotto sono obiettivi misurabili su un portatile recente con un archivio di 50.000 transazioni, salvo dove indicato.

| Area | Requisito | Obiettivo |
| --- | --- | --- |
| Prestazioni | Avvio di un comando CLI semplice (`add`, `account list`) | < 100 ms |
| Prestazioni | Apertura della TUI fino alla Dashboard | < 300 ms |
| Prestazioni | Import di un CSV | 10.000 righe in < 5 s |
| Prestazioni | Scorrimento liste in TUI | fluido fino a 100.000 transazioni |
| Correttezza | Calcoli sugli importi | nessun errore di arrotondamento; interi in unità minime |
| Affidabilità | Integrità dei dati | nessuna perdita o corruzione dopo un'interruzione forzata |
| Portabilità | Sistemi supportati | Linux e macOS; Windows solo se non richiede cambi strutturali o funzionali |
| Distribuzione | Installazione | binario singolo, senza runtime esterni |
| Offline | Rete | nessuna funzione richiede connessione |
| Accessibilità | Colore | rispetto di `NO_COLOR`; nessuna informazione solo cromatica |
| Qualità | Test | logica di dominio coperta da test unitari; CLI coperta da test end-to-end su archivi temporanei |

## Perimetro MVP e roadmap

L'MVP copre il ciclo base registra, importa, consulta; budget, ricorrenze e automazioni arrivano nelle due versioni successive.

| Versione | Contenuto | Uscita quando |
| --- | --- | --- |
| v0.1 (MVP) | `init`; conti, transazioni, trasferimenti, categorie; CLI completa per queste entità; TUI con Dashboard, Transazioni e Conti; import tramite connettori (formato FUI, connettori esterni Widiba e Revolut, connettore CSV integrato), dedup e annullamento; multi-valuta con tassi di cambio manuali; export CSV/JSON e in formato FUI; backup manuale e automatico; report spese per categoria e flusso di cassa; KPI di autonomia; skill per agenti AI; ricerca delle transazioni con filtri; reset dell'archivio; attivazione dei moduli | Un mese reale di movimenti gestito solo con FUI |
| v0.2 | Budget, ricorrenze, mutui e prestiti, patrimonio netto, report salvati, previsione di cassa, regole di categorizzazione, riconciliazione, split, tag, schermate Budget, Report, Import e Impostazioni, completamento shell | Budget e riconciliazione usati per due mesi consecutivi |
| v0.3 | Connettori integrati OFX e CAMT.053, investimenti con posizioni dettagliate, moduli Crypto e Beni, analisi automatiche, simulazioni, confronto periodi, annullamento generico | Da definire |

**Versioni free e premium** — la versione free comprende tutte le funzioni descritte in questo documento, incluse le funzioni AI basate su un modello locale. La versione premium aggiunge la sincronizzazione dei dati su più dispositivi e le interfacce web e mobile, e permette di usare modelli AI di frontiera al posto del modello locale, ad esempio per la simulazione Monte Carlo; l'invio di dati a un modello esterno richiede un consenso esplicito. Restano da definire le funzioni AI e il modello locale da usare, il modello di distribuzione e come realizzare la sincronizzazione senza rinunciare al principio local-first.

**Fuori perimetro**

- Sincronizzazione tra dispositivi nella versione free (prevista nella premium).
- Collegamento diretto alle banche (open banking PSD2).
- Più utenti o archivi condivisi.
- App mobile o interfaccia web nella versione free (previste nella premium).
- Valorizzazione di titoli con prezzi di mercato in tempo reale.
- Calcolo di imposte o dichiarazione dei redditi.

## Decisioni e domande aperte

Lo stack è deciso; le domande rimaste riguardano il perimetro e si possono chiudere in parallelo all'avvio dello sviluppo.

**Decisioni prese**

| Tema | Scelta | Motivo |
| --- | --- | --- |
| Nome del comando | `fui` | Breve da digitare e coincide con il nome del progetto |
| Linguaggio | Go | Iterazione rapida, build quasi istantanee, binario singolo per Linux e macOS |
| Framework TUI | Bubble Tea | Architettura Elm (modello, messaggi, update, view) e componenti già pronti nell'ecosistema |
| Archivio | SQLite incluso nel binario | Nessuna installazione separata; portabilità tramite export completo CSV/JSON |
| Ingestion | Connettori integrati ed esterni | Ogni banca o sorgente si aggiunge senza modificare FUI |
| Banche iniziali | Widiba e Revolut | Sono le banche in uso; implementate come connettori esterni che producono il formato FUI già nell'MVP |
| Valute | Multi-valuta già nell'MVP | Conti in valute diverse, totali convertiti nella valuta base con tassi manuali per data |
| Lingua dell'interfaccia | Inglese | Comandi, messaggi e testi della TUI in inglese; formati di date e numeri seguono il `locale` configurato |
| Ricorrenze | Registrate in automatico in stato in sospeso | Diventano registrate quando l'utente le conferma o quando un import trova il movimento corrispondente |
| Budget | Tetti di spesa o buste, scelta globale con `budget.mode` | Due approcci per esigenze diverse, senza ambiguità su come trattare le entrate |
| Mutui e prestiti | Conti passività con piano di ammortamento versionato, in v0.2 | Gestisce tasso fisso, variabile e rinegoziazioni senza alterare lo storico |
| Investimenti | Posizioni dettagliate (titoli, quantità) | Valore e rendimento per titolo, con prezzi inseriti manualmente |
| Windows | Supportato solo se non richiede cambi strutturali o funzionali | Linux e macOS restano le piattaforme di riferimento |

**Da decidere**

- [ ] Raccogliere export Widiba e Revolut su periodi più lunghi per verificare causali e tipi di movimento assenti negli export di prova.

## Cronologia delle versioni

**Versione 1.2** — 12 novembre 2026

| Sezione | Modifica |
| --- | --- |
| Moduli | Nuova sezione: attivazione selettiva delle funzioni, profili Expense tracker, Completo e Personalizzato, dipendenze tra moduli, effetti della disattivazione |
| Modello di dominio | Nuova entità Bene; precisione della quantità per titolo, necessaria per le cripto-attività |
| Requisiti funzionali — CLI | Nuovi comandi `fui module` e `fui asset`; `fui init` accetta `--preset` e `--modules` |
| Requisiti funzionali — TUI | Le Impostazioni gestiscono i moduli attivi |
| Perimetro MVP e roadmap | Attivazione dei moduli nell'MVP; moduli Crypto e Beni in v0.3 |

**Versione 1.1** — 12 novembre 2026

| Sezione | Modifica |
| --- | --- |
| Requisiti funzionali — CLI | Nuovo comando `fui tx search` per la ricerca testuale; `fui tx list` ora ha filtri definiti |
| Requisiti funzionali — CLI | Nuova parte "Ricerca e filtri delle transazioni": elenco dei filtri, combinazione in AND/OR, ordinamento e limiti, uso nella TUI, obiettivo di 200 ms su 100.000 transazioni |
| Requisiti funzionali — CLI | Nuovo comando `fui reset` e nuova parte "Reset dell'archivio": backup preventivo escluso dalla rotazione, conferma esplicita, opzione `--keep-settings`, comportamento come `fui init` se l'archivio non esiste |
| Uso con agenti AI | La regola 3 vieta anche di azzerare l'archivio senza una richiesta esplicita |
| Requisiti funzionali — TUI | La schermata Transazioni offre ricerca testuale e filtri |
| Persistenza, backup, sicurezza e configurazione | Backup automatico anche prima del reset |
| Perimetro MVP e roadmap | Ricerca delle transazioni con filtri e reset dell'archivio aggiunti all'MVP |

**Versione 1.0** — 12 novembre 2026: prima versione completa delle specifiche funzionali.
