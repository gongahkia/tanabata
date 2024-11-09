import json
import os
import re
import asyncio
from playwright.async_api import async_playwright


async def delete_file(target_url):
    """
    Helper function to delete a file at the specified URL asynchronously.
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


async def scrape_parkway_parade(base_url):
    """
    Scrapes the Parkway Parade website for store details asynchronously.
    """
    details_list = []
    errors = []
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        page = await browser.new_page()
        try:
            await page.goto(base_url)
            await page.wait_for_selector(
                "div.directory-card.relative.grid.gap-5.border.px-4.py-5.bg-brand-page"
            )
            while True:
                try:
                    load_more_button = await page.query_selector("button.button.null")
                    if load_more_button:
                        await load_more_button.click()
                        await page.wait_for_timeout(2000)
                        print("Clicked 'Load More' button, loading more stores...")
                    else:
                        print(
                            "'Load More' button not found or no more content to load."
                        )
                        break
                except Exception as e:
                    print(f"Error while clicking 'Load More' button: {e}")
                    break
            items = await page.query_selector_all(
                "div.directory-card.relative.grid.gap-5.border.px-4.py-5.bg-brand-page"
            )
            for item in items:
                url_element = await item.query_selector(
                    "div.relative.w-full.my-0.m-auto.overflow-hidden.max-w-store-logo-sm a.block.aspect-w-1.aspect-h-1"
                )
                name_element = await item.query_selector(
                    "a.inline-flex.items-center.font-bold.border-brand-link.mb-0.border-b-2"
                )
                description_elements = await item.query_selector_all("div.flex")
                url = await url_element.get_attribute("href") if url_element else ""
                name = (
                    clean_string(await name_element.inner_text())
                    if name_element
                    else ""
                )
                location = ""
                description = ""
                for desc in description_elements:
                    if (text := await desc.inner_text().strip()) != "":
                        if text.startswith("#"):
                            location = text
                        else:
                            description += text
                details = {
                    "name": name,
                    "location": location,
                    "description": description,
                    "category": "Food & Restaurant",
                    "url": f"https://www.parkwayparade.com.sg{url}",
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
    details_list, errors = await scrape_parkway_parade(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
