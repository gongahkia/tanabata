"""
~~~ FUA ~~~

try and understand what the scraper code does

debug as required

~~~ INTERNAL REFERENCE ~~~

site to scrape:
https://www.ntu.edu.sg/life-at-ntu/leisure-and-dining/general-directory?locationTypes=all&locationCategories=all&page=1

~ SITE DOM STRUCTURE ~

div.col-sm-8.col-md-9
    li.search__results-item.col-sm-6.col-md-4
        div.img-card.img-card--box
            div.img-card__body
                h3.img-card__title --> name
                span.img-card__label.location-label
                    span.location --> location
                p.img-card__info --> description
                a.link.link--icon href --> url
    
div.search__results-footer --> to go to next page
    li.active
        a.href

~ RETURNED DETAILS ~

details = {
    'name': 
    'location': 
    'description': 
    'category':
    'url':
}
"""

from requests_html import HTMLSession
from bs4 import BeautifulSoup
import json
import os

def delete_file(target_url):
    try:
        os.remove(target_url)
        print(f"Deleted file at filepath: {target_url}")
    except OSError as e:
        print(f"Error deleting file at filepath: {target_url} due to {e}")

def scrape_ntu(base_url):
    """
    scrapes the specified ntu website
    for the food vendor's name, location,
    description, category and url
    """
    session = HTMLSession()
    page = 1
    details_list = []
    errors = []

    while True:

        url = f"{base_url}{page}"
        response = session.get(url)
        response.html.render()
        soup = BeautifulSoup(response.html.html, 'html.parser')

        listings = soup.select('div.col-sm-8.col-md-9 li.search__results-item.col-sm-6.col-md-4')

        if not listings:

            errors.append("No more listings found.")
            return [details_list, errors]

        else:

            for listing in listings:

                name = listing.select_one('div.img-card__body h3.img-card__title').get_text(strip=True)
                location = listing.select_one('span.img-card__label.location-label span.location').get_text(strip=True) if listing.select_one('span.img-card__label.location-label') else ''
                description = listing.select_one('p.img-card__info').get_text(strip=True) if listing.select_one('p.img-card__info') else ''
                url = listing.select_one('a.link.link--icon')['href']
                details = {
                    'name': name,
                    'location': location,
                    'description': description,
                    'category': '', # FUA assume category is not available for now
                    'url': url
                }
                details_list.append(details)
                # print(details)

            next_page = soup.select_one('div.search__results-footer li.active a')
            if next_page and 'href' in next_page.attrs:
                page += 1
            else:
                return [details_list, errors]

# ----- execution code -----

TARGET_URL= "https://www.ntu.edu.sg/life-at-ntu/leisure-and-dining/general-directory?locationTypes=all&locationCategories=all&page="
TARGET_FILEPATH = "output/ntu_dining_details.json"
result = scrape_ntu(TARGET_URL)
details_list, errors = result[0], result[1]

if errors:
    print(f"errors encountered: {errors}")
print("scraping complete")
delete_file(TARGET_FILEPATH)
with open(TARGET_FILEPATH, 'w') as f:
    json.dump(details_list, f, indent=4)