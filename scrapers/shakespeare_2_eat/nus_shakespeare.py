import json
import re
from playwright.sync_api import sync_playwright

def clean_string(input_string):
    """
    Sanitize a provided string.
    """
    return input_string.strip()

def fetch_nus_dining_data(url):
    """
    Fetches dining details from the given NUS page using Playwright.
    """
    details_list = []
    errors = []

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        page.goto(url)
        
        if page.title() == "":
            errors.append(f"Failed to retrieve page {url}")
            browser.close()
            return details_list, errors
        
        listings = page.query_selector_all('div.vc_col-sm-12.vc_gitem-col.vc_gitem-col-align-')
        
        for listing in listings:
            name_element = listing.query_selector('h3[style="text-align: left"]')
            details_element = listing.query_selector('div.vc_custom_heading.vc_custom_1534520726761.vc_gitem-post-data.vc_gitem-post-data-source-post_excerpt p')
            
            name = name_element.inner_text().strip() if name_element else ''
            details = details_element.inner_text().strip() if details_element else ''
            
            if name and details:
                details_list.append({
                    'name': clean_string(name),
                    'details': clean_string(details)
                })
        
        browser.close()
    
    return details_list, errors

# ----- Execution Code -----

urls = [
    "https://uci.nus.edu.sg/oca/retail-dining/food-and-beverages/",
    "https://uci.nus.edu.sg/oca/retail-dining/food-and-beverage-utown/",
    "https://uci.nus.edu.sg/oca/retail-dining/food-and-beverages-bukit-timah/"
]

all_locations = {}
all_details = {}

for url in urls:
    all_locations[url] = fetch_nus_dining_data(url)[0]

all_details["nus"] = all_locations

output_file = "./../output/nus_dining_details.json"
with open(output_file, 'w') as f:
    json.dump(all_details, f, indent=4)

print(f"Scraping completed, data written to {output_file}")