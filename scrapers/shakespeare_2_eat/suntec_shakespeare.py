"""
~~~ INTERNAL REFERENCE ~~~

sites to scrape: https://www.sunteccity.com.sg/store_categories/dining

~ HTML DOM STRUCTURE ~

div.explore-list ul.explores-clearfix --> for list element in that
    li div.explore-item.clearfix --> if click(), can extract the target url from the div is url
    li div.explore-item.clearfix div.info div.name a --> inner_text() is name
    li div.explore-item.clearfix div.info div.address span a.address --> inner_text() is location
"""

import json
import os
import re
from playwright.sync_api import sync_playwright


def delete_file(target_url):
    """
    helper function that attempts to delete a file at the specified URL
    """
    try:
        os.remove(target_url)
        print(f"Deleted file at filepath: {target_url}")
    except OSError as e:
        print(f"Error deleting file at filepath: {target_url} due to {e}")


def clean_string(input_string):
    """
    sanitize a provided string
    """
    cleaned_string = re.sub(r"\n+", " ", input_string)
    cleaned_string = re.sub(r"<[^>]+>", "", cleaned_string)
    return cleaned_string.strip()


def scrape_suntec_city_dining(base_url):
    """
    scrapes the Suntec City website for dining details
    """
    details_list = []
    errors = []
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        try:
            page.goto(base_url)
            page.wait_for_selector("div.explore-list ul.explores.clearfix")
            print(f"successfully retrieved page URL: {base_url}")
            dining_categories = page.query_selector_all(
                "div.explore-list ul.explores.clearfix li div.explore-item.clearfix"
            )
            for category in dining_categories:
                name_element = category.query_selector("div.info div.name a")
                location_element = category.query_selector(
                    "div.info div.address span a.address"
                )
                name = clean_string(name_element.inner_text())
                location = clean_string(location_element.inner_text())

                """
                FUA
                
                if able to implement more advanced 
                click-through scraping in the future
                then implement those as alternatives 
                to what i currently have here
                """

                # print("entering vendor to see url...")
                # category.click()
                # page.wait_for_load_state('networkidle')
                # page.wait_for_timeout(1000)
                # print(page.url)
                # url = page.url
                # page.go_back()
                # page.wait_for_timeout(1000)
                # page.wait_for_load_state('networkidle')
                # print("exiting vendor...")

                details = {
                    "name": name,
                    "location": location,
                    "description": "",
                    "category": "Dining",
                    "url": base_url,
                }
                print(details)
                details_list.append(details)
        except Exception as e:
            errors.append(f"Error processing {base_url}: {e}")
        finally:
            browser.close()
    return details_list, errors


# ----- Execution Code -----

TARGET_URL = "https://www.sunteccity.com.sg/store_categories/dining"
TARGET_FILEPATH = "./../output/suntec_city_dining_details.json"
details_list, errors = scrape_suntec_city_dining(TARGET_URL)
if errors:
    print(f"Errors encountered: {errors}")
print("Scraping complete.")
delete_file(TARGET_FILEPATH)
with open(TARGET_FILEPATH, "w") as f:
    json.dump(details_list, f, indent=4)
