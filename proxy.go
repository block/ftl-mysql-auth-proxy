package mysqlauthproxy

import (
	"context"
	"crypto/tls"
	"database/sql/driver"
	"fmt"
	"net"
	"net/url"
)
type Proxy struct {
	bind   url.URL
	dsn    string
	logger Logger
}

func NewProxy(bind url.URL, dsn string, logger Logger) *Proxy {
	if logger == nil {
		logger = defaultLogger
	}
	return &Proxy{
		bind:   bind,
		dsn:    dsn,
		logger: logger,
	}
}

// ListenAndServe listens on the TCP network address for incoming connections
// It will not return until the context is done.
func (p *Proxy) ListenAndServe(ctx context.Context) error {
	socket, err := net.Listen("tcp", fmt.Sprintf("%s:%d", p.bind.Host, p.bind.Port))
	if err != nil {

	}
	for {
		con, err := socket.Accept()
		if err != nil {
			p.logger.Print("failed to accept connection", err)
			continue
		}
		go p.handleConnection(ctx, con)

		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
	return nil
}

func (p *Proxy) handleConnection(ctx context.Context, con net.Conn) {
	defer con.Close()
	backend, err := connectToBackend(ctx, p.dsn)
	if err != nil {
		p.logger.Print( "failed to connect to backend", err.Error())
		return
	}
	// We need to send a handshake to the client
	// The client will respond with a handshake response
	// Then we need to start just forwarding packets
	buf := newBuffer(con)
	buf.takeBuffer()
   backend.


}

// Open new Connection.
// See https://github.com/go-sql-driver/mysql#dsn-data-source-name for how
// the DSN string is formatted
func connectToBackend(ctx context.Context, dsn string) (*mysqlConn, error) {
	cfg, err := ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	c := newConnector(cfg)
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	if mc, ok := conn.(*mysqlConn); ok {
		// We have an authenticated connection
		// Now we need to start proxying
		return mc, nil
	}
	_ = conn.Close()
	return nil, fmt.Errorf("failed to connect to backend")
}


func  writeServerHandshakePacket(authResp []byte, plugin string) error {
	// Adjust client flags based on server support
	clientFlags := clientProtocol41 |
		clientSecureConn |
		clientLongPassword |
		clientTransactions |
		clientLocalFiles |
		clientPluginAuth |
		clientMultiResults |
		clientConnectAttrs |
		mc.flags&clientLongFlag

	if mc.cfg.ClientFoundRows {
		clientFlags |= clientFoundRows
	}

	// To enable TLS / SSL
	if mc.cfg.TLS != nil {
		clientFlags |= clientSSL
	}

	if mc.cfg.MultiStatements {
		clientFlags |= clientMultiStatements
	}

	// encode length of the auth plugin data
	var authRespLEIBuf [9]byte
	authRespLen := len(authResp)
	authRespLEI := appendLengthEncodedInteger(authRespLEIBuf[:0], uint64(authRespLen))
	if len(authRespLEI) > 1 {
		// if the length can not be written in 1 byte, it must be written as a
		// length encoded integer
		clientFlags |= clientPluginAuthLenEncClientData
	}

	pktLen := 4 + 4 + 1 + 23 + len(mc.cfg.User) + 1 + len(authRespLEI) + len(authResp) + 21 + 1

	// To specify a db name
	if n := len(mc.cfg.DBName); n > 0 {
		clientFlags |= clientConnectWithDB
		pktLen += n + 1
	}

	// encode length of the connection attributes
	var connAttrsLEIBuf [9]byte
	connAttrsLen := len(mc.connector.encodedAttributes)
	connAttrsLEI := appendLengthEncodedInteger(connAttrsLEIBuf[:0], uint64(connAttrsLen))
	pktLen += len(connAttrsLEI) + len(mc.connector.encodedAttributes)

	// Calculate packet length and get buffer with that size
	data, err := mc.buf.takeBuffer(pktLen + 4)
	if err != nil {
		// cannot take the buffer. Something must be wrong with the connection
		mc.log(err)
		return errBadConnNoWrite
	}

	// ClientFlags [32 bit]
	data[4] = byte(clientFlags)
	data[5] = byte(clientFlags >> 8)
	data[6] = byte(clientFlags >> 16)
	data[7] = byte(clientFlags >> 24)

	// MaxPacketSize [32 bit] (none)
	data[8] = 0x00
	data[9] = 0x00
	data[10] = 0x00
	data[11] = 0x00

	// Collation ID [1 byte]
	data[12] = defaultCollationID
	if cname := mc.cfg.Collation; cname != "" {
		colID, ok := collations[cname]
		if ok {
			data[12] = colID
		} else if len(mc.cfg.charsets) > 0 {
			// When cfg.charset is set, the collation is set by `SET NAMES <charset> COLLATE <collation>`.
			return fmt.Errorf("unknown collation: %q", cname)
		}
	}

	// Filler [23 bytes] (all 0x00)
	pos := 13
	for ; pos < 13+23; pos++ {
		data[pos] = 0
	}

	// SSL Connection Request Packet
	// http://dev.mysql.com/doc/internals/en/connection-phase-packets.html#packet-Protocol::SSLRequest
	if mc.cfg.TLS != nil {
		// Send TLS / SSL request packet
		if err := mc.writePacket(data[:(4+4+1+23)+4]); err != nil {
			return err
		}

		// Switch to TLS
		tlsConn := tls.Client(mc.netConn, mc.cfg.TLS)
		if err := tlsConn.Handshake(); err != nil {
			if cerr := mc.canceled.Value(); cerr != nil {
				return cerr
			}
			return err
		}
		mc.netConn = tlsConn
		mc.buf.nc = tlsConn
	}

	// User [null terminated string]
	if len(mc.cfg.User) > 0 {
		pos += copy(data[pos:], mc.cfg.User)
	}
	data[pos] = 0x00
	pos++

	// Auth Data [length encoded integer]
	pos += copy(data[pos:], authRespLEI)
	pos += copy(data[pos:], authResp)

	// Databasename [null terminated string]
	if len(mc.cfg.DBName) > 0 {
		pos += copy(data[pos:], mc.cfg.DBName)
		data[pos] = 0x00
		pos++
	}

	pos += copy(data[pos:], plugin)
	data[pos] = 0x00
	pos++

	// Connection Attributes
	pos += copy(data[pos:], connAttrsLEI)
	pos += copy(data[pos:], []byte(mc.connector.encodedAttributes))

	// Send Auth packet
	return mc.writePacket(data[:pos])
}