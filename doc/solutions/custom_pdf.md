# Custom PDF Viewer Integration Walkthrough



# ITERATION 1

We replaced the basic iframe PDF viewer with a premium, custom-tailored `vue-pdf-embed` implementation. This integration respects the Digital Sanctuary design philosophy, provides edge-to-edge layouts, enables continuous vertical scrolling, and adapts dynamically to active themes.

## Changes Made

### Frontend

#### [Reader.vue](file:///c:/Users/vishn/PROJECT/ai-tutor/frontend/src/pages/Reader.vue)
- **Engine Upgrade**: Swapped `<iframe class="pdf-frame">` with `<vue-pdf-embed>` wrapped in a scrollable viewport `.pdf-viewport`.
- **Seamless Scrolling**: Enabled continuous scrolling down all pages by default, omitting page limits in free browsing. In Task Flow, it automatically loads and bounds the page array using computed `renderedPages`.
- **Scroll Tracking & Sync**: Configured an `IntersectionObserver` observing PDF pages. It updates the active reading page `reader.currentPage` dynamically as the user scrolls, keeping chat contexts in perfect sync.
- **Programmatic Navigation**: Handled page scroll positioning smoothly when Next/Prev or section outline jumps are triggered, preventing loop feedback via an input lock.
- **Premium Aesthetics**:
  - Eliminated all artificial margin spacings, dropped shadows, borders, and paddings between consecutive page blocks to achieve a crisp edge-to-edge document grid.
  - Implemented CSS filter overrides for the rendering canvas to translate white background PDFs into premium styles that blend with the theme backgrounds:
    - **Warm Sepia** (`light-warm`): Warm sepia filter.
    - **Deep Indigo** (`dark-indigo`): Inverted Indigo overlay.
    - **Nord Frost** (`dark-nord`): Inverted Blue-gray/arctic dark mode.
    - **Forest Emerald** (`dark-emerald`): Inverted Moss-green dark mode.

---

# ITERATION 2
We upgraded the custom PDF viewer in [Reader.vue](file:///C:/Users/vishn/PROJECT/ai-tutor/frontend/src/pages/Reader.vue) to incorporate four standardized view modes and dynamic multi-input scaling.

## Changes Made

### Frontend

#### [Reader.vue](file:///C:/Users/vishn/PROJECT/ai-tutor/frontend/src/pages/Reader.vue)

1. **State variables added**:
   - `zoomScale` (magnification multiplier, default `1.0`).
   - `viewMode` (active mode: `'raw'`, `'light'`, `'dark'`, `'sync'`, default `'raw'`).
   - `themeMenuOpen` (bool — controls visibility of the theme flyout panel).
   - `containerWidth` (tracks client width of the viewport element).
   - `iframeKey` (declared to prevent legacy ReferenceError).

2. **Resize & Scale Ingestion**:
   - Added a `ResizeObserver` watching `pdfViewportRef`. It dynamically measures the container's width, updating `containerWidth` so resizing the window or collapsing the AI chat sidebar immediately updates the PDF canvas size without layout corruption.
   - Bound the dynamic property `:width="containerWidth * zoomScale"` to `<vue-pdf-embed>`.

3. **Multi-Input Gesture & Button Controller**:
   - Added manual increment/decrement click handlers (`zoomIn` / `zoomOut`) to the HUD. The buttons are constrained between `50%` (`0.5`) and `250%` (`2.5`).
   - Added touch listeners (`touchstart`, `touchmove`, `touchend`) to calculate pinch velocity ratios:
     $$d = \sqrt{(x_2 - x_1)^2 + (y_2 - y_1)^2}$$
     Updates `zoomScale` smoothly based on initial finger spacing delta.
     **Safeguard**: Invokes `e.preventDefault()` on two-finger movements to prevent browser-level viewport zoom.
   - Added trackpad support via `wheel` listeners where `e.ctrlKey === true`.
     **Safeguard**: Invokes `e.preventDefault()` to stop standard browser pinch-to-zoom.

4. **Right-Edge Control Strip** (replaces the old floating centered bar):
   - Anchored vertically centered on the **right edge** of `.pdf-viewport`.
   - Contains `−`, zoom percentage, `+` in a vertical column.
   - A hairline separator divides zoom from a `···` dots button.
   - Clicking `···` opens a **theme flyout** that slides in from the right. The flyout lists Raw, Light, Dark, Sync. The active mode is highlighted. Clicking a mode sets it and closes the flyout.
   - **Default view mode is `raw`** (no processing filters, white background).

5. **Style Adjustments**:
   - Changed `.pdf-viewport` to `overflow-x: auto;` to permit panning when zoomed.
   - Removed `width: 100% !important;` from the canvas and layout wrappers, opting for `margin: 0 auto !important;` and `max-width: none !important;` so that the canvas renders at its specified `:width` and centers correctly.
   - Created view mode selectors under `.pdf-viewport[data-view-mode="..."]` to enforce filter values:
     - `raw`: Bypasses all processing filters (`filter: none !important`) and enforces a white background.
     - `light`: Applies warm sepia tone (`filter: sepia(0.5) contrast(1.1) brightness(0.95) !important`) and sepia background.
     - `dark`: Applies inverted dark styling (`filter: invert(1) hue-rotate(180deg) !important`) and a deep black background.
     - `sync`: Yields to default CSS styling for active themes.

---

## Verification & Build Results

### Automated Verification
We ran the production build to ensure Vite compiles the Vue 3 component correctly:
```powershell
npm run build
```
- **Result**: Build compiled successfully with no compilation errors or linter issues.

---

# ITERATION 3

I have implemented four performance optimizations in the PDF reader to decrease memory footprint, reduce DOM query overhead, lower CPU main-thread usage, and avoid GPU filtering bottlenecks.

## Changes Made

### Frontend

#### [Reader.vue](file:///c:/Users/vishn/PROJECT/ai-tutor/frontend/src/pages/Reader.vue)

1. **DOM Caching**:
   - Introduced a `pageElements` Map in the component setup.
   - Cleared and populated the map in `handlePDFRendered()` using page numbers as keys and page elements as values.
   - Modified `scrollToPage()` to use `pageElements.get(pageNum)` instead of doing a costly `pdfViewportRef.value.querySelector` call.

2. **IntersectionObserver Virtualization & Visibility Tracking**:
   - Introduced `pdfVisibilityObserver`, a separate `IntersectionObserver` with a generous vertical margin (`50% 0px 50% 0px`).
   - The observer dynamically appends/removes the `.is-visible` class on page elements as they approach or leave the viewport.
   - The primary `pdfObserver` still monitors page number intersection triggers inside the narrow center boundary (`-40% 0px -40% 0px`).

3. **Conditional Theme Filter Application**:
   - Updated the Light/Dark/Indigo/Nord/Emerald CSS filter rules under `.pdf-viewport` to apply **only** when the target page is in the viewport (`.vue-pdf-embed__page.is-visible canvas`).
   - Added `will-change: filter` to the canvas elements to optimize GPU layout/compositing pipelines.

4. **Render Virtualization via `content-visibility`**:
   - Applied CSS `content-visibility: auto` and `contain-intrinsic-size: 800px 1100px` to the `.vue-pdf-embed__page` containers.
   - This lets the browser completely skip the layout, styling, and paint cycle of offscreen canvases while preserving exact scroll height and scrollbar alignment.

## Verification Results

- Verified that page tracking, scroll behavior, zoom levels, and CSS themes behave exactly as before.
- CPU profiling shows that off-screen canvas rendering is deferred and DOM query overhead during navigation is eliminated.

---

# ITERATION 4

Walkthrough - Removing Reading-Window Restrictions
We have successfully removed the reading-window page navigation and rendering restrictions from the PDF reader interface, allowing users to browse full documents in both browse mode and task mode while preserving initial task context.

Changes Made
frontend
useReaderBase.js
Updated canGoPrev to check if currentPage > 1 instead of using navigationMinPage as a bound.
Updated canGoNext to check if currentPage < pageCount instead of using navigationMaxPage as a bound.
Updated selectSection to clamp page navigation to [1, pageCount] rather than clamping to navigation bounds.
Kept navigationMinPage, navigationMaxPage, and initializeSession() metadata intact for initial placement and context.
Reader.vue
Updated renderedPages computed property to return undefined, ensuring vue-pdf-embed renders the complete PDF instead of only a subset of pages.
Updated handlePDFRendered page mapping logic to assign page numbers to rendered page elements using index + 1 of the entire document list, ensuring correct page tracking.
Kept the informational display of the "Reading Window" page range so users still see the assigned reading goal.
Removed the unused nextTick import from Vue to keep the codebase warning-free.
Verification Results
Ran npm run lint which completed successfully with zero compilation errors or warnings in modified code.
Ran npm run build which verified that the project compiles cleanly for production.
Ran go test ./... which confirmed all backend tests are passing successfully.