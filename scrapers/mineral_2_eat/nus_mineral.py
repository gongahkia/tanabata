"""
FUA

debug this to check if it actually runs
and work out why it isn't running if not
"""

from selenium import webdriver
from selenium.webdriver.firefox.service import Service
from webdriver_manager.firefox import GeckoDriverManager
from bs4 import BeautifulSoup
import json

def clean_string(input_string):
    """
    Sanitize a provided string
    """
    return input_string.strip()

def fetch_nus_dining_data(url):
    """
    Fetch dining details from the given NUS page using Selenium and GeckoDriver
    """
    options = webdriver.FirefoxOptions()
    options.add_argument("--headless")  # Run in headless mode, comment out if you want to see the browser
    driver = webdriver.Firefox(service=Service(GeckoDriverManager().install()), options=options)
    
    details_list = []
    errors = []
    
    try:
        driver.get(url)
        soup = BeautifulSoup(driver.page_source, 'html.parser')
        listings = soup.select('div.vc_col-sm-12.vc_gitem-col.vc_gitem-col-align-')

        for listing in listings:
            name = listing.select_one('h3[style="text-align: left"]').get_text(strip=True) if listing.select_one('h3[style="text-align: left"]') else ''
            details = listing.select_one('div.vc_custom_heading.vc_custom_1534520726761.vc_gitem-post-data.vc_gitem-post-data-source-post_excerpt p').get_text(strip=True) if listing.select_one('div.vc_custom_heading.vc_custom_1534520726761.vc_gitem-post-data.vc_gitem-post-data-source-post_excerpt p') else ''
            if name and details:
                details_list.append({
                    'name': clean_string(name),
                    'details': clean_string(details)
                })
    except Exception as e:
        errors.append(str(e))
    finally:
        driver.quit()  # Ensure the driver is closed after the task is complete

    return details_list, errors


# ----- execution code -----

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
