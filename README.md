## _This is WIP_

# Stock Balancer

This repository contains code for a microservice which balances out a stock portfolio.

## REST API
Example query:
```
curl -d '{"Stocks":[{"WKN": "ABC", "Price": 12.34, "Shares": 10, "GoalRatio": 0.5}]}' -H "Content-Type: application/json" -X POST http://localhost:3210/disp
```

## Next steps
- Enable downloading the rebalanced portfolio as JSON/CSV. https://www.alexedwards.net/blog/golang-response-snippets#json
- Update styling with CSS.
- Re-query price only when it has a certain age.
- Retrieve symbols from ISIN.
- Update REST API.
- Test code for the balancing algorithm.
