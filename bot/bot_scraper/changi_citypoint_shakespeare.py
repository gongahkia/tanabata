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
    Sanitize a provided string
    """
    cleaned_string = re.sub(r"\n+", " ", input_string)
    cleaned_string = re.sub(r"<[^>]+>", "", cleaned_string)
    return cleaned_string.strip()


async def scrape_changi_city_point(base_url):
    """
    Scrapes the Changi City Point website for dining details and handles pagination asynchronously
    """
    details_list = []
    errors = []
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        page = await browser.new_page()
        try:
            await page.goto(base_url)
            await page.wait_for_selector("div.item.item-custom")
            while True:
                items = await page.query_selector_all("div.item.item-custom")
                for item in items:
                    url_element = await item.query_selector(
                        "div.content-store div.thumb a"
                    )
                    name_element = await item.query_selector(
                        "div.box-content div.name a"
                    )
                    description_elements = await item.query_selector_all(
                        "div.box-content div.list div.item.d-flex div.content"
                    )
                    print([await el.inner_text() for el in description_elements])
                    url = await url_element.get_attribute("href") if url_element else ""
                    name = (
                        clean_string(await name_element.inner_text())
                        if name_element
                        else ""
                    )
                    description = (
                        f"{clean_string(await description_elements[0].inner_text())}, {clean_string(await description_elements[1].inner_text())}"
                        if description_elements
                        else ""
                    )
                    category = (
                        clean_string(await description_elements[2].inner_text())
                        if len(description_elements) > 2
                        else ""
                    )
                    location = (
                        clean_string(await description_elements[3].inner_text())
                        if len(description_elements) > 3
                        else ""
                    )
                    details = {
                        "name": name,
                        "location": location,
                        "description": description,
                        "category": category,
                        "url": url,
                    }
                    print(details)
                    details_list.append(details)

                next_page = await page.query_selector(
                    "li.pagi-item.pagi-action.pagi-next.is-disabled a.page-link.next"
                )
                if next_page:
                    print("Navigating to next page")
                    await next_page.click()
                    await page.wait_for_timeout(2000)
                else:
                    print("No more pages to navigate to")
                    break
        except Exception as e:
            errors.append(f"Error processing {base_url}: {e}")
        finally:
            await browser.close()

    return details_list, errors


async def run_scraper(target_url):
    """
    Actual function to call the scraper code and display it to users asynchronously
    """
    details_list, errors = await scrape_changi_city_point(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
