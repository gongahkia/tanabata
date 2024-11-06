from rest_framework import viewsets
from django.shortcuts import render
from django.http import JsonResponse
from .models import FoodPlace, UserPreference
from .serializers import FoodPlaceSerializer, UserPreferenceSerializer
from .scrapers.active_scrapers.nus_scraper import (
    fetch_nus_dining_data,
)  # Import the NUS scraper


def index_view(request):
    food_places = list(FoodPlace.objects.all())  # Directly fetch all food places
    return render(request, "food/index.html", {"food_places": food_places})


def scrape_food_places(request):
    print("Executing scrape_food_places...")

    # NUS Scraping Logic
    urls = [
        "https://uci.nus.edu.sg/oca/retail-dining/food-and-beverages/",
        "https://uci.nus.edu.sg/oca/retail-dining/food-and-beverage-utown/",
        "https://uci.nus.edu.sg/oca/retail-dining/food-and-beverages-bukit-timah/",
    ]

    all_locations = {}
    all_details = {}

    for url in urls:
        details_list, errors = fetch_nus_dining_data(url)
        all_locations[url] = details_list
        if errors:
            print(f"Errors encountered while scraping {url}: {errors}")

    all_details["nus"] = all_locations

    # Process and store the scraped data
    for url, details in all_locations.items():
        for item in details:
            FoodPlace.objects.update_or_create(
                name=item["name"],
                defaults={
                    "location": item["location"],
                    "description": item["description"],
                    "category": item.get("category", ""),
                    "price": None,
                },
            )

    print(
        f"NUS details list has {sum(len(loc) for loc in all_locations.values())} items"
    )
    print("scrape_food_places finished execution!")

    # Get all food places and render to the template
    food_places = list(FoodPlace.objects.all())  # Directly fetch all food places
    return render(request, "food/index.html", {"food_places": food_places})


class FoodPlaceViewSet(viewsets.ModelViewSet):
    queryset = FoodPlace.objects.all()
    serializer_class = FoodPlaceSerializer


class UserPreferenceViewSet(viewsets.ModelViewSet):
    queryset = UserPreference.objects.all()
    serializer_class = UserPreferenceSerializer
