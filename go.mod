module github.com/go-i2p/go-i2p-smtp

go 1.23.1

require (
	github.com/emersion/go-smtp v0.21.3
	github.com/go-i2p/i2pkeys v0.33.10-0.20241113193422-e10de5e60708
	github.com/go-i2p/onramp v0.33.9
	github.com/go-i2p/sam3 v0.33.9
)

require (
	github.com/cretz/bine v0.2.0 // indirect
	github.com/emersion/go-sasl v0.0.0-20241020182733-b788ff22d5a6 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	golang.org/x/crypto v0.11.0 // indirect
	golang.org/x/net v0.12.0 // indirect
	golang.org/x/sys v0.27.0 // indirect
)

replace github.com/go-i2p/onramp => ../onramp
