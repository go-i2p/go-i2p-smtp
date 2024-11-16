package main

import (
	"fmt"

	"github.com/emersion/go-smtp"
	i2pmail "github.com/go-i2p/go-i2p-smtp/backend"
	"github.com/go-i2p/onramp"
)

func main() {
    fmt.Println("TEST1")
    onramp.InitializeOnrampLogger()
    garlic, err := onramp.NewGarlic("smtp-server", "127.0.0.1:7656", onramp.OPT_HUGE)
    if err != nil {
        panic(err)
    }
    fmt.Println("TEST2")
    server := smtp.NewServer(&i2pmail.I2PMailBackend{})
    fmt.Println("TEST3")
    listener, err := garlic.ListenTLS()
    if err != nil {
        panic(err)
    }
    fmt.Println("TEST4")
    server.Domain = listener.Addr().String()
    server.AllowInsecureAuth = true
    if err := server.Serve(listener); err != nil {
        panic(err)
    }
}