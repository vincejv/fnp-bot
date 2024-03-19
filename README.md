# FNP IRC Bot

### docker-compose.yaml
```yaml
version: '3.1'
services:
  fnpbot:
    container_name: fnp-bot
    image: vincejv/fnp-bot:pull-mode
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
      FETCH_NO_OF_ITEMS: "25"
      ENABLE_SASL: false  # False if using ZNC or anonymous login, True if using SASL (doesn't support NickServ auth for now!)
      ENABLE_SSL: false   # SSL flag for IRC connection
      # Order is as follows: Category, type, name, size, uploader, url
      # must specify a format specifier (%s) for each field
      # %n if you want to skip the variable from printing
      ANNOUNCE_LINE_FMT: "Cat [%s] Type [%s] Name [%s] Size [%s] Uploader [%s] Url [%s]" 
    restart: unless-stopped
    logging:
      options:
        max-size: "20m"
        max-file: "5"
        compress: "true"
```
