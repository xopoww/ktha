package container

import (
	"net"
)

type wrappedConn struct {
	net.Conn
	onClose func()
}

func (w wrappedConn) Close() error {
	defer w.onClose()
	return w.Conn.Close()
}

func wrapConn(c net.Conn, onClose func()) net.Conn {
	return wrappedConn{
		Conn:    c,
		onClose: onClose,
	}
}
