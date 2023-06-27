# WebSocket ticker

A simple websocket server that ticks every configurable amount of time.

```
NAME:
   ws-ticker - A new cli application

USAGE:
   ws-ticker [global options] command [command options] [arguments...]

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --debug           enable debug logging (default: false) [$DEBUG]
   --route value     route to listen to (default: "/ticker") [$ROUTE]
   --interval value  interval between ticks (default: 1s) [$INTERVAL]
   --port value      port to listen on (default: 8080) [$PORT]
   --help, -h        show help
```

To test locally with `curl`

```
curl --include \
     --no-buffer \
     --header "Connection: Upgrade" \
     --header "Upgrade: websocket" \
     --header "Host: localhost:8080" \
     --header "Origin: http://localhost:8080" \
     --header "Sec-WebSocket-Key: SGVsbG8sIHdvcmxkIQ==" \
     --header "Sec-WebSocket-Version: 13" \
http://localhost:8080/
```
