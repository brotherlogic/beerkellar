# Clearing the Cellar

This proposal outlines how the cellar should be cleared once it has things in it.

## Process

Every hour, the system should run the RefreshUser api call for each user in the system.

This effectively updates the cellar of every user. We can throttle this process by (a) building
a list of every user on the sytem, and (b) only running the refresh for users who's last refresh was
more than two hours ago. This will reduce overall system load.