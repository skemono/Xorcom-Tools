package pbx

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// CallbackConfig holds the F-03 settings. Defaults follow the Xorcom article.
type CallbackConfig struct {
	Key        string `json:"key"`        // the key a waiting caller presses
	AppCode    string `json:"appCode"`    // custom app that runs create_callback
	Priority   int    `json:"priority"`   // queue priority of a returned call
	Retries    int    `json:"retries"`    // .call MaxRetries
	RetrySecs  int    `json:"retrySecs"`  // .call RetryTime
	RingSecs   int    `json:"ringSecs"`   // .call WaitTime
	CallerName string `json:"callerName"` // caller ID name the agent sees
	Prefix     string `json:"prefix"`     // digits dialed before the caller's number
	Recording  int    `json:"recording"`  // ombu_recordings id played after the key, 0 = none
}

func DefaultCallbackConfig() CallbackConfig {
	return CallbackConfig{Key: "0", AppCode: "799", Priority: 10, Retries: 2, RetrySecs: 1800, RingSecs: 30, CallerName: "Call Back"}
}

var (
	reDigits     = regexp.MustCompile(`^[0-9]+$`)
	reCallerName = regexp.MustCompile(`^[A-Za-z0-9 ]{1,30}$`)
)

// Validate checks every value that ends up in the dialplan or a shell command. Messages are shown to the user.
func (c CallbackConfig) Validate() error {
	in := func(v, lo, hi int) bool { return v >= lo && v <= hi }
	switch {
	case len(c.Key) != 1 || !strings.Contains("0123456789*#", c.Key):
		return errors.New("la tecla debe ser un dígito, * o #")
	case !reDigits.MatchString(c.AppCode) || len(c.AppCode) > 8:
		return errors.New("el código de la aplicación debe tener de 1 a 8 dígitos")
	case !in(c.Priority, 1, 99):
		return errors.New("la prioridad debe estar entre 1 y 99")
	case !in(c.Retries, 0, 10):
		return errors.New("los reintentos deben estar entre 0 y 10")
	case !in(c.RetrySecs, 60, 86400):
		return errors.New("el intervalo entre reintentos debe estar entre 60 y 86 400 segundos")
	case !in(c.RingSecs, 10, 120):
		return errors.New("el tiempo de timbre debe estar entre 10 y 120 segundos")
	case !reCallerName.MatchString(c.CallerName):
		return errors.New("el nombre para el agente solo admite letras sin tilde, números y espacios (máx. 30)")
	case len(c.Prefix) > 6 || c.Prefix != "" && !reDigits.MatchString(c.Prefix):
		return errors.New("el prefijo solo admite de 0 a 6 dígitos")
	case c.Recording < 0:
		return errors.New("grabación inválida")
	}
	return nil
}

// CheckDialNumber validates the route-check number before it reaches an asterisk -rx command line.
func CheckDialNumber(n string) error {
	if !reDigits.MatchString(n) || len(n) < 3 || len(n) > 20 {
		return fmt.Errorf("el número de prueba «%s» debe tener de 3 a 20 dígitos", n)
	}
	return nil
}

// CallbackRoute sends a called-back customer of Queue to App, the custom app of its Queue Priority.
type CallbackRoute struct {
	Queue string `json:"queue"`
	App   string `json:"app"`
}

const callbackHeader = "; f03-config: "

// CallbackDialplan renders extensions__65-callback.conf. c must be valid; playback is a path from
// CallbackState.RecordingPath ("" = none); routes come from the plans (digits only). The settings travel
// in a header line, so Estado and Quitar can read back what was installed.
func CallbackDialplan(c CallbackConfig, playback string, routes []CallbackRoute) string {
	cfg, _ := json.Marshal(c)
	play := ""
	if playback != "" {
		play = " same => n,Playback(" + playback + ")\n"
	}
	var rt strings.Builder
	for _, r := range routes {
		fmt.Fprintf(&rt, " same => n,GotoIf($[\"${QUEUE_NUMBER}\"=\"%s\"]?cos-all,%s,1)\n", r.Queue, r.App)
	}
	return strings.NewReplacer(
		"«config»", string(cfg),
		"«prefix»", c.Prefix,
		"«ring»", strconv.Itoa(c.RingSecs),
		"«retries»", strconv.Itoa(c.Retries),
		"«retry»", strconv.Itoa(c.RetrySecs),
		"«name»", c.CallerName,
		"«playback»\n", play,
		"«routes»\n", rt.String(),
	).Replace(callbackTemplate)
}

// ParseCallbackConfig reads the settings back from an installed file; false when the header is
// missing or does not validate (a hand-edited or foreign file).
func ParseCallbackConfig(file string) (CallbackConfig, bool) {
	for _, line := range strings.Split(file, "\n") {
		if rest, ok := strings.CutPrefix(line, callbackHeader); ok {
			var c CallbackConfig
			if json.Unmarshal([]byte(rest), &c) != nil || c.Validate() != nil {
				return CallbackConfig{}, false
			}
			return c, true
		}
	}
	return CallbackConfig{}, false
}

// callbackTemplate is spec §5. The «» tokens are replaced by CallbackDialplan; everything else is literal
// Asterisk dialplan (the \n inside printf are for the shell, not Go).
const callbackTemplate = `; Utilidades XORCOM F-03 · devolución de llamada en cola. Generado: no editar a mano.
; f03-config: «config»

[create_callback]
exten => s,1,NoOp(Callback request for queue ${QUEUE_NUMBER})
 same => n,Set(num=${CALLERID(num,${CURRENT_EXTERNAL_CID})})
 same => n,GotoIf($["${num}"="" | "${FILTER(0-9,${num})}"!="${num}" | "${QUEUE_NUMBER}"=""]?done)
 same => n,Set(id=${MASTER_CHANNEL(UNIQUEID)})
 same => n,Set(DB(${DB(pbx/instance)}/callback/${QUEUE_NUMBER}/${id})=${num})
 same => n,System(printf 'Channel: Local/«prefix»${num}@cos-all\nCallerID: "«name»" <${QUEUE_NUMBER}>\nWaitTime: «ring»\nExtension: s\nContext: callback_out\nMaxRetries: «retries»\nRetryTime: «retry»\nSetvar: CHANNEL(language)=${CHANNEL(language)}\nSetvar: __QUEUE_NUMBER=${QUEUE_NUMBER}\nSetvar: __ORIGINAL_INTERNAL_CID="«name»" <${num}>\n' > ${ASTSPOOLDIR}/${id}.call)
 same => n,Set(FILE(/var/log/cpbx/callback.log,,,al)=${STRFTIME(${EPOCH},,%F %T)},${id},${QUEUE_NUMBER},${num})
«playback»
 same => n(done),Return()

[check_position]
exten => s,1,NoOp(Check position for queue ${QUEUE_NUMBER})
 same => n,Set(waiting_callbacks=${DB_KEYS(${DB(pbx/instance)}/callback/${QUEUE_NUMBER})})
 same => n,GotoIf(${ISNULL(${waiting_callbacks})}?:checkcallback)
 same => n,Return()
 same => n(checkcallback),Set(count=${DB_KEYCOUNT(${DB(pbx/instance)}/callback/${QUEUE_NUMBER})})
 same => n,Set(first=${CUT(waiting_callbacks,\,,1)})
 same => n,GotoIf($[$[${first}<${MASTER_CHANNEL(UNIQUEID)}]&&$[${count}>0]]?callback)
 same => n,Return()
 same => n(callback),System(mv ${ASTSPOOLDIR}/${first}.call ${ASTSPOOLDIR}/outgoing/${first}.call)
 same => n,DBdeltree(${DB(pbx/instance)}/callback/${QUEUE_NUMBER}/${first})
 same => n,Return()

[callback_out]
exten => s,1,NoOp(Callback answered, back to queue ${QUEUE_NUMBER})
 same => n,Set(CALLERID(all)=${ORIGINAL_INTERNAL_CID})
 same => n,Set(__CALL_DESTINATION=${CALLERID(num,${ORIGINAL_INTERNAL_CID})})
 same => n,Set(__CURRENT_INTERNAL_CID=${ORIGINAL_INTERNAL_CID})
«routes»
 same => n,Hangup()
`
