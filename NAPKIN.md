# `NAPKIN.md`

* implement the geolocation API to be called by the telegram bot
    * implement proper geolocation API checks that will scrape for food based on available sites nearby
* should i consider implement an asynchronous playwright script that runs and screenshots google maps overhead AND street view and appends it to the response produced by each food place suggested
    * can further consider the telegram bot will produce directions on how to get to a specific food place, with a possible button that appears if that location is available
* add a report an issue button that appears in main menu under settings, as well as when a specific food place isn't open when the user has manually gone there
* implement takko as a telegram bot
    * include further considerations native for telegram bot usage
    * make takko as easy to use as possible
* users should be able to specify user-specific food settings like "halal, vegetarian" as part of their configuration
* draw an emoji-like icon for tako on my ipad similar to the one i drew for sagusa, then make that the new bot profile picture through @botfather
* look into new features of telegram miniapps that telegram is capable of to make the bot more interactive 
    * telegram miniapps
        * https://docs.ton.org/develop/dapps/telegram-apps/
        * https://core.telegram.org/bots/webapps
        * https://blockchain.oodles.io/blog/telegram-mini-apps-vs-telegram-bots/
    * existing features of telegram bots
        * https://core.telegram.org/bots/features
* add updated mermaid diagram of how the telegram bot is structured to the README.md in the root folder
* implement proper LOCAL DEPLOYMENT via docker images so that developers also have instant access to the scrapers and the telegram bot, ask GPT for help with this as needed
* work out how to deploy the telegram bot with the following
    * https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/concepts.html as that's what is being used to deploy sagasu currently
    * heroku
    * railway
    * render
    * google cloud platform (gcp)
    * aws (amazon web services)
    * azure (microsoft azure)
    * vercel
    * pythonanywhere
    * digitalocean app platform
    * caprover
    * dokku
    * coolify
    * fly.io
    * kubernetes (k3s)
    * yunohost
    * openshift (okd)
* do I want to append the hardcoded postal code of the given mall / college just for now to specify how near it is to the user at any given moment?
    * integrate scrapers with the existing telegram code and work out a way to streamline the scrapers
* Debug for scraping
    * scraper within frasers_shakespeare.py doesn't handle clicking of the Load More button properly, it doesn't ensure load all is clicked fully and instead appears to load once before beginning extraction
    * scraper within frasers_shakespeare.py doesn't handle the generation of the repsective urls properly, need to ensure the relative link in combination with the absolute link works out
    * go back to debug capitaland_shakespeare.py to click the 'Load More' event when first obtaining each store's URL, consider using the A-Z boxes at the top of the page to obtain every possible URL instead
