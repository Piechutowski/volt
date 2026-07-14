#!/usr/bin/env python3
"""Generic, polite documentation crawler: HTML pages -> markdown files.

The fetch_*.sh scripts pull the four frameworks' guides straight from
their markdown/RST sources, which is cleaner than scraping. This crawler
covers what has no markdown source — generated API references — and is
the fallback if a docs site ever stops publishing its source:

    # Laravel API reference
    ./crawl_html.py --start https://api.laravel.com/docs/12.x/ \
        --out ../corpus/laravel-api

    # Rails API reference
    ./crawl_html.py --start https://api.rubyonrails.org/ \
        --out ../corpus/rails-api

    # hexdocs module reference for Phoenix
    ./crawl_html.py --start https://hexdocs.pm/phoenix/Phoenix.html \
        --prefix https://hexdocs.pm/phoenix/ --out ../corpus/phoenix-api

It stays inside --prefix (defaults to the start URL's directory),
respects robots.txt, rate-limits itself, extracts the main content
region, and mirrors URL paths to .md files. A crawl.json manifest maps
every saved file back to its URL.

Dependencies: pip install -r requirements.txt
"""

import argparse
import json
import re
import sys
import time
import urllib.parse
import urllib.robotparser
from collections import deque
from pathlib import Path

import requests
from bs4 import BeautifulSoup
from markdownify import markdownify

UA = "volt-docs-research/1.0 (+https://github.com/Piechutowski/volt)"

# Where the actual documentation text usually lives, most-specific first.
CONTENT_SELECTORS = [
    "main", "article", "[role=main]",
    "#docs-content", "#content", ".docs-content", ".content", ".document",
]
# Chrome that should never end up in the markdown.
STRIP_SELECTORS = [
    "nav", "header", "footer", "script", "style", "noscript",
    ".sidebar", ".navbar", ".breadcrumbs", ".headerlink", ".sphinxsidebar",
]


def canonical(url: str) -> str:
    """Normalize a URL for the visited-set: drop fragment and query."""
    parts = urllib.parse.urlsplit(url)
    path = parts.path or "/"
    return urllib.parse.urlunsplit((parts.scheme, parts.netloc, path, "", ""))


def url_to_relpath(url: str, prefix: str) -> Path:
    rel = canonical(url)[len(canonical(prefix)):].lstrip("/")
    rel = urllib.parse.unquote(rel)
    if not rel or rel.endswith("/"):
        rel += "index"
    rel = re.sub(r"\.html?$", "", rel)
    rel = re.sub(r"[^A-Za-z0-9._/-]", "_", rel)
    return Path(rel + ".md")


def extract_markdown(html: str, base_url: str) -> tuple[str, list[str]]:
    """Return (markdown, in-scope links) for one fetched page."""
    soup = BeautifulSoup(html, "html.parser")

    links = [
        urllib.parse.urljoin(base_url, a["href"])
        for a in soup.find_all("a", href=True)
    ]

    for sel in STRIP_SELECTORS:
        for el in soup.select(sel):
            el.decompose()

    content = None
    for sel in CONTENT_SELECTORS:
        content = soup.select_one(sel)
        if content is not None:
            break
    if content is None:
        content = soup.body or soup

    title = soup.title.get_text(strip=True) if soup.title else ""
    md = markdownify(str(content), heading_style="ATX", bullets="-").strip()
    if title and not md.startswith("#"):
        md = f"# {title}\n\n{md}"
    return md, links


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--start", action="append", required=True,
                    help="seed URL (repeatable)")
    ap.add_argument("--prefix",
                    help="only crawl URLs under this prefix "
                         "(default: directory of the first --start)")
    ap.add_argument("--out", required=True, help="output directory")
    ap.add_argument("--delay", type=float, default=0.5,
                    help="seconds between requests (default 0.5)")
    ap.add_argument("--max-pages", type=int, default=2000)
    ap.add_argument("--ignore-robots", action="store_true")
    args = ap.parse_args()

    prefix = args.prefix or args.start[0].rsplit("/", 1)[0] + "/"
    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=True)

    robots = urllib.robotparser.RobotFileParser()
    if not args.ignore_robots:
        root = urllib.parse.urljoin(prefix, "/robots.txt")
        try:
            robots.set_url(root)
            robots.read()
        except OSError:
            robots = None  # unreadable robots.txt: allow

    session = requests.Session()
    session.headers["User-Agent"] = UA

    queue = deque(canonical(u) for u in args.start)
    seen = set(queue)
    manifest, errors = {}, {}

    while queue and len(manifest) < args.max_pages:
        url = queue.popleft()
        if robots and not args.ignore_robots and not robots.can_fetch(UA, url):
            errors[url] = "disallowed by robots.txt"
            continue
        try:
            resp = session.get(url, timeout=30)
            resp.raise_for_status()
        except requests.RequestException as exc:
            errors[url] = str(exc)
            continue
        if "html" not in resp.headers.get("content-type", ""):
            continue

        md, links = extract_markdown(resp.text, url)
        rel = url_to_relpath(url, prefix)
        dest = out / rel
        dest.parent.mkdir(parents=True, exist_ok=True)
        dest.write_text(f"<!-- source: {url} -->\n\n{md}\n", encoding="utf-8")
        manifest[url] = str(rel)
        print(f"[{len(manifest)}] {url} -> {rel}", file=sys.stderr)

        for link in links:
            link = canonical(link)
            if link.startswith(canonical(prefix)) and link not in seen:
                seen.add(link)
                queue.append(link)

        time.sleep(args.delay)

    (out / "crawl.json").write_text(json.dumps(
        {"prefix": prefix, "pages": manifest, "errors": errors}, indent=2))
    print(f"done: {len(manifest)} pages, {len(errors)} errors -> {out}",
          file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
