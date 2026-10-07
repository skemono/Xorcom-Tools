package pbx

import (
	"context"
	"fmt"
	"strings"
)

// mysqlCmd reaches the CompletePBX database as root over the socket (no DB password).
const mysqlCmd = "mysql --batch --default-character-set=utf8 ombutel"

// MySQL sends sql on stdin and returns the rows of its single result set by column name.
// Keep one SELECT per call: --batch prints consecutive result sets with no separator.
func MySQL(ctx context.Context, p Profile, sec Secrets, sql string) ([]map[string]string, error) {
	out, err := SSHExec(ctx, p, sec.SSH, sec.SSHKey, mysqlCmd, strings.NewReader(sql))
	if err != nil {
		return nil, fmt.Errorf("MySQL: %w", err)
	}
	return parseBatch(out), nil
}

// parseBatch reads mysql --batch output: a tab-separated header line, then one line per row.
func parseBatch(out string) []map[string]string {
	out = strings.TrimRight(out, "\n")
	if out == "" {
		return nil
	}
	lines := strings.Split(out, "\n")
	cols := strings.Split(lines[0], "\t")
	rows := make([]map[string]string, 0, len(lines)-1)
	for _, line := range lines[1:] {
		fields := strings.Split(line, "\t")
		row := make(map[string]string, len(cols))
		for i, c := range cols {
			if i < len(fields) {
				row[c] = unescapeBatch(fields[i])
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// unescapeBatch undoes mysql --batch escaping: \t \n \\ \0.
func unescapeBatch(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i == len(s)-1 {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 't':
			b.WriteByte('\t')
		case 'n':
			b.WriteByte('\n')
		case '0':
			b.WriteByte(0)
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
