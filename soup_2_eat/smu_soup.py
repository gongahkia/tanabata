"""
~~~ FUA ~~~

debug the scraper here, its having 
trouble loading in the webpage due to 
the the nature of headless browsers

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

import requests
from bs4 import BeautifulSoup

print("soup start")

url = "https://www.smu.edu.sg/campus-life/visiting-smu/food-beverages-listing"

headers = {
    'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/93.0.4577.63 Safari/537.36',
    'Referer': 'https://www.google.com/'
}

response = requests.get(url, headers=headers)
if response.status_code != 200:
    print(f"failed to retrieve page url: {url}")
    print(f"status code: {response.status_code}")
    exit()
else:
    print(f"succesfully retrieved page url: {url}")
    soup = BeautifulSoup(response.text, 'html.parser')
    print(soup)
    locations = soup.select('div.col-md-9')
    for location in locations:
        name = location.select_one('h4.location-title').get_text(strip=True) if location.select_one('h4.location-title') else ''
        location_text = location.select_one('div.location-address').get_text(strip=True) if location.select_one('div.location-address') else ''
        location_url = location.select_one('div.location-address a')['href'] if location.select_one('div.location-address a') else ''
        description = location.select_one('div.location-description').get_text(strip=True) if location.select_one('div.location-description') else ''
        contact_info = location.select_one('div.location-contact').get_text(strip=True) if location.select_one('div.location-contact') else ''
        hours_info = location.select_one('div.location-hours').get_text(strip=True) if location.select_one('div.location-hours') else ''
        category = "Food and Beverage"
        details = {
            'name': name,
            'location': location_text,
            'description': f"{description} {contact_info} {hours_info}".strip(),
            'category': category,
            'url': location_url
        }
        print(details)

    print("soup end")