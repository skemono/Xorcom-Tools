#!/bin/bash
# Relevamiento previo a F-03 (devolución de llamada) en una CompletePBX 5. SOLO LECTURA.
# No escribe archivos ni en la base, no recarga Asterisk: solo SELECT, comandos "show" y lecturas.
# Los números de clientes salen enmascarados y nunca se imprimen contraseñas.
#
# Uso (desde la PC del técnico, la salida queda en la PC):
#   ssh root@<ip> 'bash -s' < f03-relevamiento.sh > reporte-f03.txt
# Con números de prueba para revisar la ruta saliente (tal como los marcaría la devolución):
#   ssh root@<ip> 'bash -s -- 55551234 0055551234' < f03-relevamiento.sh > reporte-f03.txt

ax()   { asterisk -rx "$1" 2>&1; }
sql()  { mysql --batch --default-character-set=utf8 ombutel 2>&1 <<<"$1"; }
cdr()  { mysql --batch --default-character-set=utf8 asterisk 2>&1 <<<"$1"; }
sec()  { printf '\n===== %s =====\n' "$1"; }
mask() { sed -E 's/([0-9]{2})[0-9]{3,}([0-9]{2})/\1***\2/g'; }   # 7+ dígitos: deja 2 al inicio y 2 al final
nopw() { grep -v -i -E 'pass|secret|md5_cred|auth_key'; }

sec "1. Planta"
date '+%F %T %Z'; hostname; uptime
cat /etc/debian_version 2>&1
ax 'core show version'
dpkg -l 2>/dev/null | grep -i -E 'ombutel|completepbx|xorcom' | awk '{print $2, $3}'

sec "2. Recursos"
nproc; free -m; df -h / /var 2>&1

sec "3. Módulos (Call Center = supervision)"
sql "SELECT name FROM ombu_modules WHERE name IN ('supervision','statexplorer','queues','queues_priority','custom_app','ivr') ORDER BY name;"

sec "4. Cambios pendientes en el portal (reload = yes: alguien dejó cambios sin Aplicar)"
sql "SELECT s.value AS reload FROM ombu_settings s JOIN ombu_modules m ON m.module_id = s.module_id WHERE m.name = 'ombutel' AND s.name = 'reload';"

sec "5. Includes y archivos de dialplan"
grep -n -i include /etc/asterisk/extensions.conf 2>&1
ls -la /etc/asterisk/ombutel/extensions__*.conf 2>&1
ls -la /etc/asterisk/*custom* 2>/dev/null || echo "(sin archivos *custom* en /etc/asterisk)"
for f in /etc/asterisk/ombutel/extensions__6*.conf /etc/asterisk/ombutel/extensions__[7-9]*.conf; do
  [ -e "$f" ] || continue
  echo "--- contextos en $f"; grep -n '^\[' "$f"
done
for f in /etc/asterisk/ombutel/extensions__65-*.conf; do
  [ -e "$f" ] || continue
  echo "--- contenido de $f (dialplan hecho a mano)"; nopw < "$f"
done

sec "6. ¿F-03 ya instalado o a medias?"
for c in create_callback check_position callback_out; do
  ax "dialplan show $c" | grep -q "Context '$c'" && echo "contexto cargado: $c" || echo "sin contexto: $c"
done
ls -la /etc/asterisk/ombutel/extensions__65-callback.conf 2>/dev/null || echo "sin extensions__65-callback.conf"
i=$(ax 'database get pbx instance' | sed -n 's/^Value: //p')
echo "instancia AstDB: ${i:-(no encontrada)}"
[ -n "$i" ] && ax "database show $i/callback" | mask
ls -la /var/log/cpbx/ 2>&1
echo ".call en espera: $(ls /var/spool/asterisk/*.call 2>/dev/null | wc -l)  · en outgoing: $(ls /var/spool/asterisk/outgoing 2>/dev/null | wc -l)"
stat -c '%n %U:%G %a' /var/spool/asterisk/outgoing /var/log/cpbx 2>&1

sec "7. Colas (portal)"
sql "SELECT * FROM ombu_queues ORDER BY extension;"
sql "SELECT p.queue_id, q.extension, p.name, p.value FROM ombu_queue_params p JOIN ombu_queues q ON q.queue_id = p.queue_id ORDER BY q.extension, p.name;"
sql "SELECT q.extension, COUNT(m.queue_id) AS miembros FROM ombu_queues q LEFT JOIN ombu_queue_members m ON m.queue_id = q.queue_id GROUP BY q.extension ORDER BY q.extension;"
echo "--- colas generadas (IVR de salida = context, membergosub)"
grep -h -E '^\[Q|^ *(context|membergosub) *=' /etc/asterisk/ombutel/queues__*.conf 2>&1

sec "8. Colas (Asterisk en vivo: miembros, estado, en espera)"
ax 'queue show'

sec "9. IVR, apps personalizadas, prioridades de cola, contextos personalizados"
D="LEFT JOIN ombu_destination_categories c ON c.category_id = d.category_id"
sql "SELECT i.ivr_id, IFNULL(i.description,'') AS ivr, IFNULL(e.\`option\`,'') AS tecla, IFNULL(e.enabled,'') AS activa, IFNULL(c.name,'') AS destino, IFNULL(d.\`index\`,0) AS idx FROM ombu_ivrs i LEFT JOIN ombu_ivr_entries e ON e.ivr_id = i.ivr_id LEFT JOIN ombu_destinations d ON d.destination_id = e.destination_id $D ORDER BY i.ivr_id, e.sort;"
sql "SELECT a.extension, IFNULL(a.description,'') AS descripcion, IFNULL(c.name,'') AS destino, IFNULL(d.\`index\`,0) AS idx FROM ombu_custom_applications a LEFT JOIN ombu_destinations d ON d.destination_id = a.destination_id $D ORDER BY a.extension;"
sql "SELECT p.queue_priority_id, IFNULL(p.description,'') AS descripcion, p.priority, IFNULL(c.name,'') AS destino, IFNULL(d.\`index\`,0) AS idx FROM ombu_queue_priorities p LEFT JOIN ombu_destinations d ON d.destination_id = p.destination_id $D ORDER BY 1;"
sql "SELECT * FROM ombu_custom_contexts;"
sql "SELECT category_id, name FROM ombu_destination_categories ORDER BY name;"

sec "10. Códigos 790-799 ocupados (F-03 usa 799 y 797 hacia abajo)"
for c in 790 791 792 793 794 795 796 797 798 799; do
  ax "dialplan show $c@cos-all-local" | grep -q 'There is no existence' && echo "$c libre" || echo "$c OCUPADO"
done

sec "11. Grabaciones"
sql "SELECT recording_id, name FROM ombu_recordings ORDER BY name;"
ls -d /var/lib/ombutel/static/*/recordings 2>&1

sec "12. Enrutamiento saliente"
ax 'dialplan show' | grep -E "^\[ Context '(cos-|ARS-|trk-group)"
ax 'dialplan show ARS-all'
for t in $(ax 'dialplan show' | sed -n "s/^\[ Context '\(trk-group-[0-9]*\)'.*/\1/p" | sort -u); do
  echo "--- patrones de $t"; ax "dialplan show $t" | grep -E "^ *'|=>" | head -40
done
echo "--- rutas de entrada (DID -> destino)"
grep -v -i -E 'pass|secret' /etc/asterisk/ombutel/extensions__50-inbound-routes.conf 2>&1
[ $# -eq 0 ] && echo "(sin números de prueba: pasalos después de 'bash -s --')"
for n in "$@"; do
  n=$(printf '%s' "$n" | tr -cd '0-9')
  [ -n "$n" ] || continue
  echo "--- ruta para $(printf '%s' "$n" | mask)"
  ax "dialplan show $n@ARS-all" | grep -E "trk-group|There is no|=>" | head -5 | mask
done

sec "13. Troncales (sin contraseñas)"
ax 'module show like chan_' | grep -E 'chan_(sip|pjsip|dahdi|iax)'
if ax 'core show help pjsip show' | grep -q 'pjsip show endpoints'; then
  ax 'pjsip show registrations'
  ax 'pjsip show endpoints' | grep -E '^ *Endpoint:' | awk '{print $2}' | grep '[A-Za-z]' | cut -d/ -f1 | sort -u | while read -r e; do
    echo "--- $e"
    ax "pjsip show endpoint $e" | grep -i -E 'from_user|from_domain|send_rpid|send_pai|trust_id|dtmf_mode|callerid |context |allow |direct_media|rtp_symmetric' | nopw
  done
fi
if ax 'core show help sip show' | grep -q 'sip show peers'; then
  ax 'sip show registry'
  ax 'sip show peers' | awk 'NR > 1 {print $1}' | cut -d/ -f1 | grep -v -E '^[0-9]+-[a-z0-9]+$|^Name$' | sort -u | while read -r e; do   # sin los teléfonos (NNNN-xxxx)
    echo "--- $e"
    ax "sip show peer $e" | grep -i -E 'fromuser|fromdomain|sendrpid|trustrpid|dtmfmode|callerid|context|codecs|insecure|nat |status' | nopw
  done
fi
echo "--- troncales en el dialplan generado"
grep -v -i -E 'pass|secret' /etc/asterisk/ombutel/extensions__50-sip-trunks.conf 2>&1 | head -60

sec "14. Formato del número de quien llama a las colas (CDR, 7 días, enmascarado)"
cdr "SHOW TABLES LIKE 'cdr';"
cdr "SELECT LENGTH(src) AS largo, LEFT(src,1) AS primer_digito, COUNT(*) AS llamadas FROM cdr WHERE calldate > NOW() - INTERVAL 7 DAY AND lastapp = 'Queue' GROUP BY 1,2 ORDER BY 3 DESC LIMIT 15;"
cdr "SELECT COUNT(*) AS sin_numero_o_no_numerico FROM cdr WHERE calldate > NOW() - INTERVAL 7 DAY AND lastapp = 'Queue' AND src NOT REGEXP '^[0-9]+\$';"
cdr "SELECT CONCAT(LEFT(src,2),'***',RIGHT(src,2)) AS ejemplo, LENGTH(src) AS largo FROM cdr WHERE calldate > NOW() - INTERVAL 7 DAY AND lastapp = 'Queue' ORDER BY calldate DESC LIMIT 10;"

sec "15. Formato que marcan los usuarios hacia afuera (CDR, 7 días)"
cdr "SELECT LEFT(dst,2) AS empieza, LENGTH(dst) AS largo, COUNT(*) AS llamadas FROM cdr WHERE calldate > NOW() - INTERVAL 7 DAY AND lastapp = 'Dial' AND LENGTH(dst) >= 7 GROUP BY 1,2 ORDER BY 3 DESC LIMIT 15;"

sec "16. Tráfico ahora y por hora (para elegir la ventana de Aplicar)"
ax 'core show channels count'
cdr "SELECT NOW() AS ahora_mysql, MAX(calldate) AS ultima_llamada_cdr FROM cdr;"   # si difieren ~6 h, el CDR está en UTC
cdr "SELECT HOUR(calldate) AS hora, COUNT(*) AS llamadas FROM cdr WHERE calldate > NOW() - INTERVAL 7 DAY GROUP BY 1 ORDER BY 1;"

sec "FIN"
