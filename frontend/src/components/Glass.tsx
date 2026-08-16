import React from "react";

/**
 * Glass — our own liquid-glass surface.
 *
 * Inspired by liquid-glass-react (see .reference/) but rebuilt cleanly:
 *  - the frosted layer is a plain `backdrop-filter` on the element itself, so
 *    text/content on top stays perfectly sharp (it blurs what is BEHIND);
 *  - no mouse-tracking transforms, no absolute overlays → the element is an
 *    ordinary in-flow box, safe in grids and flex rows;
 *  - the Apple look: translucent fill, saturation boost, soft outer glow and a
 *    subtle top-edge highlight that catches the light.
 *
 * Parent controls the size (wrap with `w-*` / flex). Children render as-is.
 */
interface GlassProps {
  children: React.ReactNode;
  /** Border radius in px (use 999 for pills). */
  cornerRadius?: number;
  /** Frost intensity (px of backdrop blur). */
  blur?: number;
  /** Saturation boost of the backdrop. */
  saturation?: number;
  className?: string;
  style?: React.CSSProperties;
}

export const Glass: React.FC<GlassProps> = ({
  children,
  cornerRadius = 14,
  blur = 18,
  saturation = 150,
  className = "",
  style,
}) => {
  const backdrop = blur > 0 ? `blur(${blur}px) saturate(${saturation}%)` : undefined;

  return (
    <div
      className={`relative w-full overflow-hidden select-none ${className}`}
      style={{
        ...style,
        borderRadius: cornerRadius,
        background:
          "linear-gradient(135deg, rgba(255,255,255,0.10) 0%, rgba(255,255,255,0.04) 45%, rgba(255,255,255,0.02) 100%)",
        backdropFilter: backdrop,
        WebkitBackdropFilter: backdrop,
        border: "1px solid rgba(255,255,255,0.10)",
        boxShadow:
          "0 8px 32px rgba(0,0,0,0.35), inset 0 1px 0 rgba(255,255,255,0.10), inset 0 -1px 0 rgba(255,255,255,0.03)",
      }}
    >
      {/* top edge highlight — the "light catching the glass" line */}
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-x-1 top-0 h-px"
        style={{
          background: "linear-gradient(90deg, transparent, rgba(255,255,255,0.35), transparent)",
        }}
      />
      {children}
    </div>
  );
};
