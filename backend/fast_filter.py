#!/usr/bin/env python3
"""
Fast pre-filter: paginate the embed API to get ALL tracks of a playlist,
album or track URL, then compare against local disk files to instantly skip
already-downloaded songs.
"""
import os
import sys
import glob
import re
import json
import time
import urllib.request
from urllib.parse import urlparse

def normalize(text):
    """Normalize text for fuzzy duplicate matching."""
    text = re.sub(r'[\(\[\{].*?[\)\]\}]', '', text)
    text = re.sub(r'[^\w\s]', '', text)
    return text.strip().lower()

def get_existing_filenames(download_dir):
    """
    Collect all normalized filenames currently in downloads directory.

    Un set de noms normalisés permet un lookup O(1) par morceau au lieu du
    parcours O(N×M) de toute la bibliothèque pour chaque titre de playlist
    (c'était le goulot d'étranglement sur les grosses playlists).
    """
    existing = set()
    mp3_files = glob.glob(os.path.join(download_dir, '**', '*.mp3'), recursive=True)
    for f in mp3_files:
        base = os.path.basename(f)
        # spotdl écrit « Title.mp3.mp3 » : on retire jusqu'à 2 extensions pour
        # que le nom normalisé du fichier = le nom normalisé du titre.
        for _ in range(2):
            base = os.path.splitext(base)[0]
        if base:
            existing.add(normalize(base))
    return existing

def fetch_page(url, retries=3):
    """Fetch a single URL with retries."""
    for attempt in range(retries):
        try:
            req = urllib.request.Request(
                url,
                headers={'User-Agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)'}
            )
            html = urllib.request.urlopen(req, timeout=10).read().decode('utf-8')
            return html
        except Exception as e:
            sys.stderr.write(f"Fetch error (attempt {attempt+1}): {e}\n")
            if attempt < retries - 1:
                time.sleep(0.5)
    return None

def parse_tracklist_from_html(html):
    """Extract track list from the embed page HTML."""
    m = re.search(r'<script id="__NEXT_DATA__" type="application/json">(.*?)</script>', html, re.DOTALL)
    if not m:
        return []
    try:
        data = json.loads(m.group(1))
        track_list = (
            data.get('props', {})
                .get('pageProps', {})
                .get('state', {})
                .get('data', {})
                .get('entity', {})
                .get('trackList', [])
        )
        result = []
        for t in track_list:
            title = t.get('title', '').strip()
            artists = (t.get('subtitle', '') or t.get('artists', '')).strip()
            uri = t.get('uri', '')
            if title:
                result.append({
                    'title': title,
                    'artist': artists,
                    'query': f"{artists} - {title}" if artists else title,
                    'uri': uri
                })
        return result
    except Exception as e:
        sys.stderr.write(f"JSON parse error: {e}\n")
        return []

def embed_base_url(media_url):
    """
    Build the embed endpoint from the pasted link itself, so the code never
    hardcodes a provider domain. Returns None if no usable host is found.
    """
    candidate = media_url if '://' in media_url else 'https://' + media_url
    parsed = urlparse(candidate)
    host = parsed.netloc
    if not host:
        return None
    if not host.startswith('open.'):
        host = 'open.' + host
    return f'https://{host}/embed'


def fetch_all_tracks(media_url):
    """
    Paginate through the embed API to get ALL tracks of a playlist/album.
    The embed page returns 100 tracks per page.
    We keep paginating until we get fewer than 100 (or an empty page).
    """
    match = re.search(r'(playlist|album|track)/([a-zA-Z0-9]+)', media_url)
    if not match:
        return []

    base = embed_base_url(media_url)
    if not base:
        return []

    media_type, media_id = match.group(1), match.group(2)

    # For single tracks, no pagination needed
    if media_type == 'track':
        url = f'{base}/{media_type}/{media_id}'
        html = fetch_page(url)
        if html:
            tracks = parse_tracklist_from_html(html)
            if tracks:
                return tracks
            # La page d'un track n'a pas de trackList : on construit la
            # liste depuis l'entité elle-même.
            m = re.search(r'<script id="__NEXT_DATA__" type="application/json">(.*?)</script>', html, re.DOTALL)
            if m:
                try:
                    data = json.loads(m.group(1))
                    ent = data.get('props', {}).get('pageProps', {}).get('state', {}).get('data', {}).get('entity', {})
                    title = (ent.get('title') or '').strip()
                    artists = ent.get('artists') or []
                    artist = ', '.join(a.get('name', '') for a in artists if isinstance(a, dict) and a.get('name')) if isinstance(artists, list) else str(artists or '')
                    uri = ent.get('uri', '')
                    if title:
                        return [{
                            'title': title,
                            'artist': artist.strip(),
                            'query': f"{artist} - {title}" if artist.strip() else title,
                            'uri': uri,
                        }]
                except Exception:
                    pass
        return []

    # For playlists/albums: fetch the pages in parallel. The embed API
    # returns ~100 tracks per page; a short page (or an empty one) marks the
    # end of the list. Sequential fetching of 20 pages took ~15 s on big
    # playlists — with 8 threads it's a couple of seconds.
    import concurrent.futures
    PAGE_SIZE = 100
    MAX_PAGES = 20  # Safety cap at 2000 tracks
    offsets = list(range(0, PAGE_SIZE * MAX_PAGES, PAGE_SIZE))

    def fetch_one(off):
        url = f'{base}/{media_type}/{media_id}?offset={off}'
        html = fetch_page(url)
        if not html:
            return off, []
        return off, parse_tracklist_from_html(html)

    with concurrent.futures.ThreadPoolExecutor(max_workers=8) as ex:
        pages = list(ex.map(fetch_one, offsets))

    all_tracks = []
    for off, page_tracks in sorted(pages):
        if not page_tracks or len(page_tracks) < PAGE_SIZE:
            # Page vide ou dernière page : on s'arrête là.
            all_tracks.extend(page_tracks)
            break
        all_tracks.extend(page_tracks)
    return all_tracks


def main():
    download_dir = sys.argv[1] if len(sys.argv) > 1 else "./downloads"
    media_url = sys.argv[2] if len(sys.argv) > 2 else ""

    existing = get_existing_filenames(download_dir)
    sys.stderr.write(f"Found {len(existing)} existing normalized filenames on disk.\n")

    tracks = fetch_all_tracks(media_url)

    if not tracks:
        print(json.dumps({
            'fast_filter_applied': False,
            'reason': 'No tracks extracted via embed API'
        }))
        return

    sys.stderr.write(f"Total tracks extracted: {len(tracks)}\n")

    skipped = []
    missing = []

    for t in tracks:
        norm_title = normalize(t['title'])
        norm_query = normalize(t['query'])

        # Lookup O(1) dans le set des noms de fichiers normalisés (au lieu de
        # parcourir toute la bibliothèque pour chaque titre). Le nom de
        # fichier normalisé = titre normalisé (fichiers « Title.mp3 ») ou
        # requête normalisée (fichiers « Artist - Title.mp3 »).
        if (norm_title and norm_title in existing) or (norm_query and norm_query in existing):
            skipped.append(t['query'])
        else:
            missing.append(t['query'])

    print(json.dumps({
        'fast_filter_applied': True,
        'total_tracks': len(tracks),
        'already_downloaded_count': len(skipped),
        'missing_count': len(missing),
        'skipped_tracks': skipped,
        'missing_queries': missing
    }))

if __name__ == "__main__":
    main()
