import json
import os
import re
import asyncio
from playwright.async_api import async_playwright


async def delete_file(target_url):
    """
    Helper function that attempts to delete a file at the specified URL asynchronously.
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


async def scrape_smu(base_url):
    """
    Scrapes the specified SMU website for food and beverage details asynchronously
    """
    details_list = []
    errors = []

    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        page = await browser.new_page()

        try:
            await page.goto(base_url)
            await page.wait_for_selector("div.col-md-9")
            print(f"Successfully retrieved page URL: {base_url}")
            locations = await page.query_selector_all("div.col-md-9 div.col-md-9")
            for location in locations:

                name = await location.query_selector("h4.location-title").inner_text()
                location_element = await location.query_selector("div.location-address")
                location_info = (
                    await location_element.inner_text() if location_element else ""
                )
                location_url_info = (
                    await location_element.query_selector("a").get_attribute("href")
                    if location_element and await location_element.query_selector("a")
                    else ""
                )
                description = await location.query_selector(
                    "div.location-description"
                ).inner_text()
                category = "Food and Beverage"
                contact_element = await location.query_selector("div.location-contact")
                contact_info = (
                    await contact_element.inner_text().strip()
                    if contact_element
                    else ""
                )
                hours_element = await location.query_selector("div.location-hours")
                hours_info = (
                    await hours_element.inner_text().strip() if hours_element else ""
                )

                details = {
                    "name": name,
                    "location": clean_string(location_info),
                    "description": f"{clean_string(description)} {clean_string(contact_info)} {clean_string(hours_info)}".strip(),
                    "category": category,
                    "url": location_url_info,
                }

                details_list.append(details)

        except Exception as e:
            errors.append(f"Error processing {base_url}: {e}")

        finally:
            await browser.close()

    return details_list, errors


async def run_scraper(target_url):
    """
    Actual function to call the scraper code asynchronously
    and display it to users
    """
    details_list, errors = await scrape_smu(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
