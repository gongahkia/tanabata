"""
~~~ INTERNAL REFERENCE ~~~

site to scrape: https://www.paragon.com.sg/stores/category/food-beverage

~ HTML DOM STRUCTURE ~


div.mix.category-1.all.store-link a --> href() is the url
    div.text-overlay div.text-cell h5 --> inner_text() is name
""" 
import json
import os
import re
from playwright.sync_api import sync_playwright

def delete_file(target_url):
    """
    Helper function that attempts to delete a file at the specified URL.
    """
    try:
        os.remove(target_url)
        print(f"Deleted file at filepath: {target_url}")
    except OSError as e:
        print(f"Error deleting file at filepath: {target_url} due to {e}")

def clean_string(input_string):
    """
    Sanitize a provided string.
    """
    cleaned_string = re.sub(r'\n+', ' ', input_string)
    cleaned_string = re.sub(r'<[^>]+>', '', cleaned_string)
    return cleaned_string.strip()

def scrape_paragon_food_beverage(base_url):
    """
    Scrapes the Paragon Mall website for food and beverage store details.
    """
    details_list = []
    errors = []
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        try:
            page.goto(base_url)
            page.wait_for_selector('div.mix.category-1.all.store-link')
            print(f"successfully retrieved page URL: {base_url}")
            stores = page.query_selector_all('div.mix.category-1.all.store-link a')
            for store in stores:
                name_element = store.query_selector('div.text-overlay div.text-cell h5')
                name = clean_string(name_element.inner_text()) if name_element else "N/A"
                url = store.get_attribute('href') if store else "N/A"
                details = {
                    'name': name,
                    'location': "Paragon Mall", 
                    'description': "",
                    'category': "Food & Beverage",
                    'url': f"https://www.paragon.com.sg{url}"
                }
                print(details)
                details_list.append(details)
        except Exception as e:
            errors.append(f"Error processing {base_url}: {e}")
        finally:
            browser.close()
    return details_list, errors

# ----- Execution Code -----

TARGET_URL = "https://www.paragon.com.sg/stores/category/food-beverage"
TARGET_FILEPATH = "./../output/paragon_food_beverage_details.json"
details_list, errors = scrape_paragon_food_beverage(TARGET_URL)
if errors:
    print(f"Errors encountered: {errors}")
print("Scraping complete.")
delete_file(TARGET_FILEPATH)
with open(TARGET_FILEPATH, 'w') as f:
    json.dump(details_list, f, indent=4)