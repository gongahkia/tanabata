"""
https://www.rp.edu.sg/our-campus/facilities/retail-dining

div.row.mb-50
    div div div h3 --> inner_text() is name
    div div div p --> inner_text() is description
"""

from playwright.sync_api import sync_playwright
import json
import os
import re


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
    cleaned_string = re.sub(r"\n+", " ", input_string)
    cleaned_string = re.sub(r"<[^>]+>", "", cleaned_string)
    return cleaned_string.strip()


def scrape_rp_dining(url):
    """
    Scrapes the Republic Polytechnic retail and dining page
    """
    scraped_data = []
    errors = []
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        try:
            page.goto(url)
            page.wait_for_selector("div.row.mb-50")
            print(f"Scraping the URL: {url}")
            post_items = page.query_selector_all("div.row.mb-50")
            for item in post_items:
                name = item.query_selector("div div div h3").inner_text().strip()
                description = item.query_selector("div div div p").inner_text().strip()
                details = {
                    "name": name,
                    "location": "",
                    "description": clean_string(description),
                    "category": "Retail & Dining",
                    "url": url,
                }
                print(details)
                scraped_data.append(details)
        except Exception as e:
            errors.append(f"Error processing {url}: {e}")
        finally:
            browser.close()
    with open("./../output/rp_dining_data.json", "w") as f:
        json.dump(scraped_data, f, indent=4)
    return scraped_data, errors


# ----- EXECUTION CODE -----

if __name__ == "__main__":
    delete_file("./../output/rp_dining_data.json")
    scrape_rp_dining("https://www.rp.edu.sg/our-campus/facilities/retail-dining")
