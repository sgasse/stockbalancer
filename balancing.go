package main

import (
	"log"
	"math"
	"sort"
)

func rebalancePortfolio(p *portfolio, reinvest float64) {
	// Calculate portfolio value
	updatePortfolioSum(p)

	goalSum := p.SumExisting + reinvest
	p.SumWithReinvest = p.SumExisting
	for ind := range p.Stocks {
		// Calculate new shares
		st := &p.Stocks[ind]
		shareGoalSum := goalSum * st.GoalRatio
		st.pricePerPartial = st.Price / shareGoalSum
		st.NewShares = math.Round((shareGoalSum / st.Price) - float64(st.Shares))
		st.RebalanceSum = (float64(st.Shares) + st.NewShares) * st.Price
		st.RebalanceRatio = st.RebalanceSum / goalSum
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
			st.RebalanceSum = (float64(st.Shares) + st.NewShares) * st.Price
			st.RebalanceRatio = st.RebalanceSum / goalSum
			p.SumWithReinvest -= st.Price
			ind++
		}
		log.Print("Rounded shares would have been too much, rounded down ", ind, " shares.")
	} else {
		for p.SumWithReinvest < goalSum {
			st := &p.Stocks[ind]
			st.NewShares += 1.0
			st.RebalanceSum = (float64(st.Shares) + st.NewShares) * st.Price
			st.RebalanceRatio = st.RebalanceSum / goalSum
			p.SumWithReinvest += st.Price
			ind++
		}
		// Undo last step
		ind--
		st := &p.Stocks[ind]
		st.NewShares -= 1.0
		st.RebalanceSum = (float64(st.Shares) + st.NewShares) * st.Price
		st.RebalanceRatio = st.RebalanceSum / goalSum
		p.SumWithReinvest -= st.Price
		log.Print("Rounded shares would have been too little, rounded up ", ind, " shares.")
		return
	}
}

func updatePortfolioSum(p *portfolio) {
	p.SumExisting = 0.0
	for ind := range p.Stocks {
		curStock := &p.Stocks[ind]
		priceCache.RLock()
		cachedPrice, exists := priceCache.m[curStock.Symbol]
		priceCache.RUnlock()
		if exists {
			priceCache.RLock()
			curStock.Price = cachedPrice.Price
			priceCache.RUnlock()
			p.SumExisting += float64(curStock.Shares) * curStock.Price
		} else {
			priceCache.Lock()
			priceCache.m[curStock.Symbol] = stockPrice{}
			priceCache.Unlock()
			p.SumExisting += float64(curStock.Shares) * curStock.Price
		}
	}
}
