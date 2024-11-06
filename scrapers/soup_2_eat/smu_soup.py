"""
~~~ FUA ~~~

debug the scraper here, its having 
trouble loading in the webpage due to 
the the nature of incapsula which prevents
scrapers from reading webdata from 
headless browsers

might have to look into using selenium
to scrape this site instead

~~~ INTERNAL REFERENCE ~~~

site to scrape: https://www.smu.edu.sg/campus-life/visiting-smu/food-beverages-listing

~ SITE DOM STRUCTURE ~

div.col-md-9
h4.location-title --> name
div.location-address text --> location
div.location-address a::href --> url
div.location-description --> description
div.location-contact --> description
div.location-hours --> description
'' --> category

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
from requests_html import HTMLSession
from bs4 import BeautifulSoup


def delete_file(target_url):
    """
    helper function that attempts
    to delete a file at the specified
    URL
    """
    try:
        os.remove(target_url)
        print(f"deleted file at filepath: {target_url}")
    except OSError as e:
        print(f"error deleting file at filepath: {target_url} due to {e}")


def clean_string(input_string):
    """
    sanitize a provided string
    """
    cleaned_string = re.sub(r"\n+", " ", input_string)
    cleaned_string = re.sub(r"<[^>]+>", "", cleaned_string)
    return cleaned_string.strip()


def scrape_smu(base_url):
    """
    scrapes the specified SMU website
    for food and beverage details
    """
    # html_file_path = "smu_dining_details.html"
    session = HTMLSession()
    response = session.get(base_url)
    response.html.render()
    details_list = []
    errors = []

    if response.status_code != 200:
        print(f"failed to retrieve page URL: {base_url}")
        print(f"status code: {response.status_code}")
        errors.append("Failed to retrieve data")
        return details_list, errors

    print(f"successfully retrieved page URL: {base_url}")

    # with open(html_file_path, 'w', encoding='utf-8') as html_file:
    #     html_file.write(response.html.html)
    # print(f"raw HTML written to file: {html_file_path}")

    soup = BeautifulSoup(response.html.html, "html.parser")

    # print(soup)

    locations = soup.select("div.col-md-9 div.location")

    for location in locations:
        name = (
            location.select_one("h4.location-title").get_text(strip=True)
            if location.select_one("h4.location-title")
            else ""
        )
        location_text = (
            location.select_one("div.location-address").get_text(strip=True)
            if location.select_one("div.location-address")
            else ""
        )
        location_url = (
            location.select_one("div.location-address a")["href"]
            if location.select_one("div.location-address a")
            else ""
        )
        description = (
            location.select_one("div.location-description").get_text(strip=True)
            if location.select_one("div.location-description")
            else ""
        )
        contact_info = (
            location.select_one("div.location-contact").get_text(strip=True)
            if location.select_one("div.location-contact")
            else ""
        )
        hours_info = (
            location.select_one("div.location-hours").get_text(strip=True)
            if location.select_one("div.location-hours")
            else ""
        )
        category = "Food and Beverage"
        details = {
            "name": name,
            "location": clean_string(location_text),
            "description": f"{clean_string(description)} {clean_string(contact_info)} {clean_string(hours_info)}".strip(),
            "category": category,
            "url": location_url,
        }
        details_list.append(details)

    return details_list, errors


# ----- execution code -----

TARGET_URL = "https://www.smu.edu.sg/campus-life/visiting-smu/food-beverages-listing"
TARGET_FILEPATH = "./../output/smu_dining_details.json"

result = scrape_smu(TARGET_URL)
details_list, errors = result[0], result[1]

if errors:
    print(f"errors encountered: {errors}")
print("Scraping complete")
delete_file(TARGET_FILEPATH)

with open(TARGET_FILEPATH, "w") as f:
    json.dump(details_list, f, indent=4)
