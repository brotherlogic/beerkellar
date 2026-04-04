# Initialization

Once the user has fully authed we should pull in their details from untappd - we can use the https://untappd.com/api/docs#userinfo end point for this to pull in their user id number.

Users should transition from LOGGING_IN once auth starts, LOGGED_IN once auth is complete and AUTHORIZED once we have their user info.

Any API method outside of GetLogin and GetAuthToken should fail unless the user is AUTHORIZED.