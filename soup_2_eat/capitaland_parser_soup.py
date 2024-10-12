""" 
FUA

there are some issues parsing this site
as similar to smu, it relies on incapsula to 
shield its site data from other sites

might have to look into using selenium
to scrape this site instead
"""

import requests
from bs4 import BeautifulSoup

class CapitalandParser:

    def __init__(self):
        self.base_url = "https://www.capitaland.com"

    def fetch_page(self, url):
        """
        fetch the page and return 
        a BeautifulSoup object
        """
        response = requests.get(url)
        if response.status_code == 200:
            return BeautifulSoup(response.text, 'html.parser')
        else:
            print(f"Failed to retrieve page {url}")
            return None

    def parse_detail(self, detail_link):
        """
        parse the detailed information 
        from a given store page
        """
        soup = self.fetch_page(detail_link)
        if soup:
            name = soup.select_one('div.cm-details-section-head').get_text(strip=True) if soup.select_one('div.cm-details-section-head') else ''
            location = soup.select_one('section.cm.cm-property-details dd.icon-marker.icon-rounded').get_text(strip=True) if soup.select_one('section.cm.cm-property-details dd.icon-marker.icon-rounded') else ''
            description = soup.select_one('div.cm-details-section-content div.rte').get_text(strip=True) if soup.select_one('div.cm-details-section-content div.rte') else ''
            category = soup.select_one('div.cm-details-section-content div.cm').get_text(strip=True) if soup.select_one('div.cm-details-section-content div.cm') else ''
            details = {
                'name': name,
                'location': location,
                'description': description,
                'category': category,
                'url': detail_link,
            }
            print(details)
        else:
            print(f"Failed to parse detail page: {detail_link}")

# ----- execution code -----
    # !NOTE
    # code here is just for testing individual functionality of capitaland_parser_soup.py
    # actual code is to be run from some other file

if __name__ == "__main__":
    parser = CapitalandParser()
    store_link = "https://www.capitaland.com/sg/malls/plazasingapura/en/stores/example-store.html"  
    parser.parse_detail(store_link)