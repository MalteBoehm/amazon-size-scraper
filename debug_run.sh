#!/bin/bash

export SLOG_LEVEL=debug

./bin/size-scraper \
  -search "https://www.amazon.de/s?k=t-shirt+größentabelle+länge&i=fashion" \
  -headless=true \
  -db-name=tall_affiliate \
  -db-port=5433 \
  -db-password=postgres \
  -concurrent=1