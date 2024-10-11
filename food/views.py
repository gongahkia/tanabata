# handles application logic 
# bridges models (data) and templates (presentation)

from rest_framework import viewsets
from django.shortcuts import render
from .models import FoodPlace, UserPreference
from .serializers import FoodPlaceSerializer, UserPreferenceSerializer

class FoodPlaceViewSet(viewsets.ModelViewSet):
    queryset = FoodPlace.objects.all()
    serializer_class = FoodPlaceSerializer

class UserPreferenceViewSet(viewsets.ModelViewSet):
    queryset = UserPreference.objects.all()
    serializer_class = UserPreferenceSerializer

def index(request):
    return render(request, 'food/index.html')