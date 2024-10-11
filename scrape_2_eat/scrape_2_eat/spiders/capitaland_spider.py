import scrapy


class CapitalandSpiderSpider(scrapy.Spider):
    name = "capitaland_spider"
    allowed_domains = ["placeholder.com"]
    start_urls = ["https://placeholder.com"]

    def parse(self, response):
        pass
