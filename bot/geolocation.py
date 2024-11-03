import math
from geopy.geocoders import Nominatim
from geopy.exc import GeocoderTimedOut

# ~~~~~ HELPER FUNCTIONS ~~~~~

def haversine(lat1, lon1, lat2, lon2):
    """
    calculate the great-circle distance between two points 
    on the Earth (specified in decimal degrees)
    """
    lat1, lon1, lat2, lon2 = map(math.radians, [lat1, lon1, lat2, lon2])
    dlon = lon2 - lon1 
    dlat = lat2 - lat1 
    a = math.sin(dlat/2)**2 + math.cos(lat1) * math.cos(lat2) * math.sin(dlon/2)**2
    c = 2 * math.asin(math.sqrt(a)) 
    r = 6371  
    return c * r

def calculate_threshold_km(time_mins, user_speed_km_h):
    """
    calculates threshold km based on user-specified speed and time
    """
    return user_speed_km_h * (time_mins / 60.0)

def locations_travel_time(haversine_distance, user_speed_km_h):
    """
    calculates time in minutes to travel a given distance based on user-specified speed and distance
    """
    return (haversine_distance / user_speed_km_h) * 60

def locations_near(lat1, lon1, lat2, lon2, time_mins, user_speed_km_h=5.0):
    """
    determine if two locations are near or far based on latitude and longitude
    """
    distance = haversine(lat1, lon1, lat2, lon2)
    return [distance <= calculate_threshold_km(time_mins, user_speed_km_h), locations_travel_time(distance, user_speed_km_h)]

# ~~~~~ NOMINATIM CODE ~~~~~

def get_postal_code_from_coordinates(latitude, longitude):
    """
    get postal code from latitude and longitude using Nominatim
    """
    geolocator = Nominatim(user_agent="my_geocoder")  
    try:
        location = geolocator.reverse((latitude, longitude), exactly_one=True)
        if location and 'postalcode' in location.raw['address']:
            return location.raw['address']['postalcode']
        else:
            return "Postal code not found."
    except GeocoderTimedOut:
        return "Geocoding service timed out. Try again."

def get_coordinates_from_address(address):
    """
    get coordinates from an address using Nominatim
    """
    geolocator = Nominatim(user_agent="my_geocoder")  
    try:
        location = geolocator.geocode(address, exactly_one=True)
        if location:
            return (location.latitude, location.longitude)
        else:
            return None
    except GeocoderTimedOut:
        return "Geocoding service timed out. Try again."

# ~~~~~ SAMPLE EXECUTION CODE ~~~~~

if __name__ == "__main__":
    address_1 = "Plaza Singapura Mall, Singapore"
    address_2 = "Singapore Management University, Singapore"
    address_3 = "Woodlands Street 41, Singapore"
    coordinates_1 = get_coordinates_from_address(address_1)
    coordinates_2 = get_coordinates_from_address(address_2)
    coordinates_3 = get_coordinates_from_address(address_3)
    if coordinates_1 and coordinates_2 and coordinates_3:
        print(f"Coordinates for '{address_1}': {coordinates_1}")
        print(f"Coordinates for '{address_2}': {coordinates_2}")
        print(f"Coordinates for '{address_3}': {coordinates_3}")
        print(locations_near(coordinates_1[0], coordinates_1[1], coordinates_2[0], coordinates_2[1], 10))
        print(locations_near(coordinates_1[0], coordinates_1[1], coordinates_3[0], coordinates_3[1], 10))
        print(locations_near(coordinates_2[0], coordinates_2[1], coordinates_3[0], coordinates_3[1], 10))
    else:
        print("Address not found.")