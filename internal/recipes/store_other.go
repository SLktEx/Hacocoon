//go:build !linux

package recipes

import (
	"fmt"
	"os"
)

type store struct{}

func openStore(string) (*store, error) {
	return nil, fmt.Errorf("setup recipe storage requires Linux")
}
func (*store) close()                {}
func (*store) read() ([]byte, error) { return nil, fmt.Errorf("unsupported platform") }
func (*store) save([]byte) error     { return fmt.Errorf("unsupported platform") }
func (*store) remove() error         { return fmt.Errorf("unsupported platform") }

func openInput(string) (*os.File, error) { return nil, fmt.Errorf("setup script input requires Linux") }
