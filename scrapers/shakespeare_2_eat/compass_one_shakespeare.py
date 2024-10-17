"""
~~~ INTERNAL REFERENCE ~~~

sites to scrape: https://compassone.sg/category/stores/restaurant-cafe-fast-food/

~ HTML DOM STRUCTURE ~

div.w-hwrapper.usg_hwrapper_1.align_left.valign_top.wrap
    h2.w-post-elm.post_title.usg_post_title_1.entry-title.cmp-title a --> inner_text is name and href is url
    div.w-post-elm.post_content.usg_post_content_1 p --> inner_text is location

div.nav-links 
    span.page-numbers.current --> current page
    a.page-numbers --> available page numbers
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

def scrape_compass_one(base_url):
    """
    Scrapes the Compass One website for restaurant and cafe details
    """
    details_list = []
    errors = []
    travelled_pages = []
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        try:
            page.goto(base_url)
            page.wait_for_selector('div.w-hwrapper.usg_hwrapper_1.align_left.valign_top.wrap')
            print(f"successfully retrieved page URL: {base_url}")
            while True:
                store_items = page.query_selector_all('div.w-hwrapper.usg_hwrapper_1.align_left.valign_top.wrap')
                for store in store_items:
                    name_element = store.query_selector('h2.w-post-elm.post_title.usg_post_title_1.entry-title.cmp-title a')
                    location_element = store.query_selector('div.w-post-elm.post_content.usg_post_content_1 p')
                    name = clean_string(name_element.inner_text()) if name_element else None
                    location = clean_string(location_element.inner_text()) if location_element else None
                    url = name_element.get_attribute('href') if name_element else None
                    details = {
                        'name': name,
                        'location': location,
                        'category': "Restaurant, Cafe & Fast Food",
                        'description': "", 
                        'url': url
                    }
                    print(details)
                    details_list.append(details)
                current_page = page.query_selector('div.nav-links span.page-numbers.current')
                travelled_pages.append(current_page.inner_text().strip()) 
                print(f"page numbers visited: {travelled_pages}")
                next_page_array = [page_el for page_el in page.query_selector_all('div.nav-links a.page-numbers') if page_el.inner_text().isnumeric() and page_el.inner_text().strip() not in travelled_pages]
                if next_page_array:
                    next_page = next_page_array[0]
                    print(f"page numbers remaining: {[el.inner_text() for el in next_page_array]}")
                if len(next_page_array) == 0:
                    print("No more pages to navigate.")
                    break
                print(f"current page: {current_page.inner_text()}, navigating to next page {next_page.inner_text()}...")
                next_page.click()
                page.wait_for_timeout(2000)  
        except Exception as e:
            errors.append(f"Error processing {base_url}: {e}")
        finally:
            browser.close()
    return details_list, errors

# ----- Execution Code -----

TARGET_URL = "https://compassone.sg/category/stores/restaurant-cafe-fast-food/"
TARGET_FILEPATH = "./../output/compass_one_dining_details.json"
details_list, errors = scrape_compass_one(TARGET_URL)
if errors:
    print(f"Errors encountered: {errors}")
print("Scraping complete.")
delete_file(TARGET_FILEPATH)
with open(TARGET_FILEPATH, 'w') as f:
    json.dump(details_list, f, indent=4)
