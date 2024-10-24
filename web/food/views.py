from rest_framework import viewsets
from django.shortcuts import render
from django.http import JsonResponse
from .models import FoodPlace, UserPreference
from .serializers import FoodPlaceSerializer, UserPreferenceSerializer
from .scrapers.active_scrapers.ntu_scraper import scrape_ntu
from .scrapers.active_scrapers.smu_scraper import scrape_smu  # Import the SMU scraper
from asgiref.sync import sync_to_async

async def index_view(request):
    food_places = await sync_to_async(list)(FoodPlace.objects.all())
    return render(request, 'food/index.html', {'food_places': food_places})

async def scrape_food_places(request):
    print("Executing scrape_food_places...")
    
    # # NTU Scraping Logic
    # ntu_base_url = "https://www.ntu.edu.sg/life-at-ntu/leisure-and-dining/general-directory?locationTypes=all&locationCategories=all&page="
    # ntu_details_list, ntu_errors = await scrape_ntu(ntu_base_url)
    
    # # Process NTU data
    # for item in ntu_details_list:
    #     await sync_to_async(FoodPlace.objects.update_or_create)(
    #         name=item['name'],
    #         defaults={
    #             'location': item['location'],
    #             'description': item['description'],
    #             'category': item.get('category', ''),
    #             'price': None
    #         }
    #     )
    
    # SMU Scraping Logic
    smu_base_url = "https://www.smu.edu.sg/campus-life/visiting-smu/food-beverages-listing"
    smu_details_list, smu_errors = await scrape_smu(smu_base_url)
    
    # Process SMU data
    for item in smu_details_list:
        await sync_to_async(FoodPlace.objects.update_or_create)(
            name=item['name'],
            defaults={
                'location': item['location'],
                'description': item['description'],
                'category': item.get('category', ''),
                'price': None
            }
        )

    # Handle errors
    all_errors = smu_errors
    if all_errors:
        print(f"Errors encountered: {all_errors}")
    
    # print(f"NTU details list has {len(ntu_details_list)} items")
    print(f"SMU details list has {len(smu_details_list)} items")
    print("scrape_food_places finished execution!")

    # Get all food places and render to the template
    food_places = await sync_to_async(list)(FoodPlace.objects.all())
    return render(request, 'food/index.html', {'food_places': food_places})

class FoodPlaceViewSet(viewsets.ModelViewSet):
    queryset = FoodPlace.objects.all()
    serializer_class = FoodPlaceSerializer

class UserPreferenceViewSet(viewsets.ModelViewSet):
    queryset = UserPreference.objects.all()
    serializer_class = UserPreferenceSerializer
