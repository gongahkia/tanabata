"""
~~~ INTERNAL REFERENCE ~~~

sites to scrape: https://www.kinex.com.sg/dining

~ HTML DOM STRUCTURE ~

div.col-sm-3.col-xs-6
    div.item-list div.item-text.main-color1 span.item-list-title --> inner_text is name
    div.item-list div.item-text.main-color1 span --> location
    a --> href is the url
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


def scrape_kinex_dining(base_url):
    """
    Scrapes the Kinex website for dining details
    """
    details_list = []
    errors = []
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        try:
            page.goto(base_url)
            page.wait_for_selector("div.col-sm-3.col-xs-6")
            items = page.query_selector_all("div.col-sm-3.col-xs-6")
            for item in items:
                name_element = item.query_selector(
                    "div.item-list div.item-text.main-color1 span.item-list-title"
                )
                location_element = item.query_selector(
                    "div.item-list div.item-text.main-color1 span:nth-child(2)"
                )
                url_element = item.query_selector("a")
                name = clean_string(name_element.inner_text()) if name_element else ""
                location = (
                    clean_string(location_element.inner_text())
                    if location_element
                    else ""
                )
                url = url_element.get_attribute("href") if url_element else ""
                details = {
                    "name": name,
                    "location": location,
                    "description": "",
                    "category": "Dining",
                    "url": url,
                }
                print(details)
                details_list.append(details)
        except Exception as e:
            errors.append(f"Error processing {base_url}: {e}")
        finally:
            browser.close()
    return details_list, errors


# ----- Execution Code -----

# TARGET_URL = "https://www.kinex.com.sg/dining"
# TARGET_FILEPATH = "./../output/kinex_dining_details.json"
# details_list, errors = scrape_kinex_dining(TARGET_URL)
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
    details_list, errors = scrape_kinex_dining(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
