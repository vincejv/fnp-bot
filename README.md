# FNP IRC Bot

### docker-compose.yaml
```yaml
version: '3.1'
services:
  fnpbot:
    container_name: fnp-bot
    image: vincejv/fnp-bot:push-mode
    volumes:
      - ./config:/config
    environment:
      IRC_SERVER: "irc.server.chat:6697" # IRC Server
      BOT_NICK: "Jarvis" # IRC Bot Nick
      IRC_CHANNEL: "#fnp-announce-test" # Announce Channel
      IRC_BOT_PASSWORD: "jarvis@fnpbot/ircserver:zncPassword" # SASL/ZNC Server password
      FETCH_BASE_URL: "https://site.com" # Website URL
      FETCH_NO_OF_ITEMS: "25"  # No of items to fetch during manual fetches (when missing items during chat disconnects)
      ENABLE_SASL: false  # False if using ZNC or anonymous login, True if using SASL (doesn't support NickServ auth for now!)
      ENABLE_SSL: false   # SSL flag for IRC connection
      # Order is as follows: Category, type, name, size, uploader, url
      # must specify a format specifier (%s) for each field
      # %n if you want to skip the variable from printing
      ANNOUNCE_LINE_FMT: "Cat [%s] Type [%s] Name [%s] Size [%s] Uploader [%s] Url [%s]" 
      SITE_USERNAME: "myunit3dusername"  # site login is required to communicate with the websocket
      SITE_PASSWORD: "myunit3dpassword"
      SITE_TOTP_TOKEN: "mytotptoken"  # leave empty or do not set if no OTP for that account
      SITE_API_KEY: "myapikey"
      SITE_BOT_NAME: "unit3dbotusername"
    restart: unless-stopped
    logging:
      options:
        max-size: "20m"
        max-file: "5"
        compress: "true"
```
