# Utilidades XORCOM: shell, F-01 Conexiones, auto-updater

Date: 2026-10-07 · Status: approved design, pending spec review · Product record: [PRODUCT.md](../../../PRODUCT.md)

## 1. Goal and scope

A Windows desktop app holding small automation tools ("formularios") for Xorcom CompletePBX 5. This spec covers the first build:

- the app shell (form pad sidebar, sheet header with the active PBX, update area);
- **F-01 Conexiones**: PBX profiles plus a per-channel connection test (API, SSH, AMI);
- the self-updater against GitHub Releases and the release workflow that feeds it.

Out of scope until a tool needs it: other tools, the preview/Aplicar write flow (designed in section 6, built with the first write tool), web UI automation, run history/audit log, dark theme, release signing.

## 2. Stack

| Piece | Version / choice |
|---|---|
| Go | 1.26.x (winget `GoLang.Go`) |
| Wails | v3 **v3.0.0-beta.28**, pinned in `go.mod` and for the `wails3` CLI |
| Frontend | Svelte + TypeScript (`svelte-ts` template), Vite, Node 24 |
| Target | Windows 11 (WebView2), amd64 |
| Go module | `github.com/skemono/Xorcom-Tools` |
| Remote | https://github.com/skemono/Xorcom-Tools (public) |
| App name | "Utilidades XORCOM", binary `UtilidadesXorcom.exe` |

Code, identifiers and comments in English; all UI copy in Spanish, inline in components (no i18n library).

## 3. Architecture

One Go service plus one Svelte component per tool, each listed in one registry on each side. Adding a tool = `foo.go` + `frontend/src/modules/Foo.svelte` + one line in `main.go` + one line in `modules.ts`.

```
main.go                     app, window, services list, version var
profiles.go                 ProfileService (bound)
updater.go                  UpdateService (bound)
pbx/
  profile.go                Profile types, JSON store, keyring secrets
  tlspin.go                 TLS verify-or-pin
  api.go                    CompletePBX HTTP client (reachability for now)
  ssh.go                    SSH client with host-key pinning
  ami.go                    AMI login/logoff over TCP
  pbx_test.go               store, AMI fake server, httptest API, TLS pin
frontend/src/
  App.svelte                shell: form pad, sheet header, update area
  modules.ts                [{ code, id, label, component }]
  modules/Conexiones.svelte F-01
  app.css                   tokens, fonts, rules
.github/workflows/release.yml
```

Final file names follow what `wails3 init` generates where it differs (e.g. its `main.go`, Taskfiles, `build/`); the split above is the target for our code.

## 4. Go services (bound to the frontend)

### ProfileService

| Method | Behavior |
|---|---|
| `List() (ProfilesView, error)` | profiles + active id; secrets never included, only `HasSecret` flags per channel |
| `Save(p Profile, s Secrets) (Profile, error)` | create (new id) or update; empty secret field = keep existing secret |
| `Delete(id string) error` | removes profile and its keyring entries |
| `SetActive(id string) error` | persists the active profile id |
| `Test(id string) []ChannelResult` | runs enabled channels concurrently, 10 s timeout each |
| `TrustFingerprint(id, channel, fp string) error` | stores the pin for `api` (cert) or `ssh` (host key) |

Later tools get the active profile through `ProfileService` (injected into their service struct in `main.go`).

### UpdateService

| Method | Behavior |
|---|---|
| `Version() string` | build version (`dev` for local builds) |
| `Check() (UpdateInfo, error)` | latest release vs current; `dev` builds never report updates |
| `Apply() error` | download, verify checksum, replace the running exe |
| `Restart()` | start the new exe, quit this one |

## 5. `pbx` package

### Profile

```go
type Profile struct {
    ID, Name, Host string
    API APIConfig // Enabled, BaseURL (default "http://"+Host: the CPBX5 portal is plain HTTP on 80), User (default admin), CertSHA256
    SSH SSHConfig // Enabled, Port (22), User, KeyPath (optional), HostKeySHA256
    AMI AMIConfig // Enabled, Port (5038), User
}
```

- **Store:** `%APPDATA%\UtilidadesXorcom\profiles.json` (`os.UserConfigDir()`), shape `{"active": "<id>", "profiles": [...]}`. Writes are atomic (temp file + rename).
- **Corrupt file:** rename it to `profiles.json.bad-<timestamp>`, start empty, and surface a message naming the backup. Never overwrite unreadable data.
- **Secrets:** Windows Credential Manager via `github.com/zalando/go-keyring`, service `UtilidadesXorcom`, key `<profileID>/<api|ssh|sshkey|ami>`. Secrets are write-only from the UI: never returned to the frontend.

### Trust on first use (API certificate, SSH host key)

- **TLS (`tlspin.go`):** if the certificate chain verifies against system roots for the host, accept. Otherwise accept only if SHA-256 of the leaf certificate equals `API.CertSHA256`. If no pin: fail with `UntrustedError{Fingerprint}`. If a pin exists but differs: fail with `UntrustedError{Fingerprint, Changed: true}`.
- **SSH:** `HostKeyCallback` compares `ssh.FingerprintSHA256(key)` to `SSH.HostKeySHA256` with the same untrusted/changed outcomes.
- A pin is only written by `TrustFingerprint`, i.e. by an explicit user click. A changed fingerprint shows a distinct warning ("La huella cambió: posible equipo reinstalado o interceptación").

### Channels and what "Probar conexión" proves

| Channel | Test | Success means |
|---|---|---|
| API | portal login `POST /login` (form `userid`, `userpass`, `baseurl`; `X-Requested-With: XMLHttpRequest`), HTTPS through the pinned TLS config; with no stored password only a GET of `BaseURL` | `state == "success"` and a `sid` cookie (or, without password, a response < 500) |
| SSH | connect + auth (password and/or key file) + run `uname -n` | login works; reports the hostname |
| AMI | TCP dial, read `Asterisk Call Manager/x` banner, `Action: Login`, then `Logoff` | `Response: Success` |

`ChannelResult{Channel, Skipped, OK, Message (Spanish), Millis, Fingerprint, FingerprintChanged}`. One channel failing never stops the others.

**Known limitations, stated in the UI where relevant:**
- The API check logs in to the portal but runs no module calls; the portal API (class/method posts to the root URL) is described in the local, git-ignored `docs/reference/CPBX5_CONNECTION.md`. The portal is plain HTTP: the admin password crosses the LAN in cleartext.
- AMI on 5038 is plaintext; the secret crosses the LAN. Mitigation is on the PBX side (AMI permit/deny to the technician subnet). AMI over TLS is a later option.

## 6. UI: "Boleta de trabajo"

Direction chosen in the impeccable decision round (seed `017e8d93`, assigned candidate 4 of 7): the app is a carbonless work-order pad. Mode: Operate. Build path: code-led (no image generation on this machine).

**Scene:** technicians at a desk or on a laptop in an equipment room, under office light. Light theme only.

**World**
- Copy colors carry state: white `#fbfcfd` (working copy), canary `#f6e27a` (preview), pink `#f4c6cf` (failure report).
- Printed spot ink `#233d7a` for rules and labels; data in black ink `#16181d`; red folio `#c8202f`; violet stamp ink `#5b3a9e`.
- Rules are pixel-snapped 1 px / 2 px lines. No drop shadows, no paper texture, no handwriting faces, no cream.
- Type (all OFL, bundled via `@fontsource/*`, works offline): labels in Barlow Semi Condensed 600 small caps with tracking; data in Barlow 400/500 with tabular figures; fingerprints and hashes only in JetBrains Mono. Final weights tuned during the build.

**Shell**
- **Form pad (left, ~232 px):** one row per tool: code + name (`F-01 Conexiones`). Footer: version and update state.
- **Sheet header (top of every form):** ruled boxes **PBX** (active profile picker), **Host**, **Folio** (red run number: a per-session counter that increments on each test or apply, "—" before the first), **Fecha** (today). The active PBX is never off-screen.
- **Sheet body:** ruled form fields; one sheet in focus. While a form is running, everything but the active sheet steps back.

**Write flow (for tools that change a PBX; designed now, built with the first such tool)**
- Preview = canary copy with a "COPIA — VISTA PREVIA" strip; numbered lines; `Actual │ Nuevo` registered on one baseline.
- Each line has a fixed state cell: hollow pending, filled applied, red strike failed. Columns never reflow.
- **Aplicar** stamps each line as results arrive: violet `APLICADO`, red `FALLÓ`. Failures collect on a pink copy.

**F-01 Conexiones**
- Profile list (add / edit / delete / set active) and the profile form: name, host, then one ruled section per channel with an enable checkbox.
- Password fields show "guardada" when a secret exists; typing replaces it.
- "Probar conexión": three cells API / SSH / AMI. Each shows "probando…", then lands a stamp: violet `CONECTADO` with message and ms, or red `FALLÓ` with the specific reason.
- Untrusted fingerprint: the cell shows the fingerprint (mono) and a "Confiar en esta huella" button; a changed fingerprint shows the warning from section 5.

**States:** no profiles (blank form inviting the first PBX); testing; per-channel ok / fail / skipped / untrusted / changed; update available / downloading / failed / up to date; corrupt profiles file recovered.

**Constraints:** fits 1366×768 with no horizontal scroll (window minimum 1024×680); full keyboard navigation with visible focus; WCAG AA contrast for text on every copy color; motion only for the stamp landing, disabled under `prefers-reduced-motion`.

**Finish:** per impeccable new-work section 5–7: direction contract comment at the top of the root markup, one inspection round plus at most one confirm round, detector run, finish reviewer, then DESIGN.md written by the documenter from the built app.

## 7. Auto-updater

- Library: `github.com/creativeprojects/go-selfupdate`, slug `skemono/Xorcom-Tools`, `ChecksumValidator` against `checksums.txt` (SHA-256) in each release.
- Version: `main.version` set by `-ldflags "-X main.version=vX.Y.Z"` in CI; local builds stay `dev` and never update.
- Startup: background `Check()`. Network errors are silent at startup and shown only on a manual "Buscar actualizaciones".
- Available: the form pad footer shows "Nueva versión vX disponible" with **Actualizar y reiniciar** → `Apply()` → `Restart()`.
- Release asset naming follows go-selfupdate's matching rules (`<name>_windows_amd64.zip`); confirm against the library docs when implementing.
- `// ponytail:` ceiling: checksums live in the same release, so a compromised GitHub account could ship a bad build. Upgrade path: sign `checksums.txt` with an offline key and switch to go-selfupdate's signature validator.

## 8. Release workflow

`.github/workflows/release.yml` on push of tag `v*`, `windows-latest`:
1. setup-go 1.26, setup-node 24, `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.28`;
2. production build with `-X main.version=${tag}` (wire the ldflag through the generated Taskfile);
3. zip the exe as the asset name from section 7, write `checksums.txt`;
4. `gh release create ${tag}` with both files.

## 9. Testing

- `go test ./pbx/...`:
  - store round-trip, active id, atomic write, corrupt-file recovery (temp dir);
  - AMI against a fake TCP server: success, bad secret, garbage banner;
  - API against `httptest.NewTLSServer`: untrusted → fingerprint returned; pinned → ok; pin mismatch → changed;
- `wails3 build` produces `UtilidadesXorcom.exe`; app launches in dev mode with the shell and F-01 rendering.
- SSH and keyring are verified manually against a real PBX (no fake SSH server, no keyring mocking).

## 10. Environment and risks

- Install: Go via winget, `wails3` via `go install` at the pinned version, then `wails3 doctor`.
- The project lives in OneDrive under a path with spaces. Builds may be slower and `npm install` can hit sync locks; if it breaks, move the repo to a local path such as `C:\dev\xorcom-tools`.
- Wails v3 is beta: the version is pinned; upgrades are deliberate.

## 11. Open decisions

- Portal API endpoints beyond login: described in the local reference (not committed: it holds credentials); confirm per firmware when the first API-writing tool is built.
- Which tools follow F-01.
- Release signing (section 7).
