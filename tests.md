# Integration Tests

All built off an effective running app

1. Login flow
   1. Fake untappd backend that returns the auth code pages etc.
1. Initial Checkin Read
   1. Fake untapped backend that returns at least 2 pages of checkins
1. New Checkin Read
   1. Should apply to drink in cellar
   1. Should also apply to drink not in cellar
1. Add to cellar
   1. Should end up in the cellar in the right quantity
1. Get Beer
   1. Using various configurations of get
