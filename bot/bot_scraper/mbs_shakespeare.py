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


async def scrape_marina_bay_sands_restaurants(base_url):
    """
    Scrapes the Marina Bay Sands website for restaurant details asynchronously
    """
    details_list = []
    errors = []
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        page = await browser.new_page()
        try:
            await page.goto(base_url)
            await page.wait_for_selector(
                "div.restauranttitlefilter__container--carddetails__card"
            )
            print(f"Successfully retrieved page URL: {base_url}")
            restaurant_cards = await page.query_selector_all(
                "div.restauranttitlefilter__container--carddetails__card"
            )
            for card in restaurant_cards:
                name_element = await card.query_selector(
                    "div.restauranttitlefilter__container--carddetails__card--contentarea div.title div.cmp-title a.cmp-title_link h3.cmp-title__text"
                )
                url_element = await card.query_selector(
                    "div.restauranttitlefilter__container--carddetails__card--contentarea div.title div.cmp-title a.cmp-title_link"
                )
                location_element = await card.query_selector(
                    "div.restauranttitlefilter__container--carddetails__card--details div.location-details div.location-details--address div.information"
                )

                name = clean_string(await name_element.inner_text())
                url = f"https://www.marinabaysands.com{await url_element.get_attribute('href')}"
                location = clean_string(await location_element.inner_text())

                details = {
                    "name": name,
                    "location": location,
                    "description": "",
                    "category": "",
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
    Actual function to call the scraper code and display it to users asynchronously
    """
    details_list, errors = await scrape_marina_bay_sands_restaurants(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
