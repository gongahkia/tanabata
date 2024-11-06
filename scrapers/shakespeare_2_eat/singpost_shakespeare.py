"""
~~~ INTERNAL REFERENCE ~~~

site to scrape: https://www.singpostcentre.com/stores?start_with=&s=&category=cafes-restaurants-food-court

~ HTML DOM STRUCTURE ~

div.card.h-100
    div.card-subtitle.ls-2 --> category
    div.stretched-link --> inner_text() is name, a href is url

a with id loadMore --> click to laod more 
"""

import json
import os
import re
from playwright.sync_api import sync_playwright


def delete_file(target_url):
    """
    helper function that attempts to delete a
    file at the specified URL
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


def scrape_singpost_centre(base_url):
    """
    scrapes the Singpost Centre website for
    cafes and restaurant details
    """
    details_list = []
    errors = []
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        try:
            page.goto(base_url)
            page.wait_for_selector("div.card.h-100")
            print(f"successfully retrieved page URL: {base_url}")
            while True:
                try:
                    load_more_button = page.query_selector("a#loadMore.btn.loadMoreBtn")
                    print("scanning for load more button...")
                    if load_more_button:
                        if load_more_button.get_attribute("style") == "display: none;":
                            print("finished loading all pages")
                            break
                        else:
                            print("load more button found!")
                            load_more_button.click()
                            page.wait_for_timeout(2000)
                except Exception as e:
                    print(f"Error loading more entries: {e}")
                    break
            locations = page.query_selector_all("div.col div.card.h-100")
            for location in locations:
                category = location.query_selector(
                    "div.card-subtitle.ls-2"
                ).inner_text()
                name_element = location.query_selector("a.stretched-link")
                name = name_element.inner_text()
                url = name_element.get_attribute("href")
                details = {
                    "name": clean_string(name),
                    "location": "",
                    "description": "",
                    "category": clean_string(category),
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

TARGET_URL = "https://www.singpostcentre.com/stores?start_with=&s=&category=cafes-restaurants-food-court"
TARGET_FILEPATH = "./../output/singpost_centre_dining_details.json"
details_list, errors = scrape_singpost_centre(TARGET_URL)
if errors:
    print(f"Errors encountered: {errors}")
print("Scraping complete.")
delete_file(TARGET_FILEPATH)
with open(TARGET_FILEPATH, "w") as f:
    json.dump(details_list, f, indent=4)
