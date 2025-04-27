# ----- required imports -----

from playwright.sync_api import sync_playwright
import json
import os
from datetime import datetime

# ----- helper functions -----

def scrape_quotes():
    rappers = ["kendrick-lamar", "tupac-shakur", "eminem"]
    all_quotes = []
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        for rapper in rappers:
            page.goto(f"https://www.brainyquote.com/authors/{rapper}")
            for _ in range(5):
                page.mouse.wheel(0, 15000)
                page.wait_for_timeout(1000)
            quotes = page.query_selector_all('.grid-item .oncl_q')
            for quote in quotes:
                text = quote.inner_text().strip()
                print(text)
                if text:
                    all_quotes.append({
                        "author": rapper.replace("-", " ").title(),
                        "text": text
                    })
        browser.close()
    os.makedirs('../api/data', exist_ok=True)
    with open('../api/data/quotes.json', 'w') as f:
        json.dump(all_quotes, f, indent=2)
    print(f"Scraped {len(all_quotes)} quotes at {datetime.now()}")

# ----- execution code -----

if __name__ == "__main__":
    scrape_quotes()