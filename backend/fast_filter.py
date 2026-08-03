#!/usr/bin/env python3
import os
import sys
import glob
import re
import json
import urllib.request

def normalize(text):
    """Normalize text for fuzzy duplicate matching."""
    text = re.sub(r'[\(\[\{].*?[\)\]\}]', '', text)
    text = re.sub(r'[^\w\s]', '', text)
    return text.strip().lower()

def get_existing_filenames(download_dir):
    """Collect all normalized filenames currently in downloads directory."""
    existing = set()
    mp3_files = glob.glob(os.path.join(download_dir, '**', '*.mp3'), recursive=True)
    for f in mp3_files:
        basename = os.path.splitext(os.path.basename(f))[0]
        existing.add(normalize(basename))
        existing.add(basename.lower().strip())
    return existing

def fetch__embed_tracks(_url):
    """Fast extraction of  playlist tracks via Embed API."""
    match = re.search(r'(playlist|album|track)/([a-zA-Z0-9]+)', _url)
    if not match:
        return []
    
    media_type, media_id = match.group(1), match.group(2)
    embed_url = f"https://open..com/embed/{media_type}/{media_id}"
    
    req = urllib.request.Request(
        embed_url,
        headers={'User-Agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)'}
    )
    
    try:
        html = urllib.request.urlopen(req, timeout=10).read().decode('utf-8')
        m = re.search(r'<script id="__NEXT_DATA__" type="application/json">(.*?)</script>', html, re.DOTALL)
        if m:
            data = json.loads(m.group(1))
            track_list = data.get('props', {}).get('pageProps', {}).get('state', {}).get('data', {}).get('entity', {}).get('trackList', [])
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
        sys.stderr.write(f"Fast filter embed fetch error: {e}\n")
    return []

def main():
    download_dir = sys.argv[1] if len(sys.argv) > 1 else "./downloads"
    _url = sys.argv[2] if len(sys.argv) > 2 else ""

    existing = get_existing_filenames(download_dir)
    tracks = fetch__embed_tracks(_url)

    if not tracks:
        # Fallback if URL is not a playlist or embed returns empty
        print(json.dumps({
            'fast_filter_applied': False,
            'reason': 'No tracks extracted via embed API'
        }))
        return

    skipped = []
    missing = []

    for t in tracks:
        norm_title = normalize(t['title'])
        norm_query = normalize(t['query'])
        
        is_existing = False
        for ex in existing:
            if norm_title in ex or norm_query in ex:
                is_existing = True
                break

        if is_existing:
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
