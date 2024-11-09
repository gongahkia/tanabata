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


async def scrape_thomson_plaza(base_url):
    """
    Scrapes Thomson Plaza website for directory details asynchronously.
    """
    details_list = []
    errors = []
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        page = await browser.new_page()
        try:
            await page.goto(base_url)
            await page.wait_for_selector("div.l-more a.button.directory-link-btn")
            while True:
                current_item_count = len(
                    await page.query_selector_all('li[style="display: list-item;"]')
                )
                try:
                    load_more_button = await page.query_selector(
                        "div.l-more a.button.directory-link-btn"
                    )
                    if load_more_button:
                        print("clicking load button")
                        await load_more_button.click()
                        await page.wait_for_timeout(1000)
                        new_item_count = len(
                            await page.query_selector_all(
                                'li[style="display: list-item;"]'
                            )
                        )
                        if new_item_count == current_item_count:
                            print("No more items to load.")
                            break
                    else:
                        break
                except Exception as e:
                    print(f"No more load more button to click: {e}")
                    break

            list_items = await page.query_selector_all(
                'li[style="display: list-item;"]'
            )
            for item in list_items:
                name_element = await item.query_selector(
                    "div.directoryInfoBoxInner div.directoryContent div.directoryName"
                )
                category_element = await item.query_selector(
                    "div.directoryInfoBoxInner div.directoryContent div.directoryCat"
                )
                location_element = await item.query_selector(
                    "div.directoryInfoBoxInner div.directoryContent div.store-location"
                )
                description_element = await item.query_selector(
                    "div.directoryInfoBoxInner div.directoryContent div.store-tel"
                )
                url_element = await item.query_selector("a")

                name = (
                    clean_string(await name_element.inner_text())
                    if name_element
                    else ""
                )
                category = (
                    clean_string(await category_element.inner_text())
                    if category_element
                    else ""
                )
                location = (
                    clean_string(await location_element.inner_text())
                    if location_element
                    else ""
                )
                description = (
                    clean_string(await description_element.inner_text())
                    if description_element
                    else ""
                )
                url = await url_element.get_attribute("href") if url_element else ""

                details = {
                    "name": name,
                    "location": location,
                    "description": description,
                    "category": category,
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
    Actual function to call the scraper code asynchronously and display it to users.
    """
    details_list, errors = await scrape_thomson_plaza(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
