package p2p

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/n42blockchain/N42/api/protocol/sync_pb"
	"github.com/n42blockchain/N42/conf"
	"github.com/n42blockchain/N42/internal/p2p/enr"
	"github.com/n42blockchain/N42/utils"
	"net"
	"os"
	"path"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/pkg/errors"
)

const keyPath = "network-keys"

const seqPath = "network-seq"

const dialTimeout = 1 * time.Second

// SerializeENR takes the enr record in its key-value form and serializes it.
func SerializeENR(record *enr.Record) (string, error) {
	if record == nil {
		return "", errors.New("could not serialize nil record")
	}
	buf := bytes.NewBuffer([]byte{})
	if err := record.EncodeRLP(buf); err != nil {
		return "", errors.Wrap(err, "could not encode ENR record to bytes")
	}
	enrString := base64.RawURLEncoding.EncodeToString(buf.Bytes())
	return enrString, nil
}

func getSeqNumber(cfg *conf.P2PConfig) (*sync_pb.Ping, error) {
	defaultSeqPath := path.Join(cfg.DataDir, seqPath)

	_, err := os.Stat(defaultSeqPath)
	defaultKeysExist := !os.IsNotExist(err)
	if err != nil && defaultKeysExist {
		log.Error("Error reading network seqNumber from file", "err", err)
		return nil, err
	}

	if defaultKeysExist {
		src, err := os.ReadFile(defaultSeqPath) // #nosec G304
		if err != nil {
			log.Error("Error reading network seqNumber from file", "err", err)
			return nil, err
		}
		//dst := make([]byte, hex.DecodedLen(len(src)))
		//_, err = hex.Decode(dst, src)
		//if err != nil {
		//	return nil, errors.Wrap(err, "failed to decode hex string")
		//}

		seqNumber := binary.LittleEndian.Uint64(src)
		if seqNumber > 0 {
			log.Info("Load seq number from file", "seqNumber", seqNumber)
			return &sync_pb.Ping{SeqNumber: seqNumber}, nil
		}

	}
	return &sync_pb.Ping{SeqNumber: 0}, nil
}

func saveSeqNumber(cfg *conf.P2PConfig, seqNumber *sync_pb.Ping) error {

	defaultSeqPath := path.Join(cfg.DataDir, seqPath)

	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, seqNumber.SeqNumber)

	if err := os.WriteFile(defaultSeqPath, b, 0600); err != nil {
		log.Error("Failed to save p2p seq number", "err", err)
		return err
	}
	log.Info("Wrote seq number to file", "seqNumber", seqNumber.SeqNumber)
	return nil
}

// Determines a private key for p2p networking from the p2p service's
// configuration struct. If no key is found, it generates a new one.
func privKey(cfg *conf.P2PConfig) (*ecdsa.PrivateKey, error) {
	defaultKeyPath := path.Join(cfg.DataDir, keyPath)
	privateKeyPath := cfg.PrivateKey

	// PrivateKey cli flag takes highest precedence.
	if privateKeyPath != "" {
		return privKeyFromFile(cfg.PrivateKey)
	}

	_, err := os.Stat(defaultKeyPath)
	defaultKeysExist := !os.IsNotExist(err)
	if err != nil && defaultKeysExist {
		return nil, err
	}
	// Default keys have the next highest precedence, if they exist.
	if defaultKeysExist {
		return privKeyFromFile(defaultKeyPath)
	}
	// There are no keys on the filesystem, so we need to generate one.
	priv, _, err := crypto.GenerateSecp256k1Key(rand.Reader)
	if err != nil {
		return nil, err
	}
	// If the StaticPeerID flag is set, save the generated key as the default
	// key, so that it will be used by default on the next node start.
	if cfg.StaticPeerID {
		rawbytes, err := priv.Raw()
		if err != nil {
			return nil, err
		}
		dst := make([]byte, hex.EncodedLen(len(rawbytes)))
		hex.Encode(dst, rawbytes)
		if err := os.WriteFile(defaultKeyPath, dst, 0600); err != nil {
			return nil, err
		}
		log.Info("Wrote network key to file")
		// Read the key from the defaultKeyPath file just written
		// for the strongest guarantee that the next start will be the same as this one.
		return privKeyFromFile(defaultKeyPath)
	}
	return utils.ConvertFromInterfacePrivKey(priv)
}

// Retrieves a p2p networking private key from a file path.
func privKeyFromFile(path string) (*ecdsa.PrivateKey, error) {
	src, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		log.Error("Error reading private key from file", "err", err)
		return nil, err
	}
	dst := make([]byte, hex.DecodedLen(len(src)))
	_, err = hex.Decode(dst, src)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode hex string")
	}
	unmarshalledKey, err := crypto.UnmarshalSecp256k1PrivateKey(dst)
	if err != nil {
		return nil, err
	}
	return utils.ConvertFromInterfacePrivKey(unmarshalledKey)
}

// Attempt to dial an address to verify its connectivity
func verifyConnectivity(addr string, port int, protocol string) {
	if addr != "" {
		a := net.JoinHostPort(addr, fmt.Sprintf("%d", port))

		conn, err := net.DialTimeout(protocol, a, dialTimeout)
		if err != nil {
			log.Warn("IP address is not accessible", "err", err, "protocol", protocol, "address", a)
			return
		}
		if err := conn.Close(); err != nil {
			log.Debug("Could not close connection", "err", err)
		}
	}
}
