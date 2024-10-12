# define database models, which are postgresql tables, here
# after defining models, the following commands must be called
    # python3 manage.py makemigrations
    # python3 manage.py migrate

from django.db import models

class FoodPlace(models.Model):
    name = models.CharField(max_length=255)
    location = models.CharField(max_length=255)
    price = models.DecimalField(max_digits=6, decimal_places=2, null=True, blank=True) 
    category = models.CharField(max_length=100, null=True, blank=True) 
    description = models.TextField(max_length=255, null=True, blank=True) 
    url = models.TextField(max_length=255, null=True, blank=True) 
    
    def __str__(self):
        return self.name

class UserPreference(models.Model):
    user_id = models.IntegerField()
    location = models.CharField(max_length=255)
    preferred_category = models.CharField(max_length=100)