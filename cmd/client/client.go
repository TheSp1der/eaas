package main

import (
	"fmt"
	"sync"
	"time"
	"unsafe"

	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"log/slog"

	"github.com/thesp1der/eaas/internal/common"
	"github.com/thesp1der/httpclient"
	"golang.org/x/sys/unix"
)

const (
	twoMBinBits int = 1024 * 1024 * 2 * 8
)

type entropy struct {
	log            *slog.Logger
	minimumEntropy int
	shutdown       <-chan bool
	source         string
	timeout        time.Duration
	wg             *sync.WaitGroup
}

func (e *entropy) client() {
	var (
		t *time.Timer = time.NewTimer(e.timeout)
	)

	defer t.Stop()

	for {
		select {
		case <-t.C:
			if err := e.addToPool(); err != nil {
				e.log.Error("entropy client exited with error", "error", err)
			}

			t.Reset(e.timeout)
		case <-e.shutdown:
			e.log.Info("entropy client stopped")
			e.wg.Done()
			return
		default:
			time.Sleep(time.Microsecond * 500)
		}
	}
}

func (e *entropy) addToPool() error {
	for maxLoops := 10; maxLoops > 0; maxLoops-- {
		// max requestable entropy
		var requestedEntropy int = 0

		// get current entropy
		currentEntropy, err := getCurrentEntropy()
		if err != nil {
			return err
		}

		switch {
		case currentEntropy < e.minimumEntropy && e.minimumEntropy-currentEntropy < twoMBinBits:
			requestedEntropy = e.minimumEntropy - currentEntropy
		case currentEntropy < e.minimumEntropy && e.minimumEntropy-currentEntropy > twoMBinBits:
			requestedEntropy = twoMBinBits
		}

		switch {
		case requestedEntropy > 0:
			e.log.Debug("need to fill entropy", "amount", requestedEntropy)
			// request entropy from source (coverts bits to bytes in request)
			data, err := getExternalEntropy(fmt.Sprintf("%s?bytes=%d", e.source, requestedEntropy/8))
			if err != nil {
				return err
			}

			// push entropy to kernel
			if err := fillEntropy(requestedEntropy, data); err != nil {
				return err
			}
		default:
			return nil
		}
	}
	return fmt.Errorf("max attempts to fill entropy reached")
}

// Retrieves system entropy from kernel in bits.
func getCurrentEntropy() (int, error) {
	var (
		fd  int
		err error
	)

	if fd, err = unix.Open("/dev/random", unix.O_RDONLY, 0); err != nil {
		return 0, err
	}
	defer unix.Close(fd)

	var (
		ent int
	)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(0x80045200), uintptr(unsafe.Pointer(&ent)))
	if errno != 0 {
		return 0, err
	}

	return ent, nil
}

func getExternalEntropy(url string) ([]byte, error) {
	c := httpclient.DefaultClient()
	c.SetHeader("Accept", "application/json")
	rawData, err := c.Get(url)
	if err != nil {
		return []byte{}, err
	}

	var j common.DataStruct
	if err := json.Unmarshal(rawData, &j); err != nil {
		return []byte{}, err
	}

	output, err := base64.StdEncoding.DecodeString(j.Data)
	if err != nil {
		return []byte{}, err
	}

	return output, nil
}

func fillEntropy(count int, data []byte) error {
	var (
		fd  int
		err error
		hbo = binary.LittleEndian
	)

	type pInfo struct {
		count int
		size  int
		buf   uint32
	}

	blen := len(data)
	// we need to pad to 4-byte chunks since this is a uint32 array
	if blen%4 != 0 {
		for i := 0; i < 4-(blen%4); i++ {
			data = append(data, 0x00)
		}
	}
	blen = len(data)

	// pack byte slice
	const structSize = int(unsafe.Sizeof(pInfo{}))

	rpi := make([]byte, structSize+blen-1)

	hbo.PutUint32(rpi[0:], uint32(count))
	hbo.PutUint32(rpi[4:], uint32(blen))
	copy(rpi[8:], data)

	if fd, err = unix.Open("/dev/random", unix.O_RDWR, 0); err != nil {
		return err
	}
	defer unix.Close(fd)

	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(0x40085203), uintptr(unsafe.Pointer(&rpi[0])))
	if errno != 0 {
		return err
	}

	return nil
}
