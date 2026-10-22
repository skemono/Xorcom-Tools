package pbx

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// verifyContexts fails unless Asterisk has loaded all three callback contexts (want=true) or none (want=false).
func verifyContexts(want bool) string {
	cond := `|| { echo "falta el contexto $c" >&2; exit 1; }`
	if !want {
		cond = `&& { echo "el contexto $c sigue cargado" >&2; exit 1; }`
	}
	return `for c in create_callback check_position callback_out; do asterisk -rx "dialplan show $c" | grep -q "Context '$c'" ` + cond + `; done; true`
}

// InstallCallbackFile writes content (on stdin) next to the target under a name the include glob
// skips, moves it into place, reloads the dialplan and proves the 3 contexts are loaded.
func InstallCallbackFile(ctx context.Context, p Profile, sec Secrets, content string) error {
	cmd := `set -e; t=$(mktemp); trap 'rm -f "$t"' EXIT; cat > "$t"; ` +
		`install -o asterisk -g asterisk -m 644 "$t" ` + callbackFile + `.f03tmp; ` +
		`mv -f ` + callbackFile + `.f03tmp ` + callbackFile + `; ` +
		`asterisk -rx 'dialplan reload' > /dev/null; ` + verifyContexts(true)
	if _, err := SSHExec(ctx, p, sec.SSH, sec.SSHKey, cmd, strings.NewReader(content)); err != nil {
		return fmt.Errorf("no se pudo instalar el archivo de devolución de llamada: %w", err)
	}
	return nil
}

func RemoveCallbackFile(ctx context.Context, p Profile, sec Secrets) error {
	cmd := `set -e; rm -f ` + callbackFile + `; asterisk -rx 'dialplan reload' > /dev/null; ` + verifyContexts(false)
	if _, err := SSHExec(ctx, p, sec.SSH, sec.SSHKey, cmd, nil); err != nil {
		return fmt.Errorf("no se pudo quitar el archivo de devolución de llamada: %w", err)
	}
	return nil
}

// EnsureCallbackLog creates the log with 2 header lines only when it is missing (the article: FILE()
// needs at least 2 lines), and gives it to asterisk. An existing log is never truncated.
func EnsureCallbackLog(ctx context.Context, p Profile, sec Secrets) error {
	cmd := `set -e; [ -e ` + callbackLog + ` ] || printf '# Utilidades XORCOM F-03: devoluciones de llamada\n# fecha,id,cola,numero\n' > ` + callbackLog +
		`; chown asterisk:asterisk ` + callbackLog + `; chmod 644 ` + callbackLog
	if _, err := SSHExec(ctx, p, sec.SSH, sec.SSHKey, cmd, nil); err != nil {
		return fmt.Errorf("no se pudo preparar el registro: %w", err)
	}
	return nil
}

// DropCallbacks discards the callbacks waiting on queue: their AstDB keys and their .call files
// still in the spool root (files already in outgoing/ belong to Asterisk and are left alone).
func DropCallbacks(ctx context.Context, p Profile, sec Secrets, queue string) error {
	if !reDigits.MatchString(queue) {
		return fmt.Errorf("cola inválida «%s»", queue)
	}
	cmd := `set -e; for id in $(asterisk -rx "database show f03callback/` + queue + `" | sed -n 's#^/f03callback/` + queue + `/\([0-9]*\.[0-9]*\) .*#\1#p'); do ` +
		`rm -f "/var/spool/asterisk/$id.call"; done; asterisk -rx "database deltree f03callback/` + queue + `" > /dev/null`
	if _, err := SSHExec(ctx, p, sec.SSH, sec.SSHKey, cmd, nil); err != nil {
		return fmt.Errorf("no se pudieron descartar las devoluciones pendientes: %w", err)
	}
	return nil
}

var reTrunkMatch = regexp.MustCompile(`(?i)context '(trk-group-[^']+)'`)

// RouteCheck asks Asterisk whether an outbound route of cos-all (its ARS-all) accepts number, and returns
// the trunk group context that does. cos-all itself is useless here: its catch-all matches every number.
// number is the full dialed number (prefix + test number, each checked by the caller); digits only reach the shell.
func RouteCheck(ctx context.Context, p Profile, sec Secrets, number string) (string, error) {
	if !reDigits.MatchString(number) || len(number) > 26 {
		return "", fmt.Errorf("el número marcado «%s» no es válido", number)
	}
	out, err := SSHExec(ctx, p, sec.SSH, sec.SSHKey, `asterisk -rx "dialplan show `+number+`@ARS-all"`, nil)
	if err != nil {
		return "", fmt.Errorf("no se pudo revisar las rutas de salida: %w", err)
	}
	if m := reTrunkMatch.FindStringSubmatch(out); m != nil {
		return m[1], nil
	}
	return "", fmt.Errorf("ninguna ruta de salida acepta el número marcado %s: revise el prefijo", number)
}
