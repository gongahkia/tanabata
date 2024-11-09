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


async def scrape_vivocity(base_url):
    """
    Scrapes VivoCity's Dining Guide from the provided base URL asynchronously
    Stops scrolling when no further content is loaded (i.e., page height stops increasing)
    """
    details_list = []
    errors = []
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        page = await browser.new_page()
        try:
            await page.goto(base_url)
            await page.wait_for_selector("div.ais-Hits-item", timeout=10000)
            previous_height = 0
            while True:
                await page.evaluate("window.scrollTo(0, document.body.scrollHeight)")
                print("scrolling down...")
                await page.wait_for_timeout(5000)
                new_height = await page.evaluate("document.body.scrollHeight")
                print(
                    f"previous height: {previous_height}\ncurrent height: {new_height}"
                )
                if new_height == previous_height:
                    print("height hasn't changed, exiting loop")
                    break
                previous_height = new_height

            items = await page.query_selector_all("div.ais-Hits-item")
            for item in items:
                url_element = await item.query_selector("div.guideListWrapper a")
                url = await url_element.get_attribute("href") if url_element else ""
                name_element = await item.query_selector(
                    "div.guideListContentWrapper div.guideListContent a.storeName"
                )
                name = (
                    clean_string(await name_element.inner_text())
                    if name_element
                    else ""
                )
                location_element = await item.query_selector(
                    "div.guideListContentWrapper div.guideListContent div.storeContentSection span.storeTextContent"
                )
                location = (
                    clean_string(await location_element.inner_text())
                    if location_element
                    else ""
                )
                category_element = await item.query_selector(
                    "div.guideListContentWrapper div.guideListContent div.storeContentSectionSecond span.storeTextContent"
                )
                category = (
                    clean_string(await category_element.inner_text())
                    if category_element
                    else ""
                )
                if name and url and location and category:
                    details = {
                        "name": name,
                        "location": location,
                        "description": "",
                        "category": category,
                        "url": url,
                    }
                    print(details)
                    details_list.append(details)
        except Exception as e:
            errors.append(f"Error processing {base_url}: {str(e)}")
        await browser.close()
    return details_list, errors


async def run_scraper(target_url):
    """
    Actual function to call the scraper code asynchronously and display it to users
    """
    details_list, errors = await scrape_vivocity(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
