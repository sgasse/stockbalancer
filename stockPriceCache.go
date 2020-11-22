package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"sync"
	"time"
)

type globalQuote struct {
	Symbol           string `json:"01. symbol"`
	Open             string `json:"02. open"`
	High             string `json:"03. high"`
	Low              string `json:"04. low"`
	Price            string `json:"05. price"`
	Volume           string `json:"06. volume"`
	LatestTradingDay string `json:"07. latest trading day"`
	PreviousClose    string `json:"08. previous close"`
	Change           string `json:"09. change"`
	ChangePercent    string `json:"10. change percent"`
}

type stockPrice struct {
	LastQueried time.Time   `json:"LastQueried"`
	Price       float64     `json:"Price"`
	GlobalQuote globalQuote `json:"Global Quote"`
}

var lastCacheWrite = time.Now()
var cacheFile = ".priceCache.json"
var priceCache = struct {
	sync.RWMutex
	m map[string]stockPrice
}{m: make(map[string]stockPrice)}

func getPrice(symbol string, avAPIKey string) stockPrice {
	url := fmt.Sprintf("https://www.alphavantage.co/query?function=GLOBAL_QUOTE&symbol=%s&apikey=%s",
		symbol, avAPIKey)

	client := http.Client{Timeout: time.Second * 5}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		fmt.Println(err)
	}

	res, getErr := client.Do(req)
	if getErr != nil {
		fmt.Println(getErr)
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		fmt.Println(readErr)
	}

	var price stockPrice

	jsonErr := json.Unmarshal(body, &price)
	if jsonErr != nil {
		fmt.Println(jsonErr)
	}

	// read price and update timestamp
	price.LastQueried = time.Now()
	price.Price, _ = strconv.ParseFloat(price.GlobalQuote.Price, 64)

	return price
}

func updatePriceCache(avAPIKey string) {
	cwd, pathErr := os.Getwd()
	if pathErr != nil {
		fmt.Println(pathErr)
	}

	cacheStr, readErr := ioutil.ReadFile(path.Join(cwd, cacheFile))
	if readErr != nil {
		fmt.Println(readErr)
	} else {
		priceCache.Lock()
		jsonErr := json.Unmarshal(cacheStr, &priceCache.m)
		if jsonErr != nil {
			fmt.Println(jsonErr)
		}
		log.Print("Loaded price cache", priceCache.m)
		priceCache.Unlock()
	}

	// endless loop
	for {
		// find next symbol to update
		oldestQuery := time.Now()
		symbolToQuery := ""
		priceCache.RLock()
		for symbol, sPrice := range priceCache.m {
			if sPrice.LastQueried.Before(oldestQuery) {
				oldestQuery = sPrice.LastQueried
				symbolToQuery = symbol
			}
		}
		priceCache.RUnlock()

		// only 5 queries per minute are allowed
		time.Sleep(13 * time.Second)

		if symbolToQuery == "" {
			continue
		}

		priceCache.Lock()
		priceCache.m[symbolToQuery] = getPrice(symbolToQuery, avAPIKey)
		priceCache.Unlock()

		priceCache.RLock()
		cacheJSON, jsonErr := json.MarshalIndent(priceCache.m, "", "    ")
		if jsonErr != nil {
			fmt.Println(jsonErr)
		}
		priceCache.RUnlock()

		ioutil.WriteFile(path.Join(cwd, cacheFile), cacheJSON, 0644)
	}
}
