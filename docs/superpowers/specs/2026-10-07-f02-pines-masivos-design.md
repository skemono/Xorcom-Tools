# F-02 PINes masivos: bulk PIN load into a CompletePBX 5 PIN list

Date: 2026-10-07 · Status: design approved in conversation, pending spec review · Builds on: [shell + F-01 spec](2026-10-07-utilidades-xorcom-design.md), [DESIGN.md](../../../DESIGN.md), [PRODUCT.md](../../../PRODUCT.md)
Process source: `docs/tool_processes/GuiaPinMasivo.pdf` (manual SQL method).

## 1. Goal and scope

Replace the guide's manual steps 2–4 (find `pin_list_id` over SSH, build a CSV, upload it and run `LOAD DATA LOCAL INFILE`) with one form: pick the PIN list on the active PBX, load a CSV, review a canary preview, press **Aplicar**, read a stamp per line.

Decided with the user:
- Source: a CSV file (Excel "Guardar como CSV"). No `.xlsx`.
- The PIN list itself is still created by hand in the portal (guide step 1); F-02 only picks an existing list.
- Write mode: **add only**. A PIN already in the list is skipped and reported; existing entries are never changed or deleted.
- Target: the active PBX profile only, one per run.

Out of scope until asked: creating lists, editing/deleting PINs, several PBXs per run, `.xlsx`, exporting results.

## 2. Facts confirmed on the lab box (2026-10-07, read-only)

| Item | Finding |
|---|---|
| DB | MariaDB 10.11.6, database `ombutel`, reached as SSH `root` via `mysql` socket auth (no DB password) |
| `ombu_pin_lists` | `pin_list_id int unsigned AUTO_INCREMENT PK`, `description varchar(255) NOT NULL UNIQUE` |
| `ombu_pin_list_entries` | `pin_list_entry_id int unsigned AUTO_INCREMENT PK`, `pin_list_id int unsigned NOT NULL` (FK → lists, `ON DELETE CASCADE`), `password varchar(255) NOT NULL`, `description varchar(255) NULL` |
| Duplicates | **No unique key on (pin_list_id, password)**: the DB accepts duplicate PINs, so F-02 must prevent them |
| Engine / charset | InnoDB (transactions roll back), `utf8mb3` (no 4-byte characters such as emoji) |
| `local_infile` | ON on the lab, but F-02 does not depend on it |
| Call path | The generated dialplan reads config from AstDB (`${DB(...)}`), which CompletePBX fills from MySQL. The code that renders PIN lists is not plain text on the box, and the lab had no PIN list or trunk group yet, so whether SQL-inserted PINs work **without pressing Apply in the portal** is still open (section 8). |

The F-01 connection test also passed against this box: portal login (API) and SSH both stamped CONECTADO, and the real Credential Manager path stored and deleted secrets correctly.

## 3. Flow on the F-02 sheet

1. **Lista.** F-02 reads `ombu_pin_lists` (with an entry count per list) from the active PBX; the user picks one. No lists → the sheet says to create one in the portal first.
2. **Archivo.** The user picks a CSV (native file dialog). F-02 shows what it detected: separator, encoding, header row yes/no, row count.
3. **Vista previa (canary copy).** "COPIA — VISTA PREVIA" strip; one numbered line per CSV row: Nº, state cell, PIN, Descripción, Resultado previsto (`Nuevo`, `Ya existe: se omite`, `Error: <motivo>`). Summary: "48 nuevos · 2 ya existen · 0 con error".
   - **Any error row blocks Aplicar** ("Corrija el archivo y vuelva a cargarlo"). Zero `Nuevo` lines also disables it.
4. **Aplicar.** Issues a folio, runs the insert (section 5), then **reads the list back** and stamps each line from the read-back, not from the request: violet `APLICADO`, quiet `OMITIDO` (already existed), red `FALLÓ`. The sheet turns from canary to the white working copy; failures turn pink.

Changing the active PBX or the list discards the preview.

## 4. CSV rules

- **Encoding:** UTF-8 BOM → UTF-8 (BOM stripped); valid UTF-8 → as is; otherwise Windows-1252 (Excel's "CSV" on Spanish Windows), so "José Pérez" survives.
- **Separator:** the first non-empty line decides: `;`, `,` or tab, whichever occurs most outside quotes. Parsed with `encoding/csv` (quoted fields with commas are fine). Blank lines are skipped.
- **Columns:** 2 (`PIN, descripción`) or 3 as in the guide (`pin_list_id, PIN, descripción`). Any other count is a file-level error. With 3 columns, a `pin_list_id` different from the chosen list is a row error.
- **Header:** the first row is a header when its PIN cell is not all digits.
- **Size guard:** files over 1 MB or 10 000 rows are refused (a hospital list is hundreds).

## 5. Validation (per row, all reported, nothing silently dropped)

| Field | Rule |
|---|---|
| PIN | required; digits `0-9` only; 1–255 characters after trimming spaces |
| Descripción | optional (stored as `NULL` when empty); ≤ 255 characters; no control characters or line breaks; no 4-byte characters (`utf8mb3`) |
| Duplicates in file | the same PIN on two rows → both rows are errors |
| Against the list | a PIN already in the chosen list → `Ya existe: se omite` (not an error) |

## 6. Write path (approach A)

Over the profile's SSH connection, F-02 runs `mysql --batch --default-character-set=utf8 ombutel` and sends SQL on **stdin**. Nothing is written to disk on the PBX, and no PIN appears in a command line.

```sql
START TRANSACTION;
INSERT INTO ombu_pin_list_entries (pin_list_id, password, description)
  SELECT 7, '4321', 'Dr. José Pérez' FROM DUAL
  WHERE NOT EXISTS (SELECT 1 FROM ombu_pin_list_entries WHERE pin_list_id = 7 AND password = '4321');
-- … one statement per Nuevo line …
COMMIT;
```

- String literals escape `\` → `\\` and `'` → `''`; control characters never reach SQL (validation rejects them); ids and PINs are digits only.
- `WHERE NOT EXISTS` keeps the add-only promise even if someone added the same PIN between preview and Aplicar.
- Any SQL error aborts the transaction (InnoDB rollback); every line then stamps `FALLÓ` with the MySQL message.
- Reads (`SELECT … FROM ombu_pin_lists`, entries of one list) use the same channel; mysql `--batch` output is tab-separated with `\t \n \\ \0` escapes, which the parser undoes.

## 7. Code

| Unit | Responsibility |
|---|---|
| `pbx/ssh.go` | add `SSHExec(ctx, p, password, keyPassphrase, cmd string, stdin io.Reader) (stdout string, err error)`; stderr goes into the error. `SSHRun` stays for F-01. |
| `pbx/mysql.go` | `MySQL(ctx, p, sec, sql) ([]map[string]string, error)`: runs the batch client over `SSHExec`, parses and unescapes TSV with header |
| `pbx/pins.go` | pure: `DecodeCSV`, `ParsePinRows`, `ValidatePins`, `PlanPins(rows, existing)`, `InsertSQL(listID, lines)`; plus `PinLists`, `PinEntries` reading through `MySQL` |
| `pins.go` | bound `PinService`: `Lists()`, `PickCSV()` (Wails open-file dialog, `*.csv`), `Preview(listID, path)`, `Apply()` applying exactly the last preview (kept in Go memory, tied to profile id + list id) |
| `profiles.go` | small unexported helper giving `PinService` the active profile and its secrets |
| `frontend/src/modules/Pines.svelte` | the F-02 form; registry line `{ code: 'F-02', id: 'pines', label: 'PINes masivos' }` |

UI extends the established Boleta world per DESIGN.md (no new identity): first build of the canary preview copy, `Actual │ Nuevo` is not needed here (add-only), fixed state cells per line, columns never reflow, stamps per line, pink failure copy. Large tables scroll inside the sheet; the sticky action bar keeps Aplicar in reach.

## 8. Open decision resolved by plan step 1: is a portal Apply needed?

Procedure on the lab box (writes only with the user's go-ahead):
1. The user creates a PIN list in the portal and assigns it to a trunk group, then presses Apply (baseline).
2. F-02's SQL path inserts one test PIN.
3. Read-only checks over SSH: does the PIN appear in AstDB (`asterisk -rx "database show"`) or the generated `/etc/asterisk/ombutel/*.conf`, before and after the user presses Apply again? A test call with the PIN settles any doubt.
4. The test PIN is deleted afterwards.

Outcomes:
- **Live** (works without Apply): nothing more to build.
- **Apply needed:** after a successful Aplicar, F-02 shows a canary notice "Falta aplicar cambios en el portal para que los PINes funcionen" with a button that opens the portal (`APIBase()` URL) in the browser. Automating the portal's apply call is a later step, once its endpoint is known.

## 9. Error handling

- SSH/mysql unreachable or rejected: the step's area shows the F-01-style Spanish message; nothing is written.
- File unreadable, wrong column count, too large: file-level error, no preview.
- Row errors: listed in the preview, Aplicar blocked.
- Apply failure: transaction rolled back, every line `FALLÓ` with the reason; preview kept so the user can retry.
- Read-back mismatch (a `Nuevo` line not found after commit): that line `FALLÓ` "no se encontró después de aplicar".

## 10. Testing

- Unit (`pbx`): CSV decoding (UTF-8 with/without BOM, Windows-1252 "José", `;`/`,`/tab, quoted commas, header/no header, 2 and 3 columns, blank lines), validation table, duplicate detection, planning against existing entries, SQL builder (escaping of `'` and `\`, NOT EXISTS guard), TSV parsing/unescaping.
- Scripted UI check (headless Edge, server mode): preview renders, error rows block Aplicar, list/PBX change discards the preview.
- Lab box: section 8 procedure, then one real Aplicar of a small CSV with the user's go-ahead, read-back verified in the portal, test entries removed.
