import json
import os
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
    return input_string.strip()


async def scrape_great_world_dining(base_url):
    """
    Scrapes the Great World shopping website for dining details and handles pagination asynchronously
    """
    details_list = []
    errors = []

    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        page = await browser.new_page()

        try:
            await page.goto(base_url)
            await page.wait_for_selector("div.shopboxwrap")
            print(f"successfully retrieved page URL: {base_url}")

            while True:
                dining_stores = await page.query_selector_all("div.shopboxwrap")
                for store in dining_stores:
                    name_element = await store.query_selector("div.shoptitle")
                    location_element = await store.query_selector("div.shopunit")
                    category_element = await store.query_selector("div.shopcategory")
                    url_element = await store.query_selector("div.shopbox a")
                    name = (
                        clean_string(await name_element.inner_text())
                        if name_element
                        else ""
                    )
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
                    url = await url_element.get_attribute("href") if url_element else ""
                    details = {
                        "name": name,
                        "location": location,
                        "description": "",
                        "category": category,
                        "url": url,
                    }
                    print(details)
                    details_list.append(details)

                pagination_container = await page.query_selector(
                    "div.paginationcontainer div.wp-pagenavi"
                )
                current_page = (
                    await pagination_container.query_selector("span.current")
                    if pagination_container
                    else None
                )
                current_page_text = (
                    await current_page.inner_text() if current_page else None
                )
                final_breakpoint = (
                    await pagination_container.query_selector("a.page.larger")
                    if pagination_container
                    and await pagination_container.query_selector("a.page.larger")
                    else None
                )
                print(f"current page is {current_page_text}")

                if final_breakpoint is None:
                    print("no more pages to scrape.")
                    break
                else:
                    next_page = await pagination_container.query_selector(
                        "a.page.larger"
                    )
                    print(f"navigating to next page {await next_page.inner_text()}")
                    await next_page.click()
                    await page.wait_for_timeout(2000)

        except Exception as e:
            errors.append(f"Error processing {base_url}: {e}")

        finally:
            await browser.close()

    return details_list, errors


async def run_scraper(target_url):
    """
    Actual function to call the scraper code and display it to users asynchronously
    """
    details_list, errors = await scrape_great_world_dining(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
