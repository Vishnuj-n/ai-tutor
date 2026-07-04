# Design System: The Digital Sanctuary

## Overview

Editorial clarity for academic focus. Extreme white space, asymmetric type, tonal nesting. Interface feels like an architectural blueprint — precise, quiet, functional.

---

## Colors & Surfaces

**No-Line Rule:** No 1px borders. Boundaries via background shifts only.

**Surface hierarchy:**
- `background`: `#f9f9fb` (base)
- `surface-container`: `#ebeef2` (secondary areas)
- `surface-container-lowest`: `#ffffff` (interactive/floating)

**Glass & Gradient:** CTAs use 15-degree gradient from `primary` (`#005bc1`) to `primary_dim` (`#004faa`).

---

## Typography

Dual-typeface: **Manrope** (headers, geometric authority) + **Inter** (body, legibility).

- `display-lg`: empty states/dashboard greetings, tracking -2%
- `body-md`/`body-lg`: all body text
- Use Font Weight (SemiBold vs Regular) to distinguish hierarchy, not color
- Keep `on-surface` (`#2d3338`) for most text (high contrast, accessible)

---

## Elevation & Depth

Shadows only for elements physically "above" workflow (modals).

**Layering:**
- Level 0: `background`
- Level 1: `surface-container-low` (content groupings)
- Level 2: `surface-container-lowest` (active cards)

**Ambient shadow (modals):** `box-shadow: 0 20px 40px rgba(45, 51, 56, 0.06)`

**Ghost border:** 1px stroke of `outline-variant` at 20% opacity for inputs/search bars.

---

## Components

**Buttons:**
- Primary: `primary` gradient, `on-primary` text, roundedness `xl` (0.75rem)
- Secondary: `surface-container-highest` bg, `primary` text, no border
- Tertiary: text-only, SemiBold, `primary` color (cancel/low-priority)

**Cards & Lists:**
- No `<hr>` dividers. Use 1.5rem vertical space or background shift
- Hover: `surface-container-low` → `surface-container-lowest` + 2px ambient shadow

**Inputs:**
- Minimalist underline or ghost border
- Focus: border opacity → 100% `primary`, label shifts to `primary`
- Error: `error` (`#9f403d`) only on helper text, not input box

---

## Do's and Don'ts

**Do:** Use asymmetry (large headlines left, wide right margins). Trust white space. Respect 8px grid.

**Don't:** Use pure black (use `on-surface` `#2d3338`). Use `#007AFF` for everything. Use standard `0,0,0,0.5` shadows (always low-opacity tinted blurs).
