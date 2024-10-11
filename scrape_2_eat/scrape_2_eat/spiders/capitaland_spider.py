"""
FUA

test the scraping code below first!

then after ensuring data can be scraped, 
link the below scraped data to the existing
postgresql database via django orm

~~~ CAPITALAND.COM DOMAIN DOM STRUCTURE ~~~

div.listing-container
ul.listing-items
for item in ul.listing-items:
    item = article.listing-item.listing-tenants
    a.href -> click into each a.href
        div.cm-details-section-head
        div.cm-details-section-content
            div.cm --> all text content
            div.rte --> all text content
        section.cm.cm-property-details
            dd.icon-marker.icon-rounded --> all text content
    div.listing-image.intrinsic-6x5 -> get the style attribute
    div.content
        div.cg
        h3.title
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
        general execution function that
        engages parsing and loading of 
        CapitaLand webpage

        also note that each_listing below 
        can also be extracted as 
        article.listing-item.listing-tenants
        """
        # - extraction code -
        listings = response.css('div.listing-container ul.listing-items') 
        for each_listing in listings:
            detail_link = each_listing.css('a::attr(href)').get()
            if detail_link:
                yield response.follow(detail_link, self.parse_detail)
            else:
                print("no detail link found!")
        # - load more listings -
        next_page = response.css('div.listing-cta a.cta.cta-see-more.fn_see-more::attr(href)').get()
        if next_page:
            yield response.follow(next_page, self.parse)

    def parse_detail(self, response):
        """
        parse detailed information 
        from each listing
        """
        details = {
            'name': response.css('div.cm-details-section-head::text').get(default='').strip(),
            'location': response.css('section.cm.cm-property-details dd.icon-marker.icon-rounded::text').get(default='').strip(),
            'description': response.css('div.cm-details-section-content div.rte::text').get(default='').strip(),
            'category': response.css('div.cm-details-section-content div.cm::text').get(default='').strip(),
        }
        yield details