"""
~~~ INTERNAL REFERENCE ~~~

sites to scrape: https://theseletarmall.com.sg/dine/

~ HTML DOM STRUCTURE ~

div.shopboxwrap
    div.shopbox a --> href is the url
    div.shopbox div.shopsummarybox div.shoplevelunitbox --> inner_text is the location
    div.shopbox div.shopsummarybox div.shopcategory --> inner_text is the category
    div.shopbox div.shopsummarybox div.shoptitle --> inner_text is the name
    div.shopbox div.shopsummarybox
    
div.wp-pagenavi
    a.nextpostslink --> click to go to next page
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


def scrape_seletar_mall(base_url):
    """
    Scrapes The Seletar Mall website for dining details and navigates through pages by clicking the "Next" button
    """
    details_list = []
    errors = []
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        try:
            page.goto(base_url)
            page.wait_for_selector("div.shopboxwrap")
            while True:
                items = page.query_selector_all("div.shopboxwrap div.shopbox")
                for item in items:
                    url_element = item.query_selector("a")
                    location_element = item.query_selector(
                        "div.shopsummarybox div.shoplevelunitbox"
                    )
                    category_element = item.query_selector(
                        "div.shopsummarybox div.shopcategory"
                    )
                    name_element = item.query_selector(
                        "div.shopsummarybox div.shoptitle"
                    )
                    url = url_element.get_attribute("href") if url_element else ""
                    location = (
                        clean_string(location_element.inner_text())
                        if location_element
                        else ""
                    )
                    category = (
                        clean_string(category_element.inner_text())
                        if category_element
                        else ""
                    )
                    name = (
                        clean_string(name_element.inner_text()) if name_element else ""
                    )
                    if name == "" and location == "" and category == "":
                        continue
                    details = {
                        "name": name,
                        "location": location,
                        "description": "",
                        "category": category,
                        "url": url,
                    }
                    print(details)
                    details_list.append(details)
                next_button = page.query_selector("div.wp-pagenavi a.nextpostslink")
                if next_button:
                    next_button.click()
                    page.wait_for_timeout(2000)
                    print("Navigating to next page...")
                else:
                    print("No more pages to navigate.")
                    break
        except Exception as e:
            errors.append(f"Error processing {base_url}: {e}")
        finally:
            browser.close()
    return details_list, errors


# ----- Execution Code -----

TARGET_URL = "https://theseletarmall.com.sg/dine/"
TARGET_FILEPATH = "./../output/seletar_mall_dining_details.json"
details_list, errors = scrape_seletar_mall(TARGET_URL)
if errors:
    print(f"Errors encountered: {errors}")
print("Scraping complete.")
delete_file(TARGET_FILEPATH)
with open(TARGET_FILEPATH, "w") as f:
    json.dump(details_list, f, indent=4)
