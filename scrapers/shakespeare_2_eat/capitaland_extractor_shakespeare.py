import json
from playwright.sync_api import sync_playwright

def fetch_capitaland_data():
    start_urls = [
        "https://www.capitaland.com/sg/malls/plazasingapura/en/stores.html?category=foodandbeverage",
        "https://www.capitaland.com/sg/malls/aperia/en/stores.html?category=foodandbeverage",
        "https://www.capitaland.com/sg/malls/bedokmall/en/stores.html?category=foodandbeverage",
        "https://www.capitaland.com/sg/malls/bugisjunction/en/stores.html?category=foodandbeverage",
        "https://www.capitaland.com/sg/malls/bugisplus/en/stores.html?category=foodandbeverage",
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

    store_links = {}

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        for url in start_urls:
            print(f"Extracting from: {url}")
            page = browser.new_page()
            page.goto(url)
            page.wait_for_selector('div.listing-container ul.listing-items article.listing-item.listing-tenants a')
            if page.title() == "":
                print(f"failed to retrieve page from URL: {url}")
                page.close()
                continue
            listings = page.query_selector_all('div.listing-container ul.listing-items article.listing-item.listing-tenants a')
            links = [listing.get_attribute('href') for listing in listings if listing.get_attribute('href')]
            sanitized_url = url.split('/')[-3] 
            store_links[sanitized_url] = links
            page.close()
        browser.close()
    return store_links

# ----- Execution Code -----

if __name__ == "__main__":
    store_links = fetch_capitaland_data()
    output_file = "./../output/capitaland_store_links.json"
    with open(output_file, 'w') as f:
        json.dump(store_links, f, indent=4)
    print(f"scraping completed, data written to {output_file}")