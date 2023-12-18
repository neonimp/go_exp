.PHONY: clean build

build:
	@echo "Building..."
	go build -o bin/smtpbridge -v

clean: build
	@echo "Cleaning..."
	rm -rf bin/*
