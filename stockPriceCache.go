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
var rateLimitOK = make(chan bool, 1)
var avAPIKey string

func launchCache(inAvAPIKey string) {
	avAPIKey = inAvAPIKey
	go limitRate()

	var cachePath string
	cwd, pathErr := os.Getwd()
	if pathErr != nil {
		log.Print(pathErr, ", will continue without loading/storing cache.")
	} else {
		cachePath = path.Join(cwd, cacheFile)
		loadPriceCache(cachePath)

		// check all symbols for updating after loading
		for symbol := range priceCache.m {
			go getCachedPrice(symbol)
		}

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
}

func queryPrice(symbol string) stockPrice {
	<-rateLimitOK
	log.Print("Querying symbol ", symbol)

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

func getCachedPrice(symbol string) float64 {
	priceCache.RLock()
	sPrice, exists := priceCache.m[symbol]
	priceCache.RUnlock()

	if !exists || time.Now().Sub(sPrice.LastQueried) > 24*time.Hour {
		sPrice = queryPrice(symbol)
		priceCache.Lock()
		priceCache.m[symbol] = sPrice
		priceCache.Unlock()
	}

	return sPrice.Price
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

func limitRate() {
	for {
		rateLimitOK <- true
		time.Sleep(queryTimeout)
	}
}
