---
name: Utilidades XORCOM
description: Desktop utilities for CompletePBX 5, signed like a hospital corridor; one big sign names the PBX, numbered steps say what to fill next, the current step is lit.
colors:
  sign: "#0e4d64"
  sign-hover: "#0b4256"
  sign-deep: "#0a3a4c"
  sign-line: "#1d5468"
  on-sign: "#ffffff"
  on-sign-2: "#c3dae2"
  ground: "#f3f5f6"
  panel: "#ffffff"
  tint: "#e6eff2"
  ink: "#12202b"
  mute: "#56656f"
  rule: "#d5dde1"
  rule-strong: "#b7c4cb"
  lit: "#ffd23f"
  lit-soft: "#fff4c7"
  lit-rule: "#ecdc96"
  lit-mute: "#6a5710"
  fail: "#c62828"
  fail-soft: "#fbe1df"
  ok: "#1b7f4b"
  fail-rule: "#e8aaa6"
  fail-on-sign: "#ffc9c4"
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
  directory-sign:
    backgroundColor: "{colors.sign-deep}"
    textColor: "{colors.on-sign-2}"
    width: "220px"
  directory-entry-on:
    backgroundColor: "{colors.ground}"
    textColor: "{colors.ink}"
  notice:
    backgroundColor: "{colors.fail-soft}"
    textColor: "{colors.ink}"
    rounded: "{rounded.notice}"
    padding: "12px 14px"
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

Every screen is signed the way a hospital signs its corridors. A dark directory sign on the left lists the tools like a floor directory; a sign-teal band across the top names the PBX you are standing in, large enough to read from across the desk; numbered step panels say what to fill next, and the step you are on is lit yellow. Results land as inked stamps, the one physical mark carried over from the earlier world.

The system is light, calm and dense enough for a technician at an office desk in a bright room. Color is functional only: teal means "sign" (where you are, what to press, where to type), yellow means "you are here, look now", red means failure, green means done. It refuses the paper form and the card-grid admin panel, and it is never flashy or gamer-styled.

**Key Characteristics:**
- Light UI on a cool off-white ground (#f3f5f6) with white step panels on 2px rules.
- Two signs frame every tool: the directory sign (220px, left) and the PBX sign band (sticky, top).
- Numbered step discs; the current step is lit; step 4 is always the Aplicar button.
- Inputs are unmistakable: white, 44px, 2px sign-teal border at rest.
- Outcomes are stamps: CONECTADO / FALLÓ per channel, APLICADO / FALLÓ per line, APLICADO / CON FALLAS per run.
- No folio numbers, run numbers or dates in the UI.

## Colors

A sign-teal and white wayfinding palette on a cool neutral ground, with exactly three signal colors (lit yellow, failure red, done green), each bound to one meaning.

### Primary
- **Sign Teal** (sign): the PBX sign band, primary buttons, step discs, input borders, links, the focus outline and caret. If it is teal, it is signage or something to act on.
- **Sign Teal Pressed** (sign-hover): hover on primary buttons only.
- **Deep Directory Teal** (sign-deep): the directory sign behind the tool list.
- **Sign Rule** (sign-line): 1px rules between directory entries and the pictogram tiles on the dark sign.
- **Sign White** (on-sign) and **Sign Haze** (on-sign-2): primary and secondary text on the teal signs.

### Secondary
- **Lit Yellow** (lit): "you are here". The current step's disc, the preview's sticky bar, the update button on the directory sign, and the text-selection highlight.
- **Lit Wash** (lit-soft): the body of the lit preview.
- **Lit Rule** (lit-rule): row rules inside the lit preview.
- **Lit Umber** (lit-mute): secondary text on yellow (row numbers, column heads, tags), tinted from the yellow; 6.4:1 on Lit Wash.

### Tertiary
- **Failure Red** (fail) and **Failure Wash** (fail-soft): FALLÓ stamps, failed row backgrounds, error reasons, the notice panel, destructive confirmations.
- **Done Green** (ok): CONECTADO / APLICADO stamps, completed step discs and check marks, applied-row marks.

### Neutral
- **Corridor Floor** (ground): the page ground, and the lit directory entry, which runs into the page.
- **Panel White** (panel): step panels, inputs, secondary buttons, the action bar, table heads.
- **Selected Tint** (tint): the registry row being edited.
- **Ink** (ink): body text; also the border of a lit panel.
- **Mute** (mute): secondary text, AA on ground, panel, lit-soft and fail-soft.
- **Rule** (rule) and **Strong Rule** (rule-strong): panel borders and dividers; secondary button borders, pending marks, scrollbar thumb.

### Named Rules
**The One Meaning Rule.** Yellow is only "the step you are on" and "the preview to review"; red is only failure; green is only done. No color is used for decoration.

**The Tinted-On-Yellow Rule.** Secondary text on yellow uses Lit Umber, never the grey Mute. On a failed row inside the preview, secondary text goes to Ink.

**The Light Desk Rule.** The UI is light (`color-scheme: light`): it is used at office desks in bright rooms. There is no dark theme; the only dark surfaces are the two signs.

## Typography

**Display Font:** Atkinson Hyperlegible Next Variable (with Segoe UI, sans-serif)
**Body Font:** Atkinson Hyperlegible Next Variable (with Segoe UI, sans-serif)
**Label/Mono Font:** JetBrains Mono (with Consolas, monospace), for hosts, file paths and sample CSV only.

**Character:** One legibility-first family does every job, carried by weight (400 / 500 / 700 / 800) rather than by a second face. Tabular numerals are on everywhere so counts and PINs line up.

### Hierarchy
- **Display** (800, 28px, 1.1): the PBX name in the sign band, which is also the PBX switcher.
- **Headline** (800, 26px, 1.15): the tool's page title, followed by one lead line (max 72ch, Mute).
- **Title** (800, 16px, 1.2): step heads, the preview bar title, registry and section heads; primary button text.
- **Body** (400, 15px, 1.45): everything else; input values at 500.
- **Label** (700, 14px, 1.2): field names. Table column heads are 700 13px uppercase at 0.06em, a column label, not a section kicker.
- **Hint** (400, 13px, 1.4): format hints under inputs, versions, timings.
- **Stamp** (800, 15px, 0.12em, uppercase): result stamps; 12px / 0.1em for per-line stamps.

### Named Rules
**The One Family Rule.** Hierarchy comes from weight and size inside Atkinson Hyperlegible Next. Mono appears only for machine strings the user may have to compare character by character.

## Layout

A fixed two-column shell: the 220px directory sign, then the tool view. The PBX sign band is sticky at the top of the view; its measured height (`--head-h`) is what tool headers pin beneath. Each tool is a `page` with 22px top and 28px side gutters, a title and lead, then step panels stacked full width with 14px gaps.

Steps 1 and 2 stack full width so long names and paths get room; from 1500px they sit side by side (1fr : 1.5fr). The preview's bar sticks under the PBX sign, and the table head sticks under the bar. Tables use fixed layout so columns never reflow as filters or stamps change a row.

The action bar is sticky at the bottom and the page is at least the window height below the sign, so the bar sits at the window foot even on a short page. It carries the numbered primary action, secondary actions, and a status note.

### Named Rules
**The Window Foot Rule.** The action bar always sits at the window's foot, never floating mid-page after short content.

**The No Nested Box Rule.** Nothing boxed inside a step panel. Sub-areas (the three channel columns, the three outcomes) are split by 2px rules, not by inner cards.

## Elevation & Depth

Flat. There are no shadows anywhere. Depth is said by color and rule weight: dark signs against a light ground, white panels on 2px rules, the lit preview's ink border and yellow bar. Stacking is only z-order for sticky layers (PBX sign, preview bar, table head, action bar), separated by 2px rules.

### Named Rules
**The Flat Sign Rule.** Signs and panels sit flat. Do not add shadows to show elevation; use a 2px rule or a sign color.

## Shapes

Gently rounded and consistent: 12px for panels, the registry list and the preview; 10px for notices and the large PBX pictogram tile; 8px for inputs, buttons and pictogram tiles; 4px for stamps; full pills for the channel chips and the active marker; full circles for step discs and row marks. Borders are 2px solid (2.5px on a stamp); 1px only for table row rules and directory dividers. The format guide is the one dashed 2px outline, marking reference material rather than a step.

Pictograms are an authored line set on a 24px grid: 2px stroke, round caps and joins, `currentColor`. On signs they sit in a rounded tile (34px in the directory, 50px white in the PBX band).

## Components

### Buttons
Signage you press: solid, square-shouldered, never glossy.
- **Shape:** gently rounded (8px).
- **Primary:** sign teal with white 800 16px text, 48px tall, led by a white step disc carrying its number (step 4, Aplicar). One per tool, in the action bar.
- **Secondary:** white with a 2px Strong Rule border, ink 700 text, 44px; the border turns sign teal on hover. Small variant is 36px.
- **Lit:** yellow fill, ink text; only for the update call on the directory sign.
- **Danger / Confirm:** red text with a red-tinted border; the two-step confirm swaps the label to "Confirmar: ..." and the border to full red.
- **Disabled:** 45% opacity, no hover change. A disabled Aplicar always has its reason in the status note beside it.
- **Link:** sign teal 700 text with a 1.5px underline at 3px offset.

### Chips
- **Channel chips (PBX sign):** pills on 14% white, 700 13px at 0.04em, led by a 9px dot. Enabled: filled dot. Disabled: hollow dot in Sign Haze on no fill; never struck through.
- **Active marker (registry):** a 36px sign-teal pill with white text.

### Cards / Containers (step panels)
- **Corner Style:** 12px.
- **Background:** Panel White on a 2px Rule border.
- **Head:** a 28px sign-teal disc with the step number, title text, and an optional muted aside at the right. A done step shows a green check after the title.
- **Lit step:** ink border and a yellow disc with ink numeral.
- **Internal Padding:** 14px 16px 16px.

### Inputs / Fields
- **Style:** white, 44px, 2px sign-teal border at rest, 8px radius, 500 15px value; field name above in Label. Selects use the same box with a teal chevron. Paths use mono 13px.
- **Focus:** 3px lit-yellow outline flush to the border.
- **Disabled:** border drops to Rule, background to ground, text to Mute.
- **Checkboxes:** native, 18px, sign-teal accent.

### Navigation (the directory sign)
- Deep teal column, product name at 800 18px, then tools as full-width rows divided by 1px Sign Rule lines: pictogram tile, name, chevron.
- **Hover:** 6% white wash, text to white. **Current:** the row turns ground-colored with ink text and a sign-teal tile, so it runs into the page.
- The directory dims to 40% while a tool runs; the PBX sign does not.
- Version and update controls sit at the foot.

### PBX Sign Band (signature)
Sticky sign-teal band: a 50px white tile with the phone pictogram, the PBX name at Display size as a borderless select (the switcher), host in mono plus "PBX activa" below, and channel chips at the right. It never dims, including while a tool writes to the PBX.

### Lit Preview (signature)
The review step as a panel: Lit Wash body, ink border, a sticky 54px yellow bar (ink disc, title, counts at right, 5px teal progress line at its foot during a run), a toggle row, then a fixed-layout table. Row marks: hollow pending, green applied, red failed, dashed skipped. Failed rows take Failure Wash. Once a result exists, the panel returns to white.

### Result Stamps (signature)
Inked uppercase caps in a 2.5px current-color frame, rotated a few degrees with a varied tilt per impression (-3deg, -1.5deg, -4.5deg), landing once with a 260ms scale-and-unblur. Green for CONECTADO / APLICADO, red for FALLÓ / CON FALLAS. Per-line stamps in long tables are smaller (12px, 2px frame) and static.

### Notice
A problem the user must read: Failure Wash fill, full 2px red-tinted border, 10px radius, alert pictogram in red.

## Do's and Don'ts

### Do:
- **Do** keep every tool to one Svelte module plus one registry line (id, label, icon) in `frontend/src/modules.ts`; the directory sign builds itself from it.
- **Do** number the steps and light only the current one; the preview is the only lit panel.
- **Do** make every input a white 44px box with a 2px sign-teal border so it is always obvious where to type.
- **Do** put the reason a primary action is disabled in the status note beside it.
- **Do** report outcomes with stamps: CONECTADO / FALLÓ per channel, APLICADO / FALLÓ per line, APLICADO / CON FALLAS per run.
- **Do** split sub-areas inside a panel with 2px rules.
- **Do** use Lit Umber for secondary text on yellow.
- **Do** honor reduced motion: stamps and transitions switch off.

### Don't:
- **Don't** show folio numbers, run numbers or dates in the UI.
- **Don't** use a dark theme, neon, glow or any flashy or gamer look.
- **Don't** nest boxes inside a step panel.
- **Don't** use grey secondary text on yellow.
- **Don't** strike through a disabled channel; use a hollow dot.
- **Don't** dim the PBX sign during a write.
- **Don't** let the action bar float mid-page; it sits at the window foot.
- **Don't** use colored side stripes on notices, rows or panels; use full borders and fills.
- **Don't** use yellow, red or green for anything but lit, failure and done.
- **Don't** add shadows or a card grid.
