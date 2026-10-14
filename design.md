---
version: 2.0
name: Beacon
description: Deep-ink night surfaces, signal-amber accent, instrument-grade mono data, live pulse motif.
colors:
  bg: "#0C1220"
  surface: "#131B2C"
  surface-raised: "#1A2438"
  hairline: "#263450"
  text: "#EAF0FB"
  text-secondary: "#8FA0BC"
  text-faint: "#55637F"
  accent: "#FFB454"
  accent-hover: "#FFC474"
  accent-pressed: "#E69A38"
  on-accent: "#201503"
  ok: "#53D993"
  danger: "#FF7B72"
  relay: "#7AA2FF"
typography:
  display:
    fontFamily: Bricolage Grotesque
    fontWeight: 800
    letterSpacing: "-0.03em"
  heading:
    fontFamily: Bricolage Grotesque
    fontWeight: 700
    letterSpacing: "-0.02em"
  body:
    fontFamily: Inter
    fontSize: 0.9rem
    lineHeight: 1.55
  data:
    fontFamily: JetBrains Mono
    fontSize: 0.8rem
    letterSpacing: "0"
rounded:
  sm: 8px
  md: 12px
  lg: 16px
  pill: 999px
spacing:
  sm: 8px
  md: 16px
  lg: 24px
  xl: 40px
components:
  button-primary:
    backgroundColor: "{colors.accent}"
    textColor: "{colors.on-accent}"
    rounded: "{rounded.sm}"
    padding: 10px 18px
  card:
    backgroundColor: "{colors.surface}"
    border: "1px solid {colors.hairline}"
    textColor: "{colors.text}"
    rounded: "{rounded.md}"
    padding: 20px
---
## Overview

**Beacon** — PALT's design language. The app's whole job is devices announcing
themselves over mDNS and streaming files point-to-point, so the UI borrows its
world from network hardware: dark instrument panels, amber "on air" lamps,
readouts set in mono. Calm ink-blue surfaces, one warm signal color, and data
that reads like a meter, not a paragraph.

## Colors

Dark, hue-rich ink — never pure black. One interactive accent.

- **bg (`#0C1220`):** The room. Every screen sits on this.
- **surface (`#131B2C`) / surface-raised (`#1A2438`):** Cards and sheets. Depth
  comes from these two steps plus a hairline — no drop shadows.
- **hairline (`#263450`):** All borders and dividers, 1px.
- **accent (`#FFB454`, signal amber):** The transmit lamp. Primary actions,
  live progress, the beacon pulse, focus rings. Exactly one solid-amber action
  per view.
- **ok (`#53D993`):** Completed transfers and incoming direction only.
- **danger (`#FF7B72`):** Failures and destructive actions only.
- **relay (`#7AA2FF`):** Route tag for peers/transfers going through the relay
  server. Direct (same network) routes use text-secondary — direct is the norm.

## Typography

Three voices, each with one job:

- **Bricolage Grotesque** — display and headings. Device names, screen titles,
  the wordmark. Weights 700–800, tight tracking.
- **Inter** — UI body, labels, buttons. Weights 400–600.
- **JetBrains Mono** — every machine-readable value: IP:port, speeds, sizes,
  percentages, timestamps, versions, `_palt._tcp.local`. If a value comes off
  the wire, it is set in mono.

## Signature: the beacon pulse

The local device is drawn as a beacon — concentric rings emanate from it while
discovery is broadcasting (2.8s ease-out, fade to nothing). Peers ripple into
the list as they are discovered. This is the one animated flourish; everything
else moves only to communicate state. Respect reduced-motion settings: rings
freeze to a static halo.

## Do's and Don'ts

- **Do** set every wire-derived value in JetBrains Mono.
- **Do** encode transfer direction consistently: outgoing ↑ amber, incoming ↓ green.
- **Do** use hairline borders for structure; hover states brighten the border, not the fill.
- **Don't** use drop shadows — depth is surface steps.
- **Don't** put solid amber on more than one action per view.
- **Don't** use pure black or pure white anywhere.
