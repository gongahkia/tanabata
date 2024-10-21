"""
~~~ INTERNAL REFERENCE ~~~

sites to scrape: https://shop.greatworld.com.sg/dine/

~ HTML DOM STRUCTURE ~

div.shopboxwrap
    div.shopbox a --> href is url
   div.shopunit --> inner_text() is location
   div.shopcategory --> inner_text() is category
   div.shoptitle --> inner_text() is name

div.paginationcontainer div.wp-pagenavi
    a.page --> clickable page
    span.current --> current page
"""

import json
import os
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
    return input_string.strip()

def scrape_great_world_dining(base_url):
    """
    Scrapes the Great World shopping website for dining details and handles pagination
    """
    details_list = []
    errors = []

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()

        try:
            page.goto(base_url)
            page.wait_for_selector('div.shopboxwrap')
            print(f"successfully retrieved page URL: {base_url}")

            while True:
                dining_stores = page.query_selector_all('div.shopboxwrap')
                for store in dining_stores:
                    name_element = store.query_selector('div.shoptitle')
                    location_element = store.query_selector('div.shopunit')
                    category_element = store.query_selector('div.shopcategory')
                    url_element = store.query_selector('div.shopbox a')
                    name = clean_string(name_element.inner_text()) if name_element else ''
                    location = clean_string(location_element.inner_text()) if location_element else ''
                    category = clean_string(category_element.inner_text()) if category_element else ''
                    url = url_element.get_attribute('href') if url_element else ''
                    details = {
                        'name': name,
                        'location': location,
                        'description': "",
                        'category': category,
                        'url': url
                    }
                    print(details)
                    details_list.append(details)
                pagination_container = page.query_selector('div.paginationcontainer div.wp-pagenavi')
                current_page = pagination_container.query_selector('span.current').inner_text() if pagination_container else None
                final_breakpoint = pagination_container.query_selector('a.page.larger') if pagination_container.query_selector('a.page.larger') else None
                print(f"current page is {current_page}")
                if final_breakpoint == None:
                    print("no more pages to scrape.")
                    break  
                else:
                    next_page = pagination_container.query_selector('a.page.larger') if pagination_container else None
                    print(f"navigating to next page {next_page.inner_text()}")
                    next_page.click()  
                    page.wait_for_timeout(2000)  

        except Exception as e:
            errors.append(f"Error processing {base_url}: {e}")
        
        finally:
            browser.close()

    return details_list, errors

# ----- Execution Code -----

TARGET_URL = "https://shop.greatworld.com.sg/dine/"
TARGET_FILEPATH = "./../output/great_world_dining_details.json"
details_list, errors = scrape_great_world_dining(TARGET_URL)
if errors:
    print(f"Errors encountered: {errors}")
print("Scraping complete.")
delete_file(TARGET_FILEPATH)
with open(TARGET_FILEPATH, 'w') as f:
    json.dump(details_list, f, indent=4)