
<div align="center">
	<img width="128" src="assets/logo.png"/>
	<h1>telegram inline message limiter</h1>
</div>

[![Chat on Telegram](https://img.shields.io/badge/@inline_limiter_bot-2CA5E0.svg?logo=telegram&label=Telegram)](https://t.me/inline_limiter_bot)
![GitHub Repo stars](https://img.shields.io/github/stars/szres/tg-inline-limit-bot?style=flat&color=ffaaaa)
[![Software License](https://img.shields.io/github/license/szres/tg-inline-limit-bot)](LICENSE)
![Docker](https://img.shields.io/badge/Build_with-Docker-ffaaaa)

This bot can limit the number of inline messages sent by members in a group to avoid inline message storms.

## Usage

Just add this bot to the group and give it the permission to manage and delete messages.

## Setup

The default configuration of the bot is that a burnout is triggered when a user sends more than 4 inline messages in 240 minutes.  
And the burnout is cleared 240 minutes after the user sends the first inline message.

The `admins` in the group could change the configuration by command `/setup`

You could get more info by command `/help`

## Selfhost

1. Set `TZ` in `.env` to your group`s timezone. You can access [tz database](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones) there.
2. Set `BOT_TOKEN` in `.env` with your telegram bot token.
3. Run `make`.
4. Your bot should be online now.
