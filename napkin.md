# `napkin.md`

## FUA

* add an architecture diagram of how everything works in mermaid code with icons and elements to the README.md once everything is done

* focus on this for now
    * try scraping with playwright, converting existing scraping code to playwright
    * go back to debug capitaland_shakespeare.py to click the 'Load More' event when first obtaining each store's URL, consider using the A-Z boxes at the top of the page to obtain every possible URL instead

* do I want to expose the API script to make `tako` an API wrapper that exposes food places in colleges and malls? if so add that in the README.md and learn how to write a wrapper for an API
    * API is available here http://127.0.0.1:8000/api/food-places/?format=json
    * reformat the final API for each location's food being scraped to be consistent, then define it in another file called API.md
    * standardise the API wrapper if I decide to expose it for public use at any time

* need to debug why the script.js and style.css in my ./food/static/food/ folder is not visible when i serve my index.html file

* include support for all other major unis in singapore including sutd, sim

* Sites which couldn't be scraped
    * https://www.sengkanggrandmall.com.sg/en/stores.html *(cannot click and toggle the drop-down menu)*
    * https://www.vivocity.com.sg/shopping-guide/dining-guide *(site taking an infinite amount of time to load)*
    * https://www.ngeeanncity.com.sg/shopdirectory/index.html *(can't be scraped for some odd reason, might be an issue with loading and should try an alternative to playwright)*
    * https://www.amkhub.com.sg/store-directory/?level=&cate=Food+%26+Beverage *(table is used here without tangible identifiers, consider using bs4 instead of playwright)*
    * https://www.jurongpoint.com.sg/store-directory/ *(similar to amkhub, table is used here without tangible identifiers, consider using bs4 instead of playwright)*
    * https://thestarvista.sg/stores *(must click the Restaraunt / Cafe option in the initial loading dropdown menu)*
    * https://www.cathay.com.sg *(currently undergoing renovations till end of 2024)*
    * https://www.orchardcentral.com.sg/dining *(requires clicking through an additional drop-down that specifies Restaraunts, Cafes & Desserts)*
    * https://www.hillionmall.com.sg/store-directory/ *(requires clicking a dynamic drop-down that leads to an active map before displaying any stores)*
    * https://www.anchorpoint.com.sg/shops *(there isn't any tangible html to scrape, its just uncontextualised hardcoded html content)*
    * https://www.sportshub.com.sg/shop-dine/stores?store_id=181&venue=All&payment_methods=All&combine=&custom_az_filter=character_asca *(despite everything seeming normal, unable to scrape the specified details that i want to scrape)*
    * https://www.theclementimall.com/stores *(requires clicking of a js-dynamically powered dropdown, need to iron out logic)*
    * https://www.westmall.com.sg/stores *(same issue as clementi mall)*
    * https://www.fareastplaza.com.sg/categories *(this page is just frozen, might need to try rescraping this in the future)*

* Debug for scraping
    * scraper within frasers_shakespeare.py doesn't handle clicking of the Load More button properly, it doesn't ensure load all is clicked fully and instead appears to load once before beginning extraction
    * scraper within frasers_shakespeare.py doesn't handle the generation of the repsective urls properly, need to ensure the relative link in combination with the absolute link works out

* Look into alternatives to bs4 and scrapy to enact webscraping
    * playwright
    * lxml
    * puppeteer
    * mechanicalsoup
    * requests-html
    * xpath

* Do I want to append the hardcoded postal code of the given mall / college just for now to specify how near it is to the user at any given moment?
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