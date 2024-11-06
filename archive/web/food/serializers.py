from rest_framework import serializers
from .models import FoodPlace, UserPreference


class FoodPlaceSerializer(serializers.ModelSerializer):
    class Meta:
        model = FoodPlace
        fields = "__all__"


class UserPreferenceSerializer(serializers.ModelSerializer):
    class Meta:
        model = UserPreference
        fields = "__all__"
