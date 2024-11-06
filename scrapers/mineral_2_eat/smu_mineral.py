"""
FUA

debug this to check if it actually runs
and work out why it isn't running if not
"""

import json
import os
import re
from selenium import webdriver
from selenium.webdriver.chrome.service import Service as ChromeService
from selenium.webdriver.chrome.options import Options
from selenium.webdriver.common.by import By
from bs4 import BeautifulSoup


def delete_file(target_url):
    """
    Helper function that attempts
    to delete a file at the specified
    URL
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


def scrape_smu(base_url):
    """
    Scrapes the specified SMU website
    for food and beverage details
    """
    # Setting up Selenium with Chrome in headless mode
    chrome_options = Options()
    chrome_options.add_argument("--headless")  # Run headless
    chrome_options.add_argument(
        "--no-sandbox"
    )  # Required for certain server environments
    chrome_options.add_argument(
        "--disable-dev-shm-usage"
    )  # Overcome limited resource problems

    driver = webdriver.Chrome(service=ChromeService(), options=chrome_options)

    details_list = []
    errors = []

    try:
        driver.get(base_url)
        driver.implicitly_wait(10)
        soup = BeautifulSoup(driver.page_source, "html.parser")

        print(soup)

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

    except Exception as e:
        print(f"An error occurred: {e}")
        errors.append("Failed to retrieve data")
    finally:
        driver.quit()  # Ensure the driver is closed

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
print(f"Data saved to {TARGET_FILEPATH}")
