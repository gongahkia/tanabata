"""
~~~ INTERNAL REFERENCE ~~~

sites to scrape: https://changicitypoint.com.sg/stores/?search=&level=&mall=&cat=12&apply_filter=

~ HTML DOM STRUCTURE ~

div.item.item-custom
    div.content-store div.thumb a --> href is the url
    div.box-content div.name a --> inner_text is the name
    div.box-content div.list div.item.d-flex div.content --> inner_text, first 2 inner_texts are description, 3rd inner_text is category, 4th inner_text is location

div.group-btn.justify-content-center ul.pagi.d-flex
    li.current.pagi-item.is-active --> current page
    li a --> other pages
"""

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
    Sanitize a provided string
    """
    cleaned_string = re.sub(r"\n+", " ", input_string)
    cleaned_string = re.sub(r"<[^>]+>", "", cleaned_string)
    return cleaned_string.strip()


def scrape_changi_city_point(base_url):
    """
    Scrapes the Changi City Point website for dining details and handles pagination
    """
    details_list = []
    errors = []
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        try:
            page.goto(base_url)
            page.wait_for_selector("div.item.item-custom")
            while True:
                items = page.query_selector_all("div.item.item-custom")
                for item in items:
                    url_element = item.query_selector("div.content-store div.thumb a")
                    name_element = item.query_selector("div.box-content div.name a")
                    description_elements = item.query_selector_all(
                        "div.box-content div.list div.item.d-flex div.content"
                    )
                    print([el.inner_text() for el in description_elements])
                    url = url_element.get_attribute("href") if url_element else ""
                    name = (
                        clean_string(name_element.inner_text()) if name_element else ""
                    )
                    description = (
                        f"{clean_string(description_elements[0].inner_text())}, {clean_string(description_elements[1].inner_text())}"
                        if description_elements
                        else ""
                    )
                    category = (
                        clean_string(description_elements[2].inner_text())
                        if len(description_elements) > 2
                        else ""
                    )
                    location = (
                        clean_string(description_elements[3].inner_text())
                        if len(description_elements) > 3
                        else ""
                    )
                    details = {
                        "name": name,
                        "location": location,
                        "description": description,
                        "category": category,
                        "url": url,
                    }
                    print(details)
                    details_list.append(details)
                next_page = (
                    page.query_selector(
                        "li.pagi-item.pagi-action.pagi-next.is-disabled a.page-link.next"
                    )
                    if page.query_selector(
                        "li.pagi-item.pagi-action.pagi-next.is-disabled a.page-link.next"
                    )
                    else None
                )
                if next_page:
                    print("navigating to next page")
                    next_page.click()
                    page.wait_for_timeout(2000)
                else:
                    print("no more pages to navigate to")
                    break
        except Exception as e:
            errors.append(f"Error processing {base_url}: {e}")
        finally:
            browser.close()
    return details_list, errors


# ----- Execution Code -----

# TARGET_URL = (
#     "https://changicitypoint.com.sg/stores/?search=&level=&mall=&cat=12&apply_filter="
# )
# TARGET_FILEPATH = "./../output/changi_city_point_dining_details.json"
# details_list, errors = scrape_changi_city_point(TARGET_URL)
# if errors:
#     print(f"Errors encountered: {errors}")
# print("Scraping complete.")
# delete_file(TARGET_FILEPATH)
# with open(TARGET_FILEPATH, "w") as f:
#     json.dump(details_list, f, indent=4)


def run_scraper(target_url):
    """
    actual function to call the scraper code
    and display it to users
    """
    details_list, errors = scrape_changi_city_point(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
