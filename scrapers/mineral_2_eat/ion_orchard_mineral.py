import json
import os
import re
from selenium import webdriver
from selenium.webdriver.common.by import By
from selenium.webdriver.chrome.service import Service as ChromeService
from webdriver_manager.chrome import ChromeDriverManager
from selenium.webdriver.chrome.options import Options

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
    sanitizes a provided string
    """
    cleaned_string = re.sub(r'\n+', ' ', input_string)
    cleaned_string = re.sub(r'<[^>]+>', '', cleaned_string)
    return cleaned_string.strip()

def scrape_ion_orchard(base_urls):
    """
    scrapes the specified ion orchard 
    websites for the food vendor's name,
    location, description, category, and url
    """
    options = Options()
    options.headless = True 
    driver = webdriver.Chrome(service=ChromeService(ChromeDriverManager().install()), options=options)
    details_list = []
    errors = []
    for base_url in base_urls:
        page = 1
        while True:
            url = f"{base_url}&page={page}"
            try:
                driver.get(url)
                driver.implicitly_wait(10) 
                listings = driver.find_elements(By.CSS_SELECTOR, 'div.cmp-dynamic-list-dine-shop-grid-item')
                if not listings:
                    errors.append("No more listings found.")
                    break
                for listing in listings:
                    name = listing.find_element(By.CSS_SELECTOR, 'div.cmp-dynamic-list-dine-shop-item-content.cmp-dynamic-list-dine-shop-item-content-item-title').text.strip()
                    raw_location = listing.find_element(By.CSS_SELECTOR, 'div.cmp-dynamic-list-dine-shop-item-content.cmp-dynamic-list-dine-shop-item-content-item-num').text.strip() if listing.find_elements(By.CSS_SELECTOR, 'div.cmp-dynamic-list-dine-shop-item-content.cmp-dynamic-list-dine-shop-item-content-item-num') else ''
                    clean_location = clean_string(raw_location)
                    description = "" 
                    category = ""
                    vendor_url = base_url 
                    details = {
                        'name': name,
                        'location': clean_location,
                        'description': description,
                        'category': category,
                        'url': vendor_url
                    }
                    details_list.append(details)
                next_page = driver.find_elements(By.CSS_SELECTOR, 'div.cmp-dynamic-list-pagination-container span.cmp-dynamic-list-paginate-item.active')
                if next_page:
                    page += 1
                else:
                    break 
            except Exception as e:
                errors.append(f"Error processing {url}: {e}")
                break 

    driver.quit() 
    return details_list, errors

# ----- execution Code -----

BASE_URLS = [
    "https://www.ionorchard.com/en/dine.html?category=Casual%20Dining%20and%20Takeaways",
    "https://www.ionorchard.com/en/dine.html?category=Restaurants%20and%20Cafes"
]
TARGET_FILEPATH = "./../output/ion_orchard_dining_details.json"

details_list, errors = scrape_ion_orchard(BASE_URLS)
if errors:
    print(f"Errors encountered: {errors}")
print("Scraping complete.")
delete_file(TARGET_FILEPATH)
with open(TARGET_FILEPATH, 'w') as f:
    json.dump(details_list, f, indent=4)