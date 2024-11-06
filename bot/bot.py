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


async def find_nearby_food(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """
    FUA

    add logic here
    """
    nearby_places = []
    LOCATION_FILEPATH = "./locations.json"
    USER_TRAVEL_TIME_MINS = 10
    USER_SPEED = 5.0
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
        if coords[0] is None or coords[1] is None:
            continue
        else:
            locations_near_array = g.locations_near(
                lat, lon, coords[0], coords[1], USER_TRAVEL_TIME_MINS, USER_SPEED
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
    print(
        nearby_places
    )  # FUA change this line later to prepare a formatted response to be returned to the user
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

    print("bot is polling...")
    app.run_polling()


if __name__ == "__main__":
    main()
