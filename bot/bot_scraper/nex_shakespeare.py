import json
import os
import re
import asyncio
from playwright.async_api import async_playwright


async def delete_file(target_url):
    """
    Helper function to delete a file at the specified URL asynchronously
    """
    try:
        os.remove(target_url)
        print(f"Deleted file at filepath: {target_url}")
    except OSError as e:
        print(f"Error deleting file at filepath: {target_url} due to {e}")


def clean_string(input_string):
    """
    Sanitize a provided string by stripping unnecessary characters
    """
    cleaned_string = re.sub(r"\n+", " ", input_string)
    cleaned_string = re.sub(r"<[^>]+>", "", cleaned_string)
    return cleaned_string.strip()


async def scrape_nex(base_url):
    """
    Scrapes NEX Mall's restaurants and cafes from the provided base URL asynchronously.
    Handles pagination by clicking the "Next" button until no more pages are available.
    """
    details_list = []
    errors = []
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        page = await browser.new_page()
        try:
            await page.goto(base_url)
            await page.wait_for_selector("div.storeLogo")
            while True:
                items = await page.query_selector_all("div.storeLogo")
                for item in items:
                    url_element = await item.query_selector("div.logoImg a")
                    name_element = await item.query_selector("div.logoTitle a")
                    url = await url_element.get_attribute("href") if url_element else ""
                    raw_name = (
                        clean_string(await name_element.inner_text())
                        if name_element
                        else ""
                    )
                    location = (
                        [el for el in raw_name.split() if el.startswith("#")][0]
                        if raw_name
                        else ""
                    )
                    clean_name = raw_name.replace(location, "")
                    details = {
                        "name": clean_name.strip(),
                        "location": location,
                        "description": "",
                        "category": "Restaurant, Cafe & Fast Food",
                        "url": url,
                    }
                    print(details)
                    details_list.append(details)
                next_button = await page.query_selector("ul li a.page-link.next")
                if next_button:
                    print("Navigating to the next page...")
                    await next_button.click()
                    await page.wait_for_timeout(2000)
                else:
                    print("No more pages to navigate to")
                    break
        except Exception as e:
            errors.append(f"Error processing {base_url}: {e}")
        finally:
            await browser.close()
    return details_list, errors


async def run_scraper(target_url):
    """
    Actual function to call the scraper code asynchronously and display results to users
    """
    details_list, errors = await scrape_nex(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
