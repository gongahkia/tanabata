# `napkin.md`

## FUA

* Add the logic for the scrapers with beautifulsoup4, scrapy and google places api
* Work out how to integrate google's / the browser's geolocation API to determine user location
* Integrate scrapers with the existing backend code
* Better understand what the backend Django code is doing 
* Write the frontend and integrate it properly with the backend
* Deploy with either Heroku or Vercel

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