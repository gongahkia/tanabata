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

~ SITES TO SCRAPE ~

https://uci.nus.edu.sg/oca/retail-dining/food-and-beverages/
https://uci.nus.edu.sg/oca/retail-dining/food-and-beverage-utown/
https://uci.nus.edu.sg/oca/retail-dining/food-and-beverages-bukit-timah/

~ SITE DOM STRUCTURE ~

div.vc_col-sm-12.vc_gitem-col.vc_gitem-col-align-
    div.vc_custom_heading heading-orange.vc_custom_1534521762134.vc_gitem-post-data.vc_gitem-post-data-source-post_title
        h3.style="text-align: left" --> name
    div.vc_custom_heading.vc_custom_1534520726761.vc_gitem-post-data.vc_gitem-post-data-source-post_excerpt
        p --> details

then insert code to parse the details later

~ RETURNED DETAILS ~

details = {
    'name': 
    'details':
    'location': 
    'description': 
    'category':
    'url':
}
"""

from requests_html import HTMLSession
from bs4 import BeautifulSoup
import json


def clean_string(input_string):
    """
    sanitize a provided string
    """
    return input_string.strip()


def fetch_nus_dining_data(url):
    """
    fetches dining details
    from the given NUS page
    using requests_html
    """
    session = HTMLSession()
    response = session.get(url)
    response.html.render()
    details_list = []
    errors = []
    if response.status_code != 200:
        errors.append(f"Failed to retrieve page {url}")
        return details_list, errors
    else:
        soup = BeautifulSoup(response.html.html, "html.parser")
        listings = soup.select("div.vc_col-sm-12.vc_gitem-col.vc_gitem-col-align-")
        for listing in listings:
            name = (
                listing.select_one('h3[style="text-align: left"]').get_text(strip=True)
                if listing.select_one('h3[style="text-align: left"]')
                else ""
            )
            details = (
                listing.select_one(
                    "div.vc_custom_heading.vc_custom_1534520726761.vc_gitem-post-data.vc_gitem-post-data-source-post_excerpt p"
                ).get_text(strip=True)
                if listing.select_one(
                    "div.vc_custom_heading.vc_custom_1534520726761.vc_gitem-post-data.vc_gitem-post-data-source-post_excerpt p"
                )
                else ""
            )
            if name and details:
                details_list.append(
                    {"name": clean_string(name), "details": clean_string(details)}
                )
        return details_list, errors


# ----- execution code -----

urls = [
    "https://uci.nus.edu.sg/oca/retail-dining/food-and-beverages/",
    "https://uci.nus.edu.sg/oca/retail-dining/food-and-beverage-utown/",
    "https://uci.nus.edu.sg/oca/retail-dining/food-and-beverages-bukit-timah/",
]
all_locations = {}
all_details = {}

for url in urls:
    all_locations[url] = fetch_nus_dining_data(url)[0]
all_details["nus"] = all_locations
output_file = "./../output/nus_dining_details.json"
with open(output_file, "w") as f:
    json.dump(all_details, f, indent=4)
print(f"scraping completed, data written to {output_file}")
