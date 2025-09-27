package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"bufio"
)

type FlickrResponse struct {
	Photos struct {
		Page    int `json:"page"`
		Pages   int `json:"pages"`
		Perpage int `json:"perpage"`
		Total   int `json:"total"`
		Photo   []Photo `json:"photo"`
	} `json:"photos"`
	Stat string `json:"stat"`
}

type Photo struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description struct {
		Content string `json:"_content"`
	} `json:"description"`
	URLLarge  string `json:"url_l"`
	URLMed    string `json:"url_m"`
	URLSmall  string `json:"url_s"`
	URLSquare string `json:"url_q"`
	Tags      string `json:"tags"`
	DateTaken string `json:"datetaken"`
}

func loadEnv(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines or comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Split on first '='
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		os.Setenv(key, value)
	}
	return scanner.Err()
}

func main() {

	// Load .env file
	if err := loadEnv(".env"); err != nil {
		fmt.Println("Warning: .env file not found or unreadable:", err)
	}

	contentDir := os.Getenv("CONTENT_DIR")
	apiKey := os.Getenv("FLICKR_API_KEY")
	userID := os.Getenv("FLICKR_USER_ID")
	if apiKey == "" || userID == "" {
		fmt.Println("Please set CONTENT_DIR, FLICKR_API_KEY and FLICKR_USER_ID in .env")
		os.Exit(1)
	}

	outDir := filepath.Join(contentDir, "photos")

	err := os.MkdirAll(outDir, 0755)
	if err != nil {
		panic(err)
	}

	const apiURL = "https://api.flickr.com/services/rest/?method=flickr.photos.search&format=json&nojsoncallback=1"

	page := 1
	for {
		url := fmt.Sprintf("%s&api_key=%s&user_id=%s&privacy_filter=1&extras=url_q,url_l,url_m,url_s,tags,description,date_taken&per_page=500&page=%d",
			apiURL, apiKey, userID, page)

		resp, err := http.Get(url)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()

		body, _ := ioutil.ReadAll(resp.Body)

		var data FlickrResponse
		if err := json.Unmarshal(body, &data); err != nil {
			panic(err)
		}

		if data.Stat != "ok" {
			panic("Flickr API error")
		}

		if len(data.Photos.Photo) == 0 {
			break
		}

		for _, photo := range data.Photos.Photo {
			writeMarkdown(outDir, photo)
		}

		if page >= data.Photos.Pages {
			break
		}
		page++
	}
}

// sanitizeSlug keeps only a-z, 0-9, replaces other chars with -
func sanitizeSlug(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	// collapse multiple dashes
	slug := strings.Trim(b.String(), "-")
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	return slug
}

func writeMarkdown(outDir string, p Photo) {

	if p.Tags == "" {
		// skip photos without tags
		return
	}

	// slug from title or fallback to photo ID
	slug := sanitizeSlug(p.Title)
	if slug == "" {
		slug = p.ID
	}

	date := p.DateTaken
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	// ensure unique filename
	filename := filepath.Join(outDir, slug+".md")
	uniqueFilename := filename
	i := 1
	for {
		if _, err := os.Stat(uniqueFilename); os.IsNotExist(err) {
			break // file does not exist → good to use
		}
		uniqueFilename = filepath.Join(outDir, fmt.Sprintf("%s-%d.md", slug, i))
		i++
	}
	filename = uniqueFilename


	// turn space-separated tags into comma-separated list
	tagList := strings.Split(strings.TrimSpace(p.Tags), " ")
	quotedTags := []string{}
	for _, t := range tagList {
		if t != "" {
			quotedTags = append(quotedTags, fmt.Sprintf("%q", t))
		}
	}

	content := fmt.Sprintf(`---
title: "%s"
date: %s
flickr_id: %s
tags: [%s]
thumbnail: %s
image: %s
---

%s
`, escapeQuotes(p.Title), date, p.ID, strings.Join(quotedTags, ", "), chooseThumbnail(p), chooseImage(p), p.Description.Content)

	err := ioutil.WriteFile(filename, []byte(content), 0644)
	if err != nil {
		panic(err)
	}

	fmt.Println("Wrote:", filename)
}

func escapeQuotes(s string) string {
	return strings.ReplaceAll(s, `"`, "'")
}

func chooseThumbnail(p Photo) string {
	if p.URLSquare != "" {
		return p.URLSquare
	}
	if p.URLSmall != "" {
		return p.URLSmall
	}
	if p.URLMed != "" {
		return p.URLMed
	}
	return p.URLLarge
}

func chooseImage(p Photo) string {
	if p.URLLarge != "" {
		return p.URLLarge
	}
	if p.URLMed != "" {
		return p.URLMed
	}
	if p.URLSmall != "" {
		return p.URLSmall
	}
	return p.URLSquare
}
