# Legacy Node.js implementation

This directory contains the original Node.js `MFCDiscordBot`, kept for
reference only. The supported bot is the Go service in the repository root
(`cmd/mfcbot`).

Contents:

- `bot.js` — Discord client, slash-command dispatch, online checker
- `commands/` — slash command implementations
- `webstuff.js` — MFC HTTP lookups and `http://mfcgo/online` polling
- `databasefunctions.js` — SQLite helpers
- `deploy-commands.js` — registers global slash commands
- `package.json`, `package-lock.json` — Node dependencies
- `node_modules/` — installed dependencies (gitignored)
- `seed.js` — seed data (gitignored)

The old bot expected `mfcbot.db` and `.env` in its working directory. If you
need to run it for comparison, restore those files or run it from a copy of
the repository before the Go rewrite.
