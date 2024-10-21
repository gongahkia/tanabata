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
    Sanitize a provided string by stripping unnecessary characters
    """
    cleaned_string = re.sub(r'\n+', ' ', input_string)
    cleaned_string = re.sub(r'<[^>]+>', '', cleaned_string)
    return cleaned_string.strip()

def scrape_custom_mall(base_url):
    """
    Scrapes a custom mall page with the provided base URL
    """
    details_list = []
    errors = []
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        try:
            page.goto(base_url)
            page.wait_for_selector('div.mix.col-md-6.cat-foodcourt-restaurants-cafes div.storelist-wrapper', timeout=10000) 
            items = page.query_selector_all('div.mix.col-md-6.cat-foodcourt-restaurants-cafes div.storelist-wrapper')
            for item in items:
                name_element = item.query_selector('div.col-xs-20 p.title')
                name = clean_string(name_element.inner_text()) if name_element else ""
                raw_elements= item.query_selector_all('div.col-xs-8.col-sm-4 div.meta-wrap')
                for element in raw_elements:
                    class_name = element.query_selector('span').get_attribute('class')
                    if 'icon-Location' in class_name:
                        location = element.inner_text().strip()
                    elif 'icon-Telephone' in class_name:
                        description = element.inner_text().strip()
                url_element = item.query_selector('div.col-xs-8.col-sm-4 div.meta-wrap a')
                url = url_element.get_attribute('href') if url_element else ""
                if name and url:
                    details = {
                        'name': name,
                        'location': location,
                        'description': description,
                        'category': "Foodcourt, Restaurants and Cafes",
                        'url': url
                    }
                    print(details)
                    details_list.append(details)
        except Exception as e:
            errors.append(f"Error processing {base_url}: {str(e)}")
        browser.close()
    return details_list, errors

base_url = "https://www.theclementimall.com/stores"
scraped_data, scraping_errors = scrape_custom_mall(base_url)
output_file = './../output/clementi_mall_dining_details.json'
with open(output_file, 'w') as f:
    json.dump(scraped_data, f, indent=4)
if scraping_errors:
    print("Errors encountered:", scraping_errors)
else:
    print(f"Scraping completed successfully. Data saved to {output_file}.")