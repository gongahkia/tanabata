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
    cleaned_string = re.sub(r"\n+", " ", input_string)
    cleaned_string = re.sub(r"<[^>]+>", "", cleaned_string)
    return cleaned_string.strip()


def scrape_vivocity(base_url):
    """
    Scrapes VivoCity's Dining Guide from the provided base URL with scrolling to load more content
    Stops scrolling when no further content is loaded (i.e., page height stops increasing)
    """
    details_list = []
    errors = []
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        try:
            page.goto(base_url)
            page.wait_for_selector("div.ais-Hits-item", timeout=10000)
            previous_height = 0
            while True:
                page.evaluate("window.scrollTo(0, document.body.scrollHeight)")
                print("scrolling down...")
                page.wait_for_timeout(5000)
                new_height = page.evaluate("document.body.scrollHeight")
                print(
                    f"previous height: {previous_height}\ncurrent height: {new_height}"
                )
                if new_height == previous_height:
                    print("height hasn't changed, exiiting loop")
                    break
                previous_height = new_height
            items = page.query_selector_all("div.ais-Hits-item")
            for item in items:
                url_element = item.query_selector("div.guideListWrapper a")
                url = url_element.get_attribute("href") if url_element else ""
                name_element = item.query_selector(
                    "div.guideListContentWrapper div.guideListContent a.storeName"
                )
                name = clean_string(name_element.inner_text()) if name_element else ""
                location_element = item.query_selector(
                    "div.guideListContentWrapper div.guideListContent div.storeContentSection span.storeTextContent"
                )
                location = (
                    clean_string(location_element.inner_text())
                    if location_element
                    else ""
                )
                category_element = item.query_selector(
                    "div.guideListContentWrapper div.guideListContent div.storeContentSectionSecond span.storeTextContent"
                )
                category = (
                    clean_string(category_element.inner_text())
                    if category_element
                    else ""
                )
                if name and url and location and category:
                    details = {
                        "name": name,
                        "location": location,
                        "description": "",
                        "category": category,
                        "url": url,
                    }
                    print(details)
                    details_list.append(details)
        except Exception as e:
            errors.append(f"Error processing {base_url}: {str(e)}")
        browser.close()
    return details_list, errors


# ----- MAIN EXECUTION CODE -----

base_url = "https://www.vivocity.com.sg/shopping-guide/dining-guide"
scraped_data, scraping_errors = scrape_vivocity(base_url)
output_file = "../output/vivocity_dining_details.json"
with open(output_file, "w") as f:
    json.dump(scraped_data, f, indent=4)
if scraping_errors:
    print("Errors encountered:", scraping_errors)
else:
    print(f"Scraping completed successfully. Data saved to {output_file}.")
