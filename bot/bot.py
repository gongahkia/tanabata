import json
import os
from dotenv import load_dotenv
from telegram.constants import ParseMode
from telegram.ext import MessageHandler, filters
from telegram import Update, InlineKeyboardButton, InlineKeyboardMarkup
from telegram.ext import ApplicationBuilder, CommandHandler, ContextTypes, CallbackQueryHandler

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
    keyboard = [
        [InlineKeyboardButton("Food near me 🍡", callback_data='find_nearby_food')],
        [InlineKeyboardButton("Spin the wheel 🎰", callback_data='find_random_food')],
        [InlineKeyboardButton("Settings ⚙️", callback_data='settings')]
    ]
    reply_markup = InlineKeyboardMarkup(keyboard)
    await update.message.reply_text("Poke one button to get started 🤏🏼", reply_markup=reply_markup)

async def find_nearby_food(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """
    FUA 
    
    add logic here
    """
    await update.callback_query.answer() 
    await update.callback_query.edit_message_text("Searching for food near you... 🍽️")

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
    await update.callback_query.edit_message_text("Settings menu is under construction ⚙️")

def main():
    app = ApplicationBuilder().token(read_token_env()).build()
    app.add_handler(CommandHandler("start", start))
    app.add_handler(CallbackQueryHandler(find_nearby_food, pattern='find_nearby_food'))
    app.add_handler(CallbackQueryHandler(find_random_food, pattern='find_random_food'))
    app.add_handler(CallbackQueryHandler(settings, pattern='settings'))
    print("bot is polling...")
    app.run_polling()

if __name__ == "__main__":
    main()