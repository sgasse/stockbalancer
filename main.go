package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

type dataModel struct {
	Portfolio portfolio
	DlLink    string
}

func restStocksHandler(w http.ResponseWriter, r *http.Request) {
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

	if p.Reinvest != 0.0 {
		rebalancePortfolio(&p, p.Reinvest)
	}

	pBytes, jsonErr := json.MarshalIndent(p, "", "    ")
	if jsonErr != nil {
		log.Print("Could not encode portfolio.")
		http.Error(w, "Could not parse request", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(pBytes)
}

func portfolioHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		// GET
		t, _ := template.ParseFiles("html/inputForm.html")
		t.Execute(w, nil)
	} else {
		// POST
		r.ParseForm()
		p, parseErr := parsePortfolio([]byte(r.Form["portfolioData"][0]))
		if parseErr != nil {
			log.Print(parseErr)
			http.Error(w, "Could not parse portfolio", http.StatusBadRequest)
			return
		}

		if p.Reinvest == 0.0 {
			log.Print("Display portfolio")
			t, _ := template.ParseFiles("html/stockView.html")
			dm := dataModel{Portfolio: p}
			t.Execute(w, dm)
		} else {
			log.Print("Rebalance portfolio")
			rebalancePortfolio(&p, p.Reinvest)

			pSHA1 := storePortfolio(&p)
			log.Print("Portfolio has SHA1: ", pSHA1)
			link := fmt.Sprintf("http://localhost:3210/download?p=%s", pSHA1)

			t, _ := template.ParseFiles("html/rebalanceView.html")
			dm := dataModel{Portfolio: p, DlLink: link}
			t.Execute(w, dm)
		}
	}
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
	go launchCache(avAPIKey)

	mux := http.NewServeMux()

	fs := http.FileServer(http.Dir("assets"))

	mux.HandleFunc("/restPortfolio", restStocksHandler)
	mux.HandleFunc("/portfolio", portfolioHandler)
	mux.HandleFunc("/download", downloadHandler)
	mux.Handle("/assets/", http.StripPrefix("/assets/", fs))
	http.ListenAndServe(":"+port, mux)
}
