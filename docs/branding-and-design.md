# AUVC branding and design plan

Tracking issue: [#33](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/33).

Status: planned. Existing AutoMuteUs and AmongUsCapture branding remains in the
imported applications until a new direction is explicitly approved by the owner.
Historical names in licenses, credits and provenance must always remain intact.

## Goal

AUVC needs its own recognizable visual identity and a coherent, accessible user
experience across the Windows capture application, Discord, GitHub, documentation,
installer, updater and release assets. It must remain visibly an independent
community rework and must not imply an official connection to AutoMuteUs or Among Us.

The normal capture experience should feel like one product:

```text
Download → Install → First-run guide → Pair → Ready to play
```

## Current surfaces

The review covers more than the current logo:

- Bot banners, profile picture, Discord embeds, status colors and messages.
- Capture logo, Windows icon, splashscreens, background/status graphics, WPF
  resources, controls, dialogs, layouts and localized strings.
- README banner, GitHub social preview, documentation and templates.
- Setup EXE/MSI, Apps & Features entry, shortcuts, updater and recovery dialogs.
- Release/download presentation and product/assembly/file names.
- Existing maps, player images, hats, pets and fonts as a separate licensed-asset
  inventory. These are not automatically part of the new AUVC brand.

## Approval gates

### Gate 1: choose a direction

Before replacing any product assets, prepare an inventory and brand brief, then
show at least three meaningfully different directions. Each preview includes:

- Primary logo, compact mark and wordmark.
- Discord avatar and banner.
- Capture main-window mockup.
- Core colors and typography.
- Small app-icon view and wide banner view.
- Use on light and dark backgrounds.
- Accessibility, technical, maintenance and licensing considerations.

The owner explicitly chooses or rejects a direction. No broad implementation
begins before that decision.

### Gate 2: approve the product system

Refine the selected direction into a brand board and realistic capture prototype.
Show key connected, disconnected, game-not-found, pairing, update and error states.
The owner explicitly approves this system before implementation PRs begin.

## Implementation packages

After approval, keep changes reviewable and separate from application logic:

1. Original logo/banner/icon masters and reproducible exports.
2. Capture design tokens, shell, navigation, controls and state screens.
3. First-run pairing, diagnostics and update presentation.
4. Discord avatar/banner/embeds and product-facing messages.
5. README, GitHub social preview and documentation visuals.
6. Installer, updater, Apps & Features and release/download visuals.
7. Accessibility, visual regression, legacy-name and asset-license audit.

Do not mix memory/offset behavior, protocol changes or voice-policy logic into a
styling PR. Functional UI changes receive their own tests and review.

## Design-system deliverables

- Editable SVG masters and documented deterministic PNG/ICO exports.
- Primary, horizontal, compact, monochrome and positive/negative logo variants.
- Central color, typography, spacing, radius, shadow, status and focus tokens.
- Iconography and rules for clear loading, success, warning and error states.
- Minimum sizes, clear space, backgrounds and incorrect-use examples.
- Asset manifest with source, license, copyright, hash, master and consumers.
- German and English layouts; keyboard, screenreader, contrast and high-DPI support.
- Screenshots or visual regression coverage for important application states.

Only original or demonstrably permitted assets may ship. Do not copy unclear
official Among Us graphics, fonts or community artwork into the new brand.

## Acceptance

- The owner approved both the visual direction and refined UI prototype.
- Capture, Discord, GitHub, installer and release surfaces consistently identify AUVC.
- Product-facing legacy branding is removed; required historical attribution remains.
- Windows scaling at 100, 125, 150 and 200 percent, long DE/EN text and keyboard
  navigation have been reviewed without clipping or inaccessible states.
- Final assets include editable masters, deterministic exports and provenance.
- Existing functional tests and CI pass; no unrelated runtime behavior changed.
- Before/after previews and known limitations are included in implementation PRs.

## Roadmap placement

Discovery and concept previews can start after the current build baseline. The
approved design system should be ready before capture UI modernization, while
surface implementation follows the dependencies it belongs to:

- Capture shell and first-run UX: phases 11, 13 and 16.
- Discord product presentation: slash-command and doctor phases.
- Installer/updater/release surfaces: phase 17 and issues #20–#24.
- Visual/accessibility acceptance: phases 18–20.

Issue #33 is the cross-cutting design epic. Each implementation package uses its
own `codex/<number>-<topic>` branch and PR, with issue #33 referenced throughout.
