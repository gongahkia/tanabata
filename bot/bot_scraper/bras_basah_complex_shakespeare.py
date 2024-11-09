import json
import os
import re
from playwright.async_api import async_playwright


async def delete_file(target_url):
    """
    Helper function to delete a file at the specified URL
    """
    try:
        os.remove(target_url)
        print(f"Deleted file at filepath: {target_url}")
    except OSError as e:
        print(f"Error deleting file at filepath: {target_url} due to {e}")


async def clean_string(input_string):
    """
    Sanitize a provided string by stripping unnecessary characters
    """
    cleaned_string = re.sub(r"\n+", " ", input_string)
    cleaned_string = re.sub(r"<[^>]+>", "", cleaned_string)
    return cleaned_string.strip()


async def scrape_bras_basah(base_url):
    """
    Scrapes Bras Basah Complex's shops from the provided base URL
    """
    details_list = []
    errors = []
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        page = await browser.new_page()
        try:
            await page.goto(base_url)
            await page.wait_for_selector("div.stores-list-store")
            items = await page.query_selector_all("div.stores-list-store")
            for item in items:
                url_element = await item.query_selector("a")
                name_element = await item.query_selector("div a")
                description_element = await item.query_selector("div p")
                url = await url_element.get_attribute("href") if url_element else ""
                name = (
                    await clean_string(await name_element.inner_text())
                    if name_element
                    else ""
                )
                description = (
                    await clean_string(await description_element.inner_text())
                    if description_element
                    else ""
                )
                details = {
                    "name": name,
                    "location": "",
                    "description": description,
                    "category": "Food and Dining",
                    "url": f"https://www.brasbasahcomplex.com{url}",
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
    Actual function to call the scraper code and display it to users
    """
    details_list, errors = await scrape_bras_basah(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
