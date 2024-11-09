import json
from playwright.async_api import async_playwright
import asyncio


async def fetch_capitaland_store_details(detail_link, browser):
    page = await browser.new_page()
    print(f"Fetching details from: {detail_link}")
    await page.goto(detail_link)
    await page.wait_for_selector("div.l-padding")

    name = await page.query_selector("div.cm-details-section-head h1")
    name = await name.inner_text() if name else ""

    location = await page.query_selector(
        "section.cm.cm-property-details dd.icon-marker.icon-rounded"
    )
    location = await location.inner_text() if location else ""

    description = await page.query_selector("div.cm-details-section-content div.rte")
    description = await description.inner_text() if description else ""

    category = await page.query_selector("div.cm-details-section-content div.cm")
    category = await category.inner_text() if category else ""

    details = {
        "name": name.strip(),
        "location": location.strip(),
        "description": description.strip(),
        "category": category.strip(),
        "url": detail_link,
    }
    await page.close()
    return details


async def fetch_capitaland_data(start_urls):
    malls = {}
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        for url in start_urls:
            detail_array = []
            print(f"Extracting from: {url}")
            page = await browser.new_page()
            await page.goto(url)
            await page.wait_for_selector(
                "div.listing-container ul.listing-items article.listing-item.listing-tenants a"
            )
            if await page.title() == "":
                print(f"Failed to retrieve page from URL: {url}")
                await page.close()
                continue
            listings = await page.query_selector_all(
                "div.listing-container ul.listing-items article.listing-item.listing-tenants a"
            )
            tasks = []  # List to hold tasks for async fetching details
            for listing in listings:
                detail_url = await listing.get_attribute("href")
                tasks.append(
                    fetch_capitaland_store_details(detail_url, browser)
                )  # Add to task list
            detail_array = await asyncio.gather(
                *tasks
            )  # Wait for all tasks to complete
            sanitized_url = url.split("/")[-3]
            malls[sanitized_url] = detail_array
            await page.close()
        await browser.close()
    return malls


async def run_scraper(target_url):
    """
    Actual function to call the scraper code and display it to users
    """
    details_list = await fetch_capitaland_data([target_url])  # Run async directly
    print("Scraping complete.")
    return details_list
