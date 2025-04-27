# ----- required imports -----

from playwright.async_api import async_playwright
import json
import os
import asyncio
from datetime import datetime

# ----- helper functions -----

async def scrape_quotes():
    rappers = ["kendrick-lamar", "tupac-shakur", "eminem"]
    all_quotes = []
    async with async_playwright() as p:
        browser = await p.chromium.launch()
        page = await browser.new_page()
        for rapper in rappers:
            await page.goto(f"https://www.brainyquote.com/authors/{rapper}")
            await page.wait_for_timeout(5000)  
            for _ in range(3):
                await page.mouse.wheel(0, 15000)
                await page.wait_for_timeout(2000)  
            quotes = await page.query_selector_all('.grid-item .oncl_q')
            for quote in quotes:
                text = await quote.inner_text()
                text = text.strip()
                print(text)
                if text:
                    all_quotes.append({
                        "author": rapper.replace("-", " ").title(),
                        "text": text
                    })
        await browser.close()
    os.makedirs('../api/data', exist_ok=True)
    with open('../api/data/quotes.json', 'w') as f:
        json.dump(all_quotes, f, indent=2)
    print(f"Scraped {len(all_quotes)} quotes at {datetime.now()}")

# ----- execution code -----

if __name__ == "__main__":
    asyncio.run(scrape_quotes())