.PHONY: run logs down restart

run:
	@set -eu; \
	LOG_FILE="/tmp/pro-banana-docker-compose-run.log"; \
	printf "\n"; \
	printf "========================================\n"; \
	printf "  Pro Banana AI — Telegram Bot (Docker)\n"; \
	printf "========================================\n"; \
	printf "Starting...\n"; \
	if docker-compose up -d --build >"$$LOG_FILE" 2>&1; then \
		CID="$$(docker-compose ps -q telegram-bot 2>/dev/null || true)"; \
		STATUS="$$(docker inspect -f '{{.State.Status}}' "$$CID" 2>/dev/null || echo unknown)"; \
		printf "\n✅ Bot ishga tushdi!\n"; \
		printf "Status   : %s\n" "$$STATUS"; \
		printf "Logs     : make logs\n"; \
		printf "Details  : %s\n" "$$LOG_FILE"; \
		printf "========================================\n\n"; \
	else \
		printf "\n❌ Ishga tushmadi.\n"; \
		printf "Last logs: %s\n\n" "$$LOG_FILE"; \
		tail -n 200 "$$LOG_FILE" || true; \
		exit 1; \
	fi

logs:
	@docker-compose logs -f --tail=200

down:
	@docker-compose down

restart: down run
