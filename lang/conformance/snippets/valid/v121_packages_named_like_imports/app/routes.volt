package app

import (
	rtdb rt
)

scope /app [name: app] {
  get /feed rtdb.PostFeed
}
