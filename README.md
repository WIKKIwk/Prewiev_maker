<div align="center">
🍌 **Pro Banana AI Telegram Bot**
</div>

# Telegram Bot - Gemini AI asosida (Go)

Eng kuchli Gemini 3 Pro modeli asosidagi Telegram AI bot. 2K sifatli rasm tahrirlash va yuqori mantiqiy fikrlash qobiliyatiga ega.

## Xususiyatlar

✅ **Gemini 3 Pro** - Kuchli matn generatsiya va murakkab fikrlash  
✅ **Gemini 2.5 Flash** - Rasm tahlil va generatsiya  
✅ **Rasm yaratish** - AI orqali rasm yaratish  
✅ **Suhbat tarixi** - Kontekstli suhbat  
✅ **O'zbek tili** - To'liq o'zbek tili qo'llab-quvvatlash

## Boshlash

### 1. Bot Token Olish

Telegram'da [@BotFather](https://t.me/botfather) botiga o'ting:

```
/newbot
```

Bot nomi va username kiriting. BotFather sizga token beradi.

### 2. Environment O'rnatish

`.env` fayl yarating va quyidagilarni kiriting:

```bash
TELEGRAM_BOT_TOKEN=your_telegram_bot_token_here
GEMINI_API_KEY=your_gemini_api_key_here
NODE_ENV=production
```

### 3. Lokal Ishga Tushirish

**Talablar:** Go 1.23+

```bash
go mod download
go run ./cmd/bot
```

### 4. Docker bilan Ishga Tushirish

**Talablar:** Docker va Docker Compose

```bash
# Build va run
docker-compose up -d

# Loglarni ko'rish
docker-compose logs -f

# To'xtatish
docker-compose down
```

## Bot Buyruqlari

- `/start` - Botni ishga tushirish
- `/help` - Yordam va ma'lumot
- `/image <tavsif>` - Rasm yaratish
- `/clear` - Suhbat tarixini tozalash

## Foydalanish

1. **Matnli savol** - Bot AI orqali javob beradi
2. **Rasm yuborish** - Bot rasmni tahlil qiladi  
3. **Rasm yaratish** - `/image banana robot` kabi buyruq yuboring

## Arxitektura

```
cmd/bot/
└── main.go                   # Entry point
internal/
├── config/                   # ENV/config
├── gemini/                   # Gemini API client
├── handlers/                 # Telegram update handlers
├── mediagroup/               # Album (media group) aggregator
├── session/                  # In-memory session/history
└── telegram/                 # Telegram client helpers
```

## Production Deploy

Docker Compose orqali production'da deploy qilish:

```bash
# .env faylni to'ldiring
cp .env.example .env

# Docker compose bilan ishga tushiring
docker-compose up -d
```

---

Savol va muammolar uchun issue oching! 🚀
