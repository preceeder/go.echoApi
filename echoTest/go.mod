module base-utils/echoTest

go 1.24.2

require (
	github.com/labstack/echo/v4 v4.13.3
	github.com/preceeder/base v1.0.1
)

require (
	github.com/coder/websocket v1.8.14
	github.com/preceeder/echoApi v1.0.5
	github.com/preceeder/logs v1.0.5
)

replace github.com/preceeder/echoApi v1.0.5 => ../

require (
	github.com/labstack/gommon v0.4.2 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/net v0.33.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
)
