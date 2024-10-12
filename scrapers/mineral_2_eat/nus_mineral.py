"""
FUA

debug this to check if it actually runs
and work out why it isn't running if not
"""

from selenium import webdriver
from selenium.webdriver.chrome.service import Service
from selenium.webdriver.chrome.options import Options
from webdriver_manager.chrome import ChromeDriverManager
from bs4 import BeautifulSoup
import json

def clean_string(input_string):
    """
    Sanitize a provided string
    """
    return input_string.strip()

def fetch_nus_dining_data(url):
    """
    Fetches dining details from the given NUS page using Selenium
    """
    # Set Chrome options
    chrome_options = Options()
    chrome_options.add_argument("--headless")  # Run in headless mode
    chrome_options.add_argument("--no-sandbox")  # Bypass OS security model
    chrome_options.add_argument("--disable-dev-shm-usage")  # Overcome resource limitations
    chrome_options.add_argument("--disable-gpu")  # Disable GPU in headless mode
    chrome_options.add_argument("--remote-debugging-port=9222")  # Enable remote debugging

    # Initialize the Chrome driver without specifying version
    driver = webdriver.Chrome(service=Service(ChromeDriverManager().install()), options=chrome_options)

    # Fetch the page
    driver.get(url)
    
    # Extract the HTML content
    page_source = driver.page_source
    soup = BeautifulSoup(page_source, 'html.parser')

    # Parse the listings
    details_list = []
    errors = []
    listings = soup.select('div.vc_col-sm-12.vc_gitem-col.vc_gitem-col-align-')
    
    for listing in listings:
        name = listing.select_one('h3[style="text-align: left"]').get_text(strip=True) if listing.select_one('h3[style="text-align: left"]') else ''
        details = listing.select_one('div.vc_custom_heading.vc_custom_1534520726761.vc_gitem-post-data.vc_gitem-post-data-source-post_excerpt p').get_text(strip=True) if listing.select_one('div.vc_custom_heading.vc_custom_1534520726761.vc_gitem-post-data.vc_gitem-post-data-source-post_excerpt p') else ''
        
        if name and details:
            details_list.append({
                'name': clean_string(name),
                'details': clean_string(details)
            })
    
    # Close the browser session
    driver.quit()
    
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
