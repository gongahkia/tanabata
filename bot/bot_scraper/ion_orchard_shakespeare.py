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
    Sanitizes a provided string
    """
    cleaned_string = re.sub(r"\n+", " ", input_string)
    cleaned_string = re.sub(r"<[^>]+>", "", cleaned_string)
    return cleaned_string.strip()


async def scrape_ion_orchard(base_urls):
    """
    Scrapes the specified Ion Orchard websites for the food vendor's name,
    location, description, category, and URL asynchronously
    """
    details_list = []
    errors = []

    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        page = await browser.new_page()

        for base_url in base_urls:
            try:
                print(f"trying to scrape url: {base_url}")
                await page.goto(base_url)
                await page.wait_for_selector(
                    "div.cmp-dynamic-list-dine-shop-item-content-info"
                )
                print("page 1 found")

                while True:
                    listings = await page.query_selector_all(
                        "div.cmp-dynamic-list-dine-shop-item-content-info"
                    )
                    if not listings:
                        errors.append("No more listings found.")
                        break

                    for listing in listings:
                        name = (
                            (
                                await listing.query_selector(
                                    "span.cmp-dynamic-list-dine-shop-item-content-item-title"
                                )
                            )
                            .inner_text()
                            .strip()
                        )
                        location = (
                            (
                                await listing.query_selector(
                                    "span.cmp-dynamic-list-dine-shop-item-content-item-num"
                                )
                            )
                            .inner_text()
                            .strip()
                        )
                        description = ""
                        category = "Casual Dining and Takeaways, Restaurants and Cafes"
                        vendor_url = base_url
                        details = {
                            "name": name,
                            "location": location,
                            "description": description,
                            "category": category,
                            "url": vendor_url,
                        }
                        details_list.append(details)

                    # Handle pagination asynchronously
                    next_page_button = await page.query_selector(
                        "div.cmp-dynamic-list-pagination-container span.cmp-dynamic-list-paginate-item.active + span.cmp-dynamic-list-paginate-item"
                    )
                    if (
                        next_page_button
                        and (await next_page_button.inner_text()).strip() != ""
                    ):
                        print(f"page {await next_page_button.inner_text()} found")
                        await next_page_button.click()
                        await page.wait_for_selector(
                            "div.cmp-dynamic-list-dine-shop-item-content-info"
                        )
                    else:
                        print("no more numbered pages found")
                        break

            except Exception as e:
                errors.append(f"Error processing {base_url}: {e}")
                break

        await browser.close()

    return details_list, errors


async def run_scraper(target_url_array):
    """
    Actual function to call the scraper code and display it to users asynchronously
    """
    details_list, errors = await scrape_ion_orchard(target_url_array)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
