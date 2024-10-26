import json
import os
import re
from django.conf import settings
from playwright.sync_api import sync_playwright  # Changed to sync_api

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
    Sanitize a provided string
    """
    cleaned_string = re.sub(r'\n+', ' ', input_string)
    cleaned_string = re.sub(r'<[^>]+>', '', cleaned_string)
    return cleaned_string.strip()

def scrape_smu(base_url):
    """
    Scrapes the specified SMU website for food and beverage details
    """
    details_list = []
    errors = []

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()

        try:
            page.goto(base_url)
            page.wait_for_selector('div.col-md-9')
            print(f"Successfully retrieved page URL: {base_url}")
            locations = page.query_selector_all('div.col-md-9 div.col-md-9')
            print(locations)

            for location in locations:
                name = location.query_selector('h4.location-title').inner_text()
                location_element = location.query_selector('div.location-address')
                location_info = location_element.inner_text() if location_element else ''
                location_url_info = location_element.query_selector('a').get_attribute('href') if location_element and location_element.query_selector('a') else ''
                description = location.query_selector('div.location-description').inner_text() if location.query_selector('div.location-description') else ''
                category = "Food and Beverage"
                contact_element = location.query_selector('div.location-contact')
                contact_info = contact_element.inner_text().strip() if contact_element else ''
                hours_element = location.query_selector('div.location-hours')
                hours_info = hours_element.inner_text().strip() if hours_element else ''
                
                details = {
                    'name': name,
                    'location': clean_string(location_info),
                    'description': f"{clean_string(description)} {clean_string(contact_info)} {clean_string(hours_info)}".strip(),
                    'category': category,
                    'url': location_url_info
                }

                details_list.append(details)

        except Exception as e:
            errors.append(f"Error processing {base_url}: {e}")

        finally:
            browser.close()

    return details_list, errors
