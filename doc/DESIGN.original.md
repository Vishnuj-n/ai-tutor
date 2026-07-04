# Design System Specification: The Academic Curator

## 1. Overview & Creative North Star
The Creative North Star for this design system is **"The Digital Sanctuary."** 

In an academic context, cognitive load is the enemy. This system moves beyond "minimalism" into a realm of intentional editorial clarity. We are not just building a tool; we are building an environment for deep work. The aesthetic breaks the "template" look by favoring extreme white space, asymmetric type treatments, and a structural philosophy that treats the screen like a physical gallery space. 

Instead of boxes within boxes, we use **Tonal Nesting** and **Atmospheric Depth** to guide the eye. The interface should feel like a high-end architectural blueprint—precise, quiet, and profoundly functional.

---

## 2. Colors & Surface Philosophy
The palette is rooted in a "High-Value Gray" scale, using blue only as a surgical instrument for interaction.

### The "No-Line" Rule
Traditional 1px borders are strictly prohibited for sectioning content. Boundaries must be defined through background shifts. 
*   **Implementation:** A `surface-container-low` card sitting on a `background` provides all the separation necessary. If a container needs more prominence, elevate it to `surface-container-lowest` (pure white) to make it "pop" against the slightly off-white page.

### Surface Hierarchy & Nesting
Treat the UI as a series of stacked sheets of vellum.
*   **Base Layer:** `background` (#f9f9fb)
*   **Secondary Content Areas:** `surface-container` (#ebeef2)
*   **Interactive/Floating Elements:** `surface-container-lowest` (#ffffff)
*   **System Overlays:** Use `surface-bright` with a 20px backdrop-blur to create a "Glassmorphism" effect for navigation bars and floating action menus.

### The "Glass & Gradient" Rule
To prevent the UI from feeling "flat" or "cheap," CTAs should utilize a subtle, 15-degree linear gradient from `primary` (#005bc1) to `primary_dim` (#004faa). This adds a microscopic level of curvature and "soul" to the crisp blue accent.

---

## 3. Typography: Editorial Authority
We utilize a dual-typeface system to create an "Academic Journal" feel. **Manrope** provides a geometric, authoritative voice for headers, while **Inter** ensures maximum legibility for long-form research text.

*   **Display (Manrope):** Use `display-lg` for empty states or dashboard greetings. Tracking should be set to -2% to feel tighter and more premium.
*   **Body (Inter):** All body text uses `body-md` or `body-lg`. We rely on **Font Weight** (SemiBold vs Regular) rather than color to distinguish between headers and metadata.
*   **Hierarchy Tip:** A `headline-sm` in Bold is more effective than a medium headline in a different color. Keep the `on-surface` (#2d3338) for almost all text to maintain high contrast and accessibility.

---

## 4. Elevation & Depth
In this system, "Shadows" are an admission of failure in layout. Use them only when an element is physically "above" the workflow (e.g., Modals).

*   **The Layering Principle:** 
    *   Level 0: `background`
    *   Level 1: `surface-container-low` (Content groupings)
    *   Level 2: `surface-container-lowest` (Active cards/Primary focus)
*   **Ambient Shadows:** If a shadow is required for a floating Modal, use a "Soft Ambient" style: 
    *   `box-shadow: 0 20px 40px rgba(45, 51, 56, 0.06);` (Using a tinted version of `on-surface`).
*   **The "Ghost Border":** For input fields or search bars, use a 1px stroke of `outline-variant` at **20% opacity**. This creates a "suggestion" of a container without breaking the airy aesthetic.

---

## 5. Components

### Buttons
*   **Primary:** High-gloss `primary` gradient with `on-primary` text. Roundedness: `xl` (0.75rem).
*   **Secondary:** `surface-container-highest` background with `primary` text. No border.
*   **Tertiary:** Text-only, SemiBold, using `primary` color. Reserved for "Cancel" or low-priority actions.

### Cards & Lists
*   **Forbidden:** Horizontal divider lines (`<hr>`).
*   **Replacement:** Use `1.5rem` of vertical white space or a subtle shift from `surface` to `surface-container-low` to distinguish between list items.
*   **Interactive State:** On hover, a card should transition from `surface-container-low` to `surface-container-lowest` and gain a 2px "Soft Ambient" shadow.

### Input Fields
*   **Style:** Minimalist underline or "Ghost Border." 
*   **Focus State:** The border opacity increases to 100% of `primary`, and the label (`label-md`) shifts to `primary` color. 
*   **Error:** Use `error` (#9f403d) only for the helper text; the input box should remain neutral to avoid "visual shouting."



---

## 6. Do’s and Don’ts

### Do
*   **Use Asymmetry:** Align large headlines to the left with wide right margins to mimic modern editorial layouts.
*   **Trust the White Space:** If a screen feels "empty," it is likely working. Avoid the urge to add icons or illustrations.
*   **Respect the 8px Grid:** Ensure all spacing is a multiple of 8 to maintain the mathematical rigor expected in an academic app.

### Don't
*   **Don't use pure black:** Use `on-surface` (#2d3338) for text; it is softer on the eyes for long study sessions.
*   **Don't use "Apple Blue" for everything:** Save #007AFF (Primary) for the *single* most important action on the screen.
*   **Don't use standard shadows:** Never use a `0,0,0,0.5` shadow. It destroys the "Digital Sanctuary" feel. Always use low-opacity, tinted blurs.
