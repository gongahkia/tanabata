from __future__ import annotations

import re
from pathlib import Path

try:
    from .models import AreaDefinition
except ImportError:  # pragma: no cover - script execution fallback
    from models import AreaDefinition


SCRAPER_DIR = Path(__file__).resolve().parent / "bot_scraper"


def _slugify(label: str) -> str:
    return re.sub(r"[^a-z0-9]+", "_", label.casefold()).strip("_")


RAW_AREAS = (
    {
        "label": "NTU Campus",
        "category": "campus",
        "coordinates": (1.3483, 103.6831),
        "scraper_module": "seed_only_scraper",
        "source_urls": ("seed://ntu-campus",),
        "seed_file": "ntu_dining_details.json",
    },
    {
        "label": "SMU Campus",
        "category": "campus",
        "coordinates": (1.2966, 103.8496),
        "scraper_module": "smu_shakespeare",
        "source_urls": ("https://www.smu.edu.sg/campus-life/visiting-smu/food-beverages-listing",),
        "seed_file": "smu_dining_details.json",
    },
    {
        "label": "NUS Campus",
        "category": "campus",
        "coordinates": (1.2966, 103.7764),
        "scraper_module": "nus_shakespeare",
        "source_urls": (
            "https://uci.nus.edu.sg/oca/retail-dining/food-and-beverages/",
            "https://uci.nus.edu.sg/oca/retail-dining/food-and-beverage-utown/",
            "https://uci.nus.edu.sg/oca/retail-dining/food-and-beverages-bukit-timah/",
        ),
        "seed_file": "nus_dining_details.json",
    },
    {
        "label": "SIM Campus",
        "category": "campus",
        "coordinates": (1.3294, 103.7762),
        "scraper_module": "sim_shakespeare",
        "source_urls": ("https://www.foodadvisor.com.sg/nearby/599491/",),
        "seed_file": "sim_dining_data.json",
    },
    {
        "label": "SIT Campus",
        "category": "campus",
        "coordinates": (1.3034, 103.7728),
        "scraper_module": "sit_shakespeare",
        "source_urls": ("https://www.foodadvisor.com.sg/nearby/138683/",),
        "seed_file": "sit_dining_data.json",
    },
    {
        "label": "SUSS Campus",
        "category": "campus",
        "coordinates": (1.3292, 103.7765),
        "scraper_module": "suss_shakespeare",
        "source_urls": ("https://www.foodadvisor.com.sg/nearby/599494/",),
        "seed_file": "suss_dining_data.json",
    },
    {
        "label": "Lasalle Campus",
        "category": "campus",
        "coordinates": (1.3008, 103.8496),
        "scraper_module": "laselle_shakespeare",
        "source_urls": ("https://www.foodadvisor.com.sg/nearby/187940/",),
        "seed_file": "laselle_dining_data.json",
    },
    {
        "label": "NAFA Campus",
        "category": "campus",
        "coordinates": (1.3, 103.8492),
        "scraper_module": "nafa_shakespeare",
        "source_urls": ("https://www.foodadvisor.com.sg/nearby/189655/",),
        "seed_file": "nafa_dining_data.json",
    },
    {
        "label": "NYP Campus",
        "category": "campus",
        "coordinates": (1.3795, 103.8492),
        "scraper_module": "nyp_shakespeare",
        "source_urls": ("https://www.foodadvisor.com.sg/nearby/569830/",),
        "seed_file": "nyp_dining_data.json",
    },
    {
        "label": "NP Campus",
        "category": "campus",
        "coordinates": (1.3326, 103.7749),
        "scraper_module": "np_shakespeare",
        "source_urls": ("https://www.foodadvisor.com.sg/nearby/599489/",),
        "seed_file": "np_dining_data.json",
    },
    {
        "label": "SP Campus",
        "category": "campus",
        "coordinates": (1.3092, 103.7775),
        "scraper_module": "sp_shakespeare",
        "source_urls": ("https://www.foodadvisor.com.sg/nearby/139651/",),
        "seed_file": "sp_dining_data.json",
    },
    {
        "label": "TP Campus",
        "category": "campus",
        "coordinates": (1.3464, 103.9326),
        "scraper_module": "tp_shakespeare",
        "source_urls": ("https://www.foodadvisor.com.sg/nearby/529757/",),
        "seed_file": "tp_dining_data.json",
    },
    {
        "label": "RP Campus",
        "category": "campus",
        "coordinates": (1.4422, 103.7854),
        "scraper_module": "rp_shakespeare",
        "source_urls": ("https://www.rp.edu.sg/our-campus/facilities/retail-dining",),
        "seed_file": "rp_dining_data.json",
    },
    {
        "label": "ION Orchard",
        "category": "mall",
        "coordinates": (1.304, 103.8318),
        "scraper_module": "ion_orchard_shakespeare",
        "source_urls": (
            "https://www.ionorchard.com/en/dine.html?category=Casual%20Dining%20and%20Takeaways",
            "https://www.ionorchard.com/en/dine.html?category=Restaurants%20and%20Cafes",
        ),
        "seed_file": "ion_orchard_dining_details.json",
    },
    {
        "label": "Jewel Changi Airport",
        "category": "mall",
        "coordinates": (1.3592, 103.9884),
        "scraper_module": "jewel_shakespeare",
        "source_urls": ("https://www.jewelchangiairport.com/en/dine.html",),
        "seed_file": "jewel_dining_details.json",
    },
    {
        "label": "SingPost Centre",
        "category": "mall",
        "coordinates": (1.3194, 103.8945),
        "scraper_module": "singpost_shakespeare",
        "source_urls": (
            "https://www.singpostcentre.com/stores?start_with=&s=&category=cafes-restaurants-food-court",
        ),
        "seed_file": "singpost_centre_dining_details.json",
    },
    {
        "label": "Suntec City",
        "category": "mall",
        "coordinates": (1.2931, 103.8572),
        "scraper_module": "suntec_shakespeare",
        "source_urls": ("https://www.sunteccity.com.sg/store_categories/dining",),
        "seed_file": "suntec_city_dining_details.json",
    },
    {
        "label": "Marina Bay Sands",
        "category": "mall",
        "coordinates": (1.2834, 103.8607),
        "scraper_module": "mbs_shakespeare",
        "source_urls": ("https://www.marinabaysands.com/restaurants/view-all.html",),
        "seed_file": "marina_bay_sands_restaurants.json",
    },
    {
        "label": "Paragon",
        "category": "mall",
        "coordinates": (1.3045, 103.8352),
        "scraper_module": "paragon_shakespeare",
        "source_urls": ("https://www.paragon.com.sg/stores/category/food-beverage",),
        "seed_file": "paragon_food_beverage_details.json",
    },
    {
        "label": "Great World",
        "category": "mall",
        "coordinates": (1.2938, 103.8315),
        "scraper_module": "great_world_shakespeare",
        "source_urls": ("https://shop.greatworld.com.sg/dine/",),
        "seed_file": "great_world_dining_details.json",
    },
    {
        "label": "Paya Lebar Quarter (PLQ)",
        "category": "mall",
        "coordinates": (1.3173, 103.8922),
        "scraper_module": "plq_shakespeare",
        "source_urls": ("https://www.payalebarquarter.com/directory/mall/?categories=Food+%26+Restaurant",),
        "seed_file": "paya_lebar_quarter_dining_details.json",
    },
    {
        "label": "City Square Mall",
        "category": "mall",
        "coordinates": (1.3111, 103.8563),
        "scraper_module": "city_square_mall_shakespeare",
        "source_urls": ("https://www.citysquaremall.com.sg/shops/food-beverage/",),
        "seed_file": "city_square_mall_dining_details.json",
    },
    {
        "label": "Compass One",
        "category": "mall",
        "coordinates": (1.395, 103.895),
        "scraper_module": "compass_one_shakespeare",
        "source_urls": ("https://compassone.sg/category/stores/restaurant-cafe-fast-food/",),
        "seed_file": "compass_one_dining_details.json",
    },
    {
        "label": "Thomson Plaza",
        "category": "mall",
        "coordinates": (1.3535, 103.8344),
        "scraper_module": "thomson_plaza_shakespeare",
        "source_urls": ("https://www.thomsonplaza.com.sg/store-directory/?keyword=&filter=5&payment_type=",),
        "seed_file": "thomson_plaza_details.json",
    },
    {
        "label": "Kinex",
        "category": "mall",
        "coordinates": (1.3086, 103.8973),
        "scraper_module": "kinex_shakespeare",
        "source_urls": ("https://www.kinex.com.sg/dining",),
        "seed_file": "kinex_dining_details.json",
    },
    {
        "label": "Changi City Point",
        "category": "mall",
        "coordinates": (1.3347, 103.9627),
        "scraper_module": "changi_citypoint_shakespeare",
        "source_urls": ("https://changicitypoint.com.sg/stores/page/1/?search&level&mall&cat=12&apply_filter",),
        "seed_file": "changi_city_point_dining_details.json",
    },
    {
        "label": "CityLink Mall",
        "category": "mall",
        "coordinates": (1.2929, 103.8547),
        "scraper_module": "citylink_shakespeare",
        "source_urls": ("https://citylink.com.sg/restaurants-cafes/",),
        "seed_file": "citylink_mall_dining_details.json",
    },
    {
        "label": "Parkway Parade",
        "category": "mall",
        "coordinates": (1.3029, 103.9055),
        "scraper_module": "parkway_parade_shakespeare",
        "source_urls": ("https://www.parkwayparade.com.sg/store-directory/?categories=Food+%26+Restaurant",),
        "seed_file": "parkway_parade_dining_details.json",
    },
    {
        "label": "The Seletar Mall",
        "category": "mall",
        "coordinates": (1.3966, 103.8703),
        "scraper_module": "seletar_mall_shakespeare",
        "source_urls": ("https://theseletarmall.com.sg/dine/",),
        "seed_file": "seletar_mall_dining_details.json",
    },
    {
        "label": "Bras Basah Complex",
        "category": "mall",
        "coordinates": (1.2966, 103.8518),
        "scraper_module": "bras_basah_complex_shakespeare",
        "source_urls": ("https://www.brasbasahcomplex.com/shops/?category_id=46",),
        "seed_file": "bras_basah_complex_shops.json",
    },
    {
        "label": "NEX",
        "category": "mall",
        "coordinates": (1.3502, 103.8722),
        "scraper_module": "nex_shakespeare",
        "source_urls": ("https://www.nex.com.sg/Directory/Category?EncDetail=qbjHWjcKv2GJGewRmzGQOA_3d_3d&CategoryName=Restaurant_Cafe%20_%20Fast%20Food&voucher=false&rewards=false",),
        "seed_file": "nex_dining_details.json",
    },
    {
        "label": "VivoCity",
        "category": "mall",
        "coordinates": (1.2645, 103.8224),
        "scraper_module": "vivo_shakespeare",
        "source_urls": ("https://www.vivocity.com.sg/shopping-guide/dining-guide",),
        "seed_file": "vivocity_dining_details.json",
    },
    {
        "label": "Causeway Point",
        "category": "mall",
        "coordinates": (1.4356, 103.7859),
        "scraper_module": "frasers_shakespeare",
        "source_urls": ("https://www.causewaypoint.com.sg/",),
        "seed_file": "frasers_store_links.json",
    },
    {
        "label": "Century Square",
        "category": "mall",
        "coordinates": (1.3531, 103.9436),
        "scraper_module": "frasers_shakespeare",
        "source_urls": ("https://www.centurysquare.com.sg/",),
        "seed_file": "frasers_store_links.json",
    },
    {
        "label": "Eastpoint Mall",
        "category": "mall",
        "coordinates": (1.3417, 103.953),
        "scraper_module": "frasers_shakespeare",
        "source_urls": ("https://www.eastpoint.sg/",),
        "seed_file": "frasers_store_links.json",
    },
    {
        "label": "Hougang Mall",
        "category": "mall",
        "coordinates": (1.3725, 103.8925),
        "scraper_module": "frasers_shakespeare",
        "source_urls": ("https://www.hougangmall.com.sg/",),
        "seed_file": "frasers_store_links.json",
    },
    {
        "label": "Northpoint City",
        "category": "mall",
        "coordinates": (1.4291, 103.8357),
        "scraper_module": "frasers_shakespeare",
        "source_urls": ("https://www.northpointcity.com.sg/",),
        "seed_file": "frasers_store_links.json",
    },
    {
        "label": "Robertson Walk",
        "category": "mall",
        "coordinates": (1.2927, 103.8414),
        "scraper_module": "frasers_shakespeare",
        "source_urls": ("https://www.robertsonwalk.com.sg/",),
        "seed_file": "frasers_store_links.json",
    },
    {
        "label": "Tampines 1",
        "category": "mall",
        "coordinates": (1.354, 103.9455),
        "scraper_module": "frasers_shakespeare",
        "source_urls": ("https://www.tampines1.com.sg/",),
        "seed_file": "frasers_store_links.json",
    },
    {
        "label": "The Centrepoint",
        "category": "mall",
        "coordinates": (1.301, 103.8398),
        "scraper_module": "frasers_shakespeare",
        "source_urls": ("https://www.thecentrepoint.com.sg/",),
        "seed_file": "frasers_store_links.json",
    },
    {
        "label": "Tiong Bahru Plaza",
        "category": "mall",
        "coordinates": (1.2853, 103.8238),
        "scraper_module": "frasers_shakespeare",
        "source_urls": ("https://www.tiongbahruplaza.com.sg/",),
        "seed_file": "frasers_store_links.json",
    },
    {
        "label": "Waterway Point",
        "category": "mall",
        "coordinates": (1.4060, 103.9023),
        "scraper_module": "frasers_shakespeare",
        "source_urls": ("https://www.waterwaypoint.com.sg/",),
        "seed_file": "frasers_store_links.json",
    },
    {
        "label": "White Sands",
        "category": "mall",
        "coordinates": (1.3721, 103.9522),
        "scraper_module": "frasers_shakespeare",
        "source_urls": ("https://www.whitesands.com.sg/",),
        "seed_file": "frasers_store_links.json",
    },
    {
        "label": "Plaza Singapura",
        "category": "mall",
        "coordinates": (1.2991, 103.8454),
        "scraper_module": "capitaland_shakespeare",
        "source_urls": ("https://www.capitaland.com/sg/malls/plazasingapura/en/stores.html?category=foodandbeverage",),
        "seed_file": "capitaland_store_links.json",
    },
    {
        "label": "Aperia",
        "category": "mall",
        "coordinates": (1.3102, 103.8631),
        "scraper_module": "capitaland_shakespeare",
        "source_urls": ("https://www.capitaland.com/sg/malls/aperia/en/stores.html?category=foodandbeverage",),
        "seed_file": "capitaland_store_links.json",
    },
    {
        "label": "Bedok Mall",
        "category": "mall",
        "coordinates": (1.3246, 103.9305),
        "scraper_module": "capitaland_shakespeare",
        "source_urls": ("https://www.capitaland.com/sg/malls/bedokmall/en/stores.html?category=foodandbeverage",),
        "seed_file": "capitaland_store_links.json",
    },
    {
        "label": "Bugis Junction",
        "category": "mall",
        "coordinates": (1.3006, 103.8554),
        "scraper_module": "capitaland_shakespeare",
        "source_urls": ("https://www.capitaland.com/sg/malls/bugisjunction/en/stores.html?category=foodandbeverage",),
        "seed_file": "capitaland_store_links.json",
    },
    {
        "label": "Bugis+",
        "category": "mall",
        "coordinates": (1.2994, 103.8556),
        "scraper_module": "capitaland_shakespeare",
        "source_urls": ("https://www.capitaland.com/sg/malls/bugisplus/en/stores.html?category=foodandbeverage",),
        "seed_file": "capitaland_store_links.json",
    },
    {
        "label": "Bukit Panjang Plaza",
        "category": "mall",
        "coordinates": (1.3773, 103.7639),
        "scraper_module": "capitaland_shakespeare",
        "source_urls": ("https://www.capitaland.com/sg/malls/bukitpanjangplaza/en/stores.html?category=foodandbeverage",),
        "seed_file": "capitaland_store_links.json",
    },
    {
        "label": "Funan",
        "category": "mall",
        "coordinates": (1.2938, 103.85),
        "scraper_module": "capitaland_shakespeare",
        "source_urls": ("https://www.capitaland.com/sg/malls/funan/en/stores.html?category=foodandbeverage",),
        "seed_file": "capitaland_store_links.json",
    },
    {
        "label": "IMM",
        "category": "mall",
        "coordinates": (1.3337, 103.7442),
        "scraper_module": "capitaland_shakespeare",
        "source_urls": ("https://www.capitaland.com/sg/malls/imm/en/stores.html?category=foodandbeverage",),
        "seed_file": "capitaland_store_links.json",
    },
    {
        "label": "Junction 8",
        "category": "mall",
        "coordinates": (1.3508, 103.8482),
        "scraper_module": "capitaland_shakespeare",
        "source_urls": ("https://www.capitaland.com/sg/malls/junction8/en/stores.html?category=foodandbeverage",),
        "seed_file": "capitaland_store_links.json",
    },
    {
        "label": "Lot One",
        "category": "mall",
        "coordinates": (1.3841, 103.7444),
        "scraper_module": "capitaland_shakespeare",
        "source_urls": ("https://www.capitaland.com/sg/malls/lotone/en/stores.html?category=foodandbeverage",),
        "seed_file": "capitaland_store_links.json",
    },
    {
        "label": "Raffles City",
        "category": "mall",
        "coordinates": (1.2934, 103.8523),
        "scraper_module": "capitaland_shakespeare",
        "source_urls": ("https://www.capitaland.com/sg/malls/rafflescity/en/stores.html?category=foodandbeverage",),
        "seed_file": "capitaland_store_links.json",
    },
    {
        "label": "Tampines Mall",
        "category": "mall",
        "coordinates": (1.3527, 103.944),
        "scraper_module": "capitaland_shakespeare",
        "source_urls": ("https://www.capitaland.com/sg/malls/tampinesmall/en/stores.html?category=foodandbeverage",),
        "seed_file": "capitaland_store_links.json",
    },
    {
        "label": "Westgate",
        "category": "mall",
        "coordinates": (1.3357, 103.7427),
        "scraper_module": "capitaland_shakespeare",
        "source_urls": ("https://www.capitaland.com/sg/malls/westgate/en/stores.html?category=foodandbeverage",),
        "seed_file": "capitaland_store_links.json",
    },
    {
        "label": "The Clementi Mall",
        "category": "mall",
        "coordinates": (1.3138, 103.7659),
        "scraper_module": "clementi_mall_shakespeare",
        "source_urls": ("https://www.theclementimall.com/stores",),
        "seed_file": "clementi_mall_dining_details.json",
    },
)


def load_area_registry() -> dict[str, AreaDefinition]:
    registry: dict[str, AreaDefinition] = {}
    for raw_area in RAW_AREAS:
        area_id = _slugify(raw_area["label"])
        registry[area_id] = AreaDefinition(
            id=area_id,
            label=raw_area["label"],
            category=raw_area["category"],
            latitude=raw_area["coordinates"][0],
            longitude=raw_area["coordinates"][1],
            scraper_module=raw_area["scraper_module"],
            source_urls=tuple(raw_area["source_urls"]),
            seed_file=raw_area.get("seed_file"),
        )
    return registry


def areas_by_category(registry: dict[str, AreaDefinition], category: str) -> list[AreaDefinition]:
    return sorted(
        (area for area in registry.values() if area.category == category),
        key=lambda area: area.label,
    )


def validate_area_registry(
    registry: dict[str, AreaDefinition], *, seed_dir: Path | None = None
) -> None:
    errors: list[str] = []
    seen_labels: set[str] = set()

    for area in registry.values():
        if area.id != _slugify(area.label):
            errors.append(f"{area.label}: invalid area id {area.id!r}.")
        if area.label in seen_labels:
            errors.append(f"{area.label}: duplicate label detected.")
        seen_labels.add(area.label)
        if area.category not in {"campus", "mall"}:
            errors.append(f"{area.label}: unsupported category {area.category!r}.")
        if not area.source_urls:
            errors.append(f"{area.label}: source_urls cannot be empty.")
        if not (SCRAPER_DIR / f"{area.scraper_module}.py").exists():
            errors.append(
                f"{area.label}: scraper module {area.scraper_module!r} does not exist."
            )
        if not (-90 <= area.latitude <= 90 and -180 <= area.longitude <= 180):
            errors.append(f"{area.label}: coordinates are invalid.")
        if seed_dir and area.seed_file and not (seed_dir / area.seed_file).exists():
            errors.append(f"{area.label}: seed file {area.seed_file!r} is missing.")

    if errors:
        raise RuntimeError("Area registry validation failed:\n- " + "\n- ".join(errors))
