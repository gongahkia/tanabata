# !NOTE
    # main execution code that is never meant to enter into production
    # to test all bs4 scrapers

import capitaland_soup as cs

cap_soup = cs.CapitalandSoup()
cap_soup.run()