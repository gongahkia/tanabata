"""
~~~ INTERNAL REFERENCE ~~~

sites to scrape: https://www.citysquaremall.com.sg/shops/food-beverage/

~ HTML DOM STRUCTURE ~

div.cdl-item
   div.wrap-img a --> href is the url
    div.wrap-body-detail div.business_address h2 a --> inner_text() is the name
    div.wrap-body-detail div.business_address --> inner_text() is the location
    div.wrap-body-detail div.business_phone_number --> inner_text() is the description
"""

import json
import os
import re
from playwright.sync_api import sync_playwright

def delete_file(target_url):
    """
    Helper function that attempts to delete a file at the specified URL
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
    cleaned_string = re.sub(r'\n+', ' ', input_string)
    cleaned_string = re.sub(r'<[^>]+>', '', cleaned_string)
    return cleaned_string.strip()

def scrape_city_square_mall(base_url):
    """
    Scrapes the City Square Mall website for food and beverage details
    """
    details_list = []
    errors = []
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        try:
            page.goto(base_url)
            page.wait_for_selector('div.cdl-item')
            print(f"successfully retrieved page URL: {base_url}")
            shop_items = page.query_selector_all('div.cdl-item')
            for shop in shop_items:
                name_element = shop.query_selector('div.wrap-body-detail div.business_address h2 a')
                location_element = shop.query_selector('div.wrap-body-detail div.business_address')
                description_element = shop.query_selector('div.wrap-body-detail div.business_phone_number')
                url_element = shop.query_selector('div.wrap-img a')
                name = clean_string(name_element.inner_text()) if name_element else None
                location = clean_string(location_element.inner_text()) if location_element else None
                description = clean_string(description_element.inner_text()) if description_element else None
                url = url_element.get_attribute('href') if url_element else None
                details = {
                    'name': name,
                    'location': location,
                    'description': description,
                    'category': "Food & Beverage",
                    'url': url
                }
                print(details)
                details_list.append(details)

        except Exception as e:
            errors.append(f"Error processing {base_url}: {e}")

        finally:
            browser.close()

    return details_list, errors

# ----- Execution Code -----

TARGET_URL = "https://www.citysquaremall.com.sg/shops/food-beverage/"
TARGET_FILEPATH = "./../output/city_square_mall_dining_details.json"
details_list, errors = scrape_city_square_mall(TARGET_URL)
if errors:
    print(f"Errors encountered: {errors}")
print("Scraping complete.")
delete_file(TARGET_FILEPATH)
with open(TARGET_FILEPATH, 'w') as f:
    json.dump(details_list, f, indent=4)