"""
~~~ INTERNAL REFERENCE ~~~

sites to scrape: https://www.parkwayparade.com.sg/store-directory/?categories=Food+%26+Restaurant

~ HTML DOM STRUCTURE ~

button.button.null --> just keep clicking until its no longer there

div.relative.w-full.my-0.m-auto.overflow-hidden.max-w-store-logo-sm a.block.aspect-w-1.aspect-h-1 --> href is the url
    div.relative.flex.flex-col.auto-rows-max.gap-y-2.5.text-brand-body a.inline-flex.items-center.font-bold.border-brand-link.mb-0.border-b-2 span --> inner_text() is the name
    div.flex --> every div.flex is description, but if it starts with a '#' then its location
"""

import json
import os
import re
from playwright.sync_api import sync_playwright


def delete_file(target_url):
    """
    Helper function to delete a file at the specified URL
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


def scrape_parkway_parade(base_url):
    """
    Scrapes the Parkway Parade website for store details
    """
    details_list = []
    errors = []
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        try:
            page.goto(base_url)
            page.wait_for_selector(
                "div.directory-card.relative.grid.gap-5.border.px-4.py-5.bg-brand-page"
            )
            while True:
                try:
                    load_more_button = page.query_selector("button.button.null")
                    if load_more_button:
                        load_more_button.click()
                        page.wait_for_timeout(2000)
                        print("Clicked 'Load More' button, loading more stores...")
                    else:
                        print(
                            "'Load More' button not found or no more content to load."
                        )
                        break
                except Exception as e:
                    print(f"Error while clicking 'Load More' button: {e}")
                    break
            items = page.query_selector_all(
                "div.directory-card.relative.grid.gap-5.border.px-4.py-5.bg-brand-page"
            )
            for item in items:
                url_element = item.query_selector(
                    "div.relative.w-full.my-0.m-auto.overflow-hidden.max-w-store-logo-sm a.block.aspect-w-1.aspect-h-1"
                )
                name_element = item.query_selector(
                    "a.inline-flex.items-center.font-bold.border-brand-link.mb-0.border-b-2"
                )
                description_elements = item.query_selector_all("div.flex")
                url = url_element.get_attribute("href") if url_element else ""
                name = clean_string(name_element.inner_text()) if name_element else ""
                location = ""
                description = ""
                for desc in description_elements:
                    if desc.inner_text().strip() != "":
                        el = desc.inner_text()
                        if el.startswith("#"):
                            location = el
                        else:
                            description += el
                details = {
                    "name": name,
                    "location": location,
                    "description": description,
                    "category": "Food & Restaurant",
                    "url": f"https://www.parkwayparade.com.sg{url}",
                }
                print(details)
                details_list.append(details)
        except Exception as e:
            errors.append(f"Error processing {base_url}: {e}")
        finally:
            browser.close()
    return details_list, errors


# ----- Execution Code -----

# TARGET_URL = (
#     "https://www.parkwayparade.com.sg/store-directory/?categories=Food+%26+Restaurant"
# )
# TARGET_FILEPATH = "./../output/parkway_parade_dining_details.json"
# details_list, errors = scrape_parkway_parade(TARGET_URL)
# if errors:
#     print(f"Errors encountered: {errors}")
# print("Scraping complete.")
# delete_file(TARGET_FILEPATH)
# with open(TARGET_FILEPATH, "w") as f:
#     json.dump(details_list, f, indent=4)


def run_scraper(target_url):
    """
    actual function to call the scraper code
    and display it to users
    """
    details_list, errors = scrape_parkway_parade(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
