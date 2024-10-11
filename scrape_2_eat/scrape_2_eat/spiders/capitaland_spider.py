"""
FUA

to edit the below code, most of it
probably isn't very accurate to the 
actual website's layouts

then after ensuring data can be scraped, 
link the below scraped data to the existing
postgresql database via django orm
"""

import scrapy

class CapitalandSpiderSpider(scrapy.Spider):
    name = "capitaland_spider"
    allowed_domains = [
        "capitaland.com"
    ]
    start_urls = [
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

    def parse(self, response):
        """
        FUA 
        
        edit the code placed below here
        """
        stores = response.css('.store-info')  # Adjust this selector based on the actual HTML structure

        for store in stores:
            name = store.css('.store-name::text').get().strip()  # Adjust according to the HTML structure
            location = store.css('.store-location::text').get().strip()  # Adjust according to the HTML structure
            price = store.css('.store-price::text').get().strip()  # Adjust according to the HTML structure
            category = 'Food and Beverage'  # Since you are only scraping this category

            yield {
                'name': name,
                'location': location,
                'price': price,
                'category': category,
            }