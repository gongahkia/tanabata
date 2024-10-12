import requests
from bs4 import BeautifulSoup

class CapitalandSoup:

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

    def parse_listings(self, soup):
        """
        parse the main listing page 
        and extract detail links
        """
        listings = soup.select('div.listing-container ul.listing-items article.listing-item.listing-tenants a')
        for listing in listings:
            detail_link = listing.get('href')
            if detail_link:
                full_link = self.base_url + detail_link
                self.parse_detail(full_link)
            else:
                print("No detail link found for listing!")

    def parse_detail(self, detail_link):
        """
        parse the detailed 
        information from the 
        store page
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

    def load_more_listings(self, soup):
        """
        check for a 'load more' 
        button and fetch the next 
        page if it exists
        """
        next_page = soup.select_one('div.listing-cta a.cta.cta-see-more.fn_see-more')
        if next_page and next_page.get('href'):
            next_page_link = self.base_url + next_page.get('href')
            next_soup = self.fetch_page(next_page_link)
            if next_soup:
                self.parse_listings(next_soup)
                self.load_more_listings(next_soup)
        else:
            print("No more pages found!")

    def run(self):
        """
        main execution loop for 
        scraping all the malls
        """
        for url in self.start_urls:
            print(f"Scraping: {url}")
            soup = self.fetch_page(url)
            if soup:
                self.parse_listings(soup)
                self.load_more_listings(soup)