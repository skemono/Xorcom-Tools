package pbx

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Row statuses: before Aplicar (nuevo/existe/sindesc/error) and after (aplicado/omitido/fallo).
const (
	PinNew     = "nuevo"
	PinExists  = "existe"
	PinNoDesc  = "sindesc" // empty description, skipped unless the user includes them
	PinError   = "error"
	PinApplied = "aplicado"
	PinSkipped = "omitido"
	PinFailed  = "fallo"
)

const maxPinRows = 10000

// PinRow is one CSV line on its way to ombu_pin_list_entries.
type PinRow struct {
	Line        int    `json:"line"` // line number in the file, as the user sees it
	PIN         string `json:"pin"`
	Description string `json:"description"` // after filterDescription
	Filtered    bool   `json:"filtered"`    // ñ, accents or typographic characters were replaced
	Status      string `json:"status"`
	Error       string `json:"error"`
}

// CSVInfo is what F-02 detected about the file, shown to the user.
type CSVInfo struct {
	Separator string `json:"separator"` // ";", "," or "tab"
	Encoding  string `json:"encoding"`  // "UTF-8" or "Windows-1252"
	Header    bool   `json:"header"`
	Columns   int    `json:"columns"` // 2 (PIN, descripción) or 3 (pin_list_id, PIN, descripción)
	Rows      int    `json:"rows"`
}

// ParsePinCSV decodes and validates a PIN file for listID. A returned error is a file-level problem;
// row problems come back as PinError rows so the preview can show every line.
func ParsePinCSV(raw []byte, listID int) (CSVInfo, []PinRow, error) {
	var info CSVInfo
	if bytes.HasPrefix(raw, []byte("PK\x03\x04")) {
		return info, nil, errors.New("parece un archivo de Excel (.xlsx): ábralo en Excel y guárdelo como CSV")
	}
	if bytes.IndexByte(raw, 0) >= 0 { // old .xls (OLE2) or UTF-16 "Texto Unicode": never a CSV of text
		return info, nil, errors.New("no es un archivo CSV de texto (¿Excel .xls o texto Unicode?): en Excel use Guardar como → CSV")
	}
	text, enc := decodeText(raw)
	info.Encoding = enc
	sep, name := detectSeparator(text)
	info.Separator = name

	r := csv.NewReader(strings.NewReader(text))
	r.Comma, r.FieldsPerRecord, r.LazyQuotes = sep, -1, true
	type rec struct {
		line   int
		fields []string
	}
	var recs []rec
	for {
		fields, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return info, nil, fmt.Errorf("no se pudo leer el CSV: %v", err)
		}
		line, _ := r.FieldPos(0)
		for len(fields) > 0 && strings.TrimSpace(fields[len(fields)-1]) == "" { // Excel's trailing empty cells
			fields = fields[:len(fields)-1]
		}
		if len(fields) == 0 {
			continue
		}
		recs = append(recs, rec{line, fields})
		if len(recs) > maxPinRows+1 {
			return info, nil, fmt.Errorf("el archivo tiene más de %d filas", maxPinRows)
		}
	}
	if len(recs) == 0 {
		return info, nil, errors.New("el archivo está vacío")
	}
	// The file's shape comes from its rows, not its widest row: one stray note in column C must be that
	// row's error, not turn every row into the 3-column format. A row with only a PIN fits the 2-column shape.
	multi, three, over := 0, 0, 0
	for _, rc := range recs {
		switch n := len(rc.fields); {
		case n > 3:
			over++
			info.Columns = max(info.Columns, n)
		case n == 3:
			three++
		}
		if len(rc.fields) >= 2 {
			multi++
		}
	}
	switch {
	case multi == 0:
		info.Columns = 1
	case over*2 > len(recs):
	case three*2 > len(recs):
		info.Columns = 3
	default:
		info.Columns = 2
	}
	if info.Columns < 2 || info.Columns > 3 {
		return info, nil, fmt.Errorf("el archivo tiene %d columna(s); se esperan 2 (PIN, descripción) o 3 (pin_list_id, PIN, descripción)", info.Columns)
	}
	pinCol := info.Columns - 2 // 0 for 2 columns, 1 for 3
	field := func(f []string, i int) string {
		if i < len(f) {
			return strings.TrimSpace(f[i])
		}
		return ""
	}
	if !strings.ContainsAny(field(recs[0].fields, pinCol), "0123456789") {
		info.Header, recs = true, recs[1:]
	}
	if len(recs) > maxPinRows {
		return info, nil, fmt.Errorf("el archivo tiene más de %d filas", maxPinRows)
	}
	info.Rows = len(recs)

	rows := make([]PinRow, len(recs))
	first := map[string]int{} // PIN -> index of its first row
	for i, rc := range recs {
		row := PinRow{Line: rc.line, PIN: field(rc.fields, pinCol)}
		rawDesc := field(rc.fields, pinCol+1)
		row.Description, row.Filtered = filterDescription(rawDesc)
		switch {
		case len(rc.fields) > info.Columns:
			row.Error = fmt.Sprintf("la fila tiene %d celdas; se esperan %d (¿un separador de más en la descripción?)", len(rc.fields), info.Columns)
		case strings.ContainsAny(row.PIN+rawDesc, "\r\n"):
			row.Error = "la celda ocupa varias líneas: ¿comilla sin cerrar en el archivo?"
		case info.Columns == 3 && field(rc.fields, 0) != strconv.Itoa(listID):
			row.Error = fmt.Sprintf("pin_list_id «%s» no coincide con la lista elegida (%d)", field(rc.fields, 0), listID)
		case row.PIN == "":
			row.Error = "falta el PIN"
		case strings.Trim(row.PIN, "0123456789*") != "":
			row.Error = fmt.Sprintf("PIN inválido «%s»: solo números y *", row.PIN)
		case len(row.PIN) > 255:
			row.Error = "PIN de más de 255 caracteres"
		case len(row.Description) > 255:
			row.Error = "descripción de más de 255 caracteres"
		default:
			if bad := firstNonASCII(row.Description); bad != "" {
				row.Error = fmt.Sprintf("carácter no permitido «%s» en la descripción", bad)
			}
		}
		if row.Error == "" {
			if j, dup := first[row.PIN]; dup {
				row.Error = fmt.Sprintf("PIN repetido (líneas %d y %d)", rows[j].Line, row.Line)
				rows[j].Error, rows[j].Status = row.Error, PinError
			} else {
				first[row.PIN] = i
			}
		}
		if row.Error != "" {
			row.Status = PinError
		}
		rows[i] = row
	}
	return info, rows, nil
}

// PlanPins marks every valid row as already in the list, without description (skipped unless
// includeEmpty) or new. existing maps PIN -> description.
func PlanPins(rows []PinRow, existing map[string]string, includeEmpty bool) []PinRow {
	out := append([]PinRow(nil), rows...)
	for i := range out {
		if out[i].Status == PinError {
			continue
		}
		_, inList := existing[out[i].PIN]
		switch {
		case inList:
			out[i].Status = PinExists
		case out[i].Description == "" && !includeEmpty:
			out[i].Status = PinNoDesc
		default:
			out[i].Status = PinNew
		}
	}
	return out
}

// progressEvery is how often InsertSQL reports how far it got.
const progressEvery = 100

// InsertSQL is one transaction; NOT EXISTS keeps it add-only even if a PIN appeared after the preview.
// Every progressEvery inserts, and after the last, it SELECTs the running count for ApplyPins' progress.
func InsertSQL(listID int, rows []PinRow) string {
	var b strings.Builder
	b.WriteString("START TRANSACTION;\n")
	n := 0
	for _, r := range rows {
		if r.Status != PinNew {
			continue
		}
		desc := "NULL"
		if r.Description != "" {
			desc = sqlString(r.Description)
		}
		fmt.Fprintf(&b, "INSERT INTO ombu_pin_list_entries (pin_list_id, password, description) SELECT %d, %s, %s FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM ombu_pin_list_entries WHERE pin_list_id = %d AND password = %s);\n",
			listID, sqlString(r.PIN), desc, listID, sqlString(r.PIN))
		if n++; n%progressEvery == 0 {
			fmt.Fprintf(&b, "SELECT %d AS hecho;\n", n)
		}
	}
	if n%progressEvery != 0 {
		fmt.Fprintf(&b, "SELECT %d AS hecho;\n", n)
	}
	b.WriteString("COMMIT;\n")
	return b.String()
}

// progressLines passes each count InsertSQL prints (mysql --batch: a header line, then the number) to fn.
type progressLines struct {
	fn   func(done int)
	rest []byte
}

func (w *progressLines) Write(p []byte) (int, error) {
	w.rest = append(w.rest, p...)
	for {
		i := bytes.IndexByte(w.rest, '\n')
		if i < 0 {
			return len(p), nil
		}
		if n, err := strconv.Atoi(string(w.rest[:i])); err == nil {
			w.fn(n)
		}
		w.rest = w.rest[i+1:]
	}
}

var sqlEscaper = strings.NewReplacer(`\`, `\\`, `'`, `''`)

func sqlString(s string) string { return "'" + sqlEscaper.Replace(s) + "'" }

// Descriptions are stored without ñ or any accent (user rule; the real list has grave accents too)
// and without Excel's typographic characters.
var descFilter = strings.NewReplacer(
	"à", "a", "á", "a", "â", "a", "ã", "a", "ä", "a", "å", "a",
	"è", "e", "é", "e", "ê", "e", "ë", "e",
	"ì", "i", "í", "i", "î", "i", "ï", "i",
	"ò", "o", "ó", "o", "ô", "o", "õ", "o", "ö", "o",
	"ù", "u", "ú", "u", "û", "u", "ü", "u",
	"ý", "y", "ÿ", "y", "ñ", "n", "ç", "c",
	"À", "A", "Á", "A", "Â", "A", "Ã", "A", "Ä", "A", "Å", "A",
	"È", "E", "É", "E", "Ê", "E", "Ë", "E",
	"Ì", "I", "Í", "I", "Î", "I", "Ï", "I",
	"Ò", "O", "Ó", "O", "Ô", "O", "Õ", "O", "Ö", "O",
	"Ù", "U", "Ú", "U", "Û", "U", "Ü", "U",
	"Ý", "Y", "Ñ", "N", "Ç", "C",
	"\u2018", "'", "\u2019", "'", "\u201C", `"`, "\u201D", `"`,
	"\u2013", "-", "\u2014", "-", "\u2026", "...", "\u00A0", " ",
)

// filterDescription makes a description portal-safe. The portal refuses to save a list whose entry
// description is not "alphanumeric with dash and underscore" (spaces were accepted on the lab), so
// after the accent map quotes are dropped (O'Brien -> OBrien) and other ASCII punctuation becomes a
// space (Perez; Juan -> Perez Juan). Control and non-ASCII characters are kept so validation names them.
func filterDescription(s string) (string, bool) {
	var b strings.Builder
	for _, r := range descFilter.Replace(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == ' ':
			b.WriteRune(r)
		case r == '\'' || r == '"' || r == '`':
		case r > ' ' && r < 0x7F:
			b.WriteByte(' ')
		default:
			b.WriteRune(r)
		}
	}
	f := strings.Join(strings.Fields(b.String()), " ")
	return f, f != strings.TrimSpace(s)
}

// firstNonASCII returns the first character outside printable ASCII, or "".
func firstNonASCII(s string) string {
	for _, r := range s {
		if r < 0x20 || r > 0x7E {
			if r < 0x20 {
				return fmt.Sprintf("U+%04X", r)
			}
			return string(r)
		}
	}
	return ""
}

// decodeText: UTF-8 (BOM stripped) when valid, otherwise Windows-1252 (Excel's "CSV" on Spanish Windows).
func decodeText(raw []byte) (string, string) {
	if bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}) {
		return string(raw[3:]), "UTF-8"
	}
	if utf8.Valid(raw) {
		return string(raw), "UTF-8"
	}
	var b strings.Builder
	for _, c := range raw {
		switch {
		case c < 0x80:
			b.WriteByte(c)
		case c < 0xA0:
			b.WriteRune(cp1252[c-0x80])
		default:
			b.WriteRune(rune(c)) // 0xA0-0xFF match Latin-1
		}
	}
	return b.String(), "Windows-1252"
}

var cp1252 = [32]rune{
	0x20AC, 0xFFFD, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021, 0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0xFFFD, 0x017D, 0xFFFD,
	0xFFFD, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014, 0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0xFFFD, 0x017E, 0x0178,
}

// detectSeparator picks ;, , or tab by count outside quotes on the first non-empty line.
func detectSeparator(text string) (rune, string) {
	line := ""
	for _, l := range strings.Split(text, "\n") {
		if strings.TrimSpace(l) != "" {
			line = l
			break
		}
	}
	counts := map[rune]int{}
	inQuotes := false
	for _, r := range line {
		switch {
		case r == '"':
			inQuotes = !inQuotes
		case !inQuotes && (r == ';' || r == ',' || r == '\t'):
			counts[r]++
		}
	}
	best := ','
	for _, r := range []rune{';', '\t'} {
		if counts[r] > counts[best] {
			best = r
		}
	}
	if best == '\t' {
		return best, "tab"
	}
	return best, string(best)
}

// PinList is a row of ombu_pin_lists with its current entry count.
type PinList struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
	Entries     int    `json:"entries"`
}

func PinLists(ctx context.Context, p Profile, sec Secrets) ([]PinList, error) {
	rows, err := MySQL(ctx, p, sec, "SELECT l.pin_list_id AS id, l.description AS description, COUNT(e.pin_list_entry_id) AS entries "+
		"FROM ombu_pin_lists l LEFT JOIN ombu_pin_list_entries e ON e.pin_list_id = l.pin_list_id "+
		"GROUP BY l.pin_list_id, l.description ORDER BY l.pin_list_id;")
	if err != nil {
		return nil, err
	}
	lists := make([]PinList, 0, len(rows))
	for _, r := range rows {
		id, _ := strconv.Atoi(r["id"])
		n, _ := strconv.Atoi(r["entries"])
		lists = append(lists, PinList{ID: id, Description: r["description"], Entries: n})
	}
	return lists, nil
}

// PinEntries returns PIN -> description for one list.
func PinEntries(ctx context.Context, p Profile, sec Secrets, listID int) (map[string]string, error) {
	rows, err := MySQL(ctx, p, sec, fmt.Sprintf("SELECT password AS pin, IFNULL(description, '') AS description FROM ombu_pin_list_entries WHERE pin_list_id = %d;", listID))
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(rows))
	for _, r := range rows {
		m[r["pin"]] = r["description"]
	}
	return m, nil
}

// ApplyPins runs the add-only transaction, then stamps each line from a read-back of the list.
// progress gets the number of new PINs written so far, while the transaction is still open.
func ApplyPins(ctx context.Context, p Profile, sec Secrets, listID int, rows []PinRow, progress func(done int)) []PinRow {
	out := append([]PinRow(nil), rows...)
	err := SSHStream(ctx, p, sec.SSH, sec.SSHKey, mysqlCmd, strings.NewReader(InsertSQL(listID, rows)), &progressLines{fn: progress})
	if err != nil {
		err = fmt.Errorf("MySQL: %w", err)
	}
	var after map[string]string
	verifyErr := error(nil)
	if err == nil {
		after, verifyErr = PinEntries(ctx, p, sec, listID)
	}
	for i := range out {
		switch out[i].Status {
		case PinExists:
			out[i].Status, out[i].Error = PinSkipped, "ya existía"
		case PinNoDesc:
			out[i].Status, out[i].Error = PinSkipped, "sin descripción"
		case PinNew:
			_, landed := after[out[i].PIN]
			switch {
			case err != nil:
				out[i].Status, out[i].Error = PinFailed, Describe(err)
			case verifyErr != nil:
				out[i].Status, out[i].Error = PinFailed, "no se pudo verificar: "+Describe(verifyErr)
			case !landed:
				out[i].Status, out[i].Error = PinFailed, "no se encontró después de aplicar"
			default:
				out[i].Status = PinApplied
			}
		}
	}
	return out
}

// PortalPending reports whether the portal holds saved-but-not-applied changes (its Apply banner):
// the ombutel module's "reload" setting is "yes". A portal Apply pushes all of them, not only F-02's.
func PortalPending(ctx context.Context, p Profile, sec Secrets) (bool, error) {
	rows, err := MySQL(ctx, p, sec, "SELECT s.value AS value FROM ombu_settings s JOIN ombu_modules m ON m.module_id = s.module_id WHERE m.name = 'ombutel' AND s.name = 'reload';")
	if err != nil {
		return false, err
	}
	return len(rows) > 0 && rows[0]["value"] == "yes", nil
}
