.PHONY: up up-avatar up-no-cloud stop-help build test verify images

build:
	./mvnw -B -ntp -DskipTests package

test:
	./mvnw -B -ntp test

verify:
	./mvnw -B -ntp verify

images:
	./script/build-java-images.sh

up:
	./start-cotriage.sh

up-avatar:
	./start-cotriage.sh --with-avatar

up-no-cloud:
	./start-cotriage.sh --no-cloud

stop-help:
	@echo "Run CoTriage in the foreground and press Ctrl-C to stop all owned processes."
