# UI & Design System Guidelines

## Aesthetics & Design System
- **Desktop First Quality**: Default to clean, modern, native-feeling interfaces (sleek dark modes, subtle translucent glassmorphism, refined HSL color palettes).
- **No AI Clutter**: ABSOLUTELY NO generic AI dashboards, bloated multi-colored status boxes (emerald/amber/rose grid cards), nested boxes inside boxes, or tacky decorative sparkles/emojis (`⚡`, `📝`, `❌`, `✨`).
- **Typography & Components**:
  - Use clean native menus, popovers, and tables.
  - Dropdown popovers must be minimalist translucent menus (`bg-[#1e1e22]/95 backdrop-blur-2xl border border-white/10 rounded-xl`) with subtle checkmark (`✓`) selection indicators, matching premium desktop apps.
  - Avoid noisy uppercase tracking headers (`SETTINGS MENU`, `NOTIFICATION CENTER`) or unnecessary text tags inside popovers.

## UX & Interactivity
- **Non-blocking Layouts**: Secondary panels, inspectors, or detail drawers must be integrated into non-blocking right/left columns. Never use full-screen modal overlays (`fixed inset-0 bg-black/40 backdrop-blur`) for secondary content, keeping the main interface 100% interactive.
- **Smooth Auto-Scrolling & Lists**:
  - Always separate physical hardware user interactions (`wheel`, `touchmove`, `mousedown`, `pointerdown`) from programmatic scrolling using 0ms `useRef` locks (`isUserScrolledRef`).
  - Never override manual scroll automatically with timers. Manual scrolling must remain locked at the user's position until explicitly re-synced.
  - Avoid layout-shifting methods like `scrollIntoView()`; use exact container offset calculations (`container.scrollTo({ top, behavior: 'smooth' })`).
