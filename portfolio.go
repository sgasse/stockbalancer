package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"sync"
)

type customErr struct {
	debugMsg  string
	publicMsg string
}

func (c customErr) Error() string {
	return c.publicMsg
}

type stock struct {
	WKN             string  `json:"WKN"`
	ISIN            string  `json:"ISIN"`
	Symbol          string  `json:"Symbol"`
	Shares          int     `json:"Shares"`
	Price           float64 `json:"Price"`
	CurRatio        float64 `json:"CurrentRatio"`
	GoalRatio       float64 `json:"GoalRatio"`
	NewShares       float64 `json:"NewShares"`
	RebalancedRatio float64 `json:"RebalancedRatio"`
	RebalancedSum   float64 `json:"RebalancedSum"`
	pricePerPartial float64
}

type portfolio struct {
	Stocks          []stock
	Reinvest        float64
	SumExisting     float64
	SumWithReinvest float64
}

var portfCache = struct {
	sync.RWMutex
	m map[string][]byte
}{m: make(map[string][]byte)}

func parsePortfolio(jsonData []byte) (p portfolio, err error) {
	err = json.Unmarshal(jsonData, &p)
	if err != nil {
		return
	}

	ratioSum := 0.0
	for _, st := range p.Stocks {
		ratioSum += st.GoalRatio

		if st.GoalRatio <= 0.0 {
			msg := fmt.Sprint("ISIN ", st.ISIN, ": GoalRatio = ", st.GoalRatio, " <= 0.0")
			err = customErr{publicMsg: msg}
			return
		}

	}
	if ratioSum != 1.0 {
		msg := fmt.Sprint("Sum of stocks GoalRatio = ", ratioSum, " != 1.0")
		err = customErr{publicMsg: msg}
		return
	}

	updatePortfolioValues(&p)

	return p, err
}

func updatePortfolioValues(p *portfolio) {
	p.SumExisting = 0.0
	for ind := range p.Stocks {
		curStock := &p.Stocks[ind]
		curStock.Price = getCachedPrice(curStock.Symbol)
		p.SumExisting += float64(curStock.Shares) * curStock.Price
	}

	for ind := range p.Stocks {
		curStock := &p.Stocks[ind]
		curStock.CurRatio = (curStock.Price * float64(curStock.Shares)) / p.SumExisting
	}
}

func storePortfolio(p *portfolio) string {
	pBytes, jsonErr := json.MarshalIndent(*p, "", "    ")
	if jsonErr != nil {
		log.Print("Could not encode portfolio.")
	}

	pEnc := sha1.New()
	pEnc.Write(pBytes)
	pSHA1 := fmt.Sprintf("%x", pEnc.Sum(nil))

	portfCache.Lock()
	portfCache.m[pSHA1] = pBytes
	portfCache.Unlock()

	return pSHA1
}

func rebalancePortfolio(p *portfolio, reinvest float64) {
	goalSum := p.SumExisting + reinvest
	p.SumWithReinvest = p.SumExisting
	for ind := range p.Stocks {
		// Calculate new shares
		st := &p.Stocks[ind]
		shareGoalSum := goalSum * st.GoalRatio
		st.pricePerPartial = st.Price / shareGoalSum
		st.NewShares = math.Round((shareGoalSum / st.Price) - float64(st.Shares))
		st.RebalancedSum = (float64(st.Shares) + st.NewShares) * st.Price
		st.RebalancedRatio = st.RebalancedSum / goalSum
		p.SumWithReinvest += st.NewShares * st.Price
	}

	// Sort stocks by least change impact
	sort.SliceStable(p.Stocks, func(i, j int) bool {
		return p.Stocks[i].pricePerPartial < p.Stocks[j].pricePerPartial
	})

	ind := 0

	if p.SumWithReinvest > goalSum {
		for p.SumWithReinvest > goalSum {
			st := &p.Stocks[ind]
			st.NewShares -= 1.0
			st.RebalancedSum = (float64(st.Shares) + st.NewShares) * st.Price
			st.RebalancedRatio = st.RebalancedSum / goalSum
			p.SumWithReinvest -= st.Price
			ind++
		}
		log.Print("Rounded shares would have been too much, rounded down ", ind, " shares.")
	} else {
		for p.SumWithReinvest < goalSum {
			st := &p.Stocks[ind]
			st.NewShares += 1.0
			st.RebalancedSum = (float64(st.Shares) + st.NewShares) * st.Price
			st.RebalancedRatio = st.RebalancedSum / goalSum
			p.SumWithReinvest += st.Price
			ind++
		}
		// Undo last step
		ind--
		st := &p.Stocks[ind]
		st.NewShares -= 1.0
		st.RebalancedSum = (float64(st.Shares) + st.NewShares) * st.Price
		st.RebalancedRatio = st.RebalancedSum / goalSum
		p.SumWithReinvest -= st.Price
		log.Print("Rounded shares would have been too little, rounded up ", ind, " shares.")
		return
	}
}
