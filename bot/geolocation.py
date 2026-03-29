import math


def haversine(lat1: float, lon1: float, lat2: float, lon2: float) -> float:
    """Return the great-circle distance between two points in kilometres."""

    lat1, lon1, lat2, lon2 = map(math.radians, (lat1, lon1, lat2, lon2))
    delta_lon = lon2 - lon1
    delta_lat = lat2 - lat1
    a = (
        math.sin(delta_lat / 2) ** 2
        + math.cos(lat1) * math.cos(lat2) * math.sin(delta_lon / 2) ** 2
    )
    return 6371.0 * (2 * math.asin(math.sqrt(a)))


def calculate_threshold_km(time_minutes: int, pace_kmh: float) -> float:
    if time_minutes <= 0:
        raise ValueError("Walking time must be positive.")
    if pace_kmh <= 0:
        raise ValueError("Walking pace must be positive.")
    return pace_kmh * (time_minutes / 60.0)


def travel_time_minutes(distance_km: float, pace_kmh: float) -> float:
    if pace_kmh <= 0:
        raise ValueError("Walking pace must be positive.")
    return (distance_km / pace_kmh) * 60.0


def locations_near(
    lat1: float,
    lon1: float,
    lat2: float,
    lon2: float,
    time_minutes: int,
    pace_kmh: float,
) -> tuple[bool, float]:
    distance_km = haversine(lat1, lon1, lat2, lon2)
    return (
        distance_km <= calculate_threshold_km(time_minutes, pace_kmh),
        travel_time_minutes(distance_km, pace_kmh),
    )

