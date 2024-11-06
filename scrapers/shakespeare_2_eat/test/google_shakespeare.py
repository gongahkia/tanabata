# !NOTE
# never to go into production
# just for testing the playwright library

from playwright.sync_api import sync_playwright

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True)
    page = browser.new_page()
    page.goto("https://www.google.com")
    print(page.title())
    browser.close()
