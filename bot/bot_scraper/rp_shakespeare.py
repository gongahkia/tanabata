# ASYNC VERSION

import json
import os
import re
import asyncio
from playwright.async_api import async_playwright


def delete_file(target_url):
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


async def scrape_rp_dining(url):
    """
    Asynchronously scrapes the Republic Polytechnic retail and dining page
    """
    scraped_data = []
    errors = []

    async with async_playwright() as p:
        browser = await p.chromium.launch()
        page = await browser.new_page()
        try:
            await page.goto(url)
            await page.wait_for_selector("div.row.mb-50")
            print(f"Scraping the URL: {url}")
            post_items = await page.query_selector_all("div.row.mb-50")
            for item in post_items:
                name_element = await item.query_selector("div div div h3")
                description_element = await item.query_selector("div div div p")

                name = await name_element.inner_text() if name_element else ""
                description = (
                    await description_element.inner_text()
                    if description_element
                    else ""
                )

                details = {
                    "name": name.strip(),
                    "location": "",
                    "description": clean_string(description),
                    "category": "Retail & Dining",
                    "url": url,
                }
                print(details)
                scraped_data.append(details)
        except Exception as e:
            errors.append(f"Error processing {url}: {e}")
        finally:
            await browser.close()

    # output_path = "./../output/rp_dining_data.json"
    # with open(output_path, "w") as f:
    #     json.dump(scraped_data, f, indent=4)

    return scraped_data, errors


async def run_scraper(target_url):
    """
    Asynchronously calls the scraper function
    """
    details_list, errors = await scrape_rp_dining(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
