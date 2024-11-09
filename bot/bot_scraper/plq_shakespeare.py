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
    Sanitize a provided string by removing extra whitespace and line breaks.
    """
    cleaned_string = re.sub(r"\n+", " ", input_string)
    cleaned_string = re.sub(r"<[^>]+>", "", cleaned_string)
    return cleaned_string.strip()


async def scrape_paya_lebar_quarter(base_url):
    """
    Scrapes the Payar Lebar Quarter directory website for food and restaurant details asynchronously.
    """
    details_list = []
    errors = []

    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        page = await browser.new_page()

        try:
            await page.goto(base_url)
            await page.wait_for_selector(
                "div.directory-card.relative.grid.gap-5.border.px-4.py-5.bg-brand-page"
            )
            print(f"successfully retrieved page URL: {base_url}")

            while True:
                try:
                    load_more_button = await page.query_selector(
                        "button.button.button--secondary"
                    )
                    if load_more_button:
                        print("load button found")
                        await load_more_button.click()
                        await page.wait_for_timeout(1000)
                    else:
                        print("load button no longer found, loading finished")
                        break
                except Exception as e:
                    print(f"No more pages to load: {e}")
                    break

            directory_cards = await page.query_selector_all(
                "div.directory-card.relative.grid.gap-5.border.px-4.py-5.bg-brand-page"
            )
            for card in directory_cards:
                name_element = await card.query_selector(
                    "div.relative.flex.flex-col.auto-rows-max.text-brand-body div a"
                )
                name = await name_element.inner_text() if name_element else ""
                url = await name_element.get_attribute("href") if name_element else ""
                description = ""
                location = ""
                span_elements = await card.query_selector_all("div span")
                print([await element.inner_text() for element in span_elements])
                for span in span_elements:
                    span_text = (await span.inner_text()).strip()
                    if span_text.startswith("#"):
                        location = span_text
                    else:
                        if (
                            span_text == "Food & Restaurant"
                            or span_text == name.strip()
                        ):
                            pass
                        else:
                            description += f"{span_text} "
                description = description.strip()
                details = {
                    "name": clean_string(name),
                    "location": clean_string(location),
                    "description": clean_string(description),
                    "category": "Food & Restaurant",
                    "url": f"https://www.payalebarquarter.com{url}",
                }
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
    details_list, errors = await scrape_paya_lebar_quarter(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
