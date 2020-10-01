package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	queryItems    = "https://qiita.com/api/v2/authenticated_user/items?page=%d&per_page=100"
	articleDir    = "article"
	fetchInterval = 100 // msec
	imageHost     = "qiita-image-store.s3.amazonaws.com"
)

func download(url string, token string) ([]byte, error) {
	request, _ := http.NewRequest("GET", url, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	client := new(http.Client)

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// https://qiita.com/Qiita/items/c686397e4a0f4f11683d
func parseEmbedImageLink(line string) (url string) {
	var state rune
	var sb strings.Builder
	// parse ![alt-text](url-with-title)
	for _, r := range line {
		switch state {
		case '\x00':
			if r == '!' {
				state = r
			}
		case '!':
			if r == '[' {
				state = r
			} else {
				state = 0
			}
		case '[':
			if r == ']' {
				state = r
			} else {
				// skip alt-text field
			}
		case ']':
			if r == '(' {
				state = r
			} else {
				state = 0
			}
		case '(':
			if r == ')' {
				// (url) or (url "title") -> url
				return strings.Fields(sb.String())[0]
			} else {
				sb.WriteRune(r)
			}
		}
	}
	return "" // parse failure
}

func fetchEmbedImage(mdfile string, action func(*url.URL, string)) error {
	fp, err := os.Open(mdfile)
	if err != nil {
		return err
	}
	defer fp.Close()

	scanner := bufio.NewScanner(fp)
	for scanner.Scan() {
		line := scanner.Text()
		url, _ := url.Parse(parseEmbedImageLink(line))
		if url.Host == imageHost {
			imgfile := path.Base(url.Path)
			action(url, imgfile)
		}
	}
	return nil
}

func main() {
	tokenBytes, err := ioutil.ReadFile("QIITA_TOKEN")
	if err != nil {
		panic(err)
	}
	token := strings.TrimSuffix(string(tokenBytes), "\n")

	if err := os.Mkdir(articleDir, 0777); err != nil {
		panic(err)
	}

	page := 1
	for {
		itemsJson, err := download(fmt.Sprintf(queryItems, page), token)
		if err != nil {
			panic(err)
		}
		var items []map[string]interface{}
		if err := json.Unmarshal(itemsJson, &items); err != nil {
			panic(err)
		}
		if len(items) == 0 {
			break
		}

		for _, item := range items {
			mdurl, _ := item["url"].(string)
			mdurl = mdurl + ".md"
			createdAt, _ := item["created_at"].(string)
			date, _ := time.Parse(time.RFC3339, createdAt)
			basepath := filepath.Join(articleDir, date.Format("20060102T150405"))

			// fetch markdown
			fmt.Println(mdurl, basepath, item["title"])
			body, err := download(mdurl, token)
			if err != nil {
				panic(err)
			}
			mdfile := basepath + ".md"
			ioutil.WriteFile(mdfile, body, 0644)

			// fetch all embeded images
			fetchEmbedImage(mdfile, func(imgurl *url.URL, imgfile string) {
				fmt.Println(basepath, imgfile)
				response, err := http.Get(imgurl.String())
				if err != nil {
					panic(err)
				}
				defer response.Body.Close()

				file, err := os.Create(basepath + "." + imgfile)
				if err != nil {
					panic(err)
				}
				defer file.Close()

				io.Copy(file, response.Body)
			})

			time.Sleep(time.Millisecond * fetchInterval)
		}
		page += 1
	}
}
