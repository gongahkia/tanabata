# `2 eat`

A bunch of scrapers I implemented to scrape the following sites.

## Scrapy

Scrapy spiders currently crawl the following websites.

| Spider name | Domains scraped | Sites scraped | Status |
| :--- | :--- | :--- | :--- |
| `smu_spider` | https://www.smu.edu.sg/ | https://www.smu.edu.sg/campus-life/visiting-smu/food-beverages-listing | :x: |
| `capitaland_spider` | https://www.capitaland.com/sg/ | - https://www.capitaland.com/sg/malls/plazasingapura/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/aperia/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/bedokmall/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/bugisjunction/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/bugisplus/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/bugis-street/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/bukitpanjangplaza/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/clarkequay/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/funan/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/imm/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/junction8/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/lotone/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/rafflescity/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/tampinesmall/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/westgate/en/stores.html?category=foodandbeverage<br> | :x: |

## Usage

```console
$ scrapy crawl spider_name
$ scrapy crawl spider_name -o output.json
```

## BeautifulSoup4

BeautifulSoup4 scrapers currently scrape the following websites.

| BS4 scraper name | Domains scraped | Sites scraped | Status |
| :--- | :--- | :--- | :--- |
| `ntu_soup` | https://www.ntu.edu.sg/life-at-ntu/leisure-and-dining/general-directory | https://www.ntu.edu.sg/life-at-ntu/leisure-and-dining/general-directory?locationTypes=all&locationCategories=all&page=1 | :white_check_mark: |
| `smu_soup` | https://www.smu.edu.sg/ | https://www.smu.edu.sg/campus-life/visiting-smu/food-beverages-listing | :x: |
| `capitaland_extractor_soup`<br>`capitaland_parser_soup` | https://www.capitaland.com/sg/ | - https://www.capitaland.com/sg/malls/plazasingapura/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/aperia/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/bedokmall/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/bugisjunction/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/bugisplus/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/bugis-street/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/bukitpanjangplaza/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/clarkequay/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/funan/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/imm/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/junction8/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/lotone/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/rafflescity/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/tampinesmall/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/westgate/en/stores.html?category=foodandbeverage<br> | :x: |

## Selenium

Selenium scrapers currently scrape the following websites.

| Selenium scraper name | Domains scraped | Sites scraped | Status |
| :--- | :--- | :--- | :--- |
| `capitaland_extractor_mineral`<br>`capitaland_parser_mineral.py` | https://www.capitaland.com/sg/ | - https://www.capitaland.com/sg/malls/plazasingapura/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/aperia/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/bedokmall/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/bugisjunction/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/bugisplus/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/bugis-street/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/bukitpanjangplaza/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/clarkequay/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/funan/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/imm/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/junction8/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/lotone/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/rafflescity/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/tampinesmall/en/stores.html?category=foodandbeverage<br>- https://www.capitaland.com/sg/malls/westgate/en/stores.html?category=foodandbeverage<br> |  |
| `smu_mineral` | https://www.smu.edu.sg/ | https://www.smu.edu.sg/campus-life/visiting-smu/food-beverages-listing |  |