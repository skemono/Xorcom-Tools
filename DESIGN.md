---
name: Utilidades XORCOM
description: Desktop utilities for CompletePBX 5, signed like a hospital corridor in Xorcom's own colors; one big sign names the PBX, numbered steps say what to fill next, the current step is lit.
colors:
  sign: "#0067a5"
  sign-hover: "#00578c"
  sign-deep: "#231f20"
  sign-line: "#3b3536"
  on-sign: "#ffffff"
  on-sign-2: "#d0cbcc"
  on-band-2: "#d9eaf5"
  brand-blue: "#0072b1"
  brand-red: "#ce1141"
  ground: "#f4f4f5"
  panel: "#ffffff"
  tint: "#e3eff8"
  ink: "#231f20"
  mute: "#5d5a5b"
  rule: "#d9dadc"
  rule-strong: "#b9bcc0"
  lit: "#ffd23f"
  lit-soft: "#fff4c7"
  lit-rule: "#ecdc96"
  lit-mute: "#6a5710"
  fail: "#c8103e"
  fail-soft: "#fce4ea"
  ok: "#1b7f4b"
  fail-rule: "#eeabbb"
  fail-on-sign: "#ffc2cf"
typography:
  display:
    fontFamily: "Atkinson Hyperlegible Next Variable, Segoe UI, sans-serif"
    fontSize: "28px"
    fontWeight: 800
    lineHeight: 1.1
    letterSpacing: "-0.01em"
  headline:
    fontFamily: "Atkinson Hyperlegible Next Variable, Segoe UI, sans-serif"
    fontSize: "26px"
    fontWeight: 800
    lineHeight: 1.15
    letterSpacing: "-0.01em"
  title:
    fontFamily: "Atkinson Hyperlegible Next Variable, Segoe UI, sans-serif"
    fontSize: "16px"
    fontWeight: 800
    lineHeight: 1.2
  body:
    fontFamily: "Atkinson Hyperlegible Next Variable, Segoe UI, sans-serif"
    fontSize: "15px"
    fontWeight: 400
    lineHeight: 1.45
    fontFeature: "tnum"
  label:
    fontFamily: "Atkinson Hyperlegible Next Variable, Segoe UI, sans-serif"
    fontSize: "14px"
    fontWeight: 700
    lineHeight: 1.2
  hint:
    fontFamily: "Atkinson Hyperlegible Next Variable, Segoe UI, sans-serif"
    fontSize: "13px"
    fontWeight: 400
    lineHeight: 1.4
  stamp:
    fontFamily: "Atkinson Hyperlegible Next Variable, Segoe UI, sans-serif"
    fontSize: "15px"
    fontWeight: 800
    lineHeight: 1
    letterSpacing: "0.12em"
  stamp-mini:
    fontFamily: "Atkinson Hyperlegible Next Variable, Segoe UI, sans-serif"
    fontSize: "12px"
    fontWeight: 800
    lineHeight: 1
    letterSpacing: "0.1em"
  mono:
    fontFamily: "JetBrains Mono, Consolas, monospace"
    fontSize: "13px"
    fontWeight: 400
    lineHeight: 1
rounded:
  stamp: "4px"
  control: "8px"
  notice: "10px"
  panel: "12px"
  pill: "99px"
spacing:
  gutter: "28px"
  gap: "14px"
  panel-x: "16px"
  control-x: "12px"
components:
  button-primary:
    backgroundColor: "{colors.sign}"
    textColor: "{colors.on-sign}"
    typography: "{typography.title}"
    rounded: "{rounded.control}"
    padding: "0 20px 0 12px"
    height: "48px"
  button-primary-hover:
    backgroundColor: "{colors.sign-hover}"
  button-secondary:
    backgroundColor: "{colors.panel}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    padding: "0 16px"
    height: "44px"
  button-small:
    rounded: "{rounded.control}"
    padding: "0 12px"
    height: "36px"
  button-lit:
    backgroundColor: "{colors.lit}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
  input:
    backgroundColor: "{colors.panel}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    padding: "0 12px"
    height: "44px"
  input-disabled:
    backgroundColor: "{colors.ground}"
    textColor: "{colors.mute}"
  step-panel:
    backgroundColor: "{colors.panel}"
    rounded: "{rounded.panel}"
    padding: "14px 16px 16px"
  step-disc:
    backgroundColor: "{colors.sign}"
    textColor: "{colors.on-sign}"
    size: "28px"
  step-disc-done:
    backgroundColor: "{colors.ok}"
  step-disc-lit:
    backgroundColor: "{colors.lit}"
    textColor: "{colors.ink}"
  preview-lit:
    backgroundColor: "{colors.lit-soft}"
    rounded: "{rounded.panel}"
  preview-lit-bar:
    backgroundColor: "{colors.lit}"
    textColor: "{colors.ink}"
    height: "54px"
  pbx-sign:
    backgroundColor: "{colors.sign}"
    textColor: "{colors.on-sign}"
    typography: "{typography.display}"
    padding: "14px 28px"
  pbx-talking:
    backgroundColor: "{colors.on-sign}"
    textColor: "{colors.sign}"
    rounded: "{rounded.pill}"
    padding: "6px 12px"
  directory-sign:
    backgroundColor: "{colors.sign-deep}"
    textColor: "{colors.on-sign-2}"
    width: "220px"
  directory-entry-on:
    backgroundColor: "{colors.ground}"
    textColor: "{colors.ink}"
  brand-rule:
    width: "56px"
    height: "4px"
  notice:
    backgroundColor: "{colors.fail-soft}"
    textColor: "{colors.ink}"
    rounded: "{rounded.notice}"
    padding: "12px 14px"
  toast:
    backgroundColor: "{colors.panel}"
    textColor: "{colors.ink}"
    rounded: "{rounded.notice}"
    padding: "10px 10px 10px 12px"
  toast-fail:
    backgroundColor: "{colors.fail-soft}"
  toast-update:
    backgroundColor: "{colors.sign}"
    textColor: "{colors.on-sign}"
  stamp-ok:
    textColor: "{colors.ok}"
    typography: "{typography.stamp}"
    rounded: "{rounded.stamp}"
    padding: "4px 10px 3px"
  stamp-fail:
    textColor: "{colors.fail}"
  action-bar:
    backgroundColor: "{colors.panel}"
    padding: "14px 28px 16px"
---

# Design System: Utilidades XORCOM

## Overview

**Creative North Star: "The Hospital Corridor"**

Every screen is signed the way a hospital signs its corridors. A charcoal directory sign on the left lists the tools like a floor directory; a Xorcom-blue band across the top names the PBX you are standing in, large enough to read from across the desk; numbered step panels say what to fill next, and the step you are on is lit yellow. Results land as inked stamps, the one physical mark carried over from the earlier world.

The palette is taken from the Xorcom logo, cleanly: its blue for signage, its charcoal wordmark for the directory and the ink, its crimson for failure, and its exact blue and red together only once, in a small split rule under the app name. The system is light, calm and dense enough for a technician at an office desk in a bright room. Color is functional only: blue means "sign" (where you are, what to press, where to type, what is happening), yellow means "you are here, look now", crimson means failure, green means done. It refuses the paper form and the card-grid admin panel, and it is never flashy or gamer-styled.

The corridor always says what is going on. While the app talks to the PBX, the PBX sign says so; the step at work says what it is doing; the button pressed shows the running verb; every action ends in a notice at the top right.

**Key Characteristics:**
- Light UI on a neutral off-white ground (#f4f4f5) with white step panels on 2px rules.
- Two signs frame every tool: the charcoal directory sign (220px, left) and the blue PBX sign band (sticky, top).
- Numbered step discs; the current step is lit; step 4 is always the Aplicar button.
- Inputs are unmistakable: white, 44px, 2px sign-blue border at rest.
- Outcomes are stamps: CONECTADO / FALLÓ per channel, APLICADO / FALLÓ per line, APLICADO / CON FALLAS per run.
- Every action reports: a spinner and running verb while it works, a notice when it ends.
- No folio numbers, run numbers or dates in the UI.

## Colors

Xorcom's blue, charcoal and crimson as a wayfinding palette on a neutral grey ground, with exactly three signal colors (lit yellow, failure crimson, done green), each bound to one meaning.

### Primary
- **Xorcom Sign Blue** (sign): the logo's blue taken one step deeper so white text clears 6:1. The PBX sign band, primary buttons, step discs, input borders, links, the focus outline and caret, the "doing" status line, and the update notice. If it is blue, it is signage, something to act on, or work in progress.
- **Sign Blue Pressed** (sign-hover): hover on primary buttons only.
- **Wordmark Charcoal** (sign-deep): the logo's wordmark color; the directory sign behind the tool list.
- **Charcoal Rule** (sign-line): 1px rules between directory entries and the pictogram tiles on the charcoal sign.
- **Sign White** (on-sign): primary text on both signs.
- **Charcoal Haze** (on-sign-2): secondary text on the charcoal directory sign.
- **Band Haze** (on-band-2): secondary text on the blue PBX band (host line, disabled channel chips); 4.9:1.
- **Logo Blue** (brand-blue) and **Logo Crimson** (brand-red): the logo's exact values, used only as the pair in the 56 x 4px split rule under the app name on the directory sign.

### Secondary
- **Lit Yellow** (lit): "you are here". The current step's disc, the preview's sticky bar, the update button on the directory sign and in the update notice, and the text-selection highlight.
- **Lit Wash** (lit-soft): the body of the lit preview.
- **Lit Rule** (lit-rule): row rules inside the lit preview.
- **Lit Umber** (lit-mute): secondary text on yellow (row numbers, column heads, tags), tinted from the yellow; 6.4:1 on Lit Wash.

### Tertiary
- **Xorcom Crimson** (fail): the logo's crimson one step deeper for text (5.8:1 on white). FALLÓ stamps, error reasons, the notice and failure-notice rules and icons, destructive confirmations.
- **Crimson Wash** (fail-soft): failed row backgrounds, the notice panel, failure notices.
- **Crimson Rule** (fail-rule): the border around a danger button or a notice.
- **Crimson on Charcoal** (fail-on-sign): failure text on the charcoal directory sign.
- **Done Green** (ok): CONECTADO / APLICADO stamps, completed step discs and check marks, applied-row marks, the rule of a success notice.

### Neutral
- **Corridor Floor** (ground): the page ground, and the lit directory entry, which runs into the page.
- **Panel White** (panel): step panels, inputs, secondary buttons, the action bar, table heads, notices.
- **Selected Tint** (tint): the registry row being edited; a pale wash of the sign blue.
- **Ink** (ink): body text, the same charcoal as the directory sign; also the border of a lit panel.
- **Mute** (mute): charcoal-based secondary text, AA on ground, panel, lit-soft and fail-soft.
- **Rule** (rule) and **Strong Rule** (rule-strong): panel borders and dividers; secondary button and info-notice borders, pending marks, scrollbar thumb.

### Named Rules
**The One Meaning Rule.** Yellow is only "the step you are on" and "the preview to review"; crimson is only failure; green is only done. No color is used for decoration.

**The Logo Pair Rule.** The logo's exact blue and crimson appear side by side in one place only: the split rule under the app name. Everywhere else the system uses the deepened Sign Blue and Xorcom Crimson, and crimson keeps its single meaning.

**The Tinted-On-Yellow Rule.** Secondary text on yellow uses Lit Umber, never the grey Mute. On a failed row inside the preview, secondary text goes to Ink.

**The Light Desk Rule.** The UI is light (`color-scheme: light`): it is used at office desks in bright rooms. There is no dark theme; the only dark surface is the charcoal directory sign, and the only saturated fields are the blue PBX band and the update notice.

## Typography

**Display Font:** Atkinson Hyperlegible Next Variable (with Segoe UI, sans-serif)
**Body Font:** Atkinson Hyperlegible Next Variable (with Segoe UI, sans-serif)
**Label/Mono Font:** JetBrains Mono (with Consolas, monospace), for hosts, file paths and sample CSV only.

**Character:** One legibility-first family does every job, carried by weight (400 / 500 / 600 / 700 / 800) rather than by a second face. Tabular numerals are on everywhere so counts and PINs line up.

### Hierarchy
- **Display** (800, 28px, 1.1): the PBX name in the sign band, which is also the PBX switcher.
- **Headline** (800, 26px, 1.15): the tool's page title, followed by one lead line (max 72ch, Mute).
- **Title** (800, 16px, 1.2): step heads, the preview bar title, registry and section heads; primary button text.
- **Body** (400, 15px, 1.45): everything else; input values at 500. Notice text is 600 14px.
- **Label** (700, 14px, 1.2): field names, the "doing" status line, and the PBX activity pill (700 13px). Table column heads are 700 13px uppercase at 0.06em, a column label, not a section kicker.
- **Hint** (400, 13px, 1.4): format hints under inputs, versions, timings.
- **Stamp** (800, 15px, 0.12em, uppercase): result stamps; 12px / 0.1em for per-line stamps.

### Named Rules
**The One Family Rule.** Hierarchy comes from weight and size inside Atkinson Hyperlegible Next. Mono appears only for machine strings the user may have to compare character by character.

## Layout

A fixed two-column shell: the 220px directory sign, then the tool view. The PBX sign band is sticky at the top of the view; its measured height (`--head-h`) is what tool headers and the notice stack pin beneath. Each tool is a `page` with 22px top and 28px side gutters, a title and lead, then step panels stacked full width with 14px gaps.

Steps 1 and 2 stack full width so long names and paths get room; from 1500px they sit side by side (1fr : 1.5fr). The preview's bar sticks under the PBX sign, and the table head sticks under the bar. Tables use fixed layout so columns never reflow as filters or stamps change a row.

The action bar is sticky at the bottom and the page is at least the window height below the sign, so the bar sits at the window foot even on a short page. It carries the numbered primary action, secondary actions, and a status note.

Notices stack fixed at the top right, 12px under the PBX sign and 20px from the edge, up to 400px wide with 10px gaps.

### Named Rules
**The Window Foot Rule.** The action bar always sits at the window's foot, never floating mid-page after short content.

**The No Nested Box Rule.** Nothing boxed inside a step panel. Sub-areas (the three channel columns, the three outcomes) are split by 2px rules, not by inner cards.

**The Write Lock Rule.** Only writes to the PBX (Aplicar, the portal reload) lock the app: the directory dims to 40% and the PBX switcher disables. Reads (lists, preview, channel test, pending check) never lock navigation; leaving a tool drops its pending reads quietly.

## Elevation & Depth

Flat in the page. Depth is said by color and rule weight: the charcoal and blue signs against a light ground, white panels on 2px rules, the lit preview's ink border and yellow bar. Stacking is only z-order for sticky layers (PBX sign, preview bar, table head, action bar), separated by 2px rules. The one exception is the notice stack, which floats over content and carries the system's only shadow.

### Shadow Vocabulary
- **Floating notice** (`box-shadow: 0 6px 18px rgb(35 31 32 / 0.16)`): notices only, because they sit over live content. A soft charcoal ambient, never a hard offset.

### Named Rules
**The Flat Sign Rule.** Signs and panels sit flat. Do not add shadows to show elevation; use a 2px rule or a sign color. Only a layer that floats over content (the notice stack) takes the floating-notice shadow.

## Shapes

Gently rounded and consistent: 12px for panels, the registry list and the preview; 10px for notices (inline and floating) and the large PBX pictogram tile; 8px for inputs, buttons, notice close buttons and pictogram tiles; 4px for stamps; full pills for the channel chips, the PBX activity pill and the active marker; full circles for step discs, row marks and the spinner. Borders are 2px solid (2.5px on a stamp and the spinner ring); 1px only for table row rules and directory dividers. The format guide is the one dashed 2px outline, marking reference material rather than a step.

Pictograms are an authored line set on a 24px grid: 2px stroke, round caps and joins, `currentColor`. On signs they sit in a rounded tile (34px in the directory, 50px white in the PBX band).

## Components

### Buttons
Signage you press: solid, square-shouldered, never glossy.
- **Shape:** gently rounded (8px).
- **Primary:** sign blue with white 800 16px text, 48px tall, led by a white step disc carrying its number (step 4, Aplicar). One per tool, in the action bar.
- **Secondary:** white with a 2px Strong Rule border, ink 700 text, 44px; the border turns sign blue on hover. Small variant is 36px.
- **Lit:** yellow fill, ink text; only for the update call ("Actualizar y reiniciar"), on the directory sign and in the update notice.
- **Danger / Confirm:** crimson text with a Crimson Rule border; the two-step confirm swaps the label to "Confirmar: ..." and the border to full crimson.
- **Working:** while its action runs, a button swaps its label for a spinner plus the running verb ("Guardando…", "Probando…", "Aplicando…", "Verificando cambios pendientes…", "Recargando la PBX…").
- **Disabled:** 45% opacity, no hover change. A disabled Aplicar always has its reason in the status note beside it, including while the tool is still reading.
- **Link:** sign blue 700 text with a 1.5px underline at 3px offset.

### Chips
- **Channel chips (PBX sign):** pills on 14% white, 700 13px at 0.04em, led by a 9px dot. Enabled: filled dot. Disabled: hollow dot in Band Haze on no fill; never struck through.
- **Active marker (registry):** a 36px sign-blue pill with white text.

### Cards / Containers (step panels)
- **Corner Style:** 12px.
- **Background:** Panel White on a 2px Rule border.
- **Head:** a 28px sign-blue disc with the step number, title text, and an optional muted aside at the right. A done step shows a green check after the title. While the step works, the head carries a "doing" line: spinner plus what it is doing ("Leyendo listas de la PBX…", "Leyendo el archivo y comparando con la PBX…", "Probando los canales…") in sign blue 700 14px.
- **Lit step:** ink border and a yellow disc with ink numeral.
- **Internal Padding:** 14px 16px 16px.

### Inputs / Fields
- **Style:** white, 44px, 2px sign-blue border at rest, 8px radius, 500 15px value; field name above in Label. Selects use the same box with a sign-blue chevron. Paths use mono 13px.
- **Focus:** 3px lit-yellow outline flush to the border.
- **Disabled:** border drops to Rule, background to ground, text to Mute.
- **Checkboxes:** native, 18px, sign-blue accent.

### Navigation (the directory sign)
- Charcoal column, product name at 800 18px with "CompletePBX 5" below in Charcoal Haze, then the logo pair rule (56 x 4px, 2px radius, Logo Crimson left half, Logo Blue right half). Tools follow as full-width rows divided by 1px Charcoal Rule lines: pictogram tile, name, chevron.
- **Hover:** 6% white wash, text to white. **Current:** the row turns ground-colored with ink text and a sign-blue tile, so it runs into the page.
- The directory dims to 40% only while a tool writes to the PBX; the PBX sign does not.
- Version and update controls sit at the foot; update errors read in Crimson on Charcoal.

### PBX Sign Band (signature)
Sticky sign-blue band: a 50px white tile with the phone pictogram, the PBX name at Display size as a borderless select (the switcher), host in mono plus "PBX activa" below in Band Haze, and channel chips at the right. While any PBX call is in flight, a white pill with a sign-blue spinner reading "Comunicando con la PBX…" appears at the right. It never dims, including while a tool writes to the PBX.

### Spinner and Doing Line
- **Spinner:** a 16px ring, 2.5px current-color border with one quarter open, one turn per 800ms linear. It takes the color of whatever it sits in.
- **Doing line:** spinner plus a present-tense phrase, sign blue 700 14px, placed next to where the work happens.

### Notices (toasts, signature)
Every action says how it ended, in a notice stacked at the top right under the PBX sign.
- **Shape:** 10px radius, 2px rule, Panel White fill, 600 14px text, a leading pictogram, and a 32px close button on every notice.
- **ok:** Done Green rule and icon; closes itself after 5 s.
- **fail:** full crimson rule on Crimson Wash, crimson icon; stays until closed (or 10 s where set).
- **info:** Strong Rule border, sign-blue icon; closes itself after 5 s.
- **update:** the one sign-blue notice, white text, with a lit "Actualizar y reiniciar" button; stays until closed. It appears when a new release is found.
- **Motion:** enters with a 220ms slide from 24px right on the ease-out curve; off under reduced motion.
- Switching PBX (header or "Usar") confirms with an ok notice naming the PBX.

### Lit Preview (signature)
The review step as a panel: Lit Wash body, ink border, a sticky 54px yellow bar (ink disc, title, counts at right, 5px blue progress line at its foot during a run), a toggle row, then a fixed-layout table. Row marks: hollow pending, green applied, crimson failed, dashed skipped. Failed rows take Crimson Wash. Once a result exists, the panel returns to white.

### Result Stamps (signature)
Inked uppercase caps in a 2.5px current-color frame, rotated a few degrees with a varied tilt per impression (-3deg, -1.5deg, -4.5deg), landing once with a 260ms scale-and-unblur. Green for CONECTADO / APLICADO, crimson for FALLÓ / CON FALLAS. Per-line stamps in long tables are smaller (12px, 2px frame) and static.

### Notice (inline)
A problem the user must read inside a tool: Crimson Wash fill, full 2px Crimson Rule border, 10px radius, alert pictogram in crimson.

## Do's and Don'ts

### Do:
- **Do** keep every tool to one Svelte module plus one registry line (id, label, icon) in `frontend/src/modules.ts`; the directory sign builds itself from it.
- **Do** number the steps and light only the current one; the preview is the only lit panel.
- **Do** make every input a white 44px box with a 2px sign-blue border so it is always obvious where to type.
- **Do** put the reason a primary action is disabled in the status note beside it.
- **Do** report outcomes with stamps: CONECTADO / FALLÓ per channel, APLICADO / FALLÓ per line, APLICADO / CON FALLAS per run.
- **Do** give every action feedback: a spinner and running verb while it works, a notice when it ends.
- **Do** route every PBX call through the shared `talk()` wrapper so the PBX sign shows "Comunicando con la PBX…".
- **Do** lock navigation only for PBX writes; reads never block leaving a tool.
- **Do** split sub-areas inside a panel with 2px rules.
- **Do** use Lit Umber for secondary text on yellow.
- **Do** honor reduced motion: stamps, notices, spinners and transitions switch off.

### Don't:
- **Don't** show folio numbers, run numbers or dates in the UI.
- **Don't** use a dark theme, neon, glow or any flashy or gamer look.
- **Don't** use the logo's exact blue and crimson anywhere but the brand rule under the app name.
- **Don't** nest boxes inside a step panel.
- **Don't** use grey secondary text on yellow.
- **Don't** strike through a disabled channel; use a hollow dot.
- **Don't** dim the PBX sign during a write.
- **Don't** let the action bar float mid-page; it sits at the window foot.
- **Don't** use colored side stripes on notices, rows or panels; use full borders and fills.
- **Don't** use yellow, crimson or green for anything but lit, failure and done.
- **Don't** add shadows to anything that sits in the page, or build a card grid; only floating notices carry a shadow.
