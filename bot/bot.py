import json
import os
from dotenv import load_dotenv
from telegram.constants import ParseMode
from telegram.ext import MessageHandler, filters
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
        keyboard = [
            [InlineKeyboardButton("Food near me 🍡", callback_data="find_nearby_food")],
            [
                InlineKeyboardButton(
                    "Spin the wheel 🎰", callback_data="find_random_food"
                )
            ],
            [InlineKeyboardButton("Settings ⚙️", callback_data="settings")],
        ]
        inline_reply_markup = InlineKeyboardMarkup(keyboard)

        context.user_data["location"] = (lat, lon)

        await update.message.reply_text(
            f"Location received! 😺\nLatitude: {lat}, Longitude: {lon}",
        )
        await update.message.reply_text(
            f"Poke one button to get started 🤏🏼", reply_markup=inline_reply_markup
        )
    else:
        await update.message.reply_text(
            "Location data not received.😞\nPlease try again."
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
            "No nearby food places found within walking distance."
        )

    return nearby_places


async def find_random_food(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """
    FUA

    add logic here
    """
    await update.callback_query.answer()
    await update.callback_query.edit_message_text("Spinning the wheel... 🎡")


async def settings(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """
    FUA

    add logic here
    """
    await update.callback_query.answer()
    await update.callback_query.edit_message_text(
        "Settings menu is under construction ⚙️"
    )


def main():
    app = ApplicationBuilder().token(read_token_env()).build()
    app.add_handler(CommandHandler("start", start))
    app.add_handler(MessageHandler(filters.LOCATION, handle_location))
    app.add_handler(CallbackQueryHandler(find_nearby_food, pattern="find_nearby_food"))
    app.add_handler(CallbackQueryHandler(find_random_food, pattern="find_random_food"))
    app.add_handler(CallbackQueryHandler(settings, pattern="settings"))
    app.add_handler(CallbackQueryHandler(handle_time_selection, pattern="^time_"))

    print("bot is polling...")
    app.run_polling()


if __name__ == "__main__":
    main()
