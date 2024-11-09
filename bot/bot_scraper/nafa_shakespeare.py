import json
import os
import re
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
    cleaned_string = re.sub(r"\n+", " ", input_string)
    cleaned_string = re.sub(r"<[^>]+>", "", cleaned_string)
    return cleaned_string.strip()


async def scrape_nafa_dining(url):
    """
    Scrapes the NAFA site asynchronously
    """
    scraped_data = []
    errors = []
    async with async_playwright() as p:
        browser = await p.chromium.launch(
            headless=False
        )  # Launch browser with GUI for debugging
        page = await browser.new_page()
        try:
            await page.goto(url)
            while True:
                await page.wait_for_selector("div.post-item")
                print(f"Scraping the URL: {url}")
                post_items = await page.query_selector_all("div.post-item")
                for item in post_items:
                    name = (
                        (await item.query_selector("div.post-item-info h4"))
                        .inner_text()
                        .strip()
                    )
                    url = await item.query_selector(
                        "div.post-item-image.img-container-250 a"
                    ).get_attribute("href")

                    category_el = await item.query_selector(
                        "div.post-item-info div.text-muted.small.text-ellipsis"
                    )
                    if category_el:
                        category = [
                            el.strip()
                            for el in (await category_el.inner_text())
                            .strip()
                            .split("·")
                        ]
                    else:
                        category = []

                    location = (
                        (
                            await item.query_selector(
                                "div.post-item-info div.text-ellipsis-2"
                            )
                        )
                        .inner_text()
                        .strip()
                    )
                    descriptions = [
                        (await desc.inner_text()).strip()
                        for desc in await item.query_selector_all(
                            "div.post-item-info div.scroll-x.m-t-5 div.btn.btn-default"
                        )
                    ]

                    details = {
                        "name": name,
                        "location": location,
                        "descriptions": descriptions,
                        "category": category,
                        "url": url,
                    }
                    print(details)
                    scraped_data.append(details)

                next_button = await page.query_selector(
                    "ul.pagination li.next span.track-page"
                )
                if next_button:
                    await next_button.click()
                    await page.wait_for_timeout(3000)
                else:
                    break
        except Exception as e:
            errors.append(f"Error processing {url}: {e}")
        finally:
            await browser.close()

    # with open("./../output/nafa_dining_data.json", "w") as f:
    #     json.dump(scraped_data, f, indent=4)

    return scraped_data, errors


async def run_scraper(target_url):
    """
    Actual function to call the scraper code and display it to users asynchronously
    """
    details_list, errors = await scrape_nafa_dining(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
