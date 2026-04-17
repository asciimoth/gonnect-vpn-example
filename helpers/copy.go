package helpers

import (
	"fmt"
	"sync"

	"github.com/asciimoth/gonnect-vpn-example/logger"
	"github.com/asciimoth/gonnect/helpers"
	"github.com/asciimoth/gonnect/tun"
)

func CopyWithLog(
	a, b tun.Tun,
	offset int,
	log logger.Logger,
) error {
	an, err := a.Name()
	if err != nil {
		return err
	}
	bn, err := b.Name()
	if err != nil {
		return err
	}

	errCh := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Go(func() {
		defer a.Close() // nolint
		errCh <- copyOneWay(
			a, b, offset, log,
			fmt.Sprintf("%s --IP-> %s", an, bn),
		)
	})
	wg.Go(func() {
		defer b.Close() // nolint
		errCh <- copyOneWay(
			b, a, offset, log,
			fmt.Sprintf("%s <-IP-- %s", an, bn),
		)
	})
	wg.Wait()
	close(errCh)
	_ = a.Close()
	_ = b.Close()
	for err := range errCh {
		if helpers.ClosedNetworkErrToNil(err) != nil {
			return err
		}
	}
	return nil
}

func copyOneWay(
	src, dst tun.Tun,
	offset int,
	log logger.Logger,
	msg string,
) error {
	batchSize := src.BatchSize()
	if dstBatch := dst.BatchSize(); dstBatch < batchSize {
		batchSize = dstBatch
	}
	if batchSize <= 0 {
		batchSize = 1
	}

	mtu, err := src.MTU()
	if err != nil {
		mtu = 1500
	}
	if dstMTU, err := dst.MTU(); err == nil && dstMTU > mtu {
		mtu = dstMTU
	}

	// Allocate buffers with room for the offset
	bufs := make([][]byte, batchSize)
	sizes := make([]int, batchSize)
	for i := range bufs {
		bufs[i] = make([]byte, mtu+offset)
	}

	dataBufs := make([][]byte, batchSize)
	writeBufs := make([][]byte, batchSize)
	for i := range dataBufs {
		dataBufs[i] = make([]byte, mtu+offset)
	}

	for {
		n, err := src.Read(bufs, sizes, offset)
		if err != nil {
			return err
		}
		if n == 0 {
			continue
		}

		for i := range n {
			// Copy from read buffer (at offset) to write buffer (at offset)
			copy(
				dataBufs[i][offset:offset+sizes[i]],
				bufs[i][offset:offset+sizes[i]],
			)
			// Slice to include the offset region so dst.Write can access data at offset
			writeBufs[i] = dataBufs[i][:offset+sizes[i]]
		}

		log.Print(msg)
		for written := 0; written < n; {
			// Pass the full slice (including offset region) to dst.Write
			wn, err := dst.Write(writeBufs[written:n], offset)
			if err != nil {
				return err
			}
			written += wn
		}
	}
}
