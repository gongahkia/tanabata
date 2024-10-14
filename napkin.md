# `napkin.md`

## FUA

* focus on this for now
    * continue DEBUGGING EACH playwright file and check the specified HTML DOM STRUCTURE CALL is still correct! there is hope! 
        * for nus_shakespeare
        * for capitaland_shakespeare
    * convert existing scrapers in selenium to playwright and try to see if those work
    * try scraping with playwright, converting existing scraping code to playwright

* do I want to expose the API script to make `tako` an API wrapper that exposes food places in colleges and malls? if so add that in the README.md and learn how to write a wrapper for an API
    * API is available here http://127.0.0.1:8000/api/food-places/?format=json
    * reformat the final API for each location's food being scraped to be consistent, then define it in another file called API.md

* need to debug why the script.js and style.css in my ./food/static/food/ folder is not visible when i serve my index.html file

* Sites hardcoded to be scraped :robot:
    * https://www.jewelchangiairport.com/
    * https://www.sportshub.com.sg/shop-dine
    * https://www.sengkanggrandmall.com.sg/en.html
    * https://www.singpostcentre.com/
    * https://www.vivocity.com.sg/en/dining
    * https://www.marinabaysands.com/shopping.html
    * https://www.sunteccity.com.sg/dining
    * https://www.ngeeanncity.com.sg/shops-directory/category/food-beverage
    * https://www.paragon.com.sg/dining
    * https://www.greatworld.com.sg/dining
    * https://www.payalebarquarter.com
    * https://www.citysquaremall.com.sg
    * https://www.thestarvista.com
    * https://www.orchardcentral.com.sg/dining
    * https://www.thecentrepoint.com.sg/
    * https://www.cathay.com.sg
    * https://www.robertsonwalk.com.sg/
    * https://www.whitesands.com.sg/
    * https://www.northpointcity.com.sg
    * https://www.bedokpoint.com.sg
    * https://www.hillionmall.com.sg
    * https://www.compassone.sg
    * https://www.waterwaypoint.com.sg
    * https://www.centurysquare.com.sg
    * https://www.anchorpoint.com.sg
    * https://www.bishannorth.com.sg
    * https://www.bukitbatokhillside.com
    * https://www.changicitypoint.com.sg
    * https://www.citylinkmall.com.sg
    * https://www.compasspoint.com.sg
    * https://www.downtowneast.com.sg
    * https://www.jcube.com.sg
    * https://www.kallangwavemall.com
    * https://www.lotusatjoochiat.com
    * https://www.parkwayparade.com.sg
    * https://www.theseletarmall.com
    * https://www.theclementimall.com.sg
    * https://www.theglen.com.sg
    * https://www.causewaypoint.com.sg/
    * https://www.woodlandsciviccentre.com
    * https://www.hougangmall.com.sg/
    * https://www.thomsonplaza.com.sg
    * https://www.tiongbahruplaza.com.sg/
    * https://www.westmall.com.sg
    * https://www.amkhub.com.sg
    * https://www.brasbasahcomplex.com.sg
    * https://www.eastpoint.sg/
    * https://www.fareastplaza.com.sg
    * https://www.harbourfrontcentre.com
    * https://www.jurongpoint.com.sg
    * https://www.nex.com.sg
    * https://www.onekm.com.sg
    * https://www.payalebarquarter.com
    * https://www.plazasingapura.com
    * https://www.tampines1.com.sg
    * https://www.thomsonplaza.com.sg
    * https://www.yishun107.com.sg

* Look into alternatives to bs4 and scrapy to enact webscraping
    * lxml
    * puppeteer
    * playwright
    * mechanicalsoup
    * requests-html
    * xpath

* Currently I'm just running specified URLs to scrape to validate the scraper to django pipeling, but after that pipeline is validated, implement proper geolocation API checks that will scrape for food based on available sites nearby
* Alternative workflow would be to scrape these mall's data as backup data to begin with, then store them in the DB but more specific per-location requests can be scraped later
* Add the logic for the scrapers with beautifulsoup4, scrapy and google places api
* Work out how to integrate google's / the browser's geolocation API to determine user location
* Integrate scrapers with the existing backend code
* Better understand what the backend Django code is doing 
* Write the frontend and integrate it properly with the backend
* Deploy with Heroku for complete frontend and backend support

## Internal reference

### Backend

1. Django
    * Python web framework
    * server-side logic
    * user authentication
    * database interaction
2. Django REST framework (DRF)
    * create REST API for my frontend to interact with, fetch food data and user details
    * do note that Google's geolocation API and Google Places API already exist
3. PostgreSQL as database to work with Django
4. Scrapy
    * Python library for heavy web scraping
5. BeautifulSoup4
    * Python library for lightweight web scraping
6. Celery
    * handle scraping on a scheduled basis or when requested by the userq

### Frontend

* React
* Vue
* Svelte