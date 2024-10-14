import json
from playwright.sync_api import sync_playwright

def fetch_capitaland_store_details(detail_link):

    with sync_playwright() as p:

        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        print(f"Fetching details from: {detail_link}")
        page.goto(detail_link)
        page.wait_for_selector('div.l-padding')

        name = page.query_selector('div.cm-details-section-head h1').inner_text().strip() if page.query_selector('div.cm-details-section-head h1') else ''
        location = page.query_selector('section.cm.cm-property-details dd.icon-marker.icon-rounded').inner_text().strip() if page.query_selector('section.cm.cm-property-details dd.icon-marker.icon-rounded') else ''
        description = page.query_selector('div.cm-details-section-content div.rte').inner_text().strip() if page.query_selector('div.cm-details-section-content div.rte') else ''
        category = page.query_selector('div.cm-details-section-content div.cm').inner_text().strip() if page.query_selector('div.cm-details-section-content div.cm') else ''
        details = {
            'name': name,
            'location': location,
            'description': description,
            'category': category,
            'url': detail_link,
        }

        # print(details)

        browser.close()

    return details

def fetch_capitaland_data(start_urls):
    malls = {}
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        for url in start_urls:
            detail_array = []
            print(f"Extracting from: {url}")
            page = browser.new_page()
            page.goto(url)
            page.wait_for_selector('div.listing-container ul.listing-items article.listing-item.listing-tenants a')
            if page.title() == "":
                print(f"failed to retrieve page from URL: {url}")
                page.close()
                continue
            listings = page.query_selector_all('div.listing-container ul.listing-items article.listing-item.listing-tenants a')
            for listing in listings:
                url = listing.get_attribute("href")
                detail = fetch_capitaland_store_details(url)
                # print(detail)
                detail_array.append(detail)
            sanitized_url = url.split('/')[-3] 
            malls[sanitized_url] = detail_array
            page.close()
        browser.close()
    return malls

# ----- execution Code -----

if __name__ == "__main__":
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
    malls_array = fetch_capitaland_data(start_urls)
    output_file = "./../output/capitaland_store_links.json"
    with open(output_file, 'w') as f:
        json.dump(malls_array, f, indent=4)
    print(f"scraping completed, data written to {output_file}")