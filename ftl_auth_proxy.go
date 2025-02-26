package mysqlauthproxy

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
)

type Proxy struct {
	host        string
	port        int
	cfgFn       func(context.Context) (*Config, error)
	logger      Logger
	portBinding chan int
}

func NewProxy(host string, port int, cfgFn func(context.Context) (*Config, error), logger Logger, portBinding chan int) *Proxy {
	if logger == nil {
		logger = defaultLogger
	}
	return &Proxy{
		host:        host,
		port:        port,
		cfgFn:       cfgFn,
		logger:      logger,
		portBinding: portBinding,
	}
}

// ListenAndServe listens on the TCP network address for incoming connections
// It will not return until the context is done.
func (p *Proxy) ListenAndServe(ctx context.Context) error {
	socket, err := net.Listen("tcp", fmt.Sprintf("%s:%d", p.host, p.port))
	if err != nil {
		return err
	}
	cfg, err := p.cfgFn(ctx)
	if err != nil {
		return err
	}
	if p.portBinding != nil {
		p.portBinding <- socket.Addr().(*net.TCPAddr).Port
	}
	for {
		con, err := socket.Accept()
		if err != nil {
			p.logger.Print("failed to accept connection", err)
			continue
		}
		go p.handleConnection(ctx, con, cfg)

		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
}

func (p *Proxy) handleConnection(ctx context.Context, con net.Conn, cfg *Config) {
	defer con.Close()
	backend, err := connectToBackend(ctx, cfg)
	if err != nil {
		p.logger.Print("failed to connect to backend", err.Error())
		return
	}
	defer backend.Close()
	err = writeServerHandshakePacket(con, backend)
	if err != nil {
		p.logger.Print("failed to write server handshake packet", err.Error())
		return
	}
	// Now we need to read a client handshake packet
	// We re-use the mysqlConn struct to parse the packet
	mc := &mysqlConn{
		maxAllowedPacket: maxPacketSize,
		maxWriteSize:     maxPacketSize - 1,
		closech:          make(chan struct{}),
		cfg:              cfg,
		buf:              newBuffer(con),
		netConn:          con,
		rawConn:          con,
		sequence:         1,
	}
	mc.parseTime = mc.cfg.ParseTime
	_, err = mc.readPacket()
	if err != nil {
		p.logger.Print("failed to read server handshake packet", err.Error())
		return
	}
	// Now we are authenticated, send an OK packet
	err = mc.writePacket([]byte{0, 0, 0, 0, 0, 0, 0})
	if err != nil {
		p.logger.Print(err.Error())
		return
	}
	wg := sync.WaitGroup{}
	wg.Add(2)
	// All good, lets start proxying bytes
	go func() {
		defer wg.Done()
		_, err := io.Copy(backend.netConn, con)
		if err != nil {
			p.logger.Print("failed to copy from client to backend: %s", err.Error())
		}
	}()
	go func() {
		defer wg.Done()
		_, err = io.Copy(con, backend.netConn)
		if err != nil {
			p.logger.Print("failed to copy from backend to the client %s", err.Error())
		}
	}()
	// We only return after both copies are done, to allow for half closed connections
	// I am not sure if they are possible in the mysql protocol, but better safe than sorry
	wg.Wait()
}

// Open new Connection.
// See https://github.com/go-sql-driver/mysql#dsn-data-source-name for how
// the DSN string is formatted
func connectToBackend(ctx context.Context, cfg *Config) (*mysqlConn, error) {

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

func writeServerHandshakePacket(writer io.Writer, backendConnection *mysqlConn) error {
	backendConnection.flags |= clientSecureConn
	backendConnection.flags |= clientPluginAuth
	backendConnection.flags &= ^clientSSL
	var toWrite []byte
	toWrite = append(toWrite, append([]byte("8.1.0"), 0)...)                                            // Null terminated server version string, hard coded for now, this is very crap
	toWrite = append(toWrite, 1, 0, 0, 0)                                                               // Connection id Int<4>
	toWrite = append(toWrite, 1, 2, 3, 4, 5, 6, 7, 8)                                                   // String[8]	auth-plugin-data-part-1	first 8 bytes of the plugin provided data (scramble)
	toWrite = append(toWrite, 0)                                                                        // Filler
	toWrite = binary.LittleEndian.AppendUint16(toWrite, uint16(backendConnection.flags&0xFFFF))         // Capability flags (lower 2 bytes)
	toWrite = append(toWrite, 0)                                                                        // Character set
	toWrite = append(toWrite, 0, 0)                                                                     // Status flags
	toWrite = binary.LittleEndian.AppendUint16(toWrite, uint16(((backendConnection.flags)>>16)&0xFFFF)) // Capability flags (upper 2 bytes)
	toWrite = append(toWrite, 20)                                                                       // Auth plugin data length
	toWrite = append(toWrite, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)                                             // reserved
	toWrite = append(toWrite, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 0)                         // 13 bytes of auth data? Last byte is the null terminator
	toWrite = append(toWrite, []byte("caching_sha2_password")...)                                       // auth method, any password is accepted
	toWrite = append([]byte{byte(len(toWrite))}, toWrite...)                                            // Length of the packet
	pktlen := len(toWrite)
	sizeData := []byte{byte(pktlen), byte(pktlen >> 8), byte(pktlen >> 16), 0}
	toWrite = append(sizeData, toWrite...)
	_, err := writer.Write(toWrite)
	return err
}
