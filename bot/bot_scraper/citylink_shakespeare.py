import json
import os
import re
from playwright.async_api import async_playwright
import asyncio


async def delete_file(target_url):
    """
    Helper function that attempts to delete a file at the specified URL
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


async def scrape_citylink_mall(base_url):
    """
    Scrapes the CityLink Mall website for restaurant and cafe details asynchronously
    """
    details_list = []
    errors = []
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        page = await browser.new_page()
        try:
            await page.goto(base_url)
            await page.wait_for_selector("a.user-directory-item")
            items = await page.query_selector_all("a.user-directory-item")
            for item in items:
                url_element = item
                name_element = await item.query_selector("div.user-directory-name")
                location_element = await item.query_selector(
                    "span.user-directory-unit-number"
                )
                url = await url_element.get_attribute("href") if url_element else ""
                name = (
                    clean_string(await name_element.inner_text())
                    if name_element
                    else ""
                )
                location = (
                    clean_string(await location_element.inner_text())
                    if location_element
                    else ""
                )
                details = {
                    "name": name.rstrip(location).strip(),
                    "location": location,
                    "description": "",
                    "category": "Restaurants & Cafes",
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
    details_list, errors = await scrape_citylink_mall(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
