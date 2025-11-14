package main

import (
	"encoding/json"
	"flag"
	"html"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const flickrAPI = "https://api.flickr.com/services/rest/?format=json&nojsoncallback=1"

// Album represents a Flickr photoset (album)
type Album struct {
	ID    string `json:"id"`
	Title struct {
		Content string `json:"_content"`
	} `json:"title"`
	Description struct {
		Content string `json:"_content"`
	} `json:"description"`
}

// Photo represents a Flickr photo
type Photo struct {
	Title           string `json:"title"`
	ID              string `json:"id"`
	Farm            int    `json:"farm"`
	Server          string `json:"server"`
	Secret          string `json:"secret"`
	OriginalSecret  string `json:"originalsecret"`
	OriginalFormat  string `json:"originalformat"`
	H_URL          string `json:"url_h"`
	M_URL          string `json:"url_m"`
	B_URL          string `json:"url_b"`
	O_URL          string `json:"url_o"`
	W_URL          string `json:"url_w"`
}

func main() {
	// Load .env if available
	_ = godotenv.Load()

	apiKeyFlag := flag.String("api-key", os.Getenv("FLICKR_API_KEY"), "Flickr API key (or set in .env)")
	userIDFlag := flag.String("user-id", os.Getenv("FLICKR_USER_ID"), "Flickr user ID (or set in .env)")
	outDirFlag := flag.String("out", "output", "Output directory for album files")
	extFlag := flag.String("ext", "md", "File extension for output files (md or txt)")
	flag.Parse()

	apiKey := *apiKeyFlag
	userID := *userIDFlag

	if apiKey == "" || userID == "" {
		fmt.Println("❌ Missing Flickr credentials.")
		fmt.Println("Provide via flags or .env file:")
		fmt.Println("  FLICKR_API_KEY=your_key")
		fmt.Println("  FLICKR_USER_ID=your_user_id")
		os.Exit(1)
	}

	albums, err := fetchAlbums(apiKey, userID)
	if err != nil {
		panic(err)
	}

	if err := os.MkdirAll(*outDirFlag, 0755); err != nil {
		panic(err)
	}

	fmt.Printf("Found %d albums\n", len(albums))

	for i, a := range albums {
		fmt.Printf("[%d/%d] Fetching '%s'...\n", i+1, len(albums), a.Title.Content)
		photos, err := fetchAlbumPhotos(apiKey, a.ID)
		if err != nil {
			fmt.Printf("Error fetching album %s: %v\n", a.ID, err)
			continue
		}

		if err := writeAlbumFile(a, photos, *outDirFlag, *extFlag); err != nil {
			fmt.Printf("Error writing album %s: %v\n", a.ID, err)
		}
	}

	fmt.Println("All albums exported successfully.")
}

// Fetch all albums for a user
func fetchAlbums(apiKey, userID string) ([]Album, error) {
	url := fmt.Sprintf("%s&method=flickr.photosets.getList&api_key=%s&user_id=%s", flickrAPI, apiKey, userID)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Photosets struct {
			Photoset []Album `json:"photoset"`
		} `json:"photosets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("decode error: %v\nBody: %s", err, body)
	}
	return data.Photosets.Photoset, nil
}

// Fetch photos for a specific album
func fetchAlbumPhotos(apiKey, albumID string) ([]Photo, error) {
	url := fmt.Sprintf("%s&method=flickr.photosets.getPhotos&api_key=%s&photoset_id=%s&extras=url_w,url_h,url_m,url_b,url_o,original_format,original_secret", flickrAPI, apiKey, albumID)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Photoset struct {
			Photo []Photo `json:"photo"`
		} `json:"photoset"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("decode error: %v (body: %s)", err, string(body))
	}
	return data.Photoset.Photo, nil
}

// Write album file to disk
func writeAlbumFile(a Album, photos []Photo, outDir, ext string) error {
	filename := sanitizeFilename(a.Title.Content) + "." + ext
	path := filepath.Join(outDir, filename)

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()


	writtenHeader := false

	for _, p := range photos {
		medium := p.M_URL
		if medium == "" {
			medium = p.W_URL
		}

		large := p.B_URL
		if p.H_URL != "" {
			large = p.H_URL
		}

		//some photos don't have a large or medum size, fallback to original
		if large == "" && p.O_URL != "" {
			large = p.O_URL
		}

		if medium == "" && large != "" {
			medium = large
		}

		if medium == "" || large == "" {
			fmt.Printf("Odding photo without medium or large size, skipping: %s\n", p.Title)
			fmt.Printf("%+v\n", p)
		}

		imageCaption := p.Title
		//we only want to display an image caption if it's more than one word
		if strings.Count(imageCaption, " ") < 1 {
			imageCaption = ""
		}

		if !writtenHeader {
	header := fmt.Sprintf(`+++
date = '%s'
draft = false
title = '%s'
description = '%s'
image = '%s'
+++`, time.Now().Format(time.RFC3339), a.Title.Content, html.EscapeString(a.Description.Content), p.M_URL)
	_, _ = file.WriteString(header + "\n")
			writtenHeader = true
		}

		line := fmt.Sprintf(`{{%% gallery_image title="%s" medium="%s" large="%s" original="%s" %%}}`,
    escapeQuotes(imageCaption), medium, large, p.O_URL)
		_, _ = file.WriteString(line + "\n")
	}

	fmt.Printf("Wrote %s\n", path)
	return nil
}

// Helpers
func sanitizeFilename(name string) string {
	name = strings.ToLower(name)
	re := regexp.MustCompile(`[^a-z0-9-_]+`)
	return strings.Trim(re.ReplaceAllString(name, "-"), "-")
}

func escapeQuotes(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}
