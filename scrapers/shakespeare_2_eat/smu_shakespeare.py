import json
import os
import re
from playwright.sync_api import sync_playwright

def delete_file(target_url):
    """
    Helper function that attempts 
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
    Sanitize a provided string
    """
    cleaned_string = re.sub(r'\n+', ' ', input_string)
    cleaned_string = re.sub(r'<[^>]+>', '', cleaned_string)
    return cleaned_string.strip()

def scrape_smu(base_url):
    """
    Scrapes the specified SMU website 
    for food and beverage details
    """
    details_list = []
    errors = []
    
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()

        try:
            page.goto(base_url)
            page.wait_for_selector('div.col-md-9')
            print(f"successfully retrieved page URL: {base_url}")
            locations = page.query_selector_all('div.col-md-9 div.col-md-9')
            for location in locations:

               # print(location.inner_text())

                name = location.query_selector('h4.location-title').inner_text()
                location_place = location.query_selector('div.location-address').inner_text()
                description = location.query_selector('div.location-description').inner_text()
                category = "Food and Beverage"
                contact_element = location.query_selector('div.location-contact')
                contact_info = contact_element.inner_text().strip() if contact_element else ''
                hours_element = location.query_selector('div.location-hours')
                hours_info = hours_element.inner_text().strip() if hours_element else ''
                # print(name, location_place, description, category)
                
                # location_url = location_element.query_selector('a')['href'] if location_element and location_element.query_selector('a') else ''
                
                details = {
                    'name': name,
                    'location': clean_string(location_place),
                    'description': f"{clean_string(description)} {clean_string(contact_info)} {clean_string(hours_info)}".strip(),
                    'category': category,
                    # 'url': location_url
                }
                print(details)
                # details_list.append(details)

        except Exception as e:
            errors.append(f"Error processing {base_url}: {e}")
        
        finally:
            browser.close()

    return details_list, errors

# ----- Execution Code -----

TARGET_URL = "https://www.smu.edu.sg/campus-life/visiting-smu/food-beverages-listing"
TARGET_FILEPATH = "./../output/smu_dining_details.json"

details_list, errors = scrape_smu(TARGET_URL)

if errors:
    print(f"Errors encountered: {errors}")
print("Scraping complete.")
delete_file(TARGET_FILEPATH)

with open(TARGET_FILEPATH, 'w') as f:
    json.dump(details_list, f, indent=4)