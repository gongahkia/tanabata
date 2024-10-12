import requests
from bs4 import BeautifulSoup

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

    def fetch_page(self, url):
        """
        fetch the page and return a 
        BeautifulSoup object
        """
        response = requests.get(url)
        if response.status_code == 200:
            return BeautifulSoup(response.text, 'html.parser')
        else:
            print(f"Failed to retrieve page {url}")
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
                store_links[url] = links
        return store_links

# ----- execution code -----
    # !NOTE
    # code here is just for testing individual functionality of capitaland_extractor_soup.py
    # actual code is to be run from some other file

if __name__ == "__main__":
    extractor = CapitalandExtractor()
    store_links = extractor.extract_store_links()
    print(store_links)