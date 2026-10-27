module zakki

go 1.27.2

tool github.com/egoist/mygo/cmd/mygo

require (
	github.com/egoist/mygo v0.5.1
	zakki/core v0.0.0-00010101000000-000000000000
)

require (
	github.com/ebitengine/purego v0.11.1 // indirect
	github.com/go-text/typesetting v0.3.5 // indirect
	golang.org/x/image v0.46.0 // indirect
	golang.org/x/text v0.43.0 // indirect
)

replace zakki/core => ./core
