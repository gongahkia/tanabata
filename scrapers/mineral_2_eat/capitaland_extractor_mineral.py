"""
FUA

debug this to check if it actually runs
and work out why it isn't running if not
"""

import os
from selenium import webdriver
from selenium.webdriver.chrome.service import Service as ChromeService
from selenium.webdriver.common.by import By
from selenium.webdriver.chrome.options import Options
from webdriver_manager.chrome import ChromeDriverManager
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
            "https://www.capitaland.com/sg/malls/westgate/en/stores.html?category=foodandbeverage",
        ]
        chrome_options = Options()
        chrome_options.add_argument("--headless")
        chrome_options.add_argument("--no-sandbox")
        chrome_options.add_argument("--disable-dev-shm-usage")
        self.driver = webdriver.Chrome(
            service=ChromeService(ChromeDriverManager().install()),
            options=chrome_options,
        )

    def sanitize_url(self, url):
        """
        sanitize the given URL to
        extract the mall name
        """
        try:
            parts = url.split("/")
            mall_name = parts[5]
            return mall_name
        except IndexError:
            print(f"Invalid URL format found with the URL: {url}")
            return None

    def fetch_page(self, url):
        """
        fetch the page and
        return a BeautifulSoup object
        """
        self.driver.get(url)
        self.driver.implicitly_wait(10)

        html = self.driver.page_source
        return BeautifulSoup(html, "html.parser")

    def extract_store_links(self):
        """
        extract store links from the main
        listing pages, returning a dictionary
        with URLs as keys and lists of store
        links as values
        """
        store_links = {}
        for url in self.start_urls:
            print(f"Extracting from: {url}")
            soup = self.fetch_page(url)
            if soup:
                listings = soup.select(
                    "div.listing-container ul.listing-items article.listing-item.listing-tenants a"
                )
                links = [
                    self.base_url + listing.get("href")
                    for listing in listings
                    if listing.get("href")
                ]
                sanitized_url = self.sanitize_url(url)
                store_links[sanitized_url] = links
        return store_links

    def close(self):
        """
        close the Selenium WebDriver
        """
        self.driver.quit()


# ----- execution code -----
# !NOTE
# code here is just for testing individual functionality of capitaland_extractor_mineral.py
# actual code is to be run from some other file

if __name__ == "__main__":
    extractor = CapitalandExtractor()
    store_links = extractor.extract_store_links()
    print(store_links)
    extractor.close()
