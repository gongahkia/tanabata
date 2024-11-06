"""
FUA

debug this to check if it actually runs
and work out why it isn't running if not
"""

from selenium import webdriver
from selenium.webdriver.chrome.service import Service as ChromeService
from selenium.webdriver.chrome.options import Options
from webdriver_manager.chrome import ChromeDriverManager
from bs4 import BeautifulSoup


class CapitalandParser:

    def __init__(self):
        self.base_url = "https://www.capitaland.com"
        chrome_options = Options()
        chrome_options.add_argument("--headless")
        chrome_options.add_argument("--no-sandbox")
        chrome_options.add_argument("--disable-dev-shm-usage")
        self.driver = webdriver.Chrome(
            service=ChromeService(ChromeDriverManager().install()),
            options=chrome_options,
        )

    def fetch_page(self, url):
        """
        fetch the page and return
        a BeautifulSoup object
        """
        self.driver.get(url)
        self.driver.implicitly_wait(10)
        html = self.driver.page_source
        return BeautifulSoup(html, "html.parser")

    def parse_detail(self, detail_link):
        """
        parse the detailed information
        from a given store page
        """
        soup = self.fetch_page(detail_link)
        if soup:
            name = (
                soup.select_one("div.cm-details-section-head").get_text(strip=True)
                if soup.select_one("div.cm-details-section-head")
                else ""
            )
            location = (
                soup.select_one(
                    "section.cm.cm-property-details dd.icon-marker.icon-rounded"
                ).get_text(strip=True)
                if soup.select_one(
                    "section.cm.cm-property-details dd.icon-marker.icon-rounded"
                )
                else ""
            )
            description = (
                soup.select_one("div.cm-details-section-content div.rte").get_text(
                    strip=True
                )
                if soup.select_one("div.cm-details-section-content div.rte")
                else ""
            )
            category = (
                soup.select_one("div.cm-details-section-content div.cm").get_text(
                    strip=True
                )
                if soup.select_one("div.cm-details-section-content div.cm")
                else ""
            )
            details = {
                "name": name,
                "location": location,
                "description": description,
                "category": category,
                "url": detail_link,
            }
            print(details)
        else:
            print(f"Failed to parse detail page: {detail_link}")

    def close(self):
        """
        close the Selenium WebDriver
        """
        self.driver.quit()


# ----- execution code -----
# !NOTE
# code here is just for testing individual functionality of capitaland_parser_mineral.py
# actual code is to be run from some other file

if __name__ == "__main__":
    parser = CapitalandParser()
    store_link = "https://www.capitaland.com/sg/malls/plazasingapura/en/stores/example-store.html"
    parser.parse_detail(store_link)
    parser.close()
