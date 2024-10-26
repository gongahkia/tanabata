# `NAPKIN.md`

## FUA

* migrate code to serve the other scraper information to the django webpage

* implement the geolocation API used in the browser

* do I want to expose the API script to make `tako` an API wrapper that exposes food places in colleges and malls? if so add that in the README.md and learn how to write a wrapper for an API
    * API is available here http://127.0.0.1:8000/api/food-places/?format=json
        * figure out a standard way for users to interact with this api instead
    * standardise the API wrapper if I decide to expose it for public use at any time
    * add further API specficiation for JSON structure in my napkin.md or readme.md

* Debug for serving the webpage
    * need to debug why the script.js and style.css in my ./food/static/food/ folder is not visible when i serve my index.html file

* Debug for scraping
    * scraper within frasers_shakespeare.py doesn't handle clicking of the Load More button properly, it doesn't ensure load all is clicked fully and instead appears to load once before beginning extraction
    * scraper within frasers_shakespeare.py doesn't handle the generation of the repsective urls properly, need to ensure the relative link in combination with the absolute link works out
    * go back to debug capitaland_shakespeare.py to click the 'Load More' event when first obtaining each store's URL, consider using the A-Z boxes at the top of the page to obtain every possible URL instead

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
7. Playwright
8. lxml
9. puppeteer
10. mechanicalsoup
11. requests-html
12. xpath

### Frontend

* React
* Vue
* Svelte
