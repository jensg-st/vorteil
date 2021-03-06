// +build linux darwin

package vmware

import (
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strings"
	"time"
)

// initLogs sets the logger up and attaches to the socket
func (v *Virtualizer) initLogs() error {
	v.log("info", "Initializing Logger")

	count := 0
	for {
		if count == 15 {
			v.log("error", "Error socket not created")
			return fmt.Errorf("unable to listen on unix socket within timeframe.")
		}
		conn, err := net.Dial("unix", filepath.ToSlash(filepath.Join(v.folder, "socket")))
		if err != nil {
			if !strings.Contains(err.Error(), "no such file or directory") {
				v.log("error", "Error dialling socket: %v", err)
				return err
			}
		} else {
			v.sock = conn
			go io.Copy(v.serialLogger, conn)

			break
		}
		count++
		time.Sleep(time.Second * 1)
	}

	return nil
}
