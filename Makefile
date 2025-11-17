# Root Makefile for Hugo site

# Paths
FLICKR_DIR := scripts/flickr-albums
FLICKR_BIN := $(FLICKR_DIR)/flickralbums
PHOTOS_DIR := content/photos

# Default goal
.PHONY: all
all: build

# Ensure binary is built
$(FLICKR_BIN): $(FLICKR_DIR)/*.go
	@echo "Building flickralbums..."
	cd $(FLICKR_DIR) && go build -o flickralbums

# Regenerate photos content
.PHONY: photos
photos: $(FLICKR_BIN)
	@echo "Cleaning old photos..."
	rm -rf $(PHOTOS_DIR)
	mkdir -p $(PHOTOS_DIR)
	@echo "Generating new photo content..."
	$(FLICKR_BIN) --out $(PHOTOS_DIR)

# Run Hugo dev server
.PHONY: dev
dev: 
	@echo "Starting Hugo development server..."
	hugo serve -D

# Build production site
.PHONY: build
build: 
	@echo "Building Hugo site..."
	hugo --minify --baseURL "${BASEURL:?BASEURL not set}"

