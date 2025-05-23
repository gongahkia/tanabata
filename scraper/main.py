# ----- required imports -----

from playwright.async_api import async_playwright
import json
import os
import asyncio
from datetime import datetime

# ----- helper functions -----

async def scrape_quotes():
    print("Scraping quotes...")
    artists = [
        "kendrick-lamar",
        "tupac-shakur", 
        "eminem",
        "taylor-swift",
        "bob-dylan",
        "michael-jackson",
        "prince",
        "david-bowie",
        "bob-marley",
        "pink-floyd",
        "the-beatles",
        "queen",
        "led-zeppelin",
        "jay-z",
        "radiohead",
        "jimi-hendrix",
        "stevie-wonder",
        "frank-ocean",
        "adele",
        "kanye-west",
        "metallica",
        "rolling-stones",
        "bts",
        "rihanna",
        "lady-gaga",
        "olivia-rodrigo",
    ]
    all_quotes = []
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        context = await browser.new_context()
        page = await context.new_page()
        for artist in artists:
            target_url = f"https://quotefancy.com/{artist}-quotes"
            print(f"Navigating to {target_url}")
            try:
                await page.goto(target_url)
                await page.wait_for_selector('.quote-a', timeout=30000)
                for _ in range(3):
                    await page.mouse.wheel(0, 15000)
                    await page.wait_for_timeout(2000)
                    # print("Scrolling down...")
                quotes = await page.query_selector_all('.quote-a')
                print(f"Found {len(quotes)} quotes for {artist}")
                for quote in quotes:
                    text = await quote.inner_text()
                    text = text.strip()
                    if text:
                        # print(text)
                        all_quotes.append({
                            "author": artist.replace("-", " ").title(),
                            "text": text
                        })
            except Exception as e:
                print(f"Error scraping {artist}: {str(e)}")
                continue
        await browser.close()
    os.makedirs('../api/data', exist_ok=True)
    with open('../api/data/quotes.json', 'w') as f:
        json.dump(all_quotes, f, indent=2)
    print(f"Successfully scraped {len(all_quotes)} quotes from {len(artists)} artists at {datetime.now()}")

# ----- execution code -----

if __name__ == "__main__":
    asyncio.run(scrape_quotes())