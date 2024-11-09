"""
~~~ INTERNAL REFERENCE ~~~

sites to scrape: https://citylink.com.sg/restaurants-cafes/

~ HTML DOM STRUCTURE ~

a.user-directory-item --> href is url
    div.user-directory-name --> inner_text is name
    span.user-directory-unit-number --> inner_text is location
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


def scrape_citylink_mall(base_url):
    """
    Scrapes the CityLink Mall website for restaurant and cafe details
    """
    details_list = []
    errors = []
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        try:
            page.goto(base_url)
            page.wait_for_selector("a.user-directory-item")
            items = page.query_selector_all("a.user-directory-item")
            for item in items:
                url_element = item
                name_element = item.query_selector("div.user-directory-name")
                location_element = item.query_selector(
                    "span.user-directory-unit-number"
                )
                url = url_element.get_attribute("href") if url_element else ""
                name = clean_string(name_element.inner_text()) if name_element else ""
                location = (
                    clean_string(location_element.inner_text())
                    if location_element
                    else ""
                )
                details = {
                    "name": name.rstrip(location).strip(),
                    "location": location,
                    "description": "",
                    "category": "Restaurants & Cafes",
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

# TARGET_URL = "https://citylink.com.sg/restaurants-cafes/"
# TARGET_FILEPATH = "./../output/citylink_mall_dining_details.json"
# details_list, errors = scrape_citylink_mall(TARGET_URL)
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
    details_list, errors = scrape_citylink_mall(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
