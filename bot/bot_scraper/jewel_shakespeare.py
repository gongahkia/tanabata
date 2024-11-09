import json
import os
import re
import asyncio
from playwright.async_api import async_playwright


async def delete_file(target_url):
    """
    Helper function that attempts to delete a file at the specified URL asynchronously
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


async def scrape_jewel_changi(base_url):
    """
    Scrapes the specified Jewel Changi Airport website for food and beverage details asynchronously
    """
    details_list = []
    errors = []
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        page = await browser.new_page()
        try:
            await page.goto(base_url)
            await page.wait_for_selector(
                "div.list-item.col-lg-4.col-md-4.col-sm-4.col-xs-12"
            )
            print(f"Successfully retrieved page URL: {base_url}")
            locations = await page.query_selector_all(
                "div.list-item.col-lg-4.col-md-4.col-sm-4.col-xs-12"
            )
            for location in locations:
                name = await location.query_selector("h3.company-name")
                location_info = await location.query_selector("div.intro.shadow p.desc")
                description = await location.query_selector(
                    "div.intro.shadow p.open-times"
                )
                categories = await location.query_selector_all(
                    "div.intro.shadow p.tags span.tag"
                )

                # Fetch inner texts asynchronously
                name_text = await name.inner_text() if name else ""
                location_text = (
                    await location_info.inner_text() if location_info else ""
                )
                description_text = await description.inner_text() if description else ""
                category_texts = [
                    await category.inner_text() for category in categories
                ]

                details = {
                    "name": name_text,
                    "location": clean_string(location_text),
                    "description": clean_string(description_text),
                    "category": [cat.strip() for cat in category_texts],
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
    Actual function to call the scraper code and display it to users asynchronously
    """
    details_list, errors = await scrape_jewel_changi(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
