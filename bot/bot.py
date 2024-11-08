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

# ~~~~~ CONSTANTS ~~~~~

ENTER_TIMING = 1
FOOD_OPTIONS = 2

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
        await update.message.reply_text(
            "Please enter your 2.4km timing in the format 'mm:ss' 🙏",
            reply_markup=ReplyKeyboardRemove(),
        )
        return ENTER_TIMING
    else:
        await update.message.reply_text(
            "Location data not received.😞\nPlease try again."
        )
        return ConversationHandler.END


async def save_timing(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    """
    Save the 2.4km timing entered by the user, calculate their speed,
    and then proceed to food options and spin the wheel
    """
    timing = update.message.text
    try:
        speed_kmh = calculate_user_speed(timing)
        context.user_data["2.4km_timing"] = timing
        context.user_data["speed_kmh"] = speed_kmh
        await update.message.reply_text(
            f"✅ Your 2.4km timing has been saved as {timing}.\n"
            f"🏃 Your estimated speed is {speed_kmh:.2f} km/h."
        )
        keyboard = [
            [InlineKeyboardButton("Food near me 🍡", callback_data="find_nearby_food")],
            [
                InlineKeyboardButton(
                    "Spin the wheel 🎰", callback_data="find_random_food"
                )
            ],
        ]
        inline_reply_markup = InlineKeyboardMarkup(keyboard)
        await update.message.reply_text(
            f"Poke one button to get started 🤏🏼", reply_markup=inline_reply_markup
        )
        return FOOD_OPTIONS
    except ValueError:
        await update.message.reply_text(
            "❌ Invalid timing format. Please enter the timing in the format 'mm:ss'."
        )
        return ENTER_TIMING


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

    continue adding logic here to allow the user to not have to constantly reshare location
    and they can just click find food again after inputting location the first time, also
    so that they can choose a diff walking time if they are willing to do so
    """
    nearby_places = []
    LOCATION_FILEPATH = "./locations.json"
    USER_TRAVEL_TIME_MINS = 10
    USER_SPEED = 5.0

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
                lat, lon, coords[0], coords[1], user_walking_time, USER_SPEED
            )
            nearby_places.append(
                {
                    "walkable": locations_near_array[0],
                    "foodplace_name": place,
                    "foodplace_latitude_longitude": coords,
                    "actual_travel_time": locations_near_array[1],
                    "user_speed": USER_SPEED,
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
        await update.callback_query.edit_message_text(
            f"🔍 <i><b><u>Nearby food places</u></b></i>\n\n{walkable_results}",
            parse_mode=ParseMode.HTML,
        )
    else:
        await update.callback_query.edit_message_text(
            "No nearby food places found within walking distance. 😭"
        )

    return nearby_places


async def find_random_food(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """
    FUA

    add logic here

    add additional malls and hubs to the specified json as well
    """

    nearby_places = []
    LOCATION_FILEPATH = "./locations.json"
    USER_TRAVEL_TIME_MINS = 10
    USER_SPEED = 5.0

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
                lat, lon, coords[0], coords[1], user_walking_time, USER_SPEED
            )
            nearby_places.append(
                {
                    "walkable": locations_near_array[0],
                    "foodplace_name": place,
                    "foodplace_latitude_longitude": coords,
                    "actual_travel_time": locations_near_array[1],
                    "user_speed": USER_SPEED,
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
    app.add_handler(CallbackQueryHandler(find_nearby_food, pattern="find_nearby_food"))
    app.add_handler(CallbackQueryHandler(find_random_food, pattern="find_random_food"))
    app.add_handler(CallbackQueryHandler(handle_time_selection, pattern="^time_"))
    print("Bot is polling...")
    app.run_polling()


if __name__ == "__main__":
    main()
