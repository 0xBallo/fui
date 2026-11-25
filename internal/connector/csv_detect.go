package connector

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/pkg/fuiproto"
)

const (
	detectStatRows   = 50 // rows the statistics are computed on
	detectSampleRows = 5  // rows shown to the user
	detectMinShare   = 80 // percent of cells that must fit a shape for a column to have it
)

// detectDelimiters are the candidate delimiters, in order of preference on ties.
var detectDelimiters = []rune{';', ',', '\t', '|'}

// detectDateFormats are the candidate date formats, in order of preference on ties: day first
// before month first.
var detectDateFormats = []string{
	"DD/MM/YYYY", "DD-MM-YYYY", "DD.MM.YYYY", "YYYY-MM-DD", "YYYY/MM/DD",
	"DD/MM/YY", "DD-MM-YY", "DD.MM.YY", "MM/DD/YYYY", "MM-DD-YYYY",
}

// headerRule lists the header names of a role. A weak name is a less likely candidate for the
// role (a value date next to the booking date).
type headerRule struct {
	role   string
	strong []string
	weak   []string
}

var headerRules = []headerRule{
	{role: importing.RoleBalance, strong: []string{"saldo", "balance"}},
	{
		role: importing.RoleDate,
		strong: []string{
			"data", "date", "data operazione", "data contabile", "data registrazione",
			"booking date", "transaction date", "completed date", "posting date",
		},
		weak: []string{"data valuta", "value date", "valuta", "started date", "start date"},
	},
	{role: importing.RoleAmount, strong: []string{"importo", "amount", "ammontare"}},
	{
		role:   importing.RoleDebit,
		strong: []string{"dare", "uscite", "uscita", "addebito", "addebiti", "debit", "withdrawal", "withdrawals", "paid out", "money out"},
	},
	{
		role:   importing.RoleCredit,
		strong: []string{"avere", "entrate", "entrata", "accredito", "accrediti", "credit", "deposit", "deposits", "paid in", "money in"},
	},
	{
		role: importing.RoleDescription,
		strong: []string{
			"descrizione", "description", "causale", "details", "detail", "dettagli", "memo",
			"payee", "beneficiary", "beneficiario", "narrative", "merchant",
		},
		weak: []string{"reference", "riferimento", "note", "notes"},
	},
	{
		role: importing.RoleID,
		strong: []string{
			"id", "transaction id", "external id", "id transazione", "identificativo", "codice operazione",
			"numero operazione", "cro", "txid", "reference number",
		},
	},
}

// amountShape is a number as banks write it: optional sign or accounting parentheses, digits
// with optional thousands groups, optional decimals.
var amountShape = regexp.MustCompile(`^[+-]?\(?[+-]?(\d+|\d{1,3}([.,]\d{3})+)([.,]\d+)?\)?$`)

// CSVFileDetector is the importing.CSVDetector that reads the file from disk.
type CSVFileDetector struct{}

var _ importing.CSVDetector = CSVFileDetector{}

// DetectCSV implements importing.CSVDetector.
func (CSVFileDetector) DetectCSV(ctx context.Context, file string) (importing.CSVSuggestion, error) {
	if err := ctx.Err(); err != nil {
		return importing.CSVSuggestion{}, fmt.Errorf("detect csv: %w", err)
	}
	data, err := readCapped(file)
	if err != nil {
		return importing.CSVSuggestion{}, err
	}
	return DetectCSV(data)
}

// DetectCSV guesses the delimiter, header, date format, decimal separator and column roles of a
// CSV bank export and suggests the profile the CSV connector needs. The suggestion is checked by
// converting the file with that profile; rows that fail are reported as warnings.
func DetectCSV(data []byte) (importing.CSVSuggestion, error) {
	text, enc, err := detectText(data)
	if err != nil {
		return importing.CSVSuggestion{}, err
	}
	delim := sniffDelimiter(text)
	profile := config.CSVProfile{Delimiter: delimiterName(delim), Encoding: enc}
	rows, lines, err := readRows(text, profile)
	if err != nil {
		return importing.CSVSuggestion{}, err
	}
	if len(rows) == 0 {
		return importing.CSVSuggestion{}, errors.New("csv has no rows")
	}
	rows, profile.SkipRows = dropPreamble(rows, lines)
	hasHeader := looksLikeHeader(rows[0])
	headers, body := columnNames(rows, hasHeader)
	d := newDetector(headers, body, hasHeader)
	d.assign()
	profile.DateFormat = d.dateFormat
	profile.DecimalSeparator = d.decimalSep()
	d.fillProfile(&profile)
	sug := importing.CSVSuggestion{
		Profile: profile, Delimiter: delim, DateFormat: profile.DateFormat,
		DecimalSep: []rune(profile.DecimalSeparator)[0], HasHeader: hasHeader,
		Columns: d.columns(), Sample: sample(body), Warnings: d.warnings,
	}
	sug.Warnings = append(sug.Warnings, verifyProfile(data, profile, hasHeader)...)
	return sug, nil
}

// dropPreamble drops the leading rows with fewer fields than the data (a title, the account
// holder) and returns how many lines that skipped.
func dropPreamble(rows [][]string, lines []int) ([][]string, int) {
	width := modalWidth(rows)
	for p, row := range rows {
		if len(row) == width && nonEmptyCells(row) >= min(2, width) {
			if p == 0 {
				return rows, 0
			}
			return rows[p:], lines[p] - 1
		}
	}
	return rows, 0
}

func nonEmptyCells(row []string) int {
	n := 0
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			n++
		}
	}
	return n
}

// columnNames returns the header names and the data rows; without a header the columns are
// named by position.
func columnNames(rows [][]string, hasHeader bool) ([]string, [][]string) {
	headers := make([]string, modalWidth(rows))
	for i := range headers {
		headers[i] = fmt.Sprintf("column %d", i+1)
		if hasHeader {
			headers[i] = strings.TrimSpace(cell(rows[0], i))
		}
	}
	if hasHeader {
		return headers, rows[1:]
	}
	return headers, rows
}

// detectText decodes the data as UTF-8, falling back to Windows-1252 for legacy exports.
func detectText(data []byte) (text, encoding string, err error) {
	text, err = toUTF8(data, "")
	if err != nil {
		text, err = toUTF8(data, "windows-1252")
		encoding = "windows-1252"
	}
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(text) == "" {
		return "", "", errors.New("csv is empty")
	}
	return text, encoding, nil
}

func delimiterName(r rune) string {
	if r == '\t' {
		return `\t`
	}
	return string(r)
}

// sniffDelimiter picks the candidate that splits most of the first lines into the same number
// of fields, at least two. It falls back to a comma.
func sniffDelimiter(text string) rune {
	best, bestFreq, bestWidth := ',', 0, 0
	for _, cand := range detectDelimiters {
		r := csv.NewReader(strings.NewReader(text))
		r.Comma, r.FieldsPerRecord, r.LazyQuotes = cand, -1, true
		var rows [][]string
		for len(rows) < detectStatRows {
			rec, err := r.Read()
			if err != nil {
				break
			}
			rows = append(rows, rec)
		}
		width := modalWidth(rows)
		if width < 2 {
			continue
		}
		freq := 0
		for _, row := range rows {
			if len(row) == width {
				freq++
			}
		}
		if freq > bestFreq || (freq == bestFreq && width > bestWidth) {
			best, bestFreq, bestWidth = cand, freq, width
		}
	}
	return best
}

// modalWidth is the most common field count of the first rows; the smaller wins a tie so a
// title line does not outvote the data.
func modalWidth(rows [][]string) int {
	counts := map[int]int{}
	for i, row := range rows {
		if i == detectStatRows {
			break
		}
		counts[len(row)]++
	}
	best, bestN := 0, 0
	for w, n := range counts {
		if n > bestN || (n == bestN && w > best) {
			best, bestN = w, n
		}
	}
	return best
}

func sample(rows [][]string) [][]string {
	out := make([][]string, 0, detectSampleRows)
	for i := 0; i < len(rows) && i < detectSampleRows; i++ {
		out = append(out, rows[i])
	}
	return out
}

func parsesAsDate(layoutName, s string) bool {
	_, err := time.Parse(goLayout(layoutName), s)
	return err == nil
}

// numericCell reports whether s is an amount and, when the decimal separator is evident, which
// one it is: three digits after the last separator may be a thousands group.
func numericCell(s string) (ok bool, decimal rune) {
	s = strings.NewReplacer(" ", "", "\u00a0", "", "€", "", "$", "", "£", "").Replace(s)
	if !amountShape.MatchString(s) {
		return false, 0
	}
	s = strings.TrimSuffix(s, ")")
	i := strings.LastIndexAny(s, ".,")
	if i < 0 || len(s)-i-1 == 3 {
		return true, 0
	}
	return true, rune(s[i])
}

// looksLikeHeader is true when no cell of the row is a date or an amount.
func looksLikeHeader(row []string) bool {
	for _, c := range row {
		c = strings.TrimSpace(c)
		if ok, _ := numericCell(c); ok {
			return false
		}
		for _, f := range detectDateFormats {
			if parsesAsDate(f, c) {
				return false
			}
		}
	}
	return true
}

func headerWords(h string) []string {
	return strings.FieldsFunc(strings.ToLower(h), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// matchName scores the words of a header against names: 100 when it is exactly a name, 80 when
// it contains one as consecutive words.
func matchName(words []string, names []string) int {
	best := 0
	for _, name := range names {
		key := strings.Fields(name)
		for i := 0; i+len(key) <= len(words); i++ {
			if strings.Join(words[i:i+len(key)], " ") != name {
				continue
			}
			score := 80
			if len(words) == len(key) {
				score = 100
			}
			best = max(best, score)
		}
	}
	return best
}

// headerScores gives, per role, how well the header names it (0 when it does not).
func headerScores(h string) map[string]int {
	words := headerWords(h)
	out := map[string]int{}
	for _, r := range headerRules {
		switch {
		case matchName(words, r.weak) > 0:
			out[r.role] = matchName(words, r.weak) * 6 / 10
		default:
			out[r.role] = matchName(words, r.strong) * 9 / 10
		}
	}
	return out
}

// colStat summarizes the shapes of the cells of one column.
type colStat struct {
	nonEmpty  int
	dateCount int
	dateFmt   int // index in detectDateFormats of the format that fits most cells
	numCount  int
	dot       int // cells whose decimal separator is evidently a dot
	comma     int
	textLen   int // total length of the cells that are not amounts
}

func (s colStat) dateShare() int { return pct(s.dateCount, s.nonEmpty) }
func (s colStat) numShare() int  { return pct(s.numCount, s.nonEmpty) }

func pct(n, total int) int {
	if total == 0 {
		return 0
	}
	return n * 100 / total
}

func columnStat(rows [][]string, c int) colStat {
	var st colStat
	fmtCounts := make([]int, len(detectDateFormats))
	for _, row := range rows {
		v := cell(row, c)
		if v == "" {
			continue
		}
		st.nonEmpty++
		for i, f := range detectDateFormats {
			if parsesAsDate(f, v) {
				fmtCounts[i]++
			}
		}
		ok, dec := numericCell(v)
		switch {
		case !ok:
			st.textLen += len(v)
		case dec == '.':
			st.dot++
		case dec == ',':
			st.comma++
		}
		if ok {
			st.numCount++
		}
	}
	for i, n := range fmtCounts {
		if n > st.dateCount {
			st.dateCount, st.dateFmt = n, i
		}
	}
	return st
}

// detector accumulates the column roles of one file.
type detector struct {
	headers    []string
	hasHeader  bool
	rows       int
	stats      []colStat
	scores     []map[string]int
	roles      []string
	conf       []int
	dateFormat string
	warnings   []string
}

func newDetector(headers []string, rows [][]string, hasHeader bool) *detector {
	if len(rows) > detectStatRows {
		rows = rows[:detectStatRows]
	}
	d := &detector{headers: headers, hasHeader: hasHeader, rows: len(rows), warnings: []string{}}
	for c, h := range headers {
		d.stats = append(d.stats, columnStat(rows, c))
		if hasHeader {
			d.scores = append(d.scores, headerScores(h))
		} else {
			d.scores = append(d.scores, map[string]int{})
		}
		d.roles = append(d.roles, "")
		d.conf = append(d.conf, 0)
	}
	if !hasHeader {
		d.warnings = append(d.warnings, "the file has no header row: add one before importing, the profile names columns by header")
	}
	return d
}

func (d *detector) set(c int, role string, conf int) {
	d.roles[c], d.conf[c] = role, min(conf, 95)
}

// free reports whether the column has no role yet and is not named as a balance or an id, which
// look like amounts.
func (d *detector) free(c int) bool {
	return d.roles[c] == "" && d.scores[c][importing.RoleBalance] == 0 && d.scores[c][importing.RoleID] == 0
}

// best returns the free column with the highest header score for the role (the leftmost on
// ties) that satisfies ok, or -1.
func (d *detector) best(role string, ok func(c int) bool) int {
	best, bestScore := -1, 0
	for c := range d.headers {
		if d.roles[c] != "" || !ok(c) {
			continue
		}
		if s := d.scores[c][role]; s > bestScore {
			best, bestScore = c, s
		}
	}
	return best
}

func (d *detector) assign() {
	d.pickDate()
	d.pickAmounts()
	d.pickDescription()
	if c := d.best(importing.RoleID, func(int) bool { return true }); c >= 0 {
		d.set(c, importing.RoleID, d.scores[c][importing.RoleID])
	}
	for c := range d.roles {
		switch {
		case d.roles[c] != "":
		case d.scores[c][importing.RoleBalance] > 0:
			d.set(c, importing.RoleBalance, d.scores[c][importing.RoleBalance])
		default:
			d.roles[c] = importing.RoleOther
		}
	}
}

func (d *detector) pickDate() {
	best, bestScore := -1, 0
	for c, st := range d.stats {
		if st.dateShare() < 60 || d.scores[c][importing.RoleBalance] > 0 {
			continue
		}
		if s := d.scores[c][importing.RoleDate] + st.dateShare()/2 + 1; s > bestScore {
			best, bestScore = c, s
		}
	}
	if best < 0 {
		d.warnings = append(d.warnings, "no date column found")
		return
	}
	d.dateFormat = detectDateFormats[d.stats[best].dateFmt]
	d.set(best, importing.RoleDate, max(d.scores[best][importing.RoleDate], d.stats[best].dateShare()*8/10))
}

func (d *detector) numeric(c int) bool {
	if !d.free(c) || d.stats[c].nonEmpty == 0 {
		return false
	}
	// A column the header names as an amount may hold a few unparsable cells: the verification
	// reports them.
	named := d.scores[c][importing.RoleAmount]+d.scores[c][importing.RoleDebit]+d.scores[c][importing.RoleCredit] > 0
	return d.stats[c].numShare() >= detectMinShare || (named && d.stats[c].numShare() >= 50)
}

func (d *detector) pickAmounts() {
	if c := d.best(importing.RoleAmount, d.numeric); c >= 0 {
		d.set(c, importing.RoleAmount, d.scores[c][importing.RoleAmount])
		return
	}
	debit, credit := d.best(importing.RoleDebit, d.numeric), d.best(importing.RoleCredit, d.numeric)
	if debit >= 0 && credit >= 0 {
		d.set(debit, importing.RoleDebit, d.scores[debit][importing.RoleDebit])
		d.set(credit, importing.RoleCredit, d.scores[credit][importing.RoleCredit])
		return
	}
	var cands []int
	for c := range d.headers {
		if d.numeric(c) && d.scores[c][importing.RoleDebit] == 0 && d.scores[c][importing.RoleCredit] == 0 {
			cands = append(cands, c)
		}
	}
	switch {
	case len(cands) == 0:
		d.warnings = append(d.warnings, "no amount column found")
	case len(cands) >= 2 && d.complementary(cands[0], cands[1]):
		d.set(cands[0], importing.RoleDebit, 40)
		d.set(cands[1], importing.RoleCredit, 40)
		d.warnings = append(d.warnings, "debit and credit columns are told apart by order only: check them")
	default:
		d.set(cands[0], importing.RoleAmount, d.stats[cands[0]].numShare()*6/10)
		if len(cands) > 1 {
			d.warnings = append(d.warnings, "several numeric columns: the first was taken as the amount")
		}
	}
}

// complementary reports whether two columns are filled in different rows, as a debit and a
// credit column are.
func (d *detector) complementary(a, b int) bool {
	fa, fb := pct(d.stats[a].nonEmpty, d.rows), pct(d.stats[b].nonEmpty, d.rows)
	return fa >= 10 && fb >= 10 && fa < 90 && fb < 90 && fa+fb <= 120
}

func (d *detector) pickDescription() {
	if c := d.best(importing.RoleDescription, func(int) bool { return true }); c >= 0 {
		d.set(c, importing.RoleDescription, d.scores[c][importing.RoleDescription])
		return
	}
	best, bestLen := -1, 0
	for c, st := range d.stats {
		if !d.free(c) || st.nonEmpty == 0 || st.numShare() >= 50 || st.dateShare() >= 50 {
			continue
		}
		if avg := st.textLen / st.nonEmpty; avg > bestLen {
			best, bestLen = c, avg
		}
	}
	if best < 0 {
		d.warnings = append(d.warnings, "no description column found")
		return
	}
	d.set(best, importing.RoleDescription, 40)
}

// decimalSep is the separator most amount cells use; a dot when nothing says otherwise.
func (d *detector) decimalSep() string {
	dot, comma := 0, 0
	for c, st := range d.stats {
		switch d.roles[c] {
		case importing.RoleAmount, importing.RoleDebit, importing.RoleCredit:
			dot, comma = dot+st.dot, comma+st.comma
		}
	}
	switch {
	case comma > dot:
		return ","
	case dot == 0:
		d.warnings = append(d.warnings, `decimal separator not evident, assuming "."`)
	}
	return "."
}

func (d *detector) columns() []importing.ColumnGuess {
	out := make([]importing.ColumnGuess, len(d.headers))
	for c, h := range d.headers {
		out[c] = importing.ColumnGuess{Header: h, Role: d.roles[c], Confidence: importing.Confidence(d.conf[c])}
	}
	return out
}

func (d *detector) fillProfile(p *config.CSVProfile) {
	for c, role := range d.roles {
		switch role {
		case importing.RoleDate:
			p.DateColumn = d.headers[c]
		case importing.RoleAmount:
			p.AmountColumn = d.headers[c]
		case importing.RoleDebit:
			p.DebitColumn = d.headers[c]
		case importing.RoleCredit:
			p.CreditColumn = d.headers[c]
		case importing.RoleDescription:
			p.DescriptionColumn = d.headers[c]
		case importing.RoleID:
			p.ExternalIDColumn = d.headers[c]
		}
	}
}

// verifyProfile converts the file with the suggested profile, as the import would, and reports
// the rows that fail.
func verifyProfile(data []byte, p config.CSVProfile, hasHeader bool) []string {
	complete := p.DateColumn != "" && p.DescriptionColumn != "" &&
		(p.AmountColumn != "" || (p.DebitColumn != "" && p.CreditColumn != ""))
	if !hasHeader || !complete {
		return nil
	}
	var txs, errs int
	first := ""
	for rec, err := range csvRecords(context.Background(), data, csvOptions{profile: p, account: "detect", currency: "EUR"}) {
		if err != nil {
			return []string{"the suggested profile does not parse the file: " + err.Error()}
		}
		if rec.Type == fuiproto.RecordTypeTransaction {
			txs++
		}
		if rec.Type == fuiproto.RecordTypeError {
			if errs++; first == "" {
				first = fmt.Sprintf("line %d: %s", rec.Error.Line, rec.Error.Message)
			}
		}
	}
	switch {
	case errs > 0:
		return []string{fmt.Sprintf("%d of %d rows do not parse with the suggested profile (first: %s)", errs, txs+errs, first)}
	case txs == 0:
		return []string{"the file has no data rows"}
	}
	return nil
}
