package main

import (
	"flag"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	fullText = iota + 1
	source
	sourceType
	author
)

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

func fetchIDsFromHTML(htmlStr string) []int64 {
	ids := make([]int64, 0)

	matches := extractID.FindAllStringSubmatch(htmlStr, -1)
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

func fetchTextInfo(db *TextsDB, id int64) error {
	pitURL := fmt.Sprintf("https://data.typeracer.com/pit/text_info?id=%d", id)

	req, err := http.NewRequest(http.MethodGet, pitURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	resp, err := doWithRetriesReq(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	htmlBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to parse text info: %w", err)
	}

	return fetchTextInfoFromHTML(db, id, string(htmlBytes))
}

func fetchTextInfoFromHTML(db *TextsDB, id int64, htmlStr string) error {
	match := txtInfo.FindStringSubmatch(htmlStr)
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

func doWithRetriesReq(req *http.Request) (*http.Response, error) {
	var initialBackoff time.Duration = time.Second * 5
	var maxBackoff time.Duration = time.Second * 30
	var backoff time.Duration = initialBackoff

	for {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http request failed: %w", err)
		}

		if resp.StatusCode == 200 {
			return resp, nil
		}

		resp.Body.Close()

		if resp.StatusCode != 429 {
			return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}

		log.Printf("status code 429: sleeping %v", backoff)
		time.Sleep(backoff)

		backoff += initialBackoff
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}
