from django.urls import path, include
from rest_framework.routers import DefaultRouter
from .views import index_view, FoodPlaceViewSet, UserPreferenceViewSet

router = DefaultRouter()
router.register(r'food-places', FoodPlaceViewSet)
router.register(r'user-preferences', UserPreferenceViewSet)

urlpatterns = [
    path('', index_view, name='index'),
    path('', include(router.urls)),
]