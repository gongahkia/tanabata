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
from requests_html import HTMLSession

class CapitalandExtractor:

    def __init__(self):
        self.base_url = "https://www.capitaland.com"
        self.start_urls = [
            "https://www.capitaland.com/sg/malls/plazasingapura/en/stores.html?category=foodandbeverage",
            "https://www.capitaland.com/sg/malls/aperia/en/stores.html?category=foodandbeverage",
            "https://www.capitaland.com/sg/malls/bedokmall/en/stores.html?category=foodandbeverage",
            "https://www.capitaland.com/sg/malls/bugisjunction/en/stores.html?category=foodandbeverage",
            "https://www.capitaland.com/sg/malls/bugisplus/en/stores.html?category=foodandbeverage",
            "https://www.capitaland.com/sg/malls/bugis-street/en/stores.html?category=foodandbeverage",
            "https://www.capitaland.com/sg/malls/bukitpanjangplaza/en/stores.html?category=foodandbeverage",
            "https://www.capitaland.com/sg/malls/clarkequay/en/stores.html?category=foodandbeverage",
            "https://www.capitaland.com/sg/malls/funan/en/stores.html?category=foodandbeverage",
            "https://www.capitaland.com/sg/malls/imm/en/stores.html?category=foodandbeverage",
            "https://www.capitaland.com/sg/malls/junction8/en/stores.html?category=foodandbeverage",
            "https://www.capitaland.com/sg/malls/lotone/en/stores.html?category=foodandbeverage",
            "https://www.capitaland.com/sg/malls/rafflescity/en/stores.html?category=foodandbeverage",
            "https://www.capitaland.com/sg/malls/tampinesmall/en/stores.html?category=foodandbeverage",
            "https://www.capitaland.com/sg/malls/westgate/en/stores.html?category=foodandbeverage"
        ]
        self.session = HTMLSession() 

    def sanitize_url(self, url):
        """
        sanitize the given URL
        to extract the mall name
        """
        try:
            parts = url.split('/')
            mall_name = parts[5] 
            return mall_name
        except IndexError:
            print(f"Invalid URL format found with the URL: {url}")
            return None

    def fetch_page(self, url):
        """
        fetch the page and return a 
        BeautifulSoup object
        """
        response = self.session.get(url) 
        if response.status_code == 200:
            html_file_path = "capitaland_links.html"
            with open(html_file_path, 'w', encoding='utf-8') as html_file:
                html_file.write(response.html.html)
            print(f"Raw HTML written to file: {html_file_path}")
            return BeautifulSoup(response.html.html, 'html.parser') 
        else:
            print(f"Failed to retrieve page {url} with status code: {response.status_code}")
            return None

    def extract_store_links(self):
        """
        extract store links from the 
        main listing pages, returning
        a dictionary with URLs as keys 
        and lists of store links as values
        """
        store_links = {}
        for url in self.start_urls:
            print(f"Extracting from: {url}")
            soup = self.fetch_page(url)
            if soup:
                listings = soup.select('div.listing-container ul.listing-items article.listing-item.listing-tenants a')
                links = [self.base_url + listing.get('href') for listing in listings if listing.get('href')]
                sanitized_url = self.sanitize_url(url)
                store_links[sanitized_url] = links
        return store_links

# ----- execution code -----
    # !NOTE
    # code here is just for testing individual functionality of capitaland_extractor_soup.py
    # actual code is to be run from some other file

if __name__ == "__main__":
    extractor = CapitalandExtractor()
    store_links = extractor.extract_store_links()
    print(store_links)