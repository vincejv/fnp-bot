# FNP IRC Bot

### docker-compose.yaml
```yaml
version: '3.1'
services:
  fnpbot:
    container_name: fnp-bot
    image: vincejv/fnp-bot
    volumes:
      - ./config:/config
    environment:
      IRC_SERVER: "irc.server.chat:6697" # IRC Server
      BOT_NICK: "Jarvis" # IRC Bot Nick
      IRC_CHANNEL: "#fnp-announce-test" # Announce Channel
      IRC_BOT_PASSWORD: "jarvis@fnpbot/ircserver:zncPassword" # SASL/ZNC Server password
      CRAWLER_COOKIE: "remember_web_COOKIE_TOKEN" # Site token
      FETCH_SEC: 5   # Fetch frequency
      INIT_TORRENT_ID: 78336  # Start fetching from this torrent (used for initial setting only!)
      FETCH_BASE_URL: "https://site.com"
    restart: unless-stopped
    logging:
      options:
        max-size: "20m"
        max-file: "5"
        compress: "true"
```
