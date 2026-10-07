package pbx

import (
	"bytes"
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
	for _, rc := range recs {
		info.Columns = max(info.Columns, len(rc.fields))
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
		row.Description, row.Filtered = filterDescription(field(rc.fields, pinCol+1))
		switch {
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

// InsertSQL is one transaction; NOT EXISTS keeps it add-only even if a PIN appeared after the preview.
func InsertSQL(listID int, rows []PinRow) string {
	var b strings.Builder
	b.WriteString("START TRANSACTION;\n")
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
	}
	b.WriteString("COMMIT;\n")
	return b.String()
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

func filterDescription(s string) (string, bool) {
	f := strings.TrimSpace(descFilter.Replace(s))
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
