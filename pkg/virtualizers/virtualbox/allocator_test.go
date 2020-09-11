package virtualbox

import (
	"reflect"
	"testing"

	"code.vorteil.io/vorteil/tools/cli/pkg/compiler"
	"github.com/vorteil/vorteil/pkg/vcfg"
	"github.com/vorteil/vorteil/pkg/virtualizers"
)

var c = &Config{
	Headless:      true,
	NetworkType:   "nat",
	NetworkDevice: "",
}

var cFail = &Config{
	Headless:      true,
	NetworkType:   "hostonly",
	NetworkDevice: "",
}

var config Config

func TestRegister(t *testing.T) {
	virtualizers.Register(VirtualizerID, Allocator)
	alloc := virtualizers.RegisteredVirtualizers()
	if alloc[VirtualizerID] == nil {
		t.Errorf("registering virtualizer failed, as map lookup returned nil")
	}
}
func TestMarshalAndUnmarshal(t *testing.T) {
	data := c.Marshal()
	err := config.Unmarshal(data)
	if err != nil {
		t.Errorf("unmarshal failed, received error \"%v\"", err)
	}
	if !config.Headless {
		t.Errorf("marshal on umarshal failed, expected %v but got %v", true, c.Headless)
	}
	if config.NetworkType != "nat" {
		t.Errorf("marshal on umarshal failed, expected %v but got %v", "nat", c.NetworkType)
	}
}

func TestValidateArgs(t *testing.T) {
	data := c.Marshal()
	err := Allocator.ValidateArgs(data)
	if err != nil {
		t.Errorf("validating args failed, unable to validate config struct got err: %v", err)
	}

	data = cFail.Marshal()
	err = Allocator.ValidateArgs(data)
	if err == nil {
		t.Errorf("validating args failed, expected to error out but didn't")
	}
}

func TestAlloc(t *testing.T) {
	virt := Allocator.Alloc()
	if virt == nil {
		t.Errorf("attempting to alloc virtualizer ended up getting nil object")
	}
}

func TestDiskAlignment(t *testing.T) {
	size := 2 * size.MiB
	align := Allocator.DiskAlignment()

	if align != size {
		t.Errorf("disk alignment does not match expected %v but got %v", size, align)
	}
}

func TestDiskFormat(t *testing.T) {
	format := Allocator.DiskFormat()
	exactFormat := compiler.ImageFormatVMDK
	if format != exactFormat {
		t.Errorf("disk format does not match %v got %v instead", exactFormat, format)
	}
}

func TestIsAvailable(t *testing.T) {
	available := Allocator.IsAvailable()

	tt := reflect.TypeOf(available)
	if tt != reflect.TypeOf(true) {
		t.Errorf("Is available didn't return a 'bool' but returned '%s'", tt)
	}
}
