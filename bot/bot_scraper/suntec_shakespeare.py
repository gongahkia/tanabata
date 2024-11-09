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


async def scrape_suntec_city_dining(base_url):
    """
    Scrapes the Suntec City website for dining details asynchronously.
    """
    details_list = []
    errors = []
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        page = await browser.new_page()
        try:
            await page.goto(base_url)
            await page.wait_for_selector("div.explore-list ul.explores.clearfix")
            print(f"Successfully retrieved page URL: {base_url}")
            dining_categories = await page.query_selector_all(
                "div.explore-list ul.explores.clearfix li div.explore-item.clearfix"
            )
            for category in dining_categories:
                name_element = await category.query_selector("div.info div.name a")
                location_element = await category.query_selector(
                    "div.info div.address span a.address"
                )
                name = clean_string(await name_element.inner_text())
                location = clean_string(await location_element.inner_text())

                details = {
                    "name": name,
                    "location": location,
                    "description": "",
                    "category": "Dining",
                    "url": base_url,
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
    Actual function to call the scraper code asynchronously
    and display it to users.
    """
    details_list, errors = await scrape_suntec_city_dining(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
