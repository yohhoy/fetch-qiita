package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

const (
	queryItems = "https://qiita.com/api/v2/authenticated_user/items?page=%d&per_page=100"
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

func main() {
	tokenBytes, err := ioutil.ReadFile("QIITA_TOKEN")
	if err != nil {
		panic(err)
	}
	token := strings.TrimSuffix(string(tokenBytes), "\n")

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
			title, url := item["title"], item["url"]
			fmt.Println(url, title)
		}
		page += 1
	}
}