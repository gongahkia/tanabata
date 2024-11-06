"""
~~~ INTERNAL REFERENCE ~~~

sites to scrape: https://www.payalebarquarter.com/directory/mall/?categories=Food+%26+Restaurantc

~ HTML DOM STRUCTURE ~

span button.button.button--secondary --> click() until no more

div.directory-card.relative.grid.gap-5.border.px-4.py-5.bg-brand-page
    div.relative.flex.flex-col.auto-rows-max.gap-y-2.5.text-brand-body div a --> href is url, inner_text() is name
    div span --> inner_text() if None then pass, if startswith("#") then is location, else is description 
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
    Sanitize a provided string by removing extra whitespace and line breaks
    """
    cleaned_string = re.sub(r"\n+", " ", input_string)
    cleaned_string = re.sub(r"<[^>]+>", "", cleaned_string)
    return cleaned_string.strip()


def scrape_paya_lebar_quarter(base_url):
    """
    Scrapes the Payar Lebar Quarter directory website for food and restaurant details
    """
    details_list = []
    errors = []

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()

        try:
            page.goto(base_url)
            page.wait_for_selector(
                "div.directory-card.relative.grid.gap-5.border.px-4.py-5.bg-brand-page"
            )
            print(f"successfully retrieved page URL: {base_url}")

            while True:
                try:
                    load_more_button = page.query_selector(
                        "button.button.button--secondary"
                    )
                    if load_more_button:
                        print("load button found")
                        load_more_button.click()
                        page.wait_for_timeout(1000)
                    else:
                        print("load button no longer found, loading finished")
                        break
                except Exception as e:
                    print(f"No more pages to load: {e}")
                    break

            directory_cards = page.query_selector_all(
                "div.directory-card.relative.grid.gap-5.border.px-4.py-5.bg-brand-page"
            )
            for card in directory_cards:
                # print(card.inner_text())
                name_element = card.query_selector(
                    "div.relative.flex.flex-col.auto-rows-max.text-brand-body div a"
                )
                name = name_element.inner_text() if name_element else ""
                url = name_element.get_attribute("href") if name_element else ""
                description = ""
                location = ""
                span_elements = card.query_selector_all("div span")
                print([element.inner_text() for element in span_elements])
                for span in span_elements:
                    span_text = span.inner_text().strip()
                    if span_text.startswith("#"):
                        location = span_text
                    else:
                        if (
                            span_text == "Food & Restaurant"
                            or span_text == name.strip()
                        ):
                            pass
                        else:
                            description += f"{span_text} "
                description = description.strip()
                details = {
                    "name": clean_string(name),
                    "location": clean_string(location),
                    "description": clean_string(description),
                    "category": "Food & Restaurant",
                    "url": f"https://www.payalebarquarter.com{url}",
                }
                # print(details)
                details_list.append(details)

        except Exception as e:
            errors.append(f"Error processing {base_url}: {e}")

        finally:
            browser.close()

    return details_list, errors


# ----- Execution Code -----

TARGET_URL = (
    "https://www.payalebarquarter.com/directory/mall/?categories=Food+%26+Restaurant"
)
TARGET_FILEPATH = "./../output/paya_lebar_quarter_dining_details.json"
details_list, errors = scrape_paya_lebar_quarter(TARGET_URL)
if errors:
    print(f"Errors encountered: {errors}")
print("Scraping complete.")
delete_file(TARGET_FILEPATH)
with open(TARGET_FILEPATH, "w") as f:
    json.dump(details_list, f, indent=4)
