---
name: Utilidades XORCOM
description: A carbonless work-order pad for configuring CompletePBX 5 systems; every change is a numbered, stamped form.
colors:
  spot-ink: "#233d7a"
  spot-ink-deep: "#1a2f5c"
  secondary-print: "#4a5f8f"
  folio-red: "#c8202f"
  sello-violet: "#5b3a9e"
  white-copy: "#fbfcfd"
  canary-copy: "#f6e27a"
  pink-copy: "#f4c6cf"
  failure-ink: "#9f1526"
  data-ink: "#16181d"
  hairline: "#c9d1e3"
  ink-wash: "#233d7a0f"
  pad-quiet: "#c5cde0"
typography:
  display:
    fontFamily: "'Barlow Semi Condensed', 'Arial Narrow', sans-serif"
    fontSize: "30px"
    fontWeight: 700
    lineHeight: 1
    letterSpacing: "0.03em"
  headline:
    fontFamily: "'Barlow Semi Condensed', 'Arial Narrow', sans-serif"
    fontSize: "26px"
    fontWeight: 700
    lineHeight: 0.95
    letterSpacing: "0.02em"
  title:
    fontFamily: "'Barlow', 'Segoe UI', sans-serif"
    fontSize: "17px"
    fontWeight: 500
    lineHeight: 1.2
    fontFeature: "tnum"
  body:
    fontFamily: "'Barlow', 'Segoe UI', sans-serif"
    fontSize: "15px"
    fontWeight: 400
    lineHeight: 1.4
    fontFeature: "tnum"
  field-data:
    fontFamily: "'Barlow', 'Segoe UI', sans-serif"
    fontSize: "16px"
    fontWeight: 500
    lineHeight: 1.3
    fontFeature: "tnum"
  note:
    fontFamily: "'Barlow', 'Segoe UI', sans-serif"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.4
  fine-print:
    fontFamily: "'Barlow', 'Segoe UI', sans-serif"
    fontSize: "12px"
    fontWeight: 400
    lineHeight: 1.35
  small-label:
    fontFamily: "'Barlow Semi Condensed', 'Arial Narrow', sans-serif"
    fontSize: "12px"
    fontWeight: 600
    lineHeight: 1
    letterSpacing: "0.06em"
  label:
    fontFamily: "'Barlow Semi Condensed', 'Arial Narrow', sans-serif"
    fontSize: "11px"
    fontWeight: 600
    lineHeight: 1
    letterSpacing: "0.09em"
  button:
    fontFamily: "'Barlow Semi Condensed', 'Arial Narrow', sans-serif"
    fontSize: "13px"
    fontWeight: 600
    lineHeight: 1
    letterSpacing: "0.06em"
  stamp:
    fontFamily: "'Barlow Semi Condensed', 'Arial Narrow', sans-serif"
    fontSize: "16px"
    fontWeight: 700
    lineHeight: 1
    letterSpacing: "0.14em"
  mono:
    fontFamily: "'JetBrains Mono', Consolas, monospace"
    fontSize: "11px"
    fontWeight: 400
    lineHeight: 1.45
rounded:
  none: "0px"
  edge: "2px"
  stamp: "3px"
spacing:
  hair: "4px"
  tight: "6px"
  cell: "12px"
  section: "20px"
  gutter: "32px"
components:
  button-primary:
    backgroundColor: "{colors.spot-ink}"
    textColor: "{colors.white-copy}"
    typography: "{typography.button}"
    rounded: "{rounded.edge}"
    padding: "0 14px"
    height: "34px"
  button-primary-hover:
    backgroundColor: "{colors.spot-ink-deep}"
  button-secondary:
    backgroundColor: "{colors.white-copy}"
    textColor: "{colors.spot-ink}"
    typography: "{typography.button}"
    rounded: "{rounded.edge}"
    padding: "0 14px"
    height: "34px"
  button-secondary-hover:
    backgroundColor: "{colors.ink-wash}"
  button-danger:
    backgroundColor: "{colors.white-copy}"
    textColor: "{colors.folio-red}"
    typography: "{typography.button}"
    rounded: "{rounded.edge}"
    padding: "0 14px"
    height: "34px"
  button-small:
    typography: "{typography.button}"
    padding: "0 10px"
    height: "28px"
  ruled-field:
    backgroundColor: "transparent"
    textColor: "{colors.data-ink}"
    typography: "{typography.field-data}"
    rounded: "{rounded.none}"
    padding: "6px 12px 7px"
  header-box:
    backgroundColor: "{colors.white-copy}"
    textColor: "{colors.data-ink}"
    typography: "{typography.title}"
    rounded: "{rounded.none}"
    padding: "8px 12px 10px"
  form-tab:
    backgroundColor: "transparent"
    textColor: "{colors.white-copy}"
    rounded: "{rounded.edge}"
    padding: "10px 16px 10px 12px"
  form-tab-active:
    backgroundColor: "{colors.white-copy}"
    textColor: "{colors.spot-ink}"
  stamp-ok:
    textColor: "{colors.sello-violet}"
    typography: "{typography.stamp}"
    rounded: "{rounded.stamp}"
    padding: "4px 10px 3px"
  stamp-fail:
    textColor: "{colors.failure-ink}"
    typography: "{typography.stamp}"
    rounded: "{rounded.stamp}"
    padding: "4px 10px 3px"
  outcome-cell-fail:
    backgroundColor: "{colors.pink-copy}"
    textColor: "{colors.data-ink}"
    padding: "8px 12px 14px"
---

# Design System: Utilidades XORCOM

## Overview

**Creative North Star: "The Carbonless Work Order"**

Every PBX change is a numbered work order from a tear-off pad: printed in one spot ink on white copy, filled in with black data, numbered in red, and stamped in violet as each line lands. The interface is a sheet of ruled boxes, not a dashboard. Labels are what the printer put on the form (condensed caps in spot ink); values are what the technician filled in (Barlow in near-black, tabular figures). The copy color itself carries state: white is the working copy, canary is the preview copy, pink is the failure copy.

Density is that of a real form: tight ruled cells (12px side padding), 1px internal rules and 2px structural rules, no cards, no floating panels. One sheet is in focus at a time; while a form runs, everything except the active sheet steps back to 35% opacity. Motion exists only for the stamp landing. Light theme only, by decision (office-light scene); all UI copy is Spanish.

The world explicitly refuses the dark-sidebar SaaS dashboard of KPI cards. The dark ink column on the left is a tear-off pad with a glued binding edge, not a navigation rail.

**Key Characteristics:**
- One spot ink (#233d7a) for every rule, label, and printed element.
- Copy color = state: white working, canary preview, pink failure.
- Ruled grid fields: labels sit inside the cell above the value; borders are the structure.
- Red is reserved for the folio number, the pad's binding edge, and destructive actions.
- Violet is reserved for the sello (stamp), caret, and focus ring: the technician's own mark.
- Flat. No shadows, no texture, no gradients.

## Colors

A two-ink printed form (spot blue plus near-black fill-in) on three copy papers, with red and violet as reserved marks.

### Primary
- **Spot Ink** (spot-ink): every rule, label, form code, primary button fill, ledger marks, and the pad background. It is the printer's ink; anything printed on the form uses it.
- **Deep Spot Ink** (spot-ink-deep): primary button hover only.
- **Secondary Print** (secondary-print): instructions, hints, placeholders (italic), row numbers, ms readouts, pending states, disabled values, scrollbar thumb.

### Secondary
- **Folio Red** (folio-red): the folio number in the header block, the 10px glued binding strip at the top of the pad, and the danger button outline/text.
- **Sello Violet** (sello-violet): the success stamp, the text caret, and the 2px focus outline. It is the technician's hand-applied mark.

### Tertiary (copy papers)
- **Canary Copy** (canary-copy): the preview copy of a write flow (built in F-02; see Components), and the text selection highlight on white copy. One shipped exception: the pad's "Actualizar y reiniciar" button.
- **Pink Copy** (pink-copy): the failure copy. A failed outcome cell and the corrupt-file notice take this background; error text in the ink pad uses it as text.
- **Failure Ink** (failure-ink): failure stamp and the notice border; chosen for AA contrast on pink.

### Neutral
- **White Copy** (white-copy): the working copy; page background, header block, sticky action bar, active pad tab.
- **Data Ink** (data-ink): filled-in values and body text.
- **Hairline** (hairline): light row rules between ledger rows only.
- **Ink Wash** (ink-wash, spot ink at ~6%): hover on secondary buttons, the editing ledger row, channel head strips, and disabled channel fields.
- **Pad Quiet** (pad-quiet): secondary text on the ink pad (pad subtitle, version line).

### Named Rules
**The Copy Color Rule.** Background color changes only to signal which copy you are looking at: white working, canary preview, pink failure. Never use canary or pink as decoration or as a generic warning tint.

**The Reserved Marks Rule.** Red belongs to the folio, the binding edge, and destruction; violet belongs to the stamp and the technician's cursor/focus. Neither appears as a general accent.

## Typography

**Display Font:** Barlow Semi Condensed (with Arial Narrow)
**Body Font:** Barlow (with Segoe UI)
**Label/Mono Font:** Barlow Semi Condensed caps for labels; JetBrains Mono (with Consolas) only for data shown as data: fingerprints, hashes, file paths, and file-format samples

**Character:** The condensed face is the printer's type: uppercase, tracked, in spot ink. The regular Barlow is the fill-in: sentence case, black, tabular figures everywhere so numbers align in columns. All faces are bundled via @fontsource and work offline.

### Hierarchy
- **Display** (700, 30px, 1): form title (e.g. CONEXIONES), uppercase in spot ink, beside a 2px-boxed form code (F-01, 700 15px).
- **Headline** (700, 26px, 0.95): the pad's name block (UTILIDADES XORCOM), uppercase on ink.
- **Title** (500, 17px, 1.2): header block values (Host, Fecha); the PBX picker uses 600; the folio uses 600 in red.
- **Body** (400, 15px, 1.4): default text.
- **Note** (400, 14px, 1.4): the form's instruction line (one line, secondary print, max 72ch) and the status note beside the actions.
- **Fine print** (400, 12px, 1.35): channel hints, the pad's update area; outcome messages run at 13px.
- **Small label** (600, 12px, 0.06em, uppercase): small buttons, pending outcome states, the pad subtitle.
- **Field data** (500, 16px, 1.3): typed values inside ruled fields.
- **Label** (600, 11px, 0.09em, uppercase): every printed field label, column heading, and section heading.
- **Button** (600, 13px, 0.06em, uppercase): all buttons except small ones (Small label).
- **Stamp** (700, 16px, 0.14em, uppercase): the sello only.
- **Mono** (400, 11-13px, 1.45): fingerprints (break-all), file paths (13px in the path field), and the CSV format sample (12px).

### Named Rules
**The Printed vs. Filled Rule.** If the form printer would have put it there, it is condensed caps in spot ink. If the technician or the PBX supplied it, it is Barlow sentence case in data ink. Never set data in caps or labels in sentence case.

**The Tabular Rule.** `font-variant-numeric: tabular-nums` is set on body; ports, ms, folio, and row numbers must line up.

## Layout

A two-column desk: the tear-off pad at a fixed 232px, the sheet in the remaining width (`minmax(0, 1fr)`), full window height. The sheet scrolls; the pad does not.

The sheet has a 32px side gutter. At the top, a sticky header block (18px top, 12px bottom paper margin, so scrolled content passes under paper, never against the rule) holds four ruled boxes in a 2fr / 1.4fr / 0.8fr / 0.8fr grid with minimums of 200 / 150 / 110 / 110px: PBX, Host, Folio, Fecha. The active PBX is never off-screen.

The form body is a 6-column ruled grid. Identity fields span 3 columns each; each channel is a fieldset spanning 2 columns, with a 96px + 1fr pair row for port/user; outcome cells sit under their channel, spanning 2 columns, minimum 96px tall. Sections are separated by 20px; labels sit 4-6px above values inside cells.

A sticky action bar pins to the bottom of the sheet: 2px spot-ink top rule, white copy background, 10px gaps, wraps when narrow. Primary action first, then secondary, then destructive, then a status note.

Target: fits 1366x768 with no horizontal scroll; window minimum 1024x680.

## Elevation & Depth

Completely flat. There are no shadows anywhere; depth is expressed as paper order and rules. The pulled pad tab is paper-colored and runs into the sheet; sticky layers (header block, action bar) are opaque white copy with a 2px ink rule at their seam. The only "depth" change is the busy state, where everything but the active sheet drops to 35% opacity and stops receiving pointer events.

### Named Rules
**The Flat Form Rule.** No box-shadow, no texture, no gradient, no blur at rest. A layer is distinguished by a rule or by copy color, never by lift.

**The Two Weights Rule.** Rules are 1px (internal cell divisions) or 2px (structure: header block frame, ledger top/bottom, form top, outcome bottom, action bar top, boxed form code, buttons, stamps). Light 1px hairline-colored rules are only for ledger rows.

## Shapes

Square paper. Ruled cells, the header block, ledger, and fieldsets have 0 radius. Interactive pieces get a barely-softened 2px corner (buttons, pad tabs), as if die-cut. The stamp is the only 3px corner and the only rotated element. Ledger state marks are 12px hollow squares (2px ink border) that fill solid for the active PBX. The pad carries a 10px folio-red binding strip above a 2px white rule, and dashed perforations (1px, white at 28%) between its form rows.

## Components

### Buttons
Printed and die-cut: 2px outline, condensed caps, square-ish.
- **Shape:** near-square corners (2px), height 34px, padding 0 14px; 2px border always.
- **Primary:** spot ink fill, white copy text. One per action bar ("Guardar y probar").
- **Hover / Focus:** secondary buttons take the ink wash; primary deepens to deep spot ink. Focus is a 2px violet outline at 2px offset (global).
- **Secondary:** white copy fill, spot ink border and text.
- **Danger:** folio red border and text; requires a second press within 4 seconds ("Confirmar: eliminar").
- **Small:** 28px tall, 0 10px, 12px text; used inside outcome cells and the pad footer.
- **Disabled:** 45% opacity.
- **Link:** spot ink, 500 weight, 1px underline at 3px offset; used for ledger row names and "Usar".

### Ruled Fields
- **Style:** a cell, not a box: 1px spot-ink right and bottom rules, label (Label style) above a borderless transparent input in Field data style. Padding 6px 12px 7px.
- **Placeholder:** secondary print, italic, 400.
- **Focus:** global violet outline, 4px offset; violet caret.
- **Disabled:** value in secondary print; a disabled channel tints all its fields with the ink wash.
- **Secrets:** an existing secret shows the placeholder "guardada"/"guardado"; typing replaces it.

### Channel Fieldset
A 2-column-span column of ruled fields under an ink-wash head strip holding the channel label and an "Habilitado" checkbox (16px, accent spot ink). The fieldset draws the column's bottom rule; its last child drops its own.

### Header Block
The work-order header: 2px spot-ink frame, 1px ink dividers, each box a Label over a Title value. The PBX box is a borderless select with an ink chevron; Folio prints "Nº 0004" in folio red; empty values show an em dash.

### Navigation (Form Pad)
The tear-off pad: spot-ink column with the binding strip and perforated rows. Each row is code (Button-style caps, 13px) plus name (Barlow 500 15px) in white. Hover: white at 8%. Active: the tab turns white copy with ink text and runs into the sheet ("the pulled sheet"). Footer: version line in Label style (pad quiet) and update state messages at 12px.

### Ledger
A full-width table with 2px ink rules at top and bottom, a 1px ink rule under the Label-style heading row, and hairline row rules. Columns: state mark (34px), Nº (48px, two-digit, secondary print), name (link), host, row action (80px, right). The row being edited takes the ink wash.

### Stamp (Sello)
The signature outcome. Condensed 700 caps in a 2px currentColor border, 3px corners, rotated by a per-cell tilt (`--r`, default -3deg; the three channel cells use -3, -1.5, -4.5deg so they read as three separate impressions). Violet "CONECTADO" for success, failure ink "FALLÓ" for failure. It lands once in 260ms on the expo-out curve (scale 1.6 to 1, blur 2px to 0, fade in); disabled under reduced motion.

### Outcome Cell
Under each channel: Label title, then pending text ("Sin probar" / "Probando…" / "Deshabilitado") or a stamp row (stamp + ms readout), the message, and when relevant the mono fingerprint with a small "Confiar en esta huella" button. A failed cell turns pink copy; its ms readout switches to data ink. 2px ink bottom rule closes the sheet.

### Sticky Action Bar
The signature line of every form; see Layout. Always present at the bottom of the sheet.

### Write-Flow Preview Copy (built in F-02 PINes masivos)
The copy a write flow shows before and after "Aplicar". Shipped first in F-02; any tool that writes to a PBX reuses it.
- **Frame:** a 2px spot-ink box. Before Aplicar the paper is canary ("Copia — vista previa"); after, white ("Copia aplicada · Folio Nº 0008", the folio number in folio red) with one full-size sello in the strip (violet APLICADO, or failure ink CON FALLAS) that lands once.
- **Strip:** 38px with 2px rules above and below (it carries the copy's top rule, so the rule travels with it when pinned), Label caps on the left, Small-label counts on the right ("4 nuevos · 0 ya existen · 1 sin descripción · 0 con error"), 2px rule under it. Counts use Spanish plurals and group thousands with a narrow no-break space ("5 768").
- **While applying:** the strip reads "Copia — aplicando" and, on the right, the live step with a ticking clock ("Conectando con la PBX… · 3 s", "Escribiendo 1 200 de 5 764 PINes · 9 s", "Verificando…"); a native `<progress>` fills in spot ink, 4px, along the strip's bottom rule, from counts the PBX reports every 100 PINes. Steps with no counts (the portal reload) show only the clock: no invented percentage.
- **Pinned heads:** the strip and the column-heading row are sticky under the header block (`--head-h`, measured by the shell), on opaque paper of the copy's color, so thousands of lines scroll beneath them.
- **Lines:** a fixed-layout table (`table-layout: fixed`; Nº 72px, state 34px, PIN 130px, description auto, result 42%), so columns never reflow when a filter or the stamps change a row. Nº is the file's line number, zero-padded to the widest line (minimum two digits). Row rules on canary are spot ink at 22% (the hairline token vanishes on yellow); the last row drops its rule against the frame.
- **Description cell:** the value as it will be stored; a fine-print "(ajustada)" tag (tooltip: what was removed) when F-02 made it portal-safe (letters, digits, space, dash, underscore).
- **State cell:** the ledger's 12px mark with three states: hollow (pending, skipped), filled ink (applied), struck (failure-ink border and a 2px diagonal: error or failed).
- **Result column:** before Aplicar "Nuevo", or quiet Small-label "YA EXISTE:" / "SIN DESCRIPCIÓN:" + fine-print "se omite", or failure-ink "Error: <reason>". After: per-line mini sellos (12px, 2px border, static, tilts cycling -3 / -1.5 / -4.5deg so no impression repeats) APLICADO or FALLÓ + reason, or quiet OMITIDO + reason.
- **Failure:** error and failed rows turn pink copy; their line number and fine print switch to data ink for AA.
- **Selection:** on canary paper the canary highlight would vanish, so selection there is reversed spot ink.
- **Around it:** two checkboxes above the copy ("Incluir PINes sin descripción", "Solo filas a revisar": everything but plain new or applied lines, plus any "(ajustada)" description; it switches itself on when the file has errors, with "Mostrando N de M filas"). A two-step "Aplicar cambios en la PBX" sits in a white box with a 2px ink rule below the applied copy (never canary or pink: it is an instruction, not a preview or a failure).

### Adding a Form
A new tool is one Svelte form in `frontend/src/modules/` plus one registry line with code `F-0N` and a Spanish label. It inherits the shell (pad tab, sticky header block), and composes: the boxed code + Display title, a secondary-print instruction line, Label section headings, the 6-column ruled grid, Buttons, Stamp outcomes, and the sticky action bar.

## Do's and Don'ts

### Do:
- **Do** draw structure with spot-ink rules: 2px for frames and section seams, 1px inside.
- **Do** put labels inside ruled cells, condensed caps 11px 0.09em in spot ink, above the filled value.
- **Do** report each outcome with its own stamp at its own tilt; success violet, failure in failure ink on pink copy.
- **Do** keep the header block and action bar sticky and opaque white copy.
- **Do** dim everything but the active sheet to 35% while a form runs.
- **Do** keep text AA on every copy color (failure ink, not folio red, on pink).
- **Do** write all UI copy in Spanish.

### Don't:
- **Don't** build a dark-sidebar SaaS dashboard with KPI cards, metric tiles, or floating cards.
- **Don't** use shadows, paper texture, gradients, handwriting faces, or cream paper.
- **Don't** use canary or pink for anything but the preview and failure copies (and selection highlight).
- **Don't** use folio red or sello violet as general accents.
- **Don't** set JetBrains Mono for anything but fingerprints, hashes, and paths.
- **Don't** animate anything except the stamp landing and the busy dim.
- **Don't** add a dark theme.
