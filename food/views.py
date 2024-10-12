# handles application logic 
# bridges models (data) and templates (presentation)

from rest_framework import viewsets
from django.shortcuts import render
from django.http import JsonResponse
from .models import FoodPlace, UserPreference
from .serializers import FoodPlaceSerializer, UserPreferenceSerializer
from .scrapers.active_scrapers.ntu_scraper import scrape_ntu 

def index_view(request):
    food_places = FoodPlace.objects.all() 
    return render(request, 'food/index.html', {'food_places': food_places})

def scrape_food_places(request):
    print("executing scrape_food_places...")
    base_url = "https://www.ntu.edu.sg/life-at-ntu/leisure-and-dining/general-directory?locationTypes=all&locationCategories=all&page="
    details_list, errors = scrape_ntu(base_url) 
    for item in details_list:
        FoodPlace.objects.update_or_create(
            name=item['name'],
            defaults={
                'location': item['location'],
                'description': item['description'],
                'category': item.get('category', ''),
                'price': None,
                'url': item['url']
            }
        )
    updated_food_places = FoodPlace.objects.all().values()
    if errors:
        print(errors)
    print("scrape_food_places finished execution!")
    return JsonResponse(list(updated_food_places), safe=False)

class FoodPlaceViewSet(viewsets.ModelViewSet):
    queryset = FoodPlace.objects.all()
    serializer_class = FoodPlaceSerializer

class UserPreferenceViewSet(viewsets.ModelViewSet):
    queryset = UserPreference.objects.all()
    serializer_class = UserPreferenceSerializer