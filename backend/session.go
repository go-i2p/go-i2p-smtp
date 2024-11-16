package i2pmail

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	smtp "github.com/emersion/go-smtp"
	"github.com/go-i2p/i2pkeys"
	"github.com/go-i2p/sam3"
	"golang.org/x/crypto/argon2"
)

type UserData struct {
	Username    string
	PassHash    []byte
	Salt        []byte
	LastAccess  time.Time
	Destination string
}

type I2PMailSession struct {
	from        string
	to          []string
	data        []byte
	session     *sam3.StreamSession
	baseDataDir string
	userData    *UserData
	mu          sync.RWMutex
}

func (s *I2PMailSession) getUserPath(username string) string {
	return filepath.Join(s.baseDataDir, "users", username+".json")
}

func (s *I2PMailSession) getMailboxPath(username string) string {
	return filepath.Join(s.baseDataDir, "mailboxes", username)
}

func (s *I2PMailSession) hashPassword(password string, salt []byte) ([]byte, []byte, error) {
	if salt == nil {
		salt = make([]byte, 16)
		if _, err := rand.Read(salt); err != nil {
			return nil, nil, err
		}
	}

	// Argon2id for secure password hashing
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return hash, salt, nil
}

func (s *I2PMailSession) loadUser(username string) (*UserData, error) {
	path := s.getUserPath(username)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var user UserData
	if err := json.NewDecoder(f).Decode(&user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *I2PMailSession) saveUser(user *UserData) error {
	if err := os.MkdirAll(filepath.Dir(s.getUserPath(user.Username)), 0700); err != nil {
		return err
	}

	f, err := os.OpenFile(s.getUserPath(user.Username), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(user)
}

func (s *I2PMailSession) AuthPlain(username, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, err := s.loadUser(username)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// New user registration
	if os.IsNotExist(err) {
		hash, salt, err := s.hashPassword(password, nil)
		if err != nil {
			return err
		}

		dest, err := i2pkeys.NewDestination()
		if err != nil {
			return err
		}

		user = &UserData{
			Username:    username,
			PassHash:    hash,
			Salt:        salt,
			LastAccess:  time.Now(),
			Destination: dest.String(),
		}

		if err := os.MkdirAll(s.getMailboxPath(username), 0700); err != nil {
			return err
		}

		if err := s.saveUser(user); err != nil {
			return err
		}

		s.userData = user
		return nil
	}

	// Existing user authentication
	hash, _, err := s.hashPassword(password, user.Salt)
	if err != nil {
		return err
	}

	if !compareHashes(hash, user.PassHash) {
		return errors.New("invalid credentials")
	}

	user.LastAccess = time.Now()
	if err := s.saveUser(user); err != nil {
		return err
	}

	s.userData = user
	return nil
}

func (s *I2PMailSession) storeMail(from string, to string, data []byte) error {
	mailboxPath := s.getMailboxPath(to)
	if err := os.MkdirAll(mailboxPath, 0700); err != nil {
		return err
	}

	mailID := base64.RawURLEncoding.EncodeToString(generateRandomBytes(16))
	mailPath := filepath.Join(mailboxPath, mailID)

	mailData := struct {
		From    string
		To      string
		Date    time.Time
		Content []byte
	}{
		From:    from,
		To:      to,
		Date:    time.Now(),
		Content: data,
	}

	f, err := os.OpenFile(mailPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(mailData)
}

func (s *I2PMailSession) deliverMail(to string) error {
	conn, err := s.session.Dial("tcp", to)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := s.storeMail(s.from, to, s.data); err != nil {
		return err
	}

	_, err = conn.Write(s.data)
	return err
}

func compareHashes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}

	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}
	return result == 0
}

func generateRandomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

// Data implements smtp.Session.
func (s *I2PMailSession) Data(r io.Reader) error {
    if s.userData == nil {
        return errors.New("authentication required")
    }
    if len(s.to) == 0 {
        return errors.New("no recipients specified")
    }

    // Read email data with a reasonable size limit
    data, err := io.ReadAll(io.LimitReader(r, 25*1024*1024)) // 25MB limit
    if err != nil {
        return err
    }
    s.data = data

    // Deliver to all recipients
    var lastErr error
    for _, recipient := range s.to {
        if err := s.deliverMail(recipient); err != nil {
            lastErr = err
        }
    }

    return lastErr
}

// Mail implements smtp.Session.
func (s *I2PMailSession) Mail(from string, opts *smtp.MailOptions) error {
    if s.userData == nil {
        return errors.New("authentication required")
    }

    // Validate sender is authenticated user
    if from != s.userData.Username+"@"+s.userData.Destination {
        return errors.New("sender not authorized")
    }

    s.from = from
    s.to = nil // Reset recipients
    s.data = nil // Reset email data
    return nil
}

// Rcpt implements smtp.Session.
func (s *I2PMailSession) Rcpt(to string, opts *smtp.RcptOptions) error {
    if s.userData == nil {
        return errors.New("authentication required")
    }
    if s.from == "" {
        return errors.New("sender not specified")
    }

    // Validate recipient address format
    if !strings.Contains(to, "@") {
        return errors.New("invalid recipient address")
    }

    // Extract destination from email address
    parts := strings.Split(to, "@")
    if len(parts) != 2 {
        return errors.New("invalid recipient address format")
    }

    // Validate destination is a valid I2P address
    dest := parts[1]
    if !strings.HasSuffix(dest, ".i2p") && !strings.HasSuffix(dest, ".b32.i2p") {
        return errors.New("recipient must be an I2P destination")
    }

    s.to = append(s.to, to)
    return nil
}

// Reset implements smtp.Session.
func (s *I2PMailSession) Reset() {
    s.mu.Lock()
    defer s.mu.Unlock()

    s.from = ""
    s.to = nil
    s.data = nil
    // Note: We don't reset authentication state
}

// Logout implements smtp.Session.
func (s *I2PMailSession) Logout() error {
    s.mu.Lock()
    defer s.mu.Unlock()

    s.from = ""
    s.to = nil
    s.data = nil
    s.userData = nil
    return nil
}