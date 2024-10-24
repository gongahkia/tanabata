# !NOTE
    # important to note that all scraper functions and corresponding database operations must be converted to be asynchronous via the sync_to_async library
    # this is as Django ORM doesn't support async queries

import os
import re
import django
from requests_html import AsyncHTMLSession
from bs4 import BeautifulSoup
from food.models import FoodPlace
from asgiref.sync import sync_to_async

os.environ.setdefault("DJANGO_SETTINGS_MODULE", "what_2_eat.settings") 
django.setup()

async def delete_file(target_url):
    """
    helper function that attempts 
    to delete a file at the specified 
    filepath (not used in this implementation)
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
    cleaned_string = re.sub(r'\n+', ' ', input_string)
    cleaned_string = re.sub(r'<[^>]+>', '', cleaned_string)
    return cleaned_string.strip()

async def scrape_ntu(base_url):
    """
    scrapes the specified NTU website 
    for the food vendor's name, location, 
    description, category, and url
    """
    session = AsyncHTMLSession()
    page = 1
    details_list = []
    errors = []
    while True:
        url = f"{base_url}{page}"
        print(f"Fetching URL: {url}")
        response = await session.get(url)
        await response.html.arender() 
        if response.status_code != 200:
            errors.append(f"Failed to retrieve page {page}: {response.status_code}")
            break
        soup = BeautifulSoup(response.html.html, 'html.parser')
        listings = soup.select('div.col-sm-8.col-md-9 li.search__results-item.col-sm-6.col-md-4')
        if not listings:
            errors.append("No more listings found.")
            break
        for listing in listings:
            name = listing.select_one('div.img-card__body h3.img-card__title').get_text(strip=True)
            raw_location = listing.select_one('span.img-card__label.location-label span.location').get_text(strip=True) if listing.select_one('span.img-card__label.location-label') else ''
            clean_location = clean_string(raw_location)
            raw_description = listing.select_one('p.img-card__info').get_text(strip=True) if listing.select_one('p.img-card__info') else ''
            clean_description = clean_string(raw_description)
            url = listing.select_one('a.link.link--icon')['href']
            details = {
                'name': name,
                'location': clean_location,
                'price': None,
                'description': clean_description,
                'category': '',  
                'url': url
            }
            details_list.append(details)
            await sync_to_async(FoodPlace.objects.update_or_create)(
                name=name,
                defaults={
                    'location': clean_location,
                    'price': None,
                    'category': '',
                    'description': clean_description,
                    'url': url
                }
            )
        next_page = soup.select_one('div.search__results-footer li.active a')
        if next_page and 'href' in next_page.attrs:
            page += 1
        else:
            break
    return details_list, errors