"""
~~~ FUA ~~~

there are some issues parsing this site
as similar to smu, it relies on incapsula to 
shield its site data from other sites

might have to look into using selenium
to scrape this site instead
"""

import scrapy
from scrapy_splash import SplashRequest

class SmuFoodSpider(scrapy.Spider):

    name = "smu_spider"
    allowed_domains = ["smu.edu.sg"]
    start_urls = ["https://www.smu.edu.sg/campus-life/visiting-smu/food-beverages-listing"]
    custom_settings = {
        'USER_AGENT': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/94.0.4606.61 Safari/537.36',
    }

    def start_requests(self):
        for url in self.start_urls:
            yield SplashRequest(url, self.parse, args={'wait': 3})

    def parse(self, response):
        self.logger.info("response URL: %s", response.url)
        self.logger.info("response body:\n%s", response.text)
        listings = response.css('div.col-md-9')
        results = [] 
        if listings:
            first_listing = listings[0]
            first_class = first_listing.attrib.get('class', '')
            first_id = first_listing.attrib.get('id', '')
            if "page-content--sidebar" in first_class and first_id == "page_content":
                listings = listings[1:] 

        for listing in listings:
            name = listing.css('h4.location-title::text').get(default='').strip()
            location = listing.css('div.location-address::text').get(default='').strip()
            description = listing.css('div.location-description::text').get(default='').strip()
            contact = listing.css('div.location-contact::text').get(default='').strip()
            hours = listing.css('div.location-hours::text').get(default='').strip()
            url = listing.css('div.location-address a::attr(href)').get(default='')
            details = {
                'name': name,
                'location': location,
                'description': f"{description} {contact} {hours}",
                'category': '', 
                'url': url,
            }

            results.append(details) 

        yield {'smu': results} 