import json
import os
import re
from playwright.async_api import async_playwright
import asyncio


async def delete_file(target_url):
    """
    Helper function to delete a file at the specified URL
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


async def scrape_custom_mall(base_url):
    """
    Scrapes a custom mall page with the provided base URL asynchronously
    """
    details_list = []
    errors = []
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        page = await browser.new_page()
        try:
            await page.goto(base_url)
            await page.wait_for_selector(
                "div.mix.col-md-6.cat-foodcourt-restaurants-cafes div.storelist-wrapper",
                timeout=10000,
            )
            items = await page.query_selector_all(
                "div.mix.col-md-6.cat-foodcourt-restaurants-cafes div.storelist-wrapper"
            )
            for item in items:
                name_element = await item.query_selector("div.col-xs-20 p.title")
                name = (
                    clean_string(await name_element.inner_text())
                    if name_element
                    else ""
                )
                raw_elements = await item.query_selector_all(
                    "div.col-xs-8.col-sm-4 div.meta-wrap"
                )
                location = description = ""
                for element in raw_elements:
                    class_name = await element.query_selector("span")
                    class_name = (
                        await class_name.get_attribute("class") if class_name else ""
                    )
                    if "icon-Location" in class_name:
                        location = await element.inner_text()
                    elif "icon-Telephone" in class_name:
                        description = await element.inner_text()
                url_element = await item.query_selector(
                    "div.col-xs-8.col-sm-4 div.meta-wrap a"
                )
                url = await url_element.get_attribute("href") if url_element else ""
                if name and url:
                    details = {
                        "name": name,
                        "location": location,
                        "description": description,
                        "category": "Foodcourt, Restaurants and Cafes",
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
    Actual function to call the scraper code and display it to users asynchronously
    """
    details_list, errors = await scrape_custom_mall(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
