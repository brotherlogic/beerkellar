# Add Beer

The Add Beer call takes:

1. The untappd id of the beer
1. The quanity to add

and places beers of that quantity into the users cellar.

As extra work, it adds the untappd id into the cache queue, which
will pull the details from untappd and store in the beer cache.

Cached beers are refreshed on pull every month in order to ensure
we have the most up to date details.

## Prober Test

We should be able to:

1. Login
1. Add Beer
1. Retrieve Cellar

And find that the beer details have been correctly propogated. Since
this is queued we will run multiple cellar pulls until the details
are present.

## Tasks

1. Add API Call
1. Have API call store beer in the user cellar
1. Add cache queue infrastructure
1. Have API call insert into the cache queue
1. Have cache queue run untappd pull
1. Add API call to pull the user cellar
