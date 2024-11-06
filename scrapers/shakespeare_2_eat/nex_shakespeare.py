"""
~~~ INTERNAL REFERENCE ~~~

sites to scrape: https://www.nex.com.sg/Directory/Category?EncDetail=qbjHWjcKv2GJGewRmzGQOA_3d_3d&CategoryName=Restaurant_Cafe%20_%20Fast%20Food&voucher=false&rewards=false

~ HTML DOM STRUCTURE ~

div.storeLogo
    div.logoImg a --> href is the url
    div.logoTitle a --> inner_text() is the name

div.levelCategoryPageNum.pagination.simple-pagination
    ul li a.page-link.next --> click() to go to the next page
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


def scrape_nex(base_url):
    """
    Scrapes NEX Mall's restaurants and cafes from the provided base URL
    Handles pagination by clicking the "Next" button until no more pages are available
    """
    details_list = []
    errors = []
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        try:
            page.goto(base_url)
            page.wait_for_selector("div.storeLogo")
            while True:
                items = page.query_selector_all("div.storeLogo")
                for item in items:
                    url_element = item.query_selector("div.logoImg a")
                    name_element = item.query_selector("div.logoTitle a")
                    url = url_element.get_attribute("href") if url_element else ""
                    raw_name = (
                        clean_string(name_element.inner_text()) if name_element else ""
                    )
                    location = [el for el in raw_name.split() if el.startswith("#")][0]
                    clean_name = raw_name.replace(location, "")
                    details = {
                        "name": clean_name.strip(),
                        "location": location,
                        "description": "",
                        "category": "Restaurant, Cafe & Fast Food",
                        "url": url,
                    }
                    print(details)
                    details_list.append(details)
                next_button = (
                    page.query_selector("ul li a.page-link.next")
                    if page.query_selector("ul li a.page-link.next")
                    else None
                )
                print(next_button)
                if next_button:
                    print("navigating to the next page...")
                    next_button.click()
                    page.wait_for_timeout(2000)
                else:
                    print("no more pages to navigate to")
                    break
        except Exception as e:
            errors.append(f"Error processing {base_url}: {e}")
        finally:
            browser.close()
    return details_list, errors


# ----- Execution Code -----

TARGET_URL = "https://www.nex.com.sg/Directory/Category?EncDetail=qbjHWjcKv2GJGewRmzGQOA_3d_3d&CategoryName=Restaurant_Cafe%20_%20Fast%20Food&voucher=false&rewards=false"
TARGET_FILEPATH = "./../output/nex_dining_details.json"
details_list, errors = scrape_nex(TARGET_URL)
if errors:
    print(f"Errors encountered: {errors}")
print("Scraping complete.")
delete_file(TARGET_FILEPATH)
with open(TARGET_FILEPATH, "w") as f:
    json.dump(details_list, f, indent=4)
