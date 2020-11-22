package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
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

var queryTimeout = 13 * time.Second
var cacheFile = ".priceCache.json"
var priceCache = struct {
	sync.RWMutex
	m map[string]stockPrice
}{m: make(map[string]stockPrice)}

func queryPrice(symbol string, avAPIKey string) stockPrice {
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

func updateSymbol(symbolsToQuery <-chan string, avAPIKey string) {
	for {
		symbolToQuery := <-symbolsToQuery
		priceCache.Lock()
		priceCache.m[symbolToQuery] = queryPrice(symbolToQuery, avAPIKey)
		priceCache.Unlock()
		log.Print("Updated symbol ", symbolToQuery)

		// only 5 queries per minute are allowed
		time.Sleep(queryTimeout)
	}
}

func persistCache(cachePath string, doSleep bool) {
	for {
		priceCache.RLock()
		cacheJSON, jsonErr := json.MarshalIndent(priceCache.m, "", "    ")
		if jsonErr != nil {
			fmt.Println(jsonErr)
		}
		priceCache.RUnlock()

		ioutil.WriteFile(cachePath, cacheJSON, 0644)
		log.Print("Wrote cache to ", cachePath)

		if doSleep {
			time.Sleep(1 * time.Minute)
		} else {
			break
		}
	}
}

func checkForOutdated(symbolsToQuery chan<- string) {
	for {
		priceCache.RLock()
		for symbol, sPrice := range priceCache.m {
			if time.Now().Sub(sPrice.LastQueried) > 24*time.Hour {
				symbolsToQuery <- symbol
				log.Print("Enqueuing outdated symbol ", symbol)
			}
		}
		priceCache.RUnlock()

		sleepTime := time.Duration(len(symbolsToQuery)) * queryTimeout
		log.Print(len(symbolsToQuery), " in the queue, sleeping for ", sleepTime)
		time.Sleep(sleepTime)
	}
}

func loadPriceCache(cachePath string) {
	cacheStr, readErr := ioutil.ReadFile(cachePath)
	if readErr != nil {
		log.Print(readErr, ", will continue without loaded cache.")
	} else {
		priceCache.Lock()
		jsonErr := json.Unmarshal(cacheStr, &priceCache.m)
		if jsonErr != nil {
			fmt.Println(jsonErr)
		}
		log.Print("Loaded price cache", priceCache.m)
		priceCache.Unlock()
	}
}

func updatePriceCache(avAPIKey string) {
	var cachePath string
	cwd, pathErr := os.Getwd()
	if pathErr != nil {
		log.Print(pathErr, ", will continue without loading/storing cache.")
	} else {
		cachePath = path.Join(cwd, cacheFile)
		loadPriceCache(cachePath)
		go persistCache(cachePath, true)

		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		go func() {
			<-c
			log.Print("Received SIGINT, persisting cache")
			persistCache(cachePath, false)
			os.Exit(0)
		}()
	}

	symbolsToQuery := make(chan string, 10)
	go checkForOutdated(symbolsToQuery)
	go updateSymbol(symbolsToQuery, avAPIKey)
}
