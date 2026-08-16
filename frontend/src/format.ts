// ── Display formatting helpers ──────────────────────────────────────────
// Centralized so titles look consistent everywhere: no ".mp3" suffixes,
// no double extensions, clean casing from raw filenames.

const AUDIO_EXTS = /\.(mp3|m4a|flac|ogg|wav|aac|opus|wma)$/i;

/** Remove a trailing audio extension from a title string. */
export function cleanTitle(title: string): string {
  if (!title) return title;
  let out = title.trim().replace(AUDIO_EXTS, "");
  // Some titles end up like "song.{ext}" or "song.mp3.mp3" — clean repeatedly.
  let prev: string | null = null;
  while (prev !== out) {
    prev = out;
    out = out.replace(AUDIO_EXTS, "").trim();
  }
  return out;
}
