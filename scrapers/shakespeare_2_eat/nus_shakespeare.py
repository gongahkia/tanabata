import json
import re
import os
from playwright.sync_api import sync_playwright

def delete_file(target_url):
    """
    helper function that attempts to 
    delete a file at the specified url
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
    return input_string.strip()

def parse_details(name, details, url):
    """
    parses and sorts each field
    from the given detail string
    and sorts it into the provided
    JSON categories for easier
    API sorting
    """
    fin = {
        "name": name,
        "location": "",
        "description": "",
        "category": "Food and Beverage",
        "url": url
    }
    for line in details.split("\n"):
        if line.startswith("Location: "):
            fin["location"] = line.lstrip("Location: ")
        else:
            fin["description"] += f"{line.strip()}\n"
    return fin

def fetch_nus_dining_data(url):
    """
    fetches dining details from 
    the given NUS page using Playwright
    """
    details_list = []
    errors = []

    with sync_playwright() as p:

        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        page.goto(url)
        page.wait_for_selector('div.vc_col-sm-12.vc_gitem-col.vc_gitem-col-align-')
        if page.title() == "":
            errors.append(f"failed to retrieve page {url}")
            browser.close()
            return details_list, errors
        else:
            print(f"scraping details from {url}")

        listings = page.query_selector_all('div.vc_col-sm-12.vc_gitem-col.vc_gitem-col-align-')

        for listing in listings:

            # print(listing.inner_text())

            name = listing.query_selector('h3[style="text-align: left"]').inner_text()
            details = listing.query_selector('p[style="text-align: left"] + p').inner_text() # + p goes one element down walahi
            fin_details = parse_details(name, details, url)

            # print(fin_details)

            details_list.append(fin_details)

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
delete_file(output_file)
with open(output_file, 'w') as f:
    json.dump(all_details, f, indent=4)

print(f"Scraping completed, data written to {output_file}")