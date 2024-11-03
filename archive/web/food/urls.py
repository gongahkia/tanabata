from django.urls import path, include
from .views import index_view, scrape_food_places, FoodPlaceViewSet, UserPreferenceViewSet
from rest_framework.routers import DefaultRouter

router = DefaultRouter()
router.register(r'food-places', FoodPlaceViewSet)
router.register(r'user-preferences', UserPreferenceViewSet)

urlpatterns = [
    path('', index_view, name='index'),
    path('scrape-food-places/', scrape_food_places, name='scrape-food-places'),
    path('api/', include(router.urls)),
]
