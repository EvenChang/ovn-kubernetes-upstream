/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ip

import (
	"errors"
	"fmt"
	"math/big"
	"net"

	utilnet "k8s.io/utils/net"

	allocator "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/allocator/bitmap"
)

// Interface manages the allocation of IP addresses out of a range. Interface
// should be threadsafe.
type Interface interface {
	Allocate(net.IP) error
	AllocateNext() (net.IP, error)
	Release(net.IP)
	ForEach(func(net.IP))
	CIDR() net.IPNet
	Has(ip net.IP) bool
	Reserved(ip net.IP) bool
}

var (
	ErrFull      = errors.New("subnet address pool exhausted")
	ErrAllocated = errors.New("provided IP is already allocated")
)

// IsErrAllocated returns true if err is of type ErrAllocated
func IsErrAllocated(err error) bool {
	return errors.Is(err, ErrAllocated)
}

type ErrNotInRange struct {
	ValidRange string
}

func (e *ErrNotInRange) Error() string {
	return fmt.Sprintf("provided IP is not in the valid range. The range of valid IPs is %s", e.ValidRange)
}

// Range is a contiguous block of IPs that can be allocated atomically.
//
// The internal structure of the range is:
//
//	For CIDR 10.0.0.0/24
//	254 addresses usable out of 256 total (minus base and broadcast IPs)
//	  The number of usable addresses is r.max
//
//	CIDR base IP          CIDR broadcast IP
//	10.0.0.0                     10.0.0.255
//	|                                     |
//	0 1 2 3 4 5 ...         ... 253 254 255
//	  |                              |
//	r.base                     r.base + r.max
//	  |                              |
//	offset #0 of r.allocated   last offset of r.allocated
type Range struct {
	net *net.IPNet
	// base is a cached version of the start IP in the CIDR range as a *big.Int
	base *big.Int
	// max is the maximum size of the usable addresses in the range
	max int

	alloc allocator.Interface
}

// NewAllocatorCIDRRange creates a Range over a net.IPNet, calling allocatorFactory to construct the backing store.
func NewAllocatorCIDRRange(cidr *net.IPNet, allocatorFactory allocator.AllocatorFactory) (*Range, error) {
	max := utilnet.RangeSize(cidr)
	base := utilnet.BigForIP(cidr.IP)
	rangeSpec := cidr.String()

	if utilnet.IsIPv6CIDR(cidr) {
		// Limit the max size, since the allocator keeps a bitmap of that size.
		if max > 65536 {
			max = 65536
		}
	} else {
		// Don't use the IPv4 network's broadcast address.
		max--
	}

	// Don't use the network's ".0" address.
	base.Add(base, big.NewInt(1))
	max--

	r := Range{
		net:  cidr,
		base: base,
		max:  maximum(0, int(max)),
	}
	var err error
	r.alloc, err = allocatorFactory(r.max, rangeSpec)
	return &r, err
}

// NewCIDRRange is a helper that wraps NewAllocatorCIDRRange, for creating a range backed by an in-memory store.
func NewCIDRRange(cidr *net.IPNet) (*Range, error) {
	return NewAllocatorCIDRRange(cidr, func(max int, rangeSpec string) (allocator.Interface, error) {
		return allocator.NewAllocationMap(max, rangeSpec), nil
	})
}

func maximum(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Free returns the count of IP addresses left in the range.
func (r *Range) Free() int {
	return r.alloc.Free()
}

// Used returns the count of IP addresses used in the range.
func (r *Range) Used() int {
	return r.max - r.alloc.Free()
}

// CIDR returns the CIDR covered by the range.
func (r *Range) CIDR() net.IPNet {
	return *r.net
}

// Allocate attempts to reserve the provided IP. ErrNotInRange or
// ErrAllocated will be returned if the IP is not valid for this range
// or has already been reserved.  ErrFull will be returned if there
// are no addresses left.
func (r *Range) Allocate(ip net.IP) error {
	ok, offset := r.contains(ip)
	if !ok {
		return &ErrNotInRange{r.net.String()}
	}

	allocated, err := r.alloc.Allocate(offset)
	if err != nil {
		return err
	}
	if !allocated {
		return ErrAllocated
	}
	return nil
}

// AllocateNext reserves one of the IPs from the pool. ErrFull may
// be returned if there are no addresses left.
func (r *Range) AllocateNext() (net.IP, error) {
	offset, ok, err := r.alloc.AllocateNext()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrFull
	}
	return utilnet.AddIPOffset(r.base, offset), nil
}

// Release releases the IP back to the pool. Releasing an
// unallocated IP or an IP out of the range is a no-op and
// returns no error.
func (r *Range) Release(ip net.IP) {
	ok, offset := r.contains(ip)
	if !ok {
		return
	}

	r.alloc.Release(offset)
}

// ForEach calls the provided function for each allocated IP.
func (r *Range) ForEach(fn func(net.IP)) {
	r.alloc.ForEach(func(offset int) {
		ip, _ := utilnet.GetIndexedIP(r.net, offset+1) // +1 because Range doesn't store IP 0
		fn(ip)
	})
}

// Has returns true if the provided IP is already allocated and a call
// to Allocate(ip) would fail with ErrAllocated.
func (r *Range) Has(ip net.IP) bool {
	ok, offset := r.contains(ip)
	if !ok {
		return false
	}

	return r.alloc.Has(offset)
}

// Reserved returns true if the provided IP can't be allocated. This is *only*
// true for the network and broadcast addresses.
func (r *Range) Reserved(ip net.IP) bool {
	if !r.net.Contains(ip) {
		return false
	}

	offset := calculateIPOffset(r.base, ip)
	return offset == -1 || offset == r.max
}

// contains returns true and the offset if the ip is in the range, and false
// and nil otherwise. The first and last addresses of the CIDR are omitted.
func (r *Range) contains(ip net.IP) (bool, int) {
	if !r.net.Contains(ip) {
		return false, 0
	}

	offset := calculateIPOffset(r.base, ip)
	if offset < 0 || offset >= r.max {
		return false, 0
	}
	return true, offset
}

// calculateIPOffset calculates the integer offset of ip from base such that
// base + offset = ip. It requires ip >= base.
func calculateIPOffset(base *big.Int, ip net.IP) int {
	return int(big.NewInt(0).Sub(utilnet.BigForIP(ip), base).Int64())
}
