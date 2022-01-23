package main

import (
	"errors"
	"flag"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
)

const (
	fullText = iota + 1
	source
	sourceType
	author
)

// ErrNoMatchOnLine ...
var ErrNoMatchOnLine = errors.New("no match found on line")

var extractID = regexp.MustCompile(`/text\?id=([0-9]+)`)
var txtInfo = regexp.MustCompile(`fullTextStr">(.*?)</div>(?s).*>(.*)</a>(?s).*/>\((.*)\)(?s).*by (.*?)\n`)

func main() {
	dbPath := flag.String("db", "texts.db", "")
	flag.Parse()

	db, err := NewTextsDB(*dbPath)
	if err != nil {
		log.Fatal(err)
	}

	if err := db.CreateTables(); err != nil {
		log.Fatal(err)
	}

	html, err := fetchTextsHTML()
	if err != nil {
		log.Fatal(fmt.Errorf("failed to fetch texts html: %v", err))
	}

	ids := fetchIDsFromHTML(html)

	for _, id := range ids {
		msg := fmt.Sprintf("text %d", id)
		if err := fetchTextInfo(db, id); err != nil {
			msg = fmt.Sprintf("%s: error: %v", msg, err)
		} else {
			msg = fmt.Sprintf("%s: done", msg)
		}

		log.Println(msg)
	}
}

func fetchIDsFromHTML(html string) []int64 {
	ids := make([]int64, 0)

	matches := extractID.FindAllStringSubmatch(html, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		id, err := strconv.ParseInt(match[1], 10, 64)
		if err == nil {
			ids = append(ids, id)
		}
	}

	return ids
}

func fetchTextsHTML() (string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://typeracerdata.com/texts", nil)
	if err != nil {
		return "", fmt.Errorf("creating http request failed: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("unexpected status code while fetching text ids: %d", resp.StatusCode)
	}

	htmlBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read html response: %w", err)
	}

	return string(htmlBytes), nil
}

func fetchTextID(line string) (int64, error) {
	match := extractID.FindStringSubmatch(line)
	if match != nil {
		return 0, ErrNoMatchOnLine
	}

	id, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse id from string: %v: %w", match[1], err)
	}

	return id, nil
}

func fetchTextInfo(db *TextsDB, id int64) error {
	pitURL := fmt.Sprintf("https://data.typeracer.com/pit/text_info?id=%d", id)

	req, err := http.NewRequest(http.MethodGet, pitURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	htmlBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to parse text info: %w", err)
	}

	match := txtInfo.FindStringSubmatch(string(htmlBytes))
	if match == nil {
		return fmt.Errorf("failed to match text info")
	}

	if err := db.Insert(
		id,
		html.UnescapeString(match[fullText]),
		html.UnescapeString(match[sourceType]),
		html.UnescapeString(match[author]),
		html.UnescapeString(match[source]),
	); err != nil {
		return fmt.Errorf("failed to insert: %w", err)
	}

	return nil
}
