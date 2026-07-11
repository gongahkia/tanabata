import json
import os
import sqlite3
import time
from datetime import datetime, timezone
from html.parser import HTMLParser
from pathlib import Path
from urllib.error import HTTPError
from urllib.parse import urlencode, quote
from urllib.request import Request, urlopen


API_URL = "https://en.wikiquote.org/w/api.php"
LICENSE = "CC-BY-SA-4.0"
MAX_QUOTES_PER_ARTIST = 25
MAX_WORDS_PER_QUOTE = 250
CURATED_ARTISTS = {"David Bowie", "Jimi Hendrix", "Leonard Cohen", "Nina Simone"}


class ListItemParser(HTMLParser):
    def __init__(self):
        super().__init__()
        self.depth = 0
        self.items = []
        self.current = []

    def handle_starttag(self, tag, attrs):
        if tag == "li":
            self.depth += 1
            if self.depth == 1:
                self.current = []

    def handle_endtag(self, tag):
        if tag == "li" and self.depth:
            if self.depth == 1:
                value = " ".join("".join(self.current).split())
                if value:
                    self.items.append(value)
            self.depth -= 1

    def handle_data(self, data):
        if self.depth == 1:
            self.current.append(data)


def user_agent():
    version = os.environ.get("TANABATA_VERSION", "0.0.0")
    return f"Tanabata/{version} ( https://github.com/gongahkia/tanabata )"


def request_json(params):
    query = dict(params)
    query.update({"format": "json", "formatversion": "2", "maxlag": "5"})
    for attempt in range(4):
        request = Request(f"{API_URL}?{urlencode(query)}", headers={"User-Agent": user_agent(), "Api-User-Agent": user_agent()})
        try:
            with urlopen(request, timeout=20) as response:
                payload = json.load(response)
        except HTTPError as error:
            if error.code != 429 or attempt == 3:
                raise
            time.sleep(int(error.headers.get("Retry-After", "0")) or 2 ** attempt)
            continue
        if payload.get("error", {}).get("code") == "maxlag":
            if attempt == 3:
                raise RuntimeError(payload["error"].get("info", "Wikiquote maxlag"))
            time.sleep(2 ** attempt)
            continue
        return payload
    raise RuntimeError("Wikiquote request retry budget exhausted")


def catalog_artists(catalog_path):
    try:
        with sqlite3.connect(catalog_path) as database:
            rows = database.execute("SELECT name FROM artists WHERE trim(name) != ''").fetchall()
    except sqlite3.Error:
        rows = []
    return {row[0].strip() for row in rows if row[0].strip()} | CURATED_ARTISTS


def page_title(artist):
    payload = request_json({"action": "query", "list": "search", "srsearch": artist, "srnamespace": "0", "srlimit": "5"})
    results = payload.get("query", {}).get("search", [])
    if not results:
        return ""
    for result in results:
        if result["title"].casefold() == artist.casefold():
            return result["title"]
    return results[0]["title"]


def page_quotes(title):
    payload = request_json({"action": "parse", "page": title, "prop": "text"})
    parser = ListItemParser()
    parser.feed(payload.get("parse", {}).get("text", ""))
    seen = set()
    quotes = []
    for text in parser.items:
        normalized = " ".join(text.split())
        if len(normalized.split()) > MAX_WORDS_PER_QUOTE or len(normalized) < 12 or normalized in seen:
            continue
        seen.add(normalized)
        quotes.append(normalized)
        if len(quotes) == MAX_QUOTES_PER_ARTIST:
            break
    return quotes


def scrape_quotes():
    root = Path(__file__).resolve().parent.parent
    catalog_path = root / "api" / "data" / "catalog.sqlite"
    output_path = root / "api" / "data" / "quotes.json"
    retrieved_at = datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")
    records = []
    for artist in sorted(catalog_artists(catalog_path)):
        try:
            title = page_title(artist)
            if not title:
                continue
            source_url = f"https://en.wikiquote.org/wiki/{quote(title.replace(' ', '_'))}"
            attribution = f'Wikiquote contributors, "{title}", {LICENSE} ({source_url})'
            for text in page_quotes(title):
                records.append({
                    "author": artist,
                    "text": text,
                    "source_url": source_url,
                    "license": LICENSE,
                    "retrieved_at": retrieved_at,
                    "attribution_text": attribution,
                })
            time.sleep(0.35)
        except (HTTPError, OSError, RuntimeError) as error:
            print(f"Wikiquote fetch failed for {artist}: {error}")
    output_path.parent.mkdir(parents=True, exist_ok=True)
    temporary_path = output_path.with_suffix(".tmp")
    temporary_path.write_text(json.dumps(records, indent=2) + "\n", encoding="utf-8")
    temporary_path.replace(output_path)
    print(f"Wrote {len(records)} Wikiquote records for {len(catalog_artists(catalog_path))} artists")


if __name__ == "__main__":
    scrape_quotes()
