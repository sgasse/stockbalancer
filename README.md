## _This is WIP_

# Stock Balancer

This repository contains code for a microservice which balances out a stock portfolio.

## REST API
Example query:
```
curl -d '{"Stocks": [{"WKN": "A1W2EL", "ISIN": "IE00BBQ2W338", "Price": 0.0, "Shares": 140, "GoalRatio": 0.45, "Symbol": "H411.DE"}, {"WKN": "A12DPP", "ISIN": "IE00BQN1K901", "Price": 0.0, "Shares": 1400, "GoalRatio": 0.55, "Symbol": "CEMS.DE"}], "Reinvest": 3000.0}' -H "Content-Type: application/json" -X POST http://localhost:3210/restPortfolio
```

## Next steps
- https
- Delete outdated cached portfolios.
- Limit portfolio size?
- Better error passing for parsing and validation.
- Test code for the balancing algorithm.
- Retrieve symbols from ISIN.
