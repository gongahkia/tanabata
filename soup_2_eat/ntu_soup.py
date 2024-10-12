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

import requests
from bs4 import BeautifulSoup

base_url = "https://www.ntu.edu.sg/life-at-ntu/leisure-and-dining/general-directory?locationTypes=all&locationCategories=all&page="
page = 1
details_list = []

while True:
    # Request the page
    url = f"{base_url}{page}"
    response = requests.get(url)
    
    if response.status_code != 200:
        print(f"Failed to retrieve page {page}. Status code: {response.status_code}")
        break

    with open(f'ntu_page_{page}.html', 'w', encoding='utf-8') as file:
        file.write(response.text)

    soup = BeautifulSoup(response.content, 'html.parser')
    print(soup)

    listings = soup.select('div.col-sm-8.col-md-9 li.search__results-item.col-sm-6.col-md-4')
    
    if not listings:
        print("No more listings found.")
        break

    for listing in listings:
        name = listing.select_one('div.img-card__body h3.img-card__title').get_text(strip=True)
        location = listing.select_one('span.img-card__label.location-label span.location').get_text(strip=True)
        description = listing.select_one('p.img-card__info').get_text(strip=True)
        url = listing.select_one('a.link.link--icon')['href']
        
        # Prepare the details dictionary
        details = {
            'name': name,
            'location': location,
            'description': description,
            'category': '',  # Assuming category is not available
            'url': url
        }
        
        details_list.append(details)

    next_page = soup.select_one('div.search__results-footer li.active a')
    if next_page and 'href' in next_page.attrs:
        page += 1
    else:
        break

for detail in details_list:
    print(detail)

import json
with open('ntu_dining_details.json', 'w') as f:
    json.dump(details_list, f, indent=4)