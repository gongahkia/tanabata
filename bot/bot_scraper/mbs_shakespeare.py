"""
~~~ INTERNAL REFERENCE ~~~

sites to scrape: https://www.marinabaysands.com/restaurants/view-all.html

~ HTML DOM STRUCTURE ~

div.restauranttitlefilter__container--carddetails__card --> queryall for divs with this EXACT class, nothing more
    div.restauranttitlefilter__container--carddetails__card--contentarea
        div.title div.cmp-title a.cmp-title-link h3.cmp-title__text --> inner_text() is name
        div.title div.cmp-title a.cmp-title-link --> href is url
    div.restauranttitlefilter__container--carddetails__card--details
        div.location-details div.location-details--address div.information --> inner_text() is location
"""

import json
import os
import re
from playwright.sync_api import sync_playwright


def delete_file(target_url):
    """
    helper function that attempts to delete a file at the specified URL
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
    cleaned_string = re.sub(r"\n+", " ", input_string)
    cleaned_string = re.sub(r"<[^>]+>", "", cleaned_string)
    return cleaned_string.strip()


def scrape_marina_bay_sands_restaurants(base_url):
    """
    scrapes the Marina Bay Sands website for restaurant details
    """
    details_list = []
    errors = []
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        try:
            page.goto(base_url)
            page.wait_for_selector(
                "div.restauranttitlefilter__container--carddetails__card"
            )
            print(f"successfully retrieved page URL: {base_url}")
            restaurant_cards = page.query_selector_all(
                "div.restauranttitlefilter__container--carddetails__card"
            )
            for card in restaurant_cards:
                name_element = card.query_selector(
                    "div.restauranttitlefilter__container--carddetails__card--contentarea div.title div.cmp-title a.cmp-title_link h3.cmp-title__text"
                )
                url_element = card.query_selector(
                    "div.restauranttitlefilter__container--carddetails__card--contentarea div.title div.cmp-title a.cmp-title_link"
                )
                location_element = card.query_selector(
                    "div.restauranttitlefilter__container--carddetails__card--details div.location-details div.location-details--address div.information"
                )
                name = clean_string(name_element.inner_text())
                url = (
                    f"https://www.marinabaysands.com{url_element.get_attribute('href')}"
                )
                location = clean_string(location_element.inner_text())
                details = {
                    "name": name,
                    "location": location,
                    "description": "",
                    "category": "",
                    "url": url,
                }
                print(details)
                details_list.append(details)
        except Exception as e:
            errors.append(f"Error processing {base_url}: {e}")
        finally:
            browser.close()
    return details_list, errors


# ----- Execution Code -----

# TARGET_URL = "https://www.marinabaysands.com/restaurants/view-all.html"
# TARGET_FILEPATH = "./../output/marina_bay_sands_restaurants.json"
# details_list, errors = scrape_marina_bay_sands_restaurants(TARGET_URL)
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
    details_list, errors = scrape_marina_bay_sands_restaurants(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
