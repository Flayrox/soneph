#!/usr/bin/env python3
"""
playlist_from_url.py — crée une playlist dans l'app à partir d'un lien
(playlist / album), en résolvant les morceaux DÉJÀ présents sur disque.

Quand on colle un lien dont on possède déjà les sons (souvent des singles
déjà téléchargés), pas besoin de re-télécharger : on récupère la liste des
morceaux via l'API embed, on les résout contre la bibliothèque (URL Spotify
WOAS dans les tags), et on renvoie la correspondance pour créer la playlist.

Usage:
    playlist_from_url.py <download_dir> <url>
Sortie : JSON {name, matched: [{title, artist, rel_path}],
               missing: [{title, artist, uri}]}
"""
import os
import sys
import re
import json
import time
import urllib.request
from urllib.parse import urlparse

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from scan_identity import identity_map  # noqa: E402


def normalize(text):
    text = re.sub(r"[\(\[\{].*?[\)\]\}]", "", text)
    text = re.sub(r"[^\w\s]", "", text)
    return text.strip().lower()


def fetch_page(url, retries=3):
    for attempt in range(retries):
        try:
            req = urllib.request.Request(
                url,
                headers={"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)"},
            )
            return urllib.request.urlopen(req, timeout=10).read().decode("utf-8")
        except Exception:
            if attempt < retries - 1:
                time.sleep(0.5)
    return None


def embed_base_url(media_url):
    candidate = media_url if "://" in media_url else "https://" + media_url
    parsed = urlparse(candidate)
    host = parsed.netloc
    if not host:
        return None
    if not host.startswith("open."):
        host = "open." + host
    return f"https://{host}/embed"


def parse_embed(html):
    """(name, [track dicts]) depuis le __NEXT_DATA__ de la page embed."""
    m = re.search(r'<script id="__NEXT_DATA__" type="application/json">(.*?)</script>', html, re.DOTALL)
    if not m:
        return None, []
    try:
        data = json.loads(m.group(1))
        entity = data.get("props", {}).get("pageProps", {}).get("state", {}).get("data", {}).get("entity", {})
        name = (entity.get("name") or entity.get("title") or "").strip()
        track_list = entity.get("trackList") or []
        tracks = []
        for t in track_list:
            title = (t.get("title") or "").strip()
            if not title:
                continue
            tracks.append({
                "title": title,
                "artist": (t.get("subtitle") or "").strip(),
                "uri": t.get("uri", ""),
            })
        # Un track seul (pas de trackList) : l'entité EST le morceau.
        if not tracks and entity.get("type") == "track":
            title = (entity.get("title") or "").strip()
            if title:
                artists = entity.get("artists") or []
                artist = ", ".join(
                    a.get("name", "") for a in artists if isinstance(a, dict) and a.get("name")
                ) if isinstance(artists, list) else str(artists or "")
                tracks.append({
                    "title": title,
                    "artist": artist.strip(),
                    "uri": entity.get("uri", ""),
                })
        return name, tracks
    except Exception:
        return None, []


def fetch_all_tracks(media_url):
    match = re.search(r"(playlist|album|track)/([a-zA-Z0-9]+)", media_url)
    if not match:
        return None, [], False
    base = embed_base_url(media_url)
    if not base:
        return None, [], False
    media_type, media_id = match.group(1), match.group(2)

    all_tracks = []
    name = ""
    offset = 0
    PAGE_SIZE = 100
    MAX_PAGES = 30  # garde-fou (playlists énormes) — voir truncated ci-dessous
    truncated = False
    prev = None
    while len(all_tracks) // PAGE_SIZE < MAX_PAGES:
        url = f"{base}/{media_type}/{media_id}?offset={offset}"
        html = fetch_page(url)
        if not html:
            break
        page_name, page_tracks = parse_embed(html)
        if offset == 0 and page_name:
            name = page_name
        if not page_tracks:
            break
        if prev is not None and page_tracks == prev:
            # L'API embed IGNORE offset pour les playlists > 100 titres : la
            # même page revient en boucle. Sans ça, on tournerait indéfiniment.
            truncated = True
            break
        all_tracks.extend(page_tracks)
        prev = page_tracks
        if len(page_tracks) < PAGE_SIZE:
            break
        offset += PAGE_SIZE
        time.sleep(0.1)
    else:
        truncated = True

    # Dédoublonnage (pages qui se chevauchent ou se répètent).
    unique = []
    seen = set()
    for t in all_tracks:
        k = (t.get("title", "").strip().lower(), t.get("artist", "").strip().lower())
        if k not in seen:
            seen.add(k)
            unique.append(t)
    return name, unique, truncated


def main():
    if len(sys.argv) < 3:
        print(json.dumps({"error": "usage: playlist_from_url.py <download_dir> <url>"}))
        return
    download_dir, url = sys.argv[1], sys.argv[2]

    name, tracks, truncated = fetch_all_tracks(url)
    if not tracks:
        print(json.dumps({
            "error": "Impossible de récupérer les morceaux du lien (lien invalide ou API indisponible)",
            "name": name,
        }))
        return

    ident = identity_map(download_dir)  # "url:https://open.spotify.com/track/X" → [rel_path]

    matched, missing = [], []
    for t in tracks:
        rel = ""
        uri = (t.get("uri") or "").strip()
        if uri.startswith("spotify:track:"):
            track_id = uri.split(":", 2)[2]
            rel = ident.get(f"url:https://open.spotify.com/track/{track_id}")
            if rel:
                rel = rel[0]
        if rel:
            matched.append({"title": t["title"], "artist": t["artist"], "rel_path": rel})
        else:
            missing.append({"title": t["title"], "artist": t["artist"], "uri": uri})

    print(json.dumps({
        "name": name,
        "matched": matched,
        "missing": missing,
        "total": len(tracks),
        "truncated": truncated,
    }, ensure_ascii=False))


if __name__ == "__main__":
    main()
