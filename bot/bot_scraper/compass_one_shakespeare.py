import json
import os
import re
from playwright.async_api import async_playwright
import asyncio


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
    cleaned_string = re.sub(r"\n+", " ", input_string)
    cleaned_string = re.sub(r"<[^>]+>", "", cleaned_string)
    return cleaned_string.strip()


async def scrape_compass_one(base_url):
    """
    Scrapes the Compass One website for restaurant and cafe details asynchronously
    """
    details_list = []
    errors = []
    travelled_pages = []
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        page = await browser.new_page()
        try:
            await page.goto(base_url)
            await page.wait_for_selector(
                "div.w-hwrapper.usg_hwrapper_1.align_left.valign_top.wrap"
            )
            print(f"successfully retrieved page URL: {base_url}")
            while True:
                store_items = await page.query_selector_all(
                    "div.w-hwrapper.usg_hwrapper_1.align_left.valign_top.wrap"
                )
                for store in store_items:
                    name_element = await store.query_selector(
                        "h2.w-post-elm.post_title.usg_post_title_1.entry-title.cmp-title a"
                    )
                    location_element = await store.query_selector(
                        "div.w-post-elm.post_content.usg_post_content_1 p"
                    )
                    name = (
                        clean_string(await name_element.inner_text())
                        if name_element
                        else None
                    )
                    location = (
                        clean_string(await location_element.inner_text())
                        if location_element
                        else None
                    )
                    url = (
                        await name_element.get_attribute("href")
                        if name_element
                        else None
                    )
                    details = {
                        "name": name,
                        "location": location,
                        "description": "",
                        "category": "Restaurant, Cafe & Fast Food",
                        "url": url,
                    }
                    print(details)
                    details_list.append(details)

                current_page = await page.query_selector(
                    "div.nav-links span.page-numbers.current"
                )
                travelled_pages.append(await current_page.inner_text())
                print(f"page numbers visited: {travelled_pages}")

                next_page_array = [
                    page_el
                    for page_el in await page.query_selector_all(
                        "div.nav-links a.page-numbers"
                    )
                    if (await page_el.inner_text()).isnumeric()
                    and (await page_el.inner_text()).strip() not in travelled_pages
                ]
                if next_page_array:
                    next_page = next_page_array[0]
                    print(
                        f"page numbers remaining: {[await el.inner_text() for el in next_page_array]}"
                    )
                if len(next_page_array) == 0:
                    print("No more pages to navigate.")
                    break
                print(
                    f"current page: {await current_page.inner_text()}, navigating to next page {await next_page.inner_text()}..."
                )
                await next_page.click()
                await page.wait_for_timeout(2000)
        except Exception as e:
            errors.append(f"Error processing {base_url}: {str(e)}")
        finally:
            await browser.close()

    return details_list, errors


async def run_scraper(target_url):
    """
    Actual function to call the scraper code and display it to users asynchronously
    """
    details_list, errors = await scrape_compass_one(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
