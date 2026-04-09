# Beerkellar Drunk

We add a feature that allows a user to retrieve their last drunk beers. 

User calls:

beerkellar drunk

and the system will return

2025-05-01 Sierra Nevada - Bigfoot (x units)

where units are the units of alcohol in the beer.

This should be limited to the last 10 beers drunk to reduce load but can be configured to return up to 50 beers with a flag.

If any of the beers are not present in the cache and we don't have the brewery or beer name then this should trigger a cache refresh for that beer.