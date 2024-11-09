import json
import os
import re
from playwright.async_api import async_playwright
import asyncio


async def delete_file(target_url):
    """
    Helper function that attempts to delete a file at the specified URL
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


async def scrape_frasers_mall(base_url):
    """
    Scrapes a given Frasers Mall website for food and beverage details
    """
    details_list = []
    errors = []

    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        page = await browser.new_page()
        try:
            await page.goto(base_url)
            await page.wait_for_selector("div.details")
            print(f"successfully retrieved page URL: {base_url}")

            while True:
                initial_count = len(await page.query_selector_all("div.detail"))
                try:
                    load_more_button = await page.query_selector(
                        "a.full-btn.loadmore.loadmore_vtwo"
                    )
                    if load_more_button:
                        print("Clicking load more button...")
                        await load_more_button.click()
                        await page.wait_for_timeout(2000)
                        count = len(await page.query_selector_all("div.detail"))
                        if count == initial_count:
                            print("No more load more button found")
                            break
                    else:
                        print("No more load more button found")
                        break
                except Exception as e:
                    print(f"Error clicking load more button: {e}")
                    break

            store_items = await page.query_selector_all("div.details")
            for store in store_items:
                name_element = await store.query_selector("div.storename a")
                location_element = await store.query_selector("div.col.findus div.info")
                description_element_1 = await store.query_selector(
                    "div.col.callus div.info"
                )
                description_element_2 = await store.query_selector(
                    "div.col.openfrom div.info"
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
                description = ""
                if description_element_1:
                    description += clean_string(
                        await description_element_1.inner_text()
                    )
                if description_element_2:
                    description += (
                        f" {clean_string(await description_element_2.inner_text())}"
                    )
                url = await name_element.get_attribute("href") if name_element else None
                details = {
                    "name": name,
                    "location": location,
                    "description": description,
                    "category": "Food & Restaurants",
                    "url": f"{base_url.rstrip('/store.php?CategoryFilter=43&FRPointsFilter=&GCFilter=&HalalFilter=&NewStoresFilter=&CalmFilter=&DementiaFilter=&Node=&CategoryID=594')}{url}",
                }
                print(details)
                details_list.append(details)
        except Exception as e:
            errors.append(f"Error processing {base_url}: {e}")
        finally:
            await browser.close()
    return details_list, errors


async def scrape_all_frasers_malls():
    fin = {}
    FRASER_URL_ARRAY = [
        "https://www.causewaypoint.com.sg/",
        "https://www.centurysquare.com.sg/",
        "https://www.eastpoint.sg/",
        "https://www.hougangmall.com.sg/",
        "https://www.northpointcity.com.sg/",
        "https://www.robertsonwalk.com.sg/",
        "https://www.tampines1.com.sg/",
        "https://www.thecentrepoint.com.sg",
        "https://www.tiongbahruplaza.com.sg/",
        "https://www.waterwaypoint.com.sg/",
        "https://www.whitesands.com.sg/",
    ]
    URL_AUGMENT = "/store.php?CategoryFilter=43&FRPointsFilter=&GCFilter=&HalalFilter=&NewStoresFilter=&CalmFilter=&DementiaFilter=&Node=&CategoryID=594"
    for url in FRASER_URL_ARRAY:
        print(f"scraping {url} now...")
        result = await scrape_frasers_mall(f"{url}{URL_AUGMENT}")
        fin[url.split(".")[1]] = result[0]
    return fin


async def run_scraper(target_url):
    """
    Actual function to call the scraper code and display it to users
    """
    URL_AUGMENT = "/store.php?CategoryFilter=43&FRPointsFilter=&GCFilter=&HalalFilter=&NewStoresFilter=&CalmFilter=&DementiaFilter=&Node=&CategoryID=594"
    details_list, errors = await scrape_frasers_mall(f"{target_url}{URL_AUGMENT}")
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
