# handles application logic 
# bridges models (data) and templates (presentation)

from rest_framework import viewsets
from django.shortcuts import render
from .models import FoodPlace, UserPreference
from .serializers import FoodPlaceSerializer, UserPreferenceSerializer
from .scrapers.active_scrapers.ntu_scraper import scrape_ntu 

def scrape_and_save_food_places(request):
    base_url = "https://www.ntu.edu.sg/life-at-ntu/leisure-and-dining/general-directory?locationTypes=all&locationCategories=all&page="
    details_list, errors = scrape_ntu(base_url) 
    for item in details_list:
        FoodPlace.objects.create(
            name=item['name'],
            location=item['location'],
            description=item['description'],
            category=item.get('category', ''),
            price=None
        )
    if errors:
        print(f"Errors encountered: {errors}")
    return render(request, 'food/index.html', {'food_places': FoodPlace.objects.all()})

def index_view(request):
    scrape_and_save_food_places(request)
    return render(request, 'food/index.html', {'food_places': FoodPlace.objects.all()})

class FoodPlaceViewSet(viewsets.ModelViewSet):
    queryset = FoodPlace.objects.all()
    serializer_class = FoodPlaceSerializer

class UserPreferenceViewSet(viewsets.ModelViewSet):
    queryset = UserPreference.objects.all()
    serializer_class = UserPreferenceSerializer