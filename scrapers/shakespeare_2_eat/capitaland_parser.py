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

        print(details)

        browser.close()

    return details

# ----- execution code -----

if __name__ == "__main__":
    store_link = "https://www.capitaland.com/sg/malls/tampinesmall/en/stores/four-leaves.html"
    fetch_capitaland_store_details(store_link)