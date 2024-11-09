import json
import re
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


async def parse_details(name, details, url):
    """
    Parses and sorts each field from the given detail string
    and sorts it into the provided JSON categories for easier API sorting
    """
    fin = {
        "name": name,
        "location": "",
        "description": "",
        "category": "Food and Beverage",
        "url": url,
    }
    for line in details.split("\n"):
        if line.startswith("Location: "):
            fin["location"] = line.lstrip("Location: ")
        else:
            fin["description"] += f"{line.strip()}\n"
    return fin


async def fetch_nus_dining_data(url):
    """
    Fetches dining details from the given NUS page using Playwright asynchronously
    """
    details_list = []
    errors = []

    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        page = await browser.new_page()
        await page.goto(url)
        await page.wait_for_selector(
            "div.vc_col-sm-12.vc_gitem-col.vc_gitem-col-align-"
        )

        if await page.title() == "":
            errors.append(f"Failed to retrieve page {url}")
            await browser.close()
            return details_list, errors
        else:
            print(f"Scraping details from {url}")

        listings = await page.query_selector_all(
            "div.vc_col-sm-12.vc_gitem-col.vc_gitem-col-align-"
        )

        for listing in listings:
            name = await listing.query_selector(
                'h3[style="text-align: left"]'
            ).inner_text()
            details = await listing.query_selector(
                'p[style="text-align: left"] + p'
            ).inner_text()  # + p goes one element down
            fin_details = await parse_details(name, details, url)

            details_list.append(fin_details)

        await browser.close()

    return details_list, errors


async def run_scraper(target_url_array):
    """
    Actual function to call the scraper code asynchronously and display it to users
    """
    all_details = {}
    all_locations = {}

    for url in target_url_array:
        memo = await fetch_nus_dining_data(url)
        if memo[1]:
            errors = memo[1]
            print(f"Errors encountered: {errors}")
            return errors
        else:
            all_locations[url] = memo[0]
    all_details["nus"] = all_locations
    print("Scraping complete.")
    return all_details
