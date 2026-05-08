package session

import (
	"context"
	"net"
	"strings"

	utls "github.com/refraction-networking/utls"
)

type UTLSDialer struct {
	Dialer  *net.Dialer
	HelloID utls.ClientHelloID
}

func NewUTLSDialer() UTLSDialer {
	return UTLSDialer{Dialer: &net.Dialer{}, HelloID: utls.HelloChrome_131}
}

func (d UTLSDialer) DialTLSContext(ctx context.Context, network, address string) (net.Conn, error) {
	dialer := d.Dialer
	if dialer == nil {
		dialer = &net.Dialer{}
	}
	raw, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	conn := utls.UClient(raw, &utls.Config{ServerName: strings.Trim(host, "[]")}, d.HelloID)
	if err := conn.HandshakeContext(ctx); err != nil {
		_ = raw.Close()
		return nil, err
	}
	return conn, nil
}
