package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
)

type dataModel struct {
	Portfolio portfolio
	DlLink    string
}

func restStocksHandler(w http.ResponseWriter, r *http.Request) {
	tpl := template.Must(template.ParseFiles("html/stockView.html"))
	body, readErr := ioutil.ReadAll(r.Body)
	if readErr != nil {
		http.Error(w, "Could not parse request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	p, parseErr := parsePortfolio(body)

	if parseErr != nil {
		http.Error(w, "Could not parse request", http.StatusBadRequest)
		return
	}

	dm := dataModel{Portfolio: p}
	tpl.Execute(w, dm)
}

func dispHandler(w http.ResponseWriter, r *http.Request) {
	formHandler(w, r, false)
}

func rebalanceHandler(w http.ResponseWriter, r *http.Request) {
	formHandler(w, r, true)
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	pSHA1s, ok := r.URL.Query()["p"]

	if !ok || len(pSHA1s[0]) < 1 {
		log.Print("URL param 'p' not found.")
		http.Error(w, "URL param 'p' not found.", http.StatusBadRequest)
		return
	}

	pSHA1 := pSHA1s[0]
	portfCache.RLock()
	portStr, ok := portfCache.m[pSHA1]
	portfCache.RUnlock()
	if !ok {
		log.Print("Portfolio with hash ", pSHA1, " not found.")
		http.Error(w, "Portfolio hash for download not found.", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(portStr)
	return
}

func formHandler(w http.ResponseWriter, r *http.Request, rebalance bool) {
	log.Print("formHandler called with method:", r.Method)
	if r.Method == "GET" {
		t, _ := template.ParseFiles("html/inputForm.html")
		if rebalance {
			t.Execute(w, "/rebalance")
		} else {
			t.Execute(w, "/disp")
		}
	} else {
		r.ParseForm()
		var p portfolio
		p, parseErr := parsePortfolio([]byte(r.Form["portfolioData"][0]))
		if parseErr != nil {
			http.Error(w, "Could not parse portfolio", http.StatusBadRequest)
			return
		}
		if err := json.Unmarshal([]byte(r.Form["portfolioData"][0]), &p); err != nil {
			log.Print("Could not unmarshall string.")
			http.Error(w, "Portfolio data structure not understood - is it valid JSON?", http.StatusBadRequest)
			return
		}

		updatePortfolioSum(&p)

		if !rebalance {
			// Display existing portfolio
			t, _ := template.ParseFiles("html/stockView.html")
			dm := dataModel{Portfolio: p}
			t.Execute(w, dm)
		} else {
			// Calculate rebalancing and different view
			reinvest, floatErr := strconv.ParseFloat(r.Form["reinvest"][0], 64)
			if floatErr != nil {
				log.Print("Could not parse float")
				http.Error(w, "Invalid value for reinvest", http.StatusBadRequest)
				return
			}
			rebalancePortfolio(&p, reinvest)

			pSHA1 := storePortfolio(&p)
			log.Print("Portfolio has SHA1: ", pSHA1)
			link := fmt.Sprintf("http://localhost:3210/download?p=%s", pSHA1)

			t, _ := template.ParseFiles("html/rebalanceView.html")
			dm := dataModel{Portfolio: p, DlLink: link}
			t.Execute(w, dm)
		}

	}
}

func main() {
	port := os.Getenv("BALANCER_PORT")
	if port == "" {
		port = "3210"
	}

	avAPIKey := os.Getenv("AV_API_KEY")
	if avAPIKey == "" {
		log.Fatal("You must specify your API key from AlphaVantage as AV_API_KEY.")
	}
	go updatePriceCache(avAPIKey)

	mux := http.NewServeMux()

	fs := http.FileServer(http.Dir("assets"))

	mux.HandleFunc("/rest", restStocksHandler)
	mux.HandleFunc("/disp", dispHandler)
	mux.HandleFunc("/rebalance", rebalanceHandler)
	mux.HandleFunc("/download", downloadHandler)
	mux.Handle("/assets/", http.StripPrefix("/assets/", fs))
	http.ListenAndServe(":"+port, mux)
}
