"""
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

session = HTMLSession()
base_url = "https://www.ntu.edu.sg/life-at-ntu/leisure-and-dining/general-directory?locationTypes=all&locationCategories=all&page="
page = 1
details_list = []

while True:
    url = f"{base_url}{page}"
    response = session.get(url)

    # Execute JavaScript to render the page
    response.html.render()

    # Use BeautifulSoup to parse the rendered HTML
    soup = BeautifulSoup(response.html.html, 'html.parser')

    # Find all listings
    listings = soup.select('div.col-sm-8.col-md-9 li.search__results-item.col-sm-6.col-md-4')

    if not listings:
        print("No more listings found.")
        break

    for listing in listings:
        name = listing.select_one('div.img-card__body h3.img-card__title').get_text(strip=True)
        location = listing.select_one('span.img-card__label.location-label span.location').get_text(strip=True) if listing.select_one('span.img-card__label.location-label') else ''
        description = listing.select_one('p.img-card__info').get_text(strip=True) if listing.select_one('p.img-card__info') else ''
        url = listing.select_one('a.link.link--icon')['href']

        details = {
            'name': name,
            'location': location,
            'description': description,
            'category': '',  # Assuming category is not available
            'url': url
        }

        details_list.append(details)

    # Check for the next page link
    next_page = soup.select_one('div.search__results-footer li.active a')
    if next_page and 'href' in next_page.attrs:
        page += 1
    else:
        break

# Output the details
for detail in details_list:
    print(detail)

# Optionally, save to a file (JSON or CSV)
import json
with open('ntu_dining_details.json', 'w') as f:
    json.dump(details_list, f, indent=4)