package control

type closeWriter interface {
	CloseWrite() error
}

func (c *bufferedConn) CloseWrite() error {
	if closer, ok := c.Conn.(closeWriter); ok {
		return closer.CloseWrite()
	}
	return nil
}

func (c *contextConn) CloseWrite() error {
	if closer, ok := c.Conn.(closeWriter); ok {
		return closer.CloseWrite()
	}
	return nil
}
