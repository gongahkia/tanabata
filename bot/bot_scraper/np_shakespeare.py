"""
~~~ INTERNAL REFERENCE ~~~

sites to scrape: https://www.foodadvisor.com.sg/nearby/599489/

~ HTML DOM STRUCTURE ~

div.post-item
   div.post-item-image.img-container-250 a --> href is the url
   div.post-item-info h4 --> inner_text is name
   div.post-item-info div.text-muted.small.text-ellipsis --> inner_text is category
   div.post-item-info div.text-ellipsis-2 i.fa.fa-map-marker.fa-color --> inner_text is location
   div.post-item-info div.scroll-x.m-t-5 div.btn.btn-default --> queryselectorall() and the inner_text is description

ul.pagination li.next span.track-page --> keep clicking while visible
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


def scrape_np_dining(url):
    """
    scrapes the np site
    """
    scraped_data = []
    errors = []
    with sync_playwright() as p:
        # browser = p.chromium.launch(headless=True) # for some reason the scraping doesn't work if its headless browser
        browser = p.chromium.launch(headless=False)
        page = browser.new_page()
        try:
            page.goto(url)
            while True:
                page.wait_for_selector("div.post-item")
                print(f"scraping the URL: {url}")
                post_items = page.query_selector_all("div.post-item")
                for item in post_items:
                    name = (
                        item.query_selector("div.post-item-info h4")
                        .inner_text()
                        .strip()
                    )
                    url = item.query_selector(
                        "div.post-item-image.img-container-250 a"
                    ).get_attribute("href")
                    category_el = item.query_selector(
                        "div.post-item-info div.text-muted.small.text-ellipsis"
                    )
                    if category_el:
                        category = [
                            el.strip()
                            for el in category_el.inner_text().strip().split("·")
                        ]
                    else:
                        category = []
                    location = (
                        item.query_selector("div.post-item-info div.text-ellipsis-2")
                        .inner_text()
                        .strip()
                    )
                    descriptions = [
                        desc.inner_text().strip()
                        for desc in item.query_selector_all(
                            "div.post-item-info div.scroll-x.m-t-5 div.btn.btn-default"
                        )
                    ]
                    details = {
                        "name": name,
                        "location": location,
                        "descriptions": descriptions,
                        "category": category,
                        "url": url,
                    }
                    print(details)
                    scraped_data.append(details)
                next_button = page.query_selector(
                    "ul.pagination li.next span.track-page"
                )
                if next_button:
                    next_button.click()
                    page.wait_for_timeout(3000)
                else:
                    break
        except Exception as e:
            errors.append(f"Error processing {url}: {e}")
        finally:
            browser.close()
    with open("./../output/np_dining_data.json", "w") as f:
        json.dump(scraped_data, f, indent=4)
    return scraped_data, errors


# ----- EXECUTION CODE -----

# if __name__ == "__main__":
#     delete_file("./../output/np_dining_data.json")
#     scrape_np_dining("https://www.foodadvisor.com.sg/nearby/599489/")


def run_scraper(target_url):
    """
    actual function to call the scraper code
    and display it to users
    """
    details_list, errors = scrape_np_dining(target_url)
    if errors:
        print(f"Errors encountered: {errors}")
        return errors
    else:
        print("Scraping complete.")
        return details_list
