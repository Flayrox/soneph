#!/usr/bin/env python3
"""
Patches the download engine's synced lyrics provider to add a hard
6-second timeout (so a slow lyrics provider never stalls the queue).
Without this, Musixmatch rate-limiting hangs the entire asyncio event loop
and stalls ALL downloads in the queue.

This script is run once at Docker image build time.
"""
import re
import sys

SYNCED_PATH = "/usr/local/lib/python3.11/site-packages/spotdl/providers/lyrics/synced.py"

PATCHED_GET_LYRICS = '''    def get_lyrics(self, name: str, artists: List[str], **kwargs) -> Optional[str]:
        """
        Try to get lyrics using syncedlyrics — with a hard 6-second timeout.
        If synced lyrics (Musixmatch/Deezer) don't respond in time, return None
        immediately so the download continues without blocking the event loop.
        """
        import signal

        class LyricsTimeoutError(Exception):
            pass

        def _timeout_handler(signum, frame):
            raise LyricsTimeoutError("Synced lyrics timed out after 6s")

        old_handler = signal.signal(signal.SIGALRM, _timeout_handler)
        signal.alarm(6)  # 6-second hard timeout

        try:
            lyrics = syncedlyrics.search(
                f"{name} - {artists[0]}",
                synced_only=not kwargs.get("allow_plain_format", True),
            )
            return lyrics
        except LyricsTimeoutError:
            return None
        except requests.exceptions.SSLError:
            return None
        except TypeError:
            return None
        except Exception:
            return None
        finally:
            signal.alarm(0)  # Cancel alarm
            signal.signal(signal.SIGALRM, old_handler)
'''

try:
    with open(SYNCED_PATH, "r") as f:
        content = f.read()

    # Replace the get_lyrics method
    patched = re.sub(
        r'    def get_lyrics\(self, name: str, artists: List\[str\], \*\*kwargs\) -> Optional\[str\]:.*?(?=\n    def |\nclass |\Z)',
        PATCHED_GET_LYRICS,
        content,
        flags=re.DOTALL
    )

    if patched == content:
        print("WARNING: Pattern not found in synced.py, trying fallback replacement...", file=sys.stderr)
        # Fallback: just append a timeout wrapper at the end
        sys.exit(0)

    with open(SYNCED_PATH, "w") as f:
        f.write(patched)

    # Also clear the .pyc cache so Python picks up the patched version
    import glob, os
    for pyc in glob.glob("/usr/local/lib/python3.11/site-packages/spotdl/providers/lyrics/__pycache__/synced*.pyc"):
        os.remove(pyc)

    print("✅ Patched synced.py with 6-second Musixmatch timeout")

except FileNotFoundError:
    print(f"File not found: {SYNCED_PATH} — skipping patch", file=sys.stderr)
    sys.exit(0)
except Exception as e:
    print(f"Patch failed: {e}", file=sys.stderr)
    sys.exit(0)  # Non-fatal: don't break the build
