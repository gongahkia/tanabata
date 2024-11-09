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
    Sanitize a provided string
    """
    cleaned_string = re.sub(r"\n+", " ", input_string)
    cleaned_string = re.sub(r"<[^>]+>", "", cleaned_string)
    return cleaned_string.strip()


async def scrape_kinex_dining(base_url):
    """
    Scrapes the Kinex website for dining details asynchronously
    """
    details_list = []
    errors = []
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        page = await browser.new_page()
        try:
            await page.goto(base_url)
            await page.wait_for_selector("div.col-sm-3.col-xs-6")
            items = await page.query_selector_all("div.col-sm-3.col-xs-6")
            for item in items:
                name_element = await item.query_selector(
                    "div.item-list div.item-text.main-color1 span.item-list-title"
                )
                location_element = await item.query_selector(
                    "div.item-list div.item-text.main-color1 span:nth-child(2)"
                )
                url_element = await item.query_selector("a")

                # Extract data asynchronously
                name = await name_element.inner_text() if name_element else ""
                location = (
                    await location_element.inner_text() if location_element else ""
                )
                url = await url_element.get_attribute("href") if url_element else ""

                details = {
                    "name": clean_string(name),
                    "location": clean_string(location),
                    "description": "",
                    "category": "Dining",
                    "url": url,
                }
                print(details)
                details_list.append(details)
        except Exception as e:
            errors.append(f"Error processing {base_url}: {e}")
        finally:
            await browser.close()
    return details_list, errors


async def run_scraper(target_url):
    """
    Actual function to call the scraper code and display it to users asynchronously
    """
    details_list, errors = await scrape_kinex_dining(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
