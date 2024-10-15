"""
~~~ INTERNAL REFERENCE ~~~

sites to scrape: https://www.jewelchangiairport.com/en/dine.html

~ DOM STRUCTURE ~

a.load-more --> click() until no longer exists 

div.list-item.col-lg-4.col-md-4.col-sm-4.col-xs-12
    h3.company-name --> queryselector() name
    div.intro.shadow p.desc --> queryselector() location
    div.intro.shadow p.open-times --> queryselector() desc
    div.intro.shadow p.tags span.tag --> queryselectorall() category
"""

import json
import os
import re
from playwright.sync_api import sync_playwright

def delete_file(target_url):
    """
    helper function that attempts 
    to delete a file at the specified 
    URL
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
    cleaned_string = re.sub(r'\n+', ' ', input_string)
    cleaned_string = re.sub(r'<[^>]+>', '', cleaned_string)
    return cleaned_string.strip()

def scrape_jewel_changi(base_url):
    """
    scrapes the specified Jewel Changi Airport website 
    for food and beverage details
    """
    details_list = []
    errors = []
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        try:
            page.goto(base_url)
            page.wait_for_selector("div.list-item.col-lg-4.col-md-4.col-sm-4.col-xs-12")
            print(f"successfully retrieved page URL: {base_url}")
            locations = page.query_selector_all('div.list-item.col-lg-4.col-md-4.col-sm-4.col-xs-12')
            for location in locations:
                name = location.query_selector('h3.company-name').inner_text()
                location_info = location.query_selector('div.intro.shadow p.desc').inner_text()
                description = location.query_selector('div.intro.shadow p.open-times').inner_text()
                categories = [category.inner_text().strip() for category in location.query_selector_all('div.intro.shadow p.tags span.tag')]
                details = {
                    'name': name,
                    'location': clean_string(location_info),
                    'description': clean_string(description),
                    'category': categories,
                    'url': base_url
                }
                print(details)
                details_list.append(details)
        except Exception as e:
            errors.append(f"error processing {base_url}: {e}")
        finally:
            browser.close()
    return details_list, errors

# ----- Execution Code -----

TARGET_URL = "https://www.jewelchangiairport.com/en/dine.html"
TARGET_FILEPATH = "./../output/jewel_dining_details.json"
details_list, errors = scrape_jewel_changi(TARGET_URL)
if errors:
    print(f"Errors encountered: {errors}")
print("Scraping complete.")
delete_file(TARGET_FILEPATH)
with open(TARGET_FILEPATH, 'w') as f:
    json.dump(details_list, f, indent=4)