# F-03 Devolución de llamada: automated queue callback on CompletePBX 5

Date: 2026-10-15 · Status: design approved in conversation, pending spec review · Builds on: [shell + F-01 spec](2026-10-07-utilidades-xorcom-design.md), [F-02 spec](2026-10-07-f02-pines-masivos-design.md), [DESIGN.md](../../../DESIGN.md), [PRODUCT.md](../../../PRODUCT.md)
Process source: [Xorcom, "How to Configure Automated Queue Callback"](https://xorcom.com/how-to-configure-automated-queue-callback/) (Nov 2024, priority section added Oct 2026).

## 1. Goal and scope

A caller waiting in a queue presses a key and hangs up. When their turn comes, the PBX calls them back and puts them at the head of the queue. F-03 installs this on the active PBX in one previewed **Aplicar**, and removes it in one previewed **Quitar**.

Decided with the user:
- Only for PBXs with the **Call Center module** (detection in section 3).
- Follows the article, including **one Queue Priority + one custom app per queue**, with these changes: digits-only caller check, straight quotes and `printf`, an optional confirmation recording, and a dial prefix setting checked against the outbound routes.
- The article's release rule is kept as is (section 5), and so is its `DBdeltree`.
- The portal objects are written through the **portal API** (the same AJAX calls the browser makes on Save), never by SQL. Files go over SSH.
- v1 includes **Quitar** (uninstall) and the **confirmation message**.
- A queue that already has a breakout IVR not made by F-03 is never changed.

Out of scope until asked: recording audio (the "presione 0" prompt goes in the queue's periodic announcement, recorded in the portal), several PBXs per run, changing the retry or priority values per queue, any report on callbacks.

## 2. Facts confirmed on the IGSS CC test box (2026-10-15, read-only)

| Item | Finding |
|---|---|
| Platform | Debian 12, Asterisk 20.19.0.xor1-4, AstDB instance id read from `${DB(pbx/instance)}` |
| Includes | `/etc/asterisk/extensions.conf`: `#tryinclude ombutel/extensions__*.conf`. No `*custom*` file exists, so F-03 owns `extensions__65-callback.conf` alone |
| Queues | `ombu_queues`: 7510 "Cola CC", 75102 "Cola SF", both `ivr_id` NULL; `ombu_queue_params` (CUSTOM tab: `queue_id, name, value`, unique per name) empty |
| Destinations | polymorphic `ombu_destinations(destination_id, category_id, module_id, index)`; no custom-app or queue-priority rows exist to copy the shape from |
| Other tables | `ombu_custom_applications(extension UNIQUE, description, status, direct, destination_id, return_destination_id)`, `ombu_ivrs`, `ombu_ivr_entries(ivr_id, option, enabled, sort, destination_id)`, `ombu_queue_priorities(description, priority, destination_id)`, `ombu_custom_contexts(context, extension, priority)` |
| Modules | `queues`(21), `queues_priority`(22), `custom_app`(9), `ivr`(31), and the Call Center add-on: `supervision`(159), `statexplorer`(156) |
| Rendered queue dialplan | `[ext-queues]` sets `__QUEUE_NUMBER=<n>` and calls `Queue(Q<n>,b(...),,,30,,,)`; `queues__50-1-main.conf` holds `[Q<n>]` |
| Paths | `/var/log/cpbx` is `root:root 755` (the log needs owner `asterisk`); `/var/spool/asterisk/outgoing` is `asterisk:asterisk` |
| Portal Apply | `ombutel` setting `reload` = `no` (nothing outstanding on the day) |
| Destination categories | `ombu_destination_categories(category_id, module_id, name UNIQUE)`: `custom_application`=4, `queue`=10, `queue_priority`=11, `ivr`=15, `custom`=25 (resolved by **name** at runtime, never by id) |
| Recordings | `ombu_recordings(recording_id, name UNIQUE, …)`; the IVRs play them as `/var/lib/ombutel/static/<dir>/recordings/<md5(recording_id)>` (no extension), and there is exactly one `<dir>` |
| IVR entries | each entry runs `Gosub(<destination>)`, then the extension ends |
| `applications` context | `app-custom-application` and `app-queues-priority` are commented out while none exist; the portal turns them on when it renders the first one |
| Routing from `cos-all` | `cos-all-init` matches **every** number (`_[-+*#0-9a-zA-Z].`) and sets `__ARS=ARS-all`; the real outbound match is in `ARS-all` → `trk-group-1` (`_X.`) |
| AstDB | the `<instance>/callback/` family is unused (the only keys containing "callback" are the portal's `return_callback_prefix` and `transfer_callback`) |
| Asterisk user | `asterisk` (groups `asterisk`, `ombutel`) |

## 3. Detection

The sheet reads `ombu_modules`. Without `supervision` it shows "Esta PBX no tiene Call Center" and nothing else. *(Not yet seen on a box without CC: the user confirms that such a box lacks `supervision` before release.)*

F-03 recognizes its own objects by content, never by a local record:
- **callback app:** a custom app whose destination is Custom `create_callback,s,1`
- **callback IVR:** an IVR whose only entry goes to the callback app
- **priority app of queue N:** a custom app whose destination is a Queue Priority, whose destination is queue N, with description `Callback <N>`

## 4. Flow on the F-03 sheet

1. **Estado.** Per piece: dialplan file (missing, as expected, or different), log file, callback app, callback IVR. Per queue: IVR (none, ours, or a foreign one named), `membergosub`, Queue Priority, priority app, callbacks waiting.
2. **Ajustes** (defaults from the article):

   | Setting | Default |
   |---|---|
   | Key | `0` |
   | Callback app code | `799` |
   | Priority app codes | `797` for the first queue, then counting down, each checked to be free |
   | Priority | 10 |
   | Retries | 2 |
   | Retry interval | 1800 s |
   | Ring time | 30 s |
   | Name shown to the agent | `Call Back` |
   | Dial prefix | empty |
   | Confirmation recording | none, or one picked from the portal's recordings |
   | Test number for the route check | entered by the user |

3. **Colas.** A checklist. A queue with a foreign IVR is disabled with "ya tiene IVR «X»".
4. **Vista previa (canary).** Every change as **Actual │ Nuevo**:
   - the file (full content)
   - the log
   - the callback app and the IVR
   - per queue: its Queue Priority, its priority app, and IVR + `membergosub`

   Pieces already in place read "ya instalado: se omite". The **route check** runs `dialplan show <prefijo><número de prueba>@ARS-all`; it passes only when the match is in a `trk-group-*` context. `@cos-all` is useless for this, because its catch-all matches every number. If no outbound route matches, Aplicar is blocked: "Ninguna ruta de salida acepta <número>". The test call in section 11 is the final proof.
5. **Aplicar.** Issues a folio, runs section 6 in order, then stamps each line APLICADO, OMITIDO or FALLÓ from the read-back, not from the request.
6. **Quitar.** The same preview → confirm → read-back flow, in the order of section 7.
7. **Aplicar cambios en la PBX.** The portal Apply, two-step, the same as F-02.

Changing the active PBX, the settings, or the selected queues discards the preview.

## 5. The dialplan file

`/etc/asterisk/ombutel/extensions__65-callback.conf`. F-03 regenerates the whole file on every Aplicar and Quitar. Values in `«»` come from the settings or from the plan.

```
; Utilidades XORCOM F-03 · devolución de llamada en cola. Generado: no editar a mano.

[create_callback]
exten => s,1,NoOp(Callback request for queue ${QUEUE_NUMBER})
 same => n,Set(num=${CALLERID(num,${CURRENT_EXTERNAL_CID})})
 same => n,GotoIf($["${num}"="" | "${FILTER(0-9,${num})}"!="${num}" | "${QUEUE_NUMBER}"=""]?done)
 same => n,Set(id=${MASTER_CHANNEL(UNIQUEID)})
 same => n,Set(DB(${DB(pbx/instance)}/callback/${QUEUE_NUMBER}/${id})=${num})
 same => n,System(printf 'Channel: Local/«prefijo»${num}@cos-all\nWaitTime: «ring»\nExtension: s\nContext: callback_out\nMaxRetries: «retries»\nRetryTime: «retry»\nSetvar: CHANNEL(language)=${CHANNEL(language)}\nSetvar: __QUEUE_NUMBER=${QUEUE_NUMBER}\nSetvar: __ORIGINAL_INTERNAL_CID="«nombre»" <${num}>\n' > ${ASTSPOOLDIR}/${id}.call)
 same => n,Set(FILE(/var/log/cpbx/callback.log,,,al)=${STRFTIME(${EPOCH},,%F %T)},${id},${QUEUE_NUMBER},${num})
 same => n,Playback(«grabación»)                   ; only when a recording is chosen
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
 same => n,GotoIf($["${QUEUE_NUMBER}"="«cola»"]?cos-all,«app de prioridad»,1)   ; one line per queue
 same => n,Hangup()
```

Why each change from the article:
- **Digits-only check.** The caller's number ends up in a shell command (`System`), and a trunk caller ID is outside our control. Hidden or blank numbers cannot be called back, so they just hang up.
- **Straight quotes, `printf`.** The article's `”` quotes break the shell. `printf` reads `\n` the same way in dash and bash.
- **`Return()` kept from the article.** The portal runs the context with `Gosub(create_callback,s,1)` and then its own `Gosub(app-termination,hangup,1)` cleanup (confirmed on the test box), and its form says to end a custom context with `Return()`. The optional confirmation plays right before it.
- **Confirmation playback.** Optional. Without it, the call just ends, as in the article.
- **Prefix and route check.** The callback dials the caller ID exactly as the trunk delivered it. If the outbound routes expect something else, every callback would fail silently.
- **`%T` instead of `%r`.** 24-hour time in the log.

Settings validation:
- prefix: digits only, 0–6 characters
- name: letters, digits, space
- ring, retries, retry: whole numbers in range
- the recording comes from the portal's list

The generator is a pure function with a reference-copy unit test.

**Known limits, inherited from the article and accepted:**
- A callback is released only when an agent answers a caller who arrived after it. On a quiet queue it waits.
- `DB_KEYS` sorts as text, so two requests within the same second can leave in the wrong order.
- A customer who never answers is retried `«retries»` times, and then the callback is dropped.

## 6. Install (Aplicar)

The order is built so the step that touches live calls comes last:

1. **Files over SSH.**
   - The file content travels on stdin to a temp file, then `install -o asterisk -g asterisk -m 644` and `mv` into place.
   - The log is created only if missing, with 2 header lines (the article's rule), owner `asterisk`.
   - `asterisk -rx "dialplan reload"`.
   - Read-back: `dialplan show create_callback` / `check_position` / `callback_out` must each list their context.
2. **Callback app** (if missing): Custom → `create_callback,s,1`, enabled, direct to destination, code from the settings.
3. **Callback IVR** (if missing): one entry, the key → the callback app.
4. **Per selected queue:**
   1. the Queue Priority `Callback <N>` (priority 10 → queue N)
   2. the priority app `Callback <N>` → that priority
   3. **the queue:** IVR = callback IVR and `membergosub` = `check_position`

   The priority app codes are chosen in the preview, so the file written in step 1 already holds every selected queue's `callback_out` line. A line is harmless until its queue is attached in step iii. The queue form is **read-modify-write**: F-03 loads the portal's current form for that queue, changes only the IVR field and the `membergosub` row, and posts everything else back as it was. Before vs after, every `ombu_queues` column (except `ivr_id`) and every other `ombu_queue_params` row must be identical. Any difference = FALLÓ, in red.
5. **Read-back** of every piece from MySQL and Asterisk (`queue show Q<N>` and the rendered `queues__50-1-main.conf` after the portal Apply).

If a step fails, F-03 stops and stamps the rest "no se ejecutó". Nothing is rolled back automatically: Estado shows each piece, a second Aplicar continues where the first stopped, and Quitar cleans up. Timeouts: reads 60 s; Aplicar 5 min.

## 7. Uninstall (Quitar)

Per selected queue, in this order:
1. The queue: IVR emptied and the `membergosub` row removed, read-modify-write with the same before-vs-after check.
2. Its priority app.
3. Its Queue Priority.
4. The file is regenerated without its `callback_out` line, then `dialplan reload`.

When no queue keeps callback, the callback IVR, the callback app and the file are deleted too, then `dialplan reload`. The log is kept.

Callbacks waiting for those queues (AstDB `…/callback/<N>/*` plus `${ASTSPOOLDIR}/<id>.call`) are listed in the preview with the masked number and the time. A toggle, off by default, chooses whether to drop them (`database deltree` + `rm` of those exact files).

## 8. Portal API

The portal API works the same as the F-01 firewall notes (`docs/reference/CPBX5_CONNECTION.md`):
- logged-in session with the `sid` cookie and `X-Requested-With`
- `class` + `method` posted to the root URL
- `state == "success"` checked, `notification.text` shown on failure
- new objects take `#<unix-ms>` IDs
- forms are full replaces, so they are always read first

The exact class, method and fields for `custom_app`, `ivr`, `queues_priority` and `queues` (and their delete calls) are **recorded in plan step 1** and written into this section before any code uses them.

## 9. Plan step 1: hand-made setup on the test box (writes, agreed with the user)

Only on IGSS CC, and only with **new** objects that are easy to delete. Queues 7510 and 75102 are never touched.

1. Snapshot (read-only) of 7510/75102: `ombu_queues` rows, params, `queues__50-1-main.conf`.
2. In the portal, with Chrome recording the network requests, create:
   - queue **7599 "PRUEBA CALLBACK"** (member 2000)
   - custom app **799 "PRUEBA Callback"** → Custom `create_callback,s,1`
   - IVR **"PRUEBA Callback"**, key 0 → that app
   - queue 7599: IVR + CUSTOM `membergosub=check_position`
   - Queue Priority **"PRUEBA Callback 7599"**
   - custom app **797 "PRUEBA Callback 7599"**

   The dialplan file from section 5 goes in by hand. Then the portal Apply.
3. Settle the open facts (the routing, recording and AstDB facts are already settled in section 2):
   - the request shape of each form and of its delete
   - the queue IVR renders as `context=` in `[Q7599]`
   - `membergosub` renders in `[Q7599]`
   - `cos-all` reaches `7599` and `797` once the portal renders them
   - the `.call` file's `Setvar: CHANNEL(language)` works
   - the format in which the trunk delivers caller IDs, and the prefix the routes need
4. Test calls on 7599 (section 11), then delete everything from step 2 and compare against the snapshot.
5. Record the results in sections 2, 5 and 8 before writing code.

## 10. Code

| Unit | Responsibility |
|---|---|
| `pbx/callback.go` | pure: `CallbackDialplan(cfg, queues)`, `PlanCallback(state, cfg, selected)` → pieces with Actual/Nuevo/status, settings validation; plus `CallbackState` reads (SQL + SSH) and the install/uninstall steps |
| `pbx/api.go` | the portal form helpers: read a form, post a full form, delete; `PortalApplyChanges` stays |
| `pbx/callback_test.go` | dialplan reference copy, digits-only check, planning (foreign IVR, already installed, code collisions), queue form round-trip keeps every untouched field |
| `callback.go` | bound `CallbackService`: `State()`, `Preview(cfg, queues)`, `Apply(...)` / `Remove(...)` running exactly the last preview (tied to profile id + settings + queues), `ApplyPortal()`, `PendingPortalChanges()` |
| `main.go` | one `application.NewService` line |
| `frontend/src/modules/Callback.svelte` | the F-03 sheet; registry line `{ id: 'callback', label: 'Devolución de llamada', … }` |

The UI follows DESIGN.md: canary preview, `Actual │ Nuevo`, per-line stamps, pink failure copy, sticky action bar. No new identity.

## 11. Testing

- **Unit** (`go test ./pbx/...`): as in section 10.
- **Scripted UI check:**
  - no CC → only the notice
  - a foreign IVR disables its queue
  - a failed route check blocks Aplicar
  - changing PBX, settings or queues discards the preview
- **Test box, queue 7599 only**, with the 7510/75102 snapshot compared after each step:
  1. F-03 Aplicar on 7599 → every line APLICADO; a second Aplicar → all OMITIDO.
  2. Call 7599 from outside, press 0:
     - the confirmation plays
     - a line appears in `callback.log`
     - the AstDB key and `${ASTSPOOLDIR}/<id>.call` exist
  3. A later caller is answered by 2000 → the `.call` file moves to `outgoing/`, the outside number rings (through the prefix and route), and on answer it enters 7599 ahead of the others.
  4. F-03 Quitar → every piece removed, the box matches the snapshot.
- **User manual:** a simple Spanish manual (Claude Doc) for the release, per the team's rule.
