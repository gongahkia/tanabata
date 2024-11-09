import json
import os
import re
import asyncio
from playwright.async_api import async_playwright


async def delete_file(target_url):
    """
    Helper function that attempts to delete a file at the specified URL asynchronously.
    """
    try:
        os.remove(target_url)
        print(f"Deleted file at filepath: {target_url}")
    except OSError as e:
        print(f"Error deleting file at filepath: {target_url} due to {e}")


def clean_string(input_string):
    """
    Sanitize a provided string.
    """
    cleaned_string = re.sub(r"\n+", " ", input_string)
    cleaned_string = re.sub(r"<[^>]+>", "", cleaned_string)
    return cleaned_string.strip()


async def scrape_paragon_food_beverage(base_url):
    """
    Scrapes the Paragon Mall website for food and beverage store details asynchronously.
    """
    details_list = []
    errors = []
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        page = await browser.new_page()
        try:
            await page.goto(base_url)
            await page.wait_for_selector("div.mix.category-1.all.store-link")
            print(f"Successfully retrieved page URL: {base_url}")
            stores = await page.query_selector_all(
                "div.mix.category-1.all.store-link a"
            )
            for store in stores:
                name_element = await store.query_selector(
                    "div.text-overlay div.text-cell h5"
                )
                name = (
                    clean_string(await name_element.inner_text())
                    if name_element
                    else "N/A"
                )
                url = await store.get_attribute("href") if store else "N/A"
                details = {
                    "name": name,
                    "location": "Paragon Mall",
                    "description": "",
                    "category": "Food & Beverage",
                    "url": f"https://www.paragon.com.sg{url}",
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
    Actual function to call the scraper code asynchronously and display it to users.
    """
    details_list, errors = await scrape_paragon_food_beverage(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
