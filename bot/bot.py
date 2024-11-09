import os
import json
import random
from dotenv import load_dotenv
from telegram.constants import ParseMode
from telegram.ext import MessageHandler, filters, ContextTypes
from telegram import (
    Update,
    InlineKeyboardButton,
    InlineKeyboardMarkup,
    KeyboardButton,
    ReplyKeyboardMarkup,
    ReplyKeyboardRemove,
)
from telegram.ext import (
    ApplicationBuilder,
    CommandHandler,
    ContextTypes,
    CallbackQueryHandler,
    MessageHandler,
    filters,
    ContextTypes,
)
import geolocation as g
import schedule as s
from telegram import Update, ReplyKeyboardMarkup, ReplyKeyboardRemove
from telegram.ext import (
    ContextTypes,
    ConversationHandler,
    CommandHandler,
    MessageHandler,
    filters,
)
import json
import importlib

# ~~~~~ HELPER FUNCTIONS ~~~~~


def get_random_element(lst):
    """
    returns a random element from an array
    """
    if not lst:
        return None
    return random.choice(lst)


def float_to_minutes_seconds(time_float):
    """
    converts minutes to minutes and seconds
    """
    minutes = int(time_float)
    seconds = int((time_float - minutes) * 60)
    if seconds != 0:
        return f"{minutes} minutes {seconds} seconds"
    else:
        return f"{minutes} minutes"


def read_token_env():
    """
    read bot token from a .env file
    """
    load_dotenv()
    bot_token = os.getenv("BOT_TOKEN")
    if not bot_token:
        print("Bot token not found in the .env file")
        return None
    else:
        return bot_token


def calculate_user_speed(timing):
    """
    calculates walking speed of user based on their
    inputted timing for the 2.4km run
    """
    try:
        minutes, seconds = map(int, timing.split(":"))
    except ValueError:
        raise ValueError("Invalid timing format. Please enter the timing as 'mm:ss'.")
    total_seconds = minutes * 60 + seconds
    distance_km = 2.4
    speed_kmh = (distance_km / total_seconds) * 3600
    return round(speed_kmh, 2)


def specify_scraper_function(mall_name):
    BOT_DETAILS_FILEPATH = "bot_details.json"
    with open(BOT_DETAILS_FILEPATH, "r") as file:
        bot_details = json.load(file)
    if mall_name not in bot_details:
        print(
            f"Error hit, specified mall {mall_name} not found in file at {BOT_DETAILS_FILEPATH}"
        )
        return None
    else:
        scraper_name = f"bot_details[mall_name]['scraper_name'].py"
        site = bot_details[mall_name]["site"]
        print(scraper_name, site)
        try:
            # FUA will def need to debug the below portion later
            scraper_module = importlib.import_module(scraper_name)
            scraper_module.run_scraper(site)
            print(f"Successfully ran scraper for {mall_name} using {scraper_name}.")
        except ModuleNotFoundError:
            print(f"Scraper module '{scraper_name}' not found.")
        except AttributeError:
            print(
                f"Function 'run_scraper' not found in scraper module '{scraper_name}'."
            )
        except Exception as e:
            print(f"An error occurred while running the scraper: {e}")


# ~~~~~ BOT CODE ~~~~~


async def start(update: Update, context: ContextTypes.DEFAULT_TYPE):
    location_button = KeyboardButton("Share Location 📍", request_location=True)
    reply_markup = ReplyKeyboardMarkup(
        [[location_button]], one_time_keyboard=True, resize_keyboard=True
    )
    await update.message.reply_text(
        "Please share your location to get started 🙏", reply_markup=reply_markup
    )


async def handle_location(update: Update, context: ContextTypes.DEFAULT_TYPE):
    if update.message.location:
        user_location = update.message.location
        lat, lon = user_location.latitude, user_location.longitude
        context.user_data["location"] = (lat, lon)
        await update.message.reply_text(
            f"Location received! 😺\nLatitude: {lat}, Longitude: {lon}",
        )
        keyboard = [
            [InlineKeyboardButton("Less than 5 mins", callback_data="05:00")],
            [InlineKeyboardButton("Less than 10 mins", callback_data="10:00")],
            [InlineKeyboardButton("Less than 15 mins", callback_data="15:00")],
            [InlineKeyboardButton("Less than 20 mins", callback_data="20:00")],
            [InlineKeyboardButton("Less than 25 mins", callback_data="25:00")],
            [InlineKeyboardButton("More than 25 mins", callback_data="30:00")],
        ]
        reply_markup = InlineKeyboardMarkup(keyboard)
        await update.message.reply_text(
            "Please select your 2.4km run timing ⏱️", reply_markup=reply_markup
        )
    else:
        await update.message.reply_text(
            "Location data not received.😞\nPlease try again."
        )


async def handle_run_time_selection(update: Update, context: ContextTypes.DEFAULT_TYPE):
    selected_time = update.callback_query.data
    context.user_data["user_speed"] = calculate_user_speed(selected_time)
    await update.callback_query.answer()
    await update.callback_query.edit_message_text(
        f"Speed received! 😹\nEstimated speed: {context.user_data['user_speed']} km/h",
    )
    print(update.callback_query.data)
    await home_screen(update, context)


async def home_screen(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    """
    Save the 2.4km timing entered by the user, calculate their speed,
    and then proceed to food options and spin the wheel
    """
    keyboard = [
        [InlineKeyboardButton("Food near me 🍡", callback_data="find_nearby_food")],
        [InlineKeyboardButton("Spin the wheel 🎰", callback_data="find_random_food")],
    ]
    inline_reply_markup = InlineKeyboardMarkup(keyboard)
    await update.callback_query.message.reply_text(
        "Poke one button to get started 🤏🏼🍽️", reply_markup=inline_reply_markup
    )


async def ask_walking_time(update: Update, context: ContextTypes.DEFAULT_TYPE):
    keyboard = [
        [InlineKeyboardButton("5 mins", callback_data="time_5")],
        [InlineKeyboardButton("10 mins", callback_data="time_10")],
        [InlineKeyboardButton("15 mins", callback_data="time_15")],
        [InlineKeyboardButton("20 mins", callback_data="time_20")],
    ]
    reply_markup = InlineKeyboardMarkup(keyboard)
    await update.callback_query.answer()
    await update.callback_query.edit_message_text(
        "How long are you willing to walk? 🚶‍♂️", reply_markup=reply_markup
    )


async def handle_time_selection(update: Update, context: ContextTypes.DEFAULT_TYPE):
    time_selection = update.callback_query.data.split("_")[1]
    context.user_data["walking_time"] = int(time_selection)
    await find_nearby_food(update, context)


async def find_nearby_food(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """
    FUA

    integrate logic to randomly select a mall from the walkable_list, then randomly select a stall
    """
    nearby_places = []
    LOCATION_FILEPATH = "./locations.json"
    USER_TRAVEL_TIME_MINS = 10
    USER_SPEED = 5.0

    user_speed = context.user_data.get("user_speed")
    if user_speed is None:
        user_speed = USER_SPEED

    user_walking_time = context.user_data.get("walking_time")
    if user_walking_time is None:
        await ask_walking_time(update, context)
        return

    await update.callback_query.answer()
    await update.callback_query.edit_message_text("Searching for food near you... 🍽️")

    lat, lon = context.user_data.get("location", (None, None))
    if lat is None or lon is None:
        await update.callback_query.edit_message_text(
            "No location data found. Please send your location."
        )
        return

    with open(LOCATION_FILEPATH, "r") as file:
        locations = json.load(file)
    for place, coords in locations.items():
        # print(place)
        if coords[0] is None or coords[1] is None:
            continue
        else:
            locations_near_array = g.locations_near(
                lat, lon, coords[0], coords[1], user_walking_time, user_speed
            )
            nearby_places.append(
                {
                    "walkable": locations_near_array[0],
                    "foodplace_name": place,
                    "foodplace_latitude_longitude": coords,
                    "actual_travel_time": locations_near_array[1],
                    "user_speed": user_speed,
                    "haversine_distance": g.haversine(lat, lon, coords[0], coords[1]),
                }
            )
    if nearby_places:
        nearby_places_sorted = sorted(
            nearby_places, key=lambda place: place["actual_travel_time"]
        )
        walkable_results = "\n".join(
            [
                f"{place['foodplace_name']} - {place['haversine_distance']:.2f} km away, {place['actual_travel_time']:.1f} mins away"
                for place in nearby_places_sorted
                if place["walkable"]
            ]
        )
        not_walkable_results = "\n".join(
            [
                f"{place['foodplace_name']} - {place['haversine_distance']:.2f} km away, {place['actual_travel_time']:.1f} mins away"
                for place in nearby_places_sorted
                if not place["walkable"]
            ]
        )
        all_results = "\n".join(
            [
                f"{place['foodplace_name']} - {place['haversine_distance']:.2f} km away, {place['actual_travel_time']:.1f} mins away"
                for place in nearby_places_sorted
            ]
        )
    if walkable_results:
        await update.callback_query.edit_message_text(
            f"🔍 <i><b><u>Nearby food places</u></b></i>\n\n{walkable_results}",
            parse_mode=ParseMode.HTML,
        )
    else:

        await update.callback_query.edit_message_text(
            f"🔍 <i><b><u>Nearby food places</u></b></i>\n\nNone were found near you 😭",
            parse_mode=ParseMode.HTML,
        )

        await update.callback_query.message.reply_text(
            f"Here are food places that are outside your desired distance",
            parse_mode=ParseMode.HTML,
        )

        await update.callback_query.message.reply_text(
            f"🔍 <i><b><u>Further food places</u></b></i>\n\n{not_walkable_results}",
            parse_mode=ParseMode.HTML,
        )

    return nearby_places


async def find_random_food(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """
    FUA

    integrate the logic for actual botscraping for each of the specified malls
    """

    nearby_places = []
    LOCATION_FILEPATH = "./locations.json"
    USER_TRAVEL_TIME_MINS = 10
    USER_SPEED = 5.0

    user_speed = context.user_data.get("user_speed")
    if user_speed is None:
        user_speed = USER_SPEED

    user_walking_time = context.user_data.get("walking_time")
    if user_walking_time is None:
        await ask_walking_time(update, context)
        return

    await update.callback_query.answer()
    await update.callback_query.edit_message_text("Spinning the wheel... 🎡")

    lat, lon = context.user_data.get("location", (None, None))
    if lat is None or lon is None:
        await update.callback_query.edit_message_text(
            "No location data found. Please send your location."
        )
        return

    with open(LOCATION_FILEPATH, "r") as file:
        locations = json.load(file)
    for place, coords in locations.items():
        if coords[0] is None or coords[1] is None:
            continue
        else:
            locations_near_array = g.locations_near(
                lat, lon, coords[0], coords[1], user_walking_time, user_speed
            )
            nearby_places.append(
                {
                    "walkable": locations_near_array[0],
                    "foodplace_name": place,
                    "foodplace_latitude_longitude": coords,
                    "actual_travel_time": locations_near_array[1],
                    "user_speed": user_speed,
                    "haversine_distance": g.haversine(lat, lon, coords[0], coords[1]),
                }
            )
    if nearby_places:
        nearby_places_sorted = sorted(
            nearby_places, key=lambda place: place["actual_travel_time"]
        )
        target_place = get_random_element(
            [place for place in nearby_places_sorted if place["walkable"]]
        )
        target_place_string = f"{target_place['foodplace_name']} - {target_place['haversine_distance']:.2f} km away, {target_place['actual_travel_time']:.1f} mins away"

        await update.callback_query.edit_message_text(
            f"🍽️ <i><b><u>Go eat at...</u></b></i>\n\n{target_place_string}",
            parse_mode=ParseMode.HTML,
        )
    else:
        await update.callback_query.edit_message_text(
            "No nearby food places found within walking distance. 😭"
        )

    return nearby_places


def main():
    app = ApplicationBuilder().token(read_token_env()).build()
    app.add_handler(CommandHandler("start", start))
    app.add_handler(MessageHandler(filters.LOCATION, handle_location))
    app.add_handler(
        CallbackQueryHandler(handle_run_time_selection, pattern="^\\d{2}:\\d{2}$")
    )
    app.add_handler(CallbackQueryHandler(handle_time_selection, pattern="^time_"))
    app.add_handler(CallbackQueryHandler(find_nearby_food, pattern="find_nearby_food"))
    app.add_handler(CallbackQueryHandler(find_random_food, pattern="find_random_food"))
    app.add_handler(CallbackQueryHandler(home_screen, pattern="^home_screen$"))
    print("Bot is polling...")
    app.run_polling()


if __name__ == "__main__":
    main()
