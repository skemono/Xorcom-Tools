# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

(Wails desktop app: the UI renders in WebView2 on Windows; design language is web, not native Win32.)

## Stack

Go 1.26 + Wails v3 (v3.0.0-beta.28, user chose beta over v2 stable) + Svelte + TypeScript. Windows is the target OS.

## Users

The user and a small telecom team (integrator side) who configure Xorcom CompletePBX 5 systems for IGSS sites. Mostly at a desk; sometimes on-site on a laptop in a server/equipment room. They know Asterisk/CompletePBX well; the tool is for speed and repeatability, not hand-holding.

## Product Purpose

One desktop app holding several small automation tools ("módulos") for CompletePBX 5 configuration work that is slow or error-prone through the web admin. Success: a repetitive config job becomes a previewed, one-click, repeatable operation against the right PBX.

## Positioning

Built around the team's real fleet: several PBX profiles, three access channels per PBX (CompletePBX API, SSH/CLI, AMI), and a preview-then-confirm discipline for every change to a live hospital phone system. The CompletePBX web admin edits one box, one form at a time; this tool batches, previews, and applies across profiles.

## Operating Context

- Several PBXs, one per site; the user picks the active PBX profile before running a tool.
- Access per PBX: CompletePBX HTTP API, SSH (shell, `asterisk -rx`), AMI (TCP 5038).
- PBXs typically use self-signed HTTPS certs and unknown SSH host keys on first contact.
- Expected size: about 5–10 tools.
- The app updates itself from GitHub Releases (repo to be set later).

## Capabilities and Constraints

- Every tool that writes to a PBX shows a preview of exactly what will change and requires an explicit "Aplicar"; results are reported item by item.
- Secrets live in Windows Credential Manager, never in plain config files.
- UI copy in Spanish; code in English.
- Undecided: CompletePBX API reference not on hand yet, so endpoints beyond auth/reachability wait for it. Which tools come after "Conexiones" is not decided.

## Brand Commitments

Working name "Utilidades XORCOM" (from the project folder; not an official Xorcom product). Internal tool, no external brand assets.

## Evidence on Hand

None yet: no API docs, no sample data, no logos. Do not fabricate PBX data as if real; demo data must be labeled.

## Product Principles

1. Live phone systems serving hospitals: nothing changes without a visible preview and an explicit confirm.
2. Always obvious which PBX you are pointed at.
3. Each tool does one job; adding a tool must stay cheap.
4. Failures are specific and per-item (which PBX, which channel, which entry), never a generic error.
