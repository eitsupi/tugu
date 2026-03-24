package main

import (
	"context"
	"io"
	"log"
	"net"
	"sync"
)

// bridge accepts connections from listener and dials the connect-side for each,
// then copies data bidirectionally until either side closes.
func bridge(ctx context.Context, listener net.Listener, dial func(ctx context.Context) (net.Conn, error), verbose bool) error {
	var wg sync.WaitGroup
	defer wg.Wait()

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	for {
		src, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil // shutting down
			}
			return err
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			handleConn(ctx, src, dial, verbose)
		}()
	}
}

func handleConn(ctx context.Context, src net.Conn, dial func(ctx context.Context) (net.Conn, error), verbose bool) {
	defer src.Close()

	dst, err := dial(ctx)
	if err != nil {
		log.Printf("dial failed: %v", err)
		return
	}
	defer dst.Close()

	if verbose {
		log.Printf("connection established: %s <-> %s", src.RemoteAddr(), dst.RemoteAddr())
	}

	done := make(chan struct{})

	go func() {
		io.Copy(dst, src)
		dst.Close()
		close(done)
	}()

	io.Copy(src, dst)
	src.Close()
	<-done

	if verbose {
		log.Printf("connection closed: %s", src.RemoteAddr())
	}
}
