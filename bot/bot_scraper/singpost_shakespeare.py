import json
import os
import re
import asyncio
from playwright.async_api import async_playwright


async def delete_file(target_url):
    """
    Helper function that attempts to delete a
    file at the specified URL asynchronously.
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


async def scrape_singpost_centre(base_url):
    """
    Scrapes the Singpost Centre website for cafes and restaurant details asynchronously.
    """
    details_list = []
    errors = []
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        page = await browser.new_page()
        try:
            await page.goto(base_url)
            await page.wait_for_selector("div.card.h-100")
            print(f"Successfully retrieved page URL: {base_url}")
            while True:
                try:
                    load_more_button = await page.query_selector(
                        "a#loadMore.btn.loadMoreBtn"
                    )
                    print("Scanning for load more button...")
                    if load_more_button:
                        style = await load_more_button.get_attribute("style")
                        if style == "display: none;":
                            print("Finished loading all pages")
                            break
                        else:
                            print("Load more button found!")
                            await load_more_button.click()
                            await page.wait_for_timeout(2000)
                except Exception as e:
                    print(f"Error loading more entries: {e}")
                    break
            locations = await page.query_selector_all("div.col div.card.h-100")
            for location in locations:
                category = await location.query_selector("div.card-subtitle.ls-2")
                category = await category.inner_text() if category else ""
                name_element = await location.query_selector("a.stretched-link")
                name = await name_element.inner_text() if name_element else ""
                url = await name_element.get_attribute("href") if name_element else ""
                details = {
                    "name": clean_string(name),
                    "location": "",
                    "description": "",
                    "category": clean_string(category),
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
    Actual function to call the scraper code asynchronously
    and display it to users
    """
    details_list, errors = await scrape_singpost_centre(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
