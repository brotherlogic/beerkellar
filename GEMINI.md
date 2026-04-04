# Beerkellar

This is a project to create a CLI for managing a users beer cellar. It uses
the untappd API to pull information about beers (https://untappd.com/api/docs), and
to periodically pull the users checkins in order to see what should be removed from
the cellar.

## Style

All code is written in go and placed in the /server directory. We have the CLI
code in beerkellar_cli/ and also integration tests in /integration. We don't use
the integration tests locally typically but they run before a branch is commited. To
facilitate the integration tests, we use the fake_untappd server to act as an Untappd proxy whilst we run the tests.

## API

We use the untappd API https://untappd.com/api/docs to get info about beers and checkins. This API has rate
limiting so background calls to the API should go through the processqueue to run async.

## Coding style

Once a change is complete and all tasks are done, follow the finish.md workflow
