"""
~~~ FUA ~~~

there are some issues parsing this site
as similar to smu, it relies on incapsula to 
shield its site data from other sites

might have to look into using selenium
to scrape this site instead

~~~ INTERNAL REFERENCE ~~~

~ SITES TO SCRAPE ~

https://www.ionorchard.com/en/dine.html?category=Casual%20Dining%20and%20Takeaways
https://www.ionorchard.com/en/dine.html?category=Restaurants%20and%20Cafes

~ SITE DOM STRUCTURE ~

div.cmp-dynamic-list-dine-shop-grid-list
    div.cmp-dynamic-list-dine-shop-grid-item
        div.cmp-dynamic-list-dine-shop-item-content.cmp-dynamic-list-dine-shop-item-content-info.cmp-dynamic-list-dine-shop-item-content-item-title --> name
        div.cmp-dynamic-list-dine-shop-item-content.cmp-dynamic-list-dine-shop-item-content-info.cmp-dynamic-list-dine-shop-item-content-item-num --> location

div.cmp-dynamic-list-pagination-container --> to go to the next page
    span.cmp-dynamic-list-paginate-item --> and active is an added class if the current number is active

~ RETURNED DETAILS ~

details = {
    'name': 
    'location': 
    'description': 
    'category':
    'url':
}
"""

import json
import os
import re
from bs4 import BeautifulSoup
from requests_html import HTMLSession

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
    location, description, category, and URL
    """
    session = HTMLSession()
    details_list = []
    errors = []

    for base_url in base_urls:
        page = 1
        
        while True:
            url = f"{base_url}&page={page}"
            try:
                response = session.get(url, timeout=10)
                response.raise_for_status() 
            except Exception as e:
                errors.append(f"Error fetching {url}: {e}")
                break 

            response.html.render()
            soup = BeautifulSoup(response.html.html, 'html.parser')
            print(soup)

            listings = soup.select('div.cmp-dynamic-list-dine-shop-grid-item')

            if not listings:
                errors.append("No more listings found.")
                break

            for listing in listings:
                name = listing.select_one('div.cmp-dynamic-list-dine-shop-item-content.cmp-dynamic-list-dine-shop-item-content-item-title').get_text(strip=True)
                raw_location = listing.select_one('div.cmp-dynamic-list-dine-shop-item-content.cmp-dynamic-list-dine-shop-item-content-item-num').get_text(strip=True) if listing.select_one('div.cmp-dynamic-list-dine-shop-item-content.cmp-dynamic-list-dine-shop-item-content-item-num') else ''
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

            next_page = soup.select_one('div.cmp-dynamic-list-pagination-container span.cmp-dynamic-list-paginate-item.active')
            if next_page:
                page += 1
            else:
                break 
    session.close() 
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