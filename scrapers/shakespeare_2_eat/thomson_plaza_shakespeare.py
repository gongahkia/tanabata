"""
~~~ INTERNAL REFERENCE ~~~

sites to scrape: https://www.thomsonplaza.com.sg/store-directory/?keyword=&filter=5&payment_type=

~ HTML DOM STRUCTURE ~

div.l-more a.button directory-link-btn --> keep .click until no more

li with style="display: list-item;"
    a --> href is url
    div.directoryInfoBox div.directoryInfoBoxInner div.directoryContent div.directoryName --> inner_text is name
    div.directoryInfoBox div.directoryInfoBoxInner div.directoryContent div.directoryCat --> inner_text is category
    div.directoryInfoBox div.directoryInfoBoxInner div.directoryContent div.store-location --> inner_text is location
    div.directoryInfoBox div.directoryInfoBoxInner div.directoryContent div.store-tel --> inner_text is description
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
    cleaned_string = re.sub(r'\n+', ' ', input_string)
    cleaned_string = re.sub(r'<[^>]+>', '', cleaned_string)
    return cleaned_string.strip()

def scrape_thomson_plaza(base_url):
    """
    Scrapes Thomson Plaza website for directory details
    """
    details_list = []
    errors = []
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        try:
            page.goto(base_url)
            page.wait_for_selector('div.l-more a.button.directory-link-btn')
            while True:
                current_item_count = len(page.query_selector_all('li[style="display: list-item;"]'))
                try:
                    load_more_button = page.query_selector('div.l-more a.button.directory-link-btn')
                    if load_more_button:
                        print("clicking load button")
                        load_more_button.click()
                        page.wait_for_timeout(1000) 
                        new_item_count = len(page.query_selector_all('li[style="display: list-item;"]'))
                        if new_item_count == current_item_count:
                            print("No more items to load.")
                            break
                    else:
                        break
                except Exception as e:
                    print(f"No more load more button to click: {e}")
                    break
            list_items = page.query_selector_all('li[style="display: list-item;"]')
            for item in list_items:
                name_element = item.query_selector('div.directoryInfoBoxInner div.directoryContent div.directoryName')
                category_element = item.query_selector('div.directoryInfoBoxInner div.directoryContent div.directoryCat')
                location_element = item.query_selector('div.directoryInfoBoxInner div.directoryContent div.store-location')
                description_element = item.query_selector('div.directoryInfoBoxInner div.directoryContent div.store-tel')
                url_element = item.query_selector('a')
                name = clean_string(name_element.inner_text()) if name_element else ""
                category = clean_string(category_element.inner_text()) if category_element else ""
                location = clean_string(location_element.inner_text()) if location_element else ""
                description = clean_string(description_element.inner_text()) if description_element else ""
                url = url_element.get_attribute('href') if url_element else ""
                details = {
                    'name': name,
                    'category': category,
                    'location': location,
                    'description': description,
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

TARGET_URL = "https://www.thomsonplaza.com.sg/store-directory/?keyword=&filter=5&payment_type="
TARGET_FILEPATH = "./../output/thomson_plaza_details.json"
details_list, errors = scrape_thomson_plaza(TARGET_URL)
if errors:
    print(f"Errors encountered: {errors}")
print("Scraping complete.")
delete_file(TARGET_FILEPATH)
with open(TARGET_FILEPATH, 'w') as f:
    json.dump(details_list, f, indent=4)