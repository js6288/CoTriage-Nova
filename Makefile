.PHONY: up up-avatar up-no-cloud stop-help

up:
	./start-cotriage.sh

up-avatar:
	./start-cotriage.sh --with-avatar

up-no-cloud:
	./start-cotriage.sh --no-cloud

stop-help:
	@echo "Run CoTriage in the foreground and press Ctrl-C to stop all owned processes."
