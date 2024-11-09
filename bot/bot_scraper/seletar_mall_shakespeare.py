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


async def scrape_seletar_mall(base_url):
    """
    Scrapes The Seletar Mall website for dining details and navigates through pages by clicking the "Next" button asynchronously
    """
    details_list = []
    errors = []
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        page = await browser.new_page()
        try:
            await page.goto(base_url)
            await page.wait_for_selector("div.shopboxwrap")
            while True:
                items = await page.query_selector_all("div.shopboxwrap div.shopbox")
                for item in items:
                    url_element = await item.query_selector("a")
                    location_element = await item.query_selector(
                        "div.shopsummarybox div.shoplevelunitbox"
                    )
                    category_element = await item.query_selector(
                        "div.shopsummarybox div.shopcategory"
                    )
                    name_element = await item.query_selector(
                        "div.shopsummarybox div.shoptitle"
                    )
                    url = await url_element.get_attribute("href") if url_element else ""
                    location = (
                        clean_string(await location_element.inner_text())
                        if location_element
                        else ""
                    )
                    category = (
                        clean_string(await category_element.inner_text())
                        if category_element
                        else ""
                    )
                    name = (
                        clean_string(await name_element.inner_text())
                        if name_element
                        else ""
                    )
                    if name == "" and location == "" and category == "":
                        continue
                    details = {
                        "name": name,
                        "location": location,
                        "description": "",
                        "category": category,
                        "url": url,
                    }
                    print(details)
                    details_list.append(details)
                next_button = await page.query_selector(
                    "div.wp-pagenavi a.nextpostslink"
                )
                if next_button:
                    await next_button.click()
                    await page.wait_for_timeout(2000)
                    print("Navigating to next page...")
                else:
                    print("No more pages to navigate.")
                    break
        except Exception as e:
            errors.append(f"Error processing {base_url}: {e}")
        finally:
            await browser.close()
    return details_list, errors


async def run_scraper(target_url):
    """
    Actual function to call the scraper code asynchronously and display it to users.
    """
    details_list, errors = await scrape_seletar_mall(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
