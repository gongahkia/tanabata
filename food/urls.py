from django.urls import path, include
from rest_framework.routers import DefaultRouter
from .views import FoodPlaceViewSet, UserPreferenceViewSet, index

router = DefaultRouter()
router.register(r'food-places', FoodPlaceViewSet)
router.register(r'user-preferences', UserPreferenceViewSet)

urlpatterns = [
    path('', index, name='index'),
    path('', include(router.urls)),
]