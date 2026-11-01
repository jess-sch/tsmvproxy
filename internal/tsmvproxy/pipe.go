package tsmvproxy

import (
	"errors"
	"io"
	"net"

	"golang.org/x/sync/errgroup"
)

type closeWriter interface {
	// as implemented by e.g. net.TCPConn
	CloseWrite() error
}

func uniPipe(dst, src net.Conn) error {
	_, err := io.Copy(dst, src)

	if err != nil {
		// Real failure: tear everything down so the peer goroutine unblocks.
		src.Close()
		dst.Close()
		if errors.Is(err, net.ErrClosed) {
			err = nil // caused by the other direction shutting down
		}
		return err
	}

	// Clean EOF: propagate it as a half-close if possible.
	if cw, ok := dst.(closeWriter); ok {
		_ = cw.CloseWrite()
	} else {
		dst.Close()
	}
	return nil
}

func bidiPipe(a, b net.Conn) error {
	var wg errgroup.Group
	wg.Go(func() error { return uniPipe(a, b) })
	wg.Go(func() error { return uniPipe(b, a) })
	return wg.Wait()
}
