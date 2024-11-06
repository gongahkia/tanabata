"""
~~~ INTERNAL REFERENCE ~~~

sites to scrape: https://www.brasbasahcomplex.com/shops/?category_id=46

~ HTML DOM STRUCTURE ~

div.stores-list-store
    a --> href is the url
    div a --> inner_text() is the name
    div p --> inner_text() is the description
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
    Sanitize a provided string by stripping unnecessary characters
    """
    cleaned_string = re.sub(r"\n+", " ", input_string)
    cleaned_string = re.sub(r"<[^>]+>", "", cleaned_string)
    return cleaned_string.strip()


def scrape_bras_basah(base_url):
    """
    Scrapes Bras Basah Complex's shops from the provided base URL
    """
    details_list = []
    errors = []
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        try:
            page.goto(base_url)
            page.wait_for_selector("div.stores-list-store")
            items = page.query_selector_all("div.stores-list-store")
            for item in items:
                url_element = item.query_selector("a")
                name_element = item.query_selector("div a")
                description_element = item.query_selector("div p")
                url = url_element.get_attribute("href") if url_element else ""
                name = clean_string(name_element.inner_text()) if name_element else ""
                description = (
                    clean_string(description_element.inner_text())
                    if description_element
                    else ""
                )
                details = {
                    "name": name,
                    "location": "",
                    "description": description,
                    "category": "Food and Dining",
                    "url": f"https://www.brasbasahcomplex.com{url}",
                }
                print(details)
                details_list.append(details)
        except Exception as e:
            errors.append(f"Error processing {base_url}: {e}")
        finally:
            browser.close()
    return details_list, errors


# ----- Execution Code -----

TARGET_URL = "https://www.brasbasahcomplex.com/shops/?category_id=46"
TARGET_FILEPATH = "./../output/bras_basah_complex_shops.json"
details_list, errors = scrape_bras_basah(TARGET_URL)
if errors:
    print(f"Errors encountered: {errors}")
print("Scraping complete.")
delete_file(TARGET_FILEPATH)
with open(TARGET_FILEPATH, "w") as f:
    json.dump(details_list, f, indent=4)
