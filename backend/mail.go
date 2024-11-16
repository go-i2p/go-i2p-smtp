package i2pmail

import (
	"os"
	"sync"

	smtp "github.com/emersion/go-smtp"
	"github.com/go-i2p/i2pkeys"
	"github.com/go-i2p/sam3"
)

type I2PMailBackend struct {
	// Backend SAM management
	samAddr string
	sam     *sam3.SAM

	// Backend file-storage management, backend can store data in the base, I2PMailSession will store data in a sub-directory
	baseDataDir string

	// I2PMailSession variables, pass these to the I2PMailSession when constructing them
	keys    *i2pkeys.I2PKeys
	session *sam3.StreamSession
	mu      sync.RWMutex
}

func NewI2PMailBackend(samAddr, dataDir string) (*I2PMailBackend, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}

	return &I2PMailBackend{
		samAddr:     samAddr,
		baseDataDir: dataDir,
	}, nil
}

func (b *I2PMailBackend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &I2PMailSession{
		session: b.getOrCreateSession(c.Hostname()),
	}, nil
}

func (b *I2PMailBackend) getOrCreateSession(id string) *sam3.StreamSession {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.session != nil {
		return b.session
	}
	keys, err := i2pkeys.NewDestination()
	if err != nil {
		return nil
	}
	b.keys = keys
	b.sam, err = sam3.NewSAM(b.samAddr)
	if err != nil {
		return nil
	}
	b.session, err = b.sam.NewStreamSession(id, *b.keys, sam3.Options_Default)
	if err != nil {
		return nil
	}
	return b.session
}
